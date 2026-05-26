// Package memory implementa el Memory Core de BattOS.
//
// Concepto inspirado en Engram (Gentleman-Programming/engram): SQLite + FTS5
// + "observaciones" tipadas + búsqueda full-text. Schema y código son propios,
// adaptados a BattOS (agregamos project_id, agent_id, scope).
//
// Vive embebido dentro de battos-api — sin proceso separado. Driver:
// modernc.org/sqlite (puro Go, sin CGo, cross-compile trivial).
//
// Filosofía:
//   - Save: guarda observaciones estructuradas (decisión, bugfix, pattern, ...).
//   - Search: FTS5 MATCH con ranking BM25.
//   - Recent: últimas N.
//   - Context: snapshot consolidado por proyecto.
//   - Upsert por topic_key: misma topic_key reemplaza la observación previa.
package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // driver registrado como "sqlite"
)

// ObservationType clasifica una memoria.
//
// Estos tipos son lookup-friendly; las queries pueden filtrar por type.
type ObservationType string

const (
	TypeDecision     ObservationType = "decision"
	TypeArchitecture ObservationType = "architecture"
	TypeBugfix       ObservationType = "bugfix"
	TypePattern      ObservationType = "pattern"
	TypeDiscovery    ObservationType = "discovery"
	TypeLearning     ObservationType = "learning"
	TypeManual       ObservationType = "manual"
)

// Scope separa memoria operativa de personal.
type Scope string

const (
	ScopeProject  Scope = "project"
	ScopePersonal Scope = "personal"
)

// Observation es la unidad de memoria del Memory Core.
//
// Equivalente conceptual al "observation" de Engram + campos extra que
// BattOS necesita (project_id, agent_id, scope).
type Observation struct {
	ID         int64           `json:"id"`
	Type       ObservationType `json:"type"`
	Title      string          `json:"title"`
	Content    string          `json:"content"`
	TopicKey   string          `json:"topic_key,omitempty"` // upsert key
	ProjectID  string          `json:"project_id,omitempty"`
	AgentID    string          `json:"agent_id,omitempty"`
	Scope      Scope           `json:"scope"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// SearchResult es una Observation con un score de relevancia.
type SearchResult struct {
	Observation
	Rank float64 `json:"rank"` // BM25 (menor = mejor)
}

// SearchFilter aplica filtros a Search() además del texto.
type SearchFilter struct {
	Type      ObservationType
	ProjectID string
	AgentID   string
	Scope     Scope
	Limit     int
}

// Core es la API pública del Memory Core.
//
// Concurrencia: SQLite con WAL mode soporta múltiples lectores + 1 escritor.
// Es seguro compartir un *Core entre handlers HTTP.
type Core struct {
	db *sql.DB
}

// Open abre (o crea) el archivo SQLite y aplica el schema.
//
// El path se crea si no existe. Activamos WAL para tolerar lecturas paralelas.
func Open(dbPath string) (*Core, error) {
	if dbPath == "" {
		return nil, errors.New("memory: db_path vacío")
	}

	// Asegurar que el directorio existe.
	if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
		// No usamos os.MkdirAll acá para no traer otra dependencia;
		// el caller (config/sysmetrics) ya creó data/memory/ al boot.
	}

	// modernc.org/sqlite usa el nombre "sqlite" (no "sqlite3").
	// _journal_mode=WAL → concurrent reads.
	// _foreign_keys=on → integridad referencial.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("abriendo SQLite: %w", err)
	}

	// SQLite single-writer: limitamos pool a 1 escritor + varios lectores.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("ping SQLite: %w", err)
	}

	c := &Core{db: db}
	if err := c.applySchema(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return c, nil
}

// Close cierra el handle de DB.
func (c *Core) Close() error {
	return c.db.Close()
}

// Ping verifica que la DB esté viva. Usado por health checks.
func (c *Core) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// applySchema crea las tablas si no existen.
// Idempotente — corre en cada boot.
func (c *Core) applySchema(ctx context.Context) error {
	stmts := []string{
		// Tabla principal de observaciones.
		`CREATE TABLE IF NOT EXISTS memory_items (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			type        TEXT NOT NULL DEFAULT 'manual',
			title       TEXT NOT NULL,
			content     TEXT NOT NULL,
			topic_key   TEXT,
			project_id  TEXT,
			agent_id    TEXT,
			scope       TEXT NOT NULL DEFAULT 'project',
			created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_items_topic ON memory_items(topic_key) WHERE topic_key IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_memory_items_project ON memory_items(project_id) WHERE project_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_memory_items_type ON memory_items(type)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_items_created ON memory_items(created_at DESC)`,

		// FTS5 virtual table sincronizada via triggers (contentless syncing).
		`CREATE VIRTUAL TABLE IF NOT EXISTS memory_items_fts USING fts5(
			title, content, topic_key,
			content='memory_items',
			content_rowid='id',
			tokenize='unicode61 remove_diacritics 2'
		)`,
		// Triggers para mantener FTS5 sincronizada.
		`CREATE TRIGGER IF NOT EXISTS memory_items_ai AFTER INSERT ON memory_items BEGIN
			INSERT INTO memory_items_fts(rowid, title, content, topic_key)
			VALUES (new.id, new.title, new.content, COALESCE(new.topic_key, ''));
		END`,
		`CREATE TRIGGER IF NOT EXISTS memory_items_ad AFTER DELETE ON memory_items BEGIN
			INSERT INTO memory_items_fts(memory_items_fts, rowid, title, content, topic_key)
			VALUES ('delete', old.id, old.title, old.content, COALESCE(old.topic_key, ''));
		END`,
		`CREATE TRIGGER IF NOT EXISTS memory_items_au AFTER UPDATE ON memory_items BEGIN
			INSERT INTO memory_items_fts(memory_items_fts, rowid, title, content, topic_key)
			VALUES ('delete', old.id, old.title, old.content, COALESCE(old.topic_key, ''));
			INSERT INTO memory_items_fts(rowid, title, content, topic_key)
			VALUES (new.id, new.title, new.content, COALESCE(new.topic_key, ''));
		END`,
	}

	for _, stmt := range stmts {
		if _, err := c.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("schema: %w (stmt: %s)", err, firstLine(stmt))
		}
	}
	return nil
}

// Save inserta una observación, o actualiza la existente si TopicKey ya existe.
//
// Si TopicKey está vacío, siempre inserta una nueva fila (no upsert).
// Devuelve la observación con ID, CreatedAt y UpdatedAt poblados.
func (c *Core) Save(ctx context.Context, o Observation) (*Observation, error) {
	if strings.TrimSpace(o.Title) == "" {
		return nil, errors.New("memory: title vacío")
	}
	if o.Type == "" {
		o.Type = TypeManual
	}
	if o.Scope == "" {
		o.Scope = ScopeProject
	}

	// Upsert por topic_key si está presente.
	if o.TopicKey != "" {
		var existingID int64
		err := c.db.QueryRowContext(ctx,
			`SELECT id FROM memory_items WHERE topic_key = ?`, o.TopicKey,
		).Scan(&existingID)
		if err == nil {
			// Existe → UPDATE.
			_, err := c.db.ExecContext(ctx, `
				UPDATE memory_items
				SET type=?, title=?, content=?, project_id=?, agent_id=?, scope=?, updated_at=CURRENT_TIMESTAMP
				WHERE id=?`,
				o.Type, o.Title, o.Content,
				nullIfEmpty(o.ProjectID), nullIfEmpty(o.AgentID), o.Scope, existingID,
			)
			if err != nil {
				return nil, fmt.Errorf("memory: update: %w", err)
			}
			return c.GetByID(ctx, existingID)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("memory: lookup topic_key: %w", err)
		}
		// No existe → cae al INSERT.
	}

	res, err := c.db.ExecContext(ctx, `
		INSERT INTO memory_items (type, title, content, topic_key, project_id, agent_id, scope)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		o.Type, o.Title, o.Content,
		nullIfEmpty(o.TopicKey),
		nullIfEmpty(o.ProjectID),
		nullIfEmpty(o.AgentID),
		o.Scope,
	)
	if err != nil {
		return nil, fmt.Errorf("memory: insert: %w", err)
	}
	id, _ := res.LastInsertId()
	return c.GetByID(ctx, id)
}

