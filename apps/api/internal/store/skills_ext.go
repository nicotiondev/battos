// Package store — extensiones manuales para skills.
// Este archivo NO es generado por sqlc. Contiene metodos
// que no tienen query SQL equivalente todavia o que son
// demasiado especificos para el generador.
package store

import (
	"context"
	"database/sql"
)

// UpsertSkillParams son los parametros para insertar o actualizar un skill
// completo desde un archivo SKILL.md.
type UpsertSkillParams struct {
	ID          string
	Slug        string
	Name        string
	Description sql.NullString
	Version     sql.NullString
	// PromptTemplate contiene el cuerpo markdown del SKILL.md.
	PromptTemplate sql.NullString
}

const upsertSkillFromMD = `
INSERT INTO skills (
    id, slug, name, description, version, prompt_template,
    source, status, lifecycle,
    inputs, outputs, compatible_agents, compatible_runtimes
)
VALUES (?, ?, ?, ?, ?, ?, 'local', 'active', 'active',
        '[]', '[]', '[]', '[]')
ON CONFLICT (slug) DO UPDATE SET
    name             = EXCLUDED.name,
    description      = EXCLUDED.description,
    version          = EXCLUDED.version,
    prompt_template  = EXCLUDED.prompt_template,
    source           = 'local',
    status           = 'active',
    updated_at       = CURRENT_TIMESTAMP
RETURNING id, slug, name, description, category, risk_level, inputs, outputs, steps,
          compatible_agents, compatible_runtimes, source, source_id, source_ref, version,
          status, prompt_template, lifecycle, created_at, updated_at
`

// UpsertSkillFromMD inserta o actualiza un skill cuyo ID es el slug.
// Si ya existe un skill con ese slug, actualiza name, description, version
// y prompt_template. Devuelve el skill resultante.
func (q *Queries) UpsertSkillFromMD(ctx context.Context, arg UpsertSkillParams) (Skill, error) {
	row := q.db.QueryRowContext(ctx, upsertSkillFromMD,
		arg.ID,
		arg.Slug,
		arg.Name,
		arg.Description,
		arg.Version,
		arg.PromptTemplate,
	)
	var i Skill
	err := row.Scan(
		&i.ID,
		&i.Slug,
		&i.Name,
		&i.Description,
		&i.Category,
		&i.RiskLevel,
		&i.Inputs,
		&i.Outputs,
		&i.Steps,
		&i.CompatibleAgents,
		&i.CompatibleRuntimes,
		&i.Source,
		&i.SourceID,
		&i.SourceRef,
		&i.Version,
		&i.Status,
		&i.PromptTemplate,
		&i.Lifecycle,
		&i.CreatedAt,
		&i.UpdatedAt,
	)
	return i, err
}
