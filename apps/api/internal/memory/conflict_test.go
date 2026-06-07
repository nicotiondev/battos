package memory

import (
	"context"
	"testing"
)

// TestFindConflictCandidates_OverlappingTitle verifica que una observación con
// título que comparte términos con otra en el mismo proyecto aparece como candidata.
func TestFindConflictCandidates_OverlappingTitle(t *testing.T) {
	ctx := context.Background()
	core := openTestCore(t)

	existing, err := core.Save(ctx, Observation{
		Type:      TypeDecision,
		Title:     "Memory Core SQLite decision",
		Content:   "Usamos SQLite embebido para persistencia",
		ProjectID: "battos",
		Scope:     ScopeProject,
	})
	if err != nil {
		t.Fatalf("Save existing: %v", err)
	}

	newObs := Observation{
		Type:      TypeDecision,
		Title:     "Memory Core decision revisada",
		Content:   "Cambio de enfoque",
		ProjectID: "battos",
		Scope:     ScopeProject,
	}

	candidates, err := core.FindConflictCandidates(ctx, newObs, 5)
	if err != nil {
		t.Fatalf("FindConflictCandidates: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected at least one conflict candidate, got none")
	}
	if candidates[0].ID != existing.ID {
		t.Fatalf("expected candidate ID %d, got %d", existing.ID, candidates[0].ID)
	}
}

// TestFindConflictCandidates_NoOverlap verifica que una observación sin términos
// comunes no genera candidatos.
func TestFindConflictCandidates_NoOverlap(t *testing.T) {
	ctx := context.Background()
	core := openTestCore(t)

	_, err := core.Save(ctx, Observation{
		Type:      TypeDecision,
		Title:     "Router chi seleccionado",
		Content:   "Usamos chi para routing HTTP",
		ProjectID: "battos",
		Scope:     ScopeProject,
	})
	if err != nil {
		t.Fatalf("Save existing: %v", err)
	}

	// Título completamente diferente — sin tokens en común.
	newObs := Observation{
		Type:      TypeBugfix,
		Title:     "Corregido panic en worker goroutine",
		Content:   "Nil pointer en startup",
		ProjectID: "battos",
		Scope:     ScopeProject,
	}

	candidates, err := core.FindConflictCandidates(ctx, newObs, 5)
	if err != nil {
		t.Fatalf("FindConflictCandidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected no candidates, got %d: %+v", len(candidates), candidates)
	}
}