// GetByID lee una observación por id.
func (c *Core) GetByID(ctx context.Context, id int64) (*Observation, error) {
	row := c.db.QueryRowContext(ctx, `
		SELECT id, type, title, content,
		       COALESCE(topic_key, ''), COALESCE(project_id, ''), COALESCE(agent_id, ''),
		       scope, created_at, updated_at
		FROM memory_items WHERE id = ?`, id)
	return scanObservation(row)
}

// Recent devuelve las últimas N observaciones (más nuevas primero).
func (c *Core) Recent(ctx context.Context, limit int) ([]Observation, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := c.db.QueryContext(ctx, `
		SELECT id, type, title, content,
		       COALESCE(topic_key, ''), COALESCE(project_id, ''), COALESCE(agent_id, ''),
		       scope, created_at, updated_at
		FROM memory_items
		ORDER BY created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("memory: recent: %w", err)
	}
	defer rows.Close()
	return scanObservations(rows)
}

// Search hace búsqueda FTS5 con ranking BM25.
//
// Si query está vacío, se comporta como Recent(filter.Limit).
// Si query tiene texto, usa MATCH con sanitización (FTS5 interpreta operadores
// especiales como -, AND, OR, NOT — los wrappeamos en quotes para search literal).
func (c *Core) Search(ctx context.Context, query string, filter SearchFilter) ([]SearchResult, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}

	query = strings.TrimSpace(query)
	if query == "" {
		// Sin query → fallback a Recent con filtros.
		obs, err := c.Recent(ctx, filter.Limit)
		if err != nil {
			return nil, err
		}
		results := make([]SearchResult, len(obs))
		for i, o := range obs {
			results[i] = SearchResult{Observation: o, Rank: 0}
		}
		return results, nil
	}

	// Construir query SQL con filtros opcionales.
	var conditions []string
	var args []any

	conditions = append(conditions, "memory_items_fts MATCH ?")
	args = append(args, sanitizeFTS(query))

	if filter.Type != "" {
		conditions = append(conditions, "m.type = ?")
		args = append(args, filter.Type)
	}
	if filter.ProjectID != "" {
		conditions = append(conditions, "m.project_id = ?")
		args = append(args, filter.ProjectID)
	}
	if filter.AgentID != "" {
		conditions = append(conditions, "m.agent_id = ?")
		args = append(args, filter.AgentID)
	}
	if filter.Scope != "" {
		conditions = append(conditions, "m.scope = ?")
		args = append(args, filter.Scope)
	}

	sqlStr := fmt.Sprintf(`
		SELECT m.id, m.type, m.title, m.content,
		       COALESCE(m.topic_key, ''), COALESCE(m.project_id, ''), COALESCE(m.agent_id, ''),
		       m.scope, m.created_at, m.updated_at,
		       bm25(memory_items_fts) AS rank
		FROM memory_items_fts
		JOIN memory_items m ON m.id = memory_items_fts.rowid
		WHERE %s
		ORDER BY rank
		LIMIT ?`,
		strings.Join(conditions, " AND "))

	args = append(args, filter.Limit)

	rows, err := c.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("memory: search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		err := rows.Scan(
			&r.ID, &r.Type, &r.Title, &r.Content,
			&r.TopicKey, &r.ProjectID, &r.AgentID,
			&r.Scope, &r.CreatedAt, &r.UpdatedAt,
			&r.Rank,
		)
		if err != nil {
			return nil, fmt.Errorf("memory: scan search row: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Stats devuelve un snapshot de uso del Memory Core.
type Stats struct {
	TotalItems     int64     `json:"total_items"`
	ItemsLast24h   int64     `json:"items_last_24h"`
	UniqueProjects int64     `json:"unique_projects"`
	UniqueAgents   int64     `json:"unique_agents"`
	OldestItem     time.Time `json:"oldest_item"`
	NewestItem     time.Time `json:"newest_item"`
}

// Stats agrega métricas para mostrar en el dashboard.
func (c *Core) Stats(ctx context.Context) (*Stats, error) {
	s := &Stats{}
	// MIN/MAX(created_at) en SQLite a veces vuelven como string (depende del driver
	// y de si la columna es TEXT vs DATETIME). Scaneamos a sql.NullString y parseamos
	// si no son NULL — más robusto que pelearle al driver.
	var oldestStr, newestStr sql.NullString
	err := c.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COUNT(CASE WHEN created_at >= datetime('now', '-1 day') THEN 1 END),
			COUNT(DISTINCT project_id),
			COUNT(DISTINCT agent_id),
			MIN(created_at),
			MAX(created_at)
		FROM memory_items`,
	).Scan(&s.TotalItems, &s.ItemsLast24h, &s.UniqueProjects, &s.UniqueAgents, &oldestStr, &newestStr)
	if err != nil {
		return nil, fmt.Errorf("memory: stats: %w", err)
	}
	s.OldestItem = parseSQLiteTime(oldestStr)
	s.NewestItem = parseSQLiteTime(newestStr)
	return s, nil
}

