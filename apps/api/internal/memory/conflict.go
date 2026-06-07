package memory

import (
	"context"
	"fmt"
	"strings"
)

// FindConflictCandidates busca observaciones existentes que puedan superponerse
// léxicamente con obs, para surfacear posibles conflictos o duplicados antes de
// que un agente o usuario los juzgue.
//
// Semántica:
//   - Usa FTS5 con OR entre los tokens del título de obs (más permisivo que la
//     búsqueda AND estándar; queremos "cualquier cosa que se superponga").
//   - Scoped al mismo project_id que obs.
//   - Excluye la propia observación (obs.ID != 0).
//   - Excluye observaciones con el mismo topic_key que obs (son upserts, no
//     conflictos).
//   - Ordena por BM25 (menor = más relevante).
//
// Si el título de obs está vacío/blank, retorna nil, nil (nada que buscar).
// Si limit <= 0, usa 5 como default.
//
// Nota de diseño: esta función es puramente determinista/léxica. El juicio
// semántico (¿es realmente un conflicto?) es tarea de 2.2b, fuera de scope.
func (c *Core) FindConflictCandidates(ctx context.Context, obs Observation, limit int) ([]SearchResult, error) {
	title := strings.TrimSpace(obs.Title)
	if title == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}

	// Construir expresión FTS5 con semántica OR entre los tokens del título.
	// Cada token se envuelve en comillas dobles (escapando `"` internas como `""`)
	// para tratar cada token literalmente — evita que FTS5 interprete operadores.
	// Los tokens se unen con OR para capturar cualquier superposición parcial.
	matchExpr := buildORMatchExpr(title)

	// Construir condiciones dinámicas para las exclusiones y scope.
	var conditions []string
	var args []any

	conditions = append(conditions, "memory_items_fts MATCH ?")
	args = append(args, matchExpr)

	// Scope al mismo project_id que obs. Si obs.ProjectID está vacío buscamos
	// sin restricción de proyecto (no debería ocurrir en uso normal).
	if obs.ProjectID != "" {
		conditions = append(conditions, "m.project_id = ?")
		args = append(args, obs.ProjectID)
	}

	// Excluir la propia observación cuando obs.ID está poblado.
	if obs.ID != 0 {
		conditions = append(conditions, "m.id != ?")
		args = append(args, obs.ID)
	}

	// Excluir observaciones con el mismo topic_key (upserts, no conflictos).
	if obs.TopicKey != "" {
		conditions = append(conditions, "(m.topic_key IS NULL OR m.topic_key != ?)")
		args = append(args, obs.TopicKey)
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

	args = append(args, limit)

	rows, err := c.db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("memory: conflict candidates: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(
			&r.ID, &r.Type, &r.Title, &r.Content,
			&r.TopicKey, &r.ProjectID, &r.AgentID,
			&r.Scope, &r.CreatedAt, &r.UpdatedAt,
			&r.Rank,
		); err != nil {
			return nil, fmt.Errorf("memory: conflict candidates scan: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// buildORMatchExpr construye una expresión FTS5 MATCH con semántica OR entre
// los tokens del texto dado.
//
// Ejemplo: "Memory Core SQLite" → `"Memory" OR "Core" OR "SQLite"`
//
// Cada token se envuelve en comillas dobles y las comillas internas se escapan
// duplicándolas (convención FTS5). Los tokens de 1–2 caracteres se omiten para
// reducir ruido (artículos, preposiciones, etc.).
func buildORMatchExpr(text string) string {
	tokens := strings.Fields(text)
	wrapped := make([]string, 0, len(tokens))
	for _, t := range tokens {
		// Omitir tokens muy cortos (ruido léxico).
		if len([]rune(t)) <= 2 {
			continue
		}
		// Escapar comillas dobles internas duplicándolas (FTS5 syntax).
		t = strings.ReplaceAll(t, `"`, `""`)
		wrapped = append(wrapped, `"`+t+`"`)
	}
	if len(wrapped) == 0 {
		return ""
	}
	return strings.Join(wrapped, " OR ")
}
