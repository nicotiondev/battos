package sysmetrics

import (
	"context"
	"testing"
	"time"
)

// TestSamplerDiskAndProcesses arranca el sampler real y verifica que la
// snapshot incluye disco y al menos un proceso. Es un smoke test contra el
// host real (no mockea gopsutil) — en cualquier máquina viva debe pasar.
func TestSamplerDiskAndProcesses(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := New(500 * time.Millisecond)
	s.Start(ctx)

	// La primera muestra tarda ~200ms (intervalo interno de cpu.Percent)
	// más la enumeración de procesos. Polleamos hasta 5s antes de rendirnos.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		m := s.Latest()
		if m.DiskTotalGB > 0 && len(m.TopProcesses) > 0 {
			if m.DiskUsedGB <= 0 || m.DiskUsedGB > m.DiskTotalGB {
				t.Fatalf("DiskUsedGB fuera de rango: used=%.1f total=%.1f", m.DiskUsedGB, m.DiskTotalGB)
			}
			if m.DiskPercent <= 0 || m.DiskPercent > 100 {
				t.Fatalf("DiskPercent fuera de rango: %.1f", m.DiskPercent)
			}
			if len(m.TopProcesses) > topProcessCount {
				t.Fatalf("TopProcesses devolvió %d procesos, máximo esperado %d", len(m.TopProcesses), topProcessCount)
			}
			top := m.TopProcesses[0]
			if top.PID <= 0 || top.Name == "" {
				t.Fatalf("proceso top inválido: %+v", top)
			}
			// El top por memoria de cualquier host vivo usa más de 0 MB.
			if top.MemMB == 0 {
				t.Fatalf("proceso top con 0 MB de RSS: %+v", top)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("el sampler no produjo disco+procesos en 5s: %+v", s.Latest())
}

// TestTopProcessesUsesCache verifica que dos llamadas dentro de la ventana de
// procRefreshInterval no re-enumeran procesos (la segunda devuelve la cache).
func TestTopProcessesUsesCache(t *testing.T) {
	ctx := context.Background()
	s := New(time.Second)

	first := s.topProcesses(ctx)
	if len(first) == 0 {
		t.Fatal("la primera enumeración no devolvió procesos")
	}
	stamp := s.lastProcSample

	second := s.topProcesses(ctx)
	if !s.lastProcSample.Equal(stamp) {
		t.Fatal("la segunda llamada re-enumeró procesos en vez de usar la cache")
	}
	if len(second) != len(first) {
		t.Fatalf("la cache devolvió otra lista: %d vs %d procesos", len(second), len(first))
	}
}
