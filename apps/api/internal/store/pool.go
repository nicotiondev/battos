package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenPool abre un pool de conexiones a Postgres con timeouts razonables.
//
// El DATABASE_URL viene del env (DATABASE_URL=postgresql://...) y se valida
// con un Ping antes de devolver el pool.
//
// El pool es la unidad que se inyecta a los handlers; sqlc.New(pool) crea
// el Queries que ejecuta las queries generadas.
func OpenPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("store: DATABASE_URL vacío")
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: parseando DATABASE_URL: %w", err)
	}

	// Tamaño razonable para single-instance dev. Subir en prod si hace falta.
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: pgxpool.NewWithConfig: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping inicial: %w", err)
	}

	return pool, nil
}
