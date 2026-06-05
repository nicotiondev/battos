package worker

import (
	"context"
	"strings"
	"testing"

	"github.com/nicotion/battos/apps/api/internal/memory"
	"github.com/nicotion/battos/apps/api/internal/store"
)

func TestMemoryCoreContextProvider_ContextForRun(t *testing.T) {
	// 1. Inicializar Core en memoria
	core, err := memory.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open memory core: %v", err)
	}
	defer core.Close()

	ctx := context.Background()

	// 2. Insertar memorias de prueba
	// Proyecto: Scope project
	_, _ = core.Save(ctx, memory.Observation{
		Type:      memory.TypeDecision,
		Title:     "Decisión de Proyecto",
		Content:   "Usar Go stack en API",
		ProjectID: "proj-1",
		Scope:     memory.ScopeProject,
	})

	// Proyecto: Scope personal (preferencias del usuario en este proyecto)
	_, _ = core.Save(ctx, memory.Observation{
		Type:      memory.TypeManual,
		Title:     "Preferencia Personal",
		Content:   "Usar editor de texto oscuro",
		ProjectID: "proj-1",
		Scope:     memory.ScopePersonal,
	})

	// Agente específico
	_, _ = core.Save(ctx, memory.Observation{
		Type:      memory.TypePattern,
		Title:     "Patrón de Agente Codex",
		Content:   "Codex prefiere usar sqlc",
		AgentID:   "agent-codex",
		Scope:     memory.ScopeProject,
	})

	// Global (sin project_id)
	_, _ = core.Save(ctx, memory.Observation{
		Type:    memory.TypeArchitecture,
		Title:   "Arquitectura Global",
		Content: "Toda la comunicación de red va en HTTPS",
		Scope:   memory.ScopeProject,
	})

	// Otro proyecto (no debería inyectarse)
	_, _ = core.Save(ctx, memory.Observation{
		Type:      memory.TypeBugfix,
		Title:     "Bugfix de Proj-2",
		Content:   "Arreglar auth en proj-2",
		ProjectID: "proj-2",
		Scope:     memory.ScopeProject,
	})

	provider := MemoryCoreContextProvider{
		Core:  core,
		Limit: 10,
	}

	run := store.Run{
		ProjectID: "proj-1",
		AgentID:   "agent-codex",
	}

	// 3. Ejecutar ContextForRun
	memContext, err := provider.ContextForRun(ctx, run)
	if err != nil {
		t.Fatalf("ContextForRun failed: %v", err)
	}

	// 4. Validar resultados
	if memContext.Count != 4 {
		t.Fatalf("expected 4 memories to be injected, got %d", memContext.Count)
	}

	content := memContext.Content

	// Comprobar que contiene la memoria del proyecto
	if !strings.Contains(content, "Decisión de Proyecto") || !strings.Contains(content, "Usar Go stack en API") {
		t.Errorf("missing project memory")
	}

	// Comprobar que contiene la memoria personal
	if !strings.Contains(content, "Preferencia Personal") || !strings.Contains(content, "Usar editor de texto oscuro") {
		t.Errorf("missing personal project memory")
	}

	// Comprobar que contiene la memoria de agente
	if !strings.Contains(content, "Patrón de Agente Codex") || !strings.Contains(content, "Codex prefiere usar sqlc") {
		t.Errorf("missing agent memory")
	}

	// Comprobar que contiene la memoria global
	if !strings.Contains(content, "Arquitectura Global") || !strings.Contains(content, "Toda la comunicación de red va en HTTPS") {
		t.Errorf("missing global memory")
	}

	// Comprobar que NO contiene la memoria de otro proyecto
	if strings.Contains(content, "Bugfix de Proj-2") {
		t.Errorf("should not contain memory from another project")
	}
}
