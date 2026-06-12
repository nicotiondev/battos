package memory

import "context"

// MemoryProvider es la interfaz pública del Memory Core.
//
// Permite que los handlers dependan de una abstracción en lugar de la
// implementación concreta (*Core), facilitando testing y la incorporación
// de providers alternativos (e.g. EngramProvider en Etapa 4.2).
type MemoryProvider interface {
	Save(ctx context.Context, o Observation) (*Observation, error)
	Search(ctx context.Context, query string, filter SearchFilter) ([]SearchResult, error)
	Recent(ctx context.Context, limit int) ([]Observation, error)
	Stats(ctx context.Context) (*Stats, error)
	GetByID(ctx context.Context, id int64) (*Observation, error)
	FindConflictCandidates(ctx context.Context, obs Observation, limit int) ([]SearchResult, error)
}

// compile-time check: *Core satisface MemoryProvider.
var _ MemoryProvider = (*Core)(nil)
