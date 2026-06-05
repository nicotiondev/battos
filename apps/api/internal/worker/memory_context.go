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
