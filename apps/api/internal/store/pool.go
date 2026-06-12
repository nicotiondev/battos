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
		// Fase A1 (trust tiers): las DBs creadas antes no tienen execution_mode.
		"ALTER TABLE runs ADD COLUMN execution_mode TEXT NOT NULL DEFAULT 'sandbox' CHECK (execution_mode IN ('sandbox', 'direct', 'connected'))",
	}
	for _, stmt := range alters {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return fmt.Errorf("store: migración de columna (%s): %w", stmt, err)
		}
	}
	return migrateRunApprovalsKindCheck(ctx, db)
}

// migrateRunApprovalsKindCheck reconstruye run_approvals cuando su CHECK de
// kind es anterior a los kinds 'execution_mode' (Fase A1b). SQLite no permite
// alterar un CHECK: la única vía es renombrar, recrear e insertar.
// Idempotente: si el CHECK actual ya contiene 'execution_mode' no hace nada.
func migrateRunApprovalsKindCheck(ctx context.Context, db *sql.DB) error {
	var sqlText string
	err := db.QueryRowContext(ctx,
		"SELECT sql FROM sqlite_master WHERE type='table' AND name='run_approvals'",
	).Scan(&sqlText)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: leer schema de run_approvals: %w", err)
	}
	if strings.Contains(sqlText, "'execution_mode'") {
		return nil
	}

	// FK off durante el rebuild para que el RENAME no re-apunte referencias.
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("store: desactivar FK para migración: %w", err)
	}
	defer db.ExecContext(ctx, "PRAGMA foreign_keys = ON")

	stmts := []string{
		"ALTER TABLE run_approvals RENAME TO run_approvals_legacy",
		`CREATE TABLE run_approvals (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('execute', 'network', 'host_session', 'commit', 'push', 'remember', 'execution_mode')),
    decision TEXT NOT NULL CHECK (decision IN ('approved', 'rejected')),
    reason TEXT,
    decided_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)`,
		`INSERT INTO run_approvals (id, run_id, kind, decision, reason, decided_at)
SELECT id, run_id, kind, decision, reason, decided_at FROM run_approvals_legacy`,
		"DROP TABLE run_approvals_legacy",
		"CREATE INDEX IF NOT EXISTS idx_run_approvals_run ON run_approvals(run_id, decided_at DESC)",
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("store: rebuild de run_approvals (%s...): %w", stmt[:min(40, len(stmt))], err)
		}
	}
	return nil
}
