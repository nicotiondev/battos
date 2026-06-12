package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nicotion/battos/apps/api/internal/memory"
	"github.com/nicotion/battos/apps/api/internal/store"
)

const defaultMemoryContextLimit = 12

// PromoteRunSummary implementa MemoryPromoter: renderiza el resumen del run
// terminado y lo guarda como observación tipo learning en el Memory Core
// (B3 — sesiones→memoria, Etapa 3).
func (p MemoryCoreContextProvider) PromoteRunSummary(ctx context.Context, run store.Run, logs []store.RunLog) error {
	if p.Core == nil {
		return fmt.Errorf("memory core no disponible")
	}
	_, err := p.Core.Save(ctx, memory.Observation{
		Type:      memory.TypeLearning,
		Title:     fmt.Sprintf("Run %s %s", shortRunID(run.ID), run.Status),
		Content:   RenderRunSummary(run, logs, true, false, 20),
		TopicKey:  fmt.Sprintf("%s/runs/%s/summary", run.ProjectID, run.ID),
		ProjectID: run.ProjectID,
		AgentID:   run.AgentID,
		Scope:     memory.ScopeProject,
	})
	if err != nil {
		return fmt.Errorf("guardar resumen del run en memoria: %w", err)
	}
	return nil
}

// RenderRunSummary arma el Markdown del resumen de un run para memoria.
// Es la única implementación: el flujo manual `remember` (handlers) y la
// promoción automática del worker renderizan idéntico.
func RenderRunSummary(run store.Run, logs []store.RunLog, includeLogs, includePrompt bool, logLimit int) string {
	var b strings.Builder
	b.WriteString("# BattOS Run Summary\n\n")
	b.WriteString(fmt.Sprintf("- Run: %s\n", run.ID))
	b.WriteString(fmt.Sprintf("- Project: %s\n", run.ProjectID))
	b.WriteString(fmt.Sprintf("- Task: %s\n", run.TaskID))
	b.WriteString(fmt.Sprintf("- Agent: %s\n", run.AgentID))
	b.WriteString(fmt.Sprintf("- Runtime: %s\n", run.RuntimeAdapterID))
	b.WriteString(fmt.Sprintf("- Status: %s\n", run.Status))
	if run.ResultSummary.Valid && run.ResultSummary.String != "" {
		b.WriteString(fmt.Sprintf("- Result: %s\n", run.ResultSummary.String))
	}
	if run.ErrorMessage.Valid && run.ErrorMessage.String != "" {
		b.WriteString(fmt.Sprintf("- Error: %s\n", run.ErrorMessage.String))
	}
	if includePrompt && run.Prompt != "" {
		b.WriteString("\n## Prompt\n\n")
		b.WriteString(strings.TrimSpace(run.Prompt))
		b.WriteString("\n")
	}
	if includeLogs && len(logs) > 0 {
		b.WriteString("\n## Logs\n\n")
		start := 0
		if logLimit > 0 && len(logs) > logLimit {
			start = len(logs) - logLimit
		}
		for i := start; i < len(logs); i++ {
			b.WriteString(fmt.Sprintf("- `%s` %s\n", logs[i].Stream, strings.TrimSpace(logs[i].Message)))
		}
	}
	return b.String()
}

func shortRunID(value string) string {
	if len(value) <= 8 {
		return value
	}
	return value[:8]
}

type MemoryCoreContextProvider struct {
	Core  *memory.Core
	Limit int
}