// parseSQLiteTime acepta los formatos que SQLite emite para timestamps
// (con y sin sub-segundos, con o sin zona).
func parseSQLiteTime(s sql.NullString) time.Time {
	if !s.Valid || s.String == "" {
		return time.Time{}
	}
	formats := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s.String); err == nil {
			return t
		}
	}
	return time.Time{}
}

// --- Helpers ---

func scanObservation(row interface{ Scan(...any) error }) (*Observation, error) {
	var o Observation
	err := row.Scan(
		&o.ID, &o.Type, &o.Title, &o.Content,
		&o.TopicKey, &o.ProjectID, &o.AgentID,
		&o.Scope, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("memory: scan: %w", err)
	}
	return &o, nil
}

func scanObservations(rows *sql.Rows) ([]Observation, error) {
	var out []Observation
	for rows.Next() {
		var o Observation
		err := rows.Scan(
			&o.ID, &o.Type, &o.Title, &o.Content,
			&o.TopicKey, &o.ProjectID, &o.AgentID,
			&o.Scope, &o.CreatedAt, &o.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("memory: scan row: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// nullIfEmpty convierte string vacío a sql.NullString para que SQLite lo guarde como NULL.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// sanitizeFTS escapa el query del usuario para FTS5.
//
// FTS5 interpreta operadores especiales (-, AND, OR, NOT, ", *, :, (, )). Para
// búsqueda de texto literal, wrappeamos cada término en quotes dobles.
// Ejemplo: `fix auth bug` → `"fix" "auth" "bug"`.
func sanitizeFTS(query string) string {
	tokens := strings.Fields(query)
	wrapped := make([]string, 0, len(tokens))
	for _, t := range tokens {
		// Escapar comillas dobles internas duplicándolas (FTS5 syntax).
		t = strings.ReplaceAll(t, `"`, `""`)
		wrapped = append(wrapped, `"`+t+`"`)
	}
	return strings.Join(wrapped, " ")
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\n\r"); i >= 0 {
		return s[:i]
	}
	return s
}