// TestFindConflictCandidates_ExcludesSelf verifica que cuando obs.ID está
// seteado, no se devuelve esa misma observación como candidata.
func TestFindConflictCandidates_ExcludesSelf(t *testing.T) {
	ctx := context.Background()
	core := openTestCore(t)

	saved, err := core.Save(ctx, Observation{
		Type:      TypeDecision,
		Title:     "Memory Core SQLite decision",
		Content:   "Decisión original",
		ProjectID: "battos",
		Scope:     ScopeProject,
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// obs con el mismo ID — simula una re-evaluación de esa misma observación.
	obs := Observation{
		ID:        saved.ID,
		Type:      TypeDecision,
		Title:     "Memory Core SQLite decision",
		Content:   "Decisión original",
		ProjectID: "battos",
		Scope:     ScopeProject,
	}

	candidates, err := core.FindConflictCandidates(ctx, obs, 5)
	if err != nil {
		t.Fatalf("FindConflictCandidates: %v", err)
	}
	for _, c := range candidates {
		if c.ID == saved.ID {
			t.Fatalf("FindConflictCandidates returned self (ID=%d) as candidate", saved.ID)
		}
	}
}

// TestFindConflictCandidates_ExcludesSameTopicKey verifica que un upsert sobre
// la misma topic_key no retorna la observación existente como conflicto.
func TestFindConflictCandidates_ExcludesSameTopicKey(t *testing.T) {
	ctx := context.Background()
	core := openTestCore(t)

	_, err := core.Save(ctx, Observation{
		Type:      TypeDecision,
		Title:     "Memory Core SQLite decision",
		Content:   "Decisión v1",
		TopicKey:  "battos/memory-decision",
		ProjectID: "battos",
		Scope:     ScopeProject,
	})
	if err != nil {
		t.Fatalf("Save existing: %v", err)
	}

	// Nueva observación con la MISMA topic_key (upsert): no debe aparecer como conflicto.
	newObs := Observation{
		Type:      TypeDecision,
		Title:     "Memory Core SQLite decision",
		Content:   "Decisión v2 mejorada",
		TopicKey:  "battos/memory-decision",
		ProjectID: "battos",
		Scope:     ScopeProject,
	}

	candidates, err := core.FindConflictCandidates(ctx, newObs, 5)
	if err != nil {
		t.Fatalf("FindConflictCandidates: %v", err)
	}
	for _, c := range candidates {
		if c.TopicKey == newObs.TopicKey {
			t.Fatalf("FindConflictCandidates returned same-topic_key observation as candidate (ID=%d)", c.ID)
		}
	}
}

// TestFindConflictCandidates_DifferentProjectExcluded verifica que observaciones
// de otro project_id no aparecen como candidatas.
func TestFindConflictCandidates_DifferentProjectExcluded(t *testing.T) {
	ctx := context.Background()
	core := openTestCore(t)

	_, err := core.Save(ctx, Observation{
		Type:      TypeDecision,
		Title:     "Memory Core SQLite decision",
		Content:   "De otro proyecto",
		ProjectID: "other-project",
		Scope:     ScopeProject,
	})
	if err != nil {
		t.Fatalf("Save other-project: %v", err)
	}

	newObs := Observation{
		Type:      TypeDecision,
		Title:     "Memory Core SQLite decision",
		Content:   "Del proyecto battos",
		ProjectID: "battos",
		Scope:     ScopeProject,
	}

	candidates, err := core.FindConflictCandidates(ctx, newObs, 5)
	if err != nil {
		t.Fatalf("FindConflictCandidates: %v", err)
	}
	for _, c := range candidates {
		if c.ProjectID == "other-project" {
			t.Fatalf("FindConflictCandidates returned candidate from different project (ID=%d)", c.ID)
		}
	}
}

// TestFindConflictCandidates_EmptyTitleReturnsNil verifica que un título vacío
// retorna nil sin error (guardia de seguridad).
func TestFindConflictCandidates_EmptyTitleReturnsNil(t *testing.T) {
	ctx := context.Background()
	core := openTestCore(t)

	obs := Observation{
		Type:      TypeManual,
		Title:     "   ", // solo espacios
		Content:   "contenido",
		ProjectID: "battos",
		Scope:     ScopeProject,
	}

	candidates, err := core.FindConflictCandidates(ctx, obs, 5)
	if err != nil {
		t.Fatalf("FindConflictCandidates with blank title: %v", err)
	}
	if candidates != nil {
		t.Fatalf("expected nil candidates for blank title, got %v", candidates)
	}
}

// TestFindConflictCandidates_DefaultLimit verifica que limit <= 0 usa el default (5).
func TestFindConflictCandidates_DefaultLimit(t *testing.T) {
	ctx := context.Background()
	core := openTestCore(t)

	// Insertar más de 5 observaciones con títulos superpuestos.
	for i := 0; i < 8; i++ {
		_, err := core.Save(ctx, Observation{
			Type:      TypeDecision,
			Title:     "Memory Core SQLite decision repetida",
			Content:   "contenido variado",
			ProjectID: "battos",
			Scope:     ScopeProject,
		})
		if err != nil {
			t.Fatalf("Save[%d]: %v", i, err)
		}
	}

	newObs := Observation{
		Type:      TypeDecision,
		Title:     "Memory Core SQLite",
		ProjectID: "battos",
		Scope:     ScopeProject,
	}

	// limit=0 → debe usar default interno (5). Con 8 coincidencias disponibles,
	// el resultado debe ser exactamente 5 (no menos), para fijar el default.
	candidates, err := core.FindConflictCandidates(ctx, newObs, 0)
	if err != nil {
		t.Fatalf("FindConflictCandidates(limit=0): %v", err)
	}
	if len(candidates) != 5 {
		t.Fatalf("expected exactly 5 candidates (default limit), got %d", len(candidates))
	}
}

// TestFindConflictCandidates_AllShortTokensReturnsNil verifica que un título
// cuyos tokens son todos demasiado cortos no produce un MATCH '' inválido
// (error de sintaxis FTS5), sino que retorna nil sin error — protegiendo a los
// callers directos, no solo al handler de save.
func TestFindConflictCandidates_AllShortTokensReturnsNil(t *testing.T) {
	ctx := context.Background()
	core := openTestCore(t)

	// Una observación normal para que, sin el guard, el MATCH '' llegue a FTS5.
	if _, err := core.Save(ctx, Observation{
		Type:      TypeDecision,
		Title:     "Memory Core SQLite decision",
		Content:   "contenido",
		ProjectID: "battos",
		Scope:     ScopeProject,
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	newObs := Observation{
		Type:      TypeManual,
		Title:     "a bc de", // todos los tokens <= 2 runas → expr FTS5 vacía
		ProjectID: "battos",
		Scope:     ScopeProject,
	}
	candidates, err := core.FindConflictCandidates(ctx, newObs, 5)
	if err != nil {
		t.Fatalf("expected no error for all-short-token title, got %v", err)
	}
	if candidates != nil {
		t.Fatalf("expected nil candidates for all-short-token title, got %v", candidates)
	}
}