func (p MemoryCoreContextProvider) ContextForRun(ctx context.Context, run store.Run) (MemoryContext, error) {
	if p.Core == nil {
		return MemoryContext{}, nil
	}
	limit := p.Limit
	if limit <= 0 {
		limit = defaultMemoryContextLimit
	}

	// 1. Memorias del proyecto actual (scope=project)
	res1, err := p.Core.Search(ctx, "", memory.SearchFilter{
		ProjectID: run.ProjectID,
		Scope:     memory.ScopeProject,
		Limit:     limit,
	})
	if err != nil {
		return MemoryContext{}, fmt.Errorf("search project memory: %w", err)
	}

	// 2. Memorias personales del proyecto actual (scope=personal)
	res2, err := p.Core.Search(ctx, "", memory.SearchFilter{
		ProjectID: run.ProjectID,
		Scope:     memory.ScopePersonal,
		Limit:     limit,
	})
	if err != nil {
		return MemoryContext{}, fmt.Errorf("search personal project memory: %w", err)
	}

	// 3. Memorias del agente actual (si aplica)
	var res3 []memory.SearchResult
	if run.AgentID != "" {
		res3, err = p.Core.Search(ctx, "", memory.SearchFilter{
			AgentID: run.AgentID,
			Limit:   limit,
		})
		if err != nil {
			return MemoryContext{}, fmt.Errorf("search agent memory: %w", err)
		}
	}

	// 4. Memorias globales (project_id vacío)
	// Buscamos observaciones recientes sin filtro de proyecto y filtramos en Go
	var resGlobal []memory.SearchResult
	resAll, err := p.Core.Search(ctx, "", memory.SearchFilter{
		Limit: limit * 2,
	})
	if err == nil {
		for _, r := range resAll {
			if r.ProjectID == "" {
				resGlobal = append(resGlobal, r)
			}
		}
	}

	// Combinar y deduplicar manteniendo el orden de prioridad
	seen := make(map[int64]bool)
	var combined []memory.SearchResult

	addResult := func(r memory.SearchResult) {
		if !seen[r.ID] {
			seen[r.ID] = true
			combined = append(combined, r)
		}
	}

	// 1. Prioridad: Memorias de Proyecto (Scope=project)
	for _, r := range res1 {
		addResult(r)
	}
	// 2. Prioridad: Memorias Personales del Proyecto (Scope=personal)
	for _, r := range res2 {
		addResult(r)
	}
	// 3. Prioridad: Memorias de Agente
	for _, r := range res3 {
		addResult(r)
	}
	// 4. Prioridad: Memorias Globales
	for _, r := range resGlobal {
		addResult(r)
	}

	// Acotar al limite maximo
	if len(combined) > limit {
		combined = combined[:limit]
	}

	return MemoryContext{
		Content: renderRunMemoryContext(run.ProjectID, combined, time.Now().UTC()),
		Count:   len(combined),
	}, nil
}

func renderRunMemoryContext(projectID string, results []memory.SearchResult, generated time.Time) string {
	if len(results) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# BattOS Memory Context\n\n")
	if projectID != "" {
		b.WriteString("- Project: " + projectID + "\n")
	}
	if !generated.IsZero() {
		b.WriteString("- Generated: " + generated.Format(time.RFC3339) + "\n")
	}
	b.WriteString("- Scope: project\n\n")
	for _, result := range results {
		b.WriteString(fmt.Sprintf("## [%s] %s\n\n", emptyMemoryValue(string(result.Type)), result.Title))
		var meta []string
		if result.TopicKey != "" {
			meta = append(meta, "topic="+result.TopicKey)
		}
		if result.AgentID != "" {
			meta = append(meta, "agent="+result.AgentID)
		}
		if len(meta) > 0 {
			b.WriteString("_" + strings.Join(meta, " | ") + "_\n\n")
		}
		b.WriteString(strings.TrimSpace(result.Content))
		b.WriteString("\n\n")
	}
	return b.String()
}

func injectMemoryContext(prompt, contextPack string) string {
	contextPack = strings.TrimSpace(contextPack)
	prompt = strings.TrimSpace(prompt)
	if contextPack == "" {
		return prompt
	}
	if prompt == "" {
		return contextPack
	}
	return contextPack + "\n---\n\n# User Prompt\n\n" + prompt
}

func emptyMemoryValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
