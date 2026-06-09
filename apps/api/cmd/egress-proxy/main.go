// Package main es el entrypoint del proxy de egress de BattOS (ADR-0022).
//
// El proxy filtra conexiones salientes de los contenedores de runs con
// host_session montado, permitiendo solo los dominios en la allowlist.
// Soporta CONNECT (HTTPS tunneling) y peticiones HTTP planas.
//
// Uso:
//
//	egress-proxy [flags]
//
// Flags / env:
//
//	--addr           BATTOS_EGRESS_ADDR       direccion de escucha (default :8888)
//	--allowlist      BATTOS_EGRESS_ALLOWLIST  dominios permitidos, separados por coma
//	--mode           BATTOS_EGRESS_MODE       "enforce" o "log_only" (default log_only)
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/nicotion/battos/apps/api/internal/egress"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	// --- Flags ---
	addr := flag.String("addr", envOr("BATTOS_EGRESS_ADDR", ":8888"), "direccion de escucha del proxy")
	allowlistRaw := flag.String("allowlist", envOr("BATTOS_EGRESS_ALLOWLIST", ""), "dominios permitidos separados por coma")
	mode := flag.String("mode", envOr("BATTOS_EGRESS_MODE", egress.ModeLogOnly), "modo: enforce|log_only")
	flag.Parse()

	// --- Validar mode ---
	if *mode != egress.ModeEnforce && *mode != egress.ModeLogOnly {
		return fmt.Errorf("mode invalido %q: debe ser %q o %q", *mode, egress.ModeEnforce, egress.ModeLogOnly)
	}

	// --- Parsear allowlist ---
	var allowlist []string
	if *allowlistRaw != "" {
		for _, entry := range strings.Split(*allowlistRaw, ",") {
			entry = strings.TrimSpace(entry)
			if entry != "" {
				allowlist = append(allowlist, entry)
			}
		}
	}

	// --- Logger JSON ---
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	log.Info("egress proxy arrancando",
		"addr", *addr,
		"mode", *mode,
		"allowlist_count", len(allowlist),
		"allowlist", allowlist,
	)

	// --- Proxy ---
	p := &egress.Proxy{
		Addr:      *addr,
		Allowlist: allowlist,
		Mode:      *mode,
		Logger:    log,
	}

	// --- Shutdown graceful ---
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := p.ListenAndServe(ctx); err != nil && err != context.Canceled {
		return fmt.Errorf("egress proxy: %w", err)
	}

	log.Info("egress proxy detenido")
	return nil
}

// envOr retorna el valor de la variable de entorno name, o fallback si no esta definida.
func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
