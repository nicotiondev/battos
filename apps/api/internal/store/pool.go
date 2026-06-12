package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed sqlite_schema.sql
var sqliteSchema string

// OpenDB abre la base SQLite unificada de BattOS y aplica el schema
// idempotente. Es la fuente operacional única de v0.1.
func OpenDB(ctx context.Context, dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return nil, errors.New("store: database path vacío")
	}
	if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: crear directorio SQLite: %w", err)
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: abrir SQLite: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: ping SQLite: %w", err)
	}
	if _, err := db.ExecContext(ctx, sqliteSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: aplicar schema SQLite: %w", err)
	}
	if err := applyColumnMigrations(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// applyColumnMigrations agrega columnas a tablas ya creadas en DBs existentes.
// CREATE TABLE IF NOT EXISTS no altera tablas viejas, así que cada columna nueva
// del schema necesita su ALTER acá. Idempotente: el error "duplicate column
// name" se ignora.
func applyColumnMigrations(ctx context.Context, db *sql.DB) error {
	alters := []string{
		"ALTER TABLE cli_tools ADD COLUMN install_command TEXT",
		"ALTER TABLE cli_tools ADD COLUMN install_url TEXT",
	}
	for _, stmt := range alters {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return fmt.Errorf("store: migración de columna (%s): %w", stmt, err)
		}
	}
	return nil
}
