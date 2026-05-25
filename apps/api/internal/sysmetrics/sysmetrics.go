// Package sysmetrics muestrea CPU/MEM/NET del host donde corre el API.
//
// Hay un sampler en background que cada N segundos toma una snapshot.
// Los consumidores piden la última snapshot con Latest() — sin bloquear.
// En Fase 5 se expone un endpoint SSE que streamea estas snapshots al panel.
//
// Cómo funciona en una línea: una goroutine corre forever, llena un ring
// buffer protegido por mutex, y los clientes leen la última posición.
package sysmetrics

import (
	"context"
	"sync"
	"time"

	"github.com/nicotion/battos/packages/core"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// Sampler corre en background y mantiene la última snapshot disponible.
//
// Uso típico (en main):
//
//	s := sysmetrics.New(1 * time.Second)
//	s.Start(ctx)              // lanza la goroutine
//	metrics := s.Latest()     // lee snapshot actual (no bloquea)
type Sampler struct {
	interval time.Duration

	mu      sync.RWMutex
	latest  core.SystemMetrics

	// Estado para calcular net delta entre samples.
	lastNetTime  time.Time
	lastNetBytes net.IOCountersStat
}

// New crea un Sampler con el intervalo dado.
// Recomendado: 1 segundo para dashboard fluido, 5 segundos para servidores headless.
func New(interval time.Duration) *Sampler {
	return &Sampler{interval: interval}
}

// Start lanza la goroutine de muestreo. Termina cuando ctx se cancela.
//
// Es seguro llamar Start UNA vez. Llamar dos veces lanza dos goroutines
// (no rompe, pero gasta recursos).
func (s *Sampler) Start(ctx context.Context) {
	go s.loop(ctx)
}

func (s *Sampler) loop(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Tomar primera muestra inmediatamente, no esperar al primer tick.
	s.sample(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sample(ctx)
		}
	}
}

func (s *Sampler) sample(ctx context.Context) {
	snap := core.SystemMetrics{}

	// CPU — el primer call necesita un "intervalo" interno para calcular %.
	// Usamos 200ms que no bloquea demasiado pero da una muestra significativa.
	if vals, err := cpu.PercentWithContext(ctx, 200*time.Millisecond, false); err == nil && len(vals) > 0 {
		snap.CPUPercent = vals[0]
	}

	// Memoria.
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		snap.MemPercent = vm.UsedPercent
		snap.MemUsedMB = vm.Used / 1024 / 1024
		snap.MemTotalMB = vm.Total / 1024 / 1024
	}

	// Net — calcular delta vs muestra anterior para obtener KB/s.
	if nets, err := net.IOCountersWithContext(ctx, false); err == nil && len(nets) > 0 {
		now := time.Now()
		current := nets[0]
		s.mu.Lock()
		if !s.lastNetTime.IsZero() {
			elapsedSec := now.Sub(s.lastNetTime).Seconds()
			if elapsedSec > 0 {
				upBytes := safeDelta(current.BytesSent, s.lastNetBytes.BytesSent)
				downBytes := safeDelta(current.BytesRecv, s.lastNetBytes.BytesRecv)
				snap.NetUploadKBps = float64(upBytes) / 1024 / elapsedSec
				snap.NetDownloadKBps = float64(downBytes) / 1024 / elapsedSec
			}
		}
		s.lastNetTime = now
		s.lastNetBytes = current
		s.mu.Unlock()
	}

	s.mu.Lock()
	s.latest = snap
	s.mu.Unlock()
}

// Latest devuelve la última snapshot capturada. No bloquea.
// Si todavía no se tomó ninguna muestra, devuelve una snapshot vacía (zero value).
func (s *Sampler) Latest() core.SystemMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latest
}

// safeDelta evita underflow si el counter se reinicia (raro pero posible).
func safeDelta(current, previous uint64) uint64 {
	if current < previous {
		return 0
	}
	return current - previous
}
