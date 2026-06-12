package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicotion/battos/apps/api/internal/config"
	"github.com/nicotion/battos/apps/api/internal/memory"
	"github.com/nicotion/battos/apps/api/internal/novacore"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type NovaStore interface {
	CreateNovaConversation(context.Context, sql.NullString) (store.NovacoreConversation, error)
	GetNovaConversation(context.Context, string) (store.NovacoreConversation, error)
	ListNovaConversations(context.Context) ([]store.NovacoreConversation, error)
	CreateNovaMessage(context.Context, store.CreateNovaMessageParams) (store.NovacoreMessage, error)
	ListNovaMessagesByConversation(context.Context, string) ([]store.NovacoreMessage, error)
	UpdateNovaConversationStats(context.Context, store.UpdateNovaConversationStatsParams) (store.NovacoreConversation, error)

	ListProjects(context.Context) ([]store.Project, error)
	ListAgents(context.Context) ([]store.Agent, error)
	ListSkills(context.Context) ([]store.Skill, error)
	ListAgentRuntimes(context.Context) ([]store.AgentRuntime, error)
	ListProviders(context.Context) ([]store.Provider, error)

	// Work Board — usado por propose_runs
	ListTasks(context.Context) ([]store.Task, error)

	// Runs — usado por launch_run
	CreateRun(context.Context, store.CreateRunParams) (store.Run, error)
	GetRun(context.Context, string) (store.Run, error)
}

type NovaCoreHandler struct {
	store  NovaStore
	memory memory.MemoryProvider
	cfg    *config.Config
}

func NewNovaCoreHandler(q NovaStore, mem memory.MemoryProvider, cfg *config.Config) *NovaCoreHandler {
	return &NovaCoreHandler{
		store:  q,
		memory: mem,
		cfg:    cfg,
	}
}

// toolCallRE captura bloques <tool:nombre>{...json...}</tool:nombre> en la
// respuesta del LLM. El cuerpo puede contener saltos de línea.
// Nota: Go regexp no soporta backreferences, por lo que la validación de que
// el tag de cierre coincida con el de apertura se hace manualmente en parseToolCall.
var toolCallRE = regexp.MustCompile(`(?s)<tool:(\w+)>(.*?)</tool:\w+>`)

// proposeRunsInput es el JSON que el LLM pasa dentro de <tool:propose_runs>.
type proposeRunsInput struct {
	Goal     string   `json:"goal"`
	Runtimes []string `json:"runtimes"`
}

// launchRunInput es el JSON que el LLM pasa dentro de <tool:launch_run>.
type launchRunInput struct {
	RuntimeID     string `json:"runtime_id"`
	ExecutionMode string `json:"execution_mode"`
	Prompt        string `json:"prompt"`
	ParentRunID   string `json:"parent_run_id"`
}

type novaChatRequest struct {
	ConversationID string `json:"conversation_id,omitempty"`
	Content        string `json:"content"`
}

type novaChatResponse struct {
	ConversationID string `json:"conversation_id"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	TokensIn       int    `json:"tokens_in"`
	TokensOut      int    `json:"tokens_out"`
}

type novacoreConversationResponse struct {
	ID                 string     `json:"id"`
	UserID             string     `json:"user_id,omitempty"`
	StartedAt          time.Time  `json:"started_at"`
	EndedAt            *time.Time `json:"ended_at,omitempty"`
	MessageCount       int64      `json:"message_count"`
	TotalInputTokens   int64      `json:"total_input_tokens"`
	TotalOutputTokens  int64      `json:"total_output_tokens"`
	TotalCostUSD       float64    `json:"total_cost_usd"`
}

type novacoreMessageResponse struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	TokensIn       int64     `json:"tokens_in"`
	TokensOut      int64     `json:"tokens_out"`
	CreatedAt      time.Time `json:"created_at"`
}

func (h *NovaCoreHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListNovaConversations(r.Context())
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]novacoreConversationResponse, 0, len(items))
	for _, item := range items {
		out = append(out, novacoreConversationDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *NovaCoreHandler) GetConversationMessages(w http.ResponseWriter, r *http.Request) {
	convIDStr := chi.URLParam(r, "id")
	convID, ok := parseIDInput(w, convIDStr, "id")
	if !ok {
		return
	}

	// Verificar si la conversacion existe
	_, errConv := h.store.GetNovaConversation(r.Context(), convID)
	if errConv != nil {
		if errors.Is(errConv, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "conversacion no encontrada", "code": 404}})
			return
		}
		writeWorkError(w, errConv)
		return
	}

	messages, err := h.store.ListNovaMessagesByConversation(r.Context(), convID)
	if err != nil {
		writeWorkError(w, err)
		return
	}

	out := make([]novacoreMessageResponse, 0, len(messages))
	for _, msg := range messages {
		out = append(out, novacoreMessageDTO(msg))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *NovaCoreHandler) Chat(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.NovaCore.Enabled {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "NovaCore esta deshabilitado en la configuracion", "code": 400}})
		return
	}

	var in novaChatRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "JSON invalido: " + err.Error(), "code": 400}})
		return
	}
	in.Content = strings.TrimSpace(in.Content)
	if in.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "content es obligatorio", "code": 400}})
		return
	}

	var convID string
	if in.ConversationID == "" {
		conv, errCreate := h.store.CreateNovaConversation(r.Context(), sql.NullString{})
		if errCreate != nil {
			writeWorkError(w, errCreate)
			return
		}
		convID = conv.ID
	} else {
		var ok bool
		convID, ok = parseIDInput(w, in.ConversationID, "conversation_id")
		if !ok {
			return
		}
		// Verificar existencia
		_, errGet := h.store.GetNovaConversation(r.Context(), convID)
		if errGet != nil {
			if errors.Is(errGet, sql.ErrNoRows) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "conversacion no encontrada", "code": 404}})
				return
			}
			writeWorkError(w, errGet)
			return
		}
	}

	// 1. Guardar mensaje del usuario
	_, errUserMsg := h.store.CreateNovaMessage(r.Context(), store.CreateNovaMessageParams{
		ConversationID: convID,
		Role:           "user",
		Content:        sql.NullString{String: in.Content, Valid: true},
		ToolCalls:      sql.NullString{String: "[]", Valid: true},
		ToolResult:     sql.NullString{String: "{}", Valid: true},
		TokensIn:       0,
		TokensOut:      0,
	})
	if errUserMsg != nil {
		writeWorkError(w, errUserMsg)
		return
	}

	// 2. Traer historial de mensajes
	history, errHist := h.store.ListNovaMessagesByConversation(r.Context(), convID)
	if errHist != nil {
		writeWorkError(w, errHist)
		return
	}

	var llmHistory []novacore.Message
	for _, m := range history {
		if m.Content.Valid && m.Content.String != "" {
			llmHistory = append(llmHistory, novacore.Message{
				Role:    m.Role,
				Content: m.Content.String,
			})
		}
	}

	// 3. Generar System Prompt dinamico con snapshot del OS
	systemPrompt := h.buildSystemPrompt(r.Context())

	// 4. Invocar al LLM
	client := novacore.NewLLMClient(h.cfg.NovaCore.Provider, h.cfg.NovaCore.Model)
	client.BaseURL = h.cfg.NovaCore.BaseURL
	client.APIKeyEnv = h.cfg.NovaCore.APIKeyEnv
	responseStr, usage, errLLM := client.Generate(r.Context(), systemPrompt, llmHistory)
	if errLLM != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "Error al invocar el LLM: " + errLLM.Error(), "code": 500}})
		return
	}

	// 5. Procesar tool calls embebidos en la respuesta del LLM.
	// Si hay tool calls, ejecutarlas y reemplazar los bloques por el resultado.
	processedResponse, toolCallsJSON := h.processToolCalls(r.Context(), responseStr)

	// 6. Guardar respuesta del asistente (con resultado de tools si aplica)
	_, errAssistMsg := h.store.CreateNovaMessage(r.Context(), store.CreateNovaMessageParams{
		ConversationID: convID,
		Role:           "assistant",
		Content:        sql.NullString{String: processedResponse, Valid: true},
		ToolCalls:      sql.NullString{String: toolCallsJSON, Valid: true},
		ToolResult:     sql.NullString{String: "{}", Valid: true},
		TokensIn:       int64(usage.InputTokens),
		TokensOut:      int64(usage.OutputTokens),
	})
	if errAssistMsg != nil {
		writeWorkError(w, errAssistMsg)
		return
	}

	// 7. Actualizar estadisticas de la conversacion (message_count y tokens)
	// Para el costo, guardamos tokens y lo multiplicamos por la estimacion en decimal
	_, errStats := h.store.UpdateNovaConversationStats(r.Context(), store.UpdateNovaConversationStatsParams{
		ID:                convID,
		TotalInputTokens:  int64(usage.InputTokens),
		TotalOutputTokens: int64(usage.OutputTokens),
		TotalCostUsd:      usage.CostUSD,
	})
	if errStats != nil {
		// Logueamos pero no fallamos el request si ya guardamos el mensaje
		_ = errStats
	}

	writeJSON(w, http.StatusOK, novaChatResponse{
		ConversationID: convID,
		Role:           "assistant",
		Content:        processedResponse,
		TokensIn:       usage.InputTokens,
		TokensOut:      usage.OutputTokens,
	})
}

func (h *NovaCoreHandler) buildSystemPrompt(ctx context.Context) string {
	var b strings.Builder
	b.WriteString(`Sos NovaCore, el asistente de sistema y orquestador conversacional meta de BattOS.
Tu rol es guiar al usuario, explicar conceptos (MCPs, Skills, Runtimes, etc.), diagnosticar problemas y sugerir comandos de CLI.

Reglas inquebrantables:
1. NUNCA inventes recursos que no existen. Trabaja con el estado real del sistema proporcionado a continuación.
2. Si sugieres una acción mutante (crear proyecto, agente, skill o ejecutar un run), muestra siempre el comando exacto del CLI (ej: "battos project create ...", "battos run propose ...") y aclara que requiere confirmación.
3. Responde de manera clara, concisa y profesional en el mismo idioma que el usuario te hable.

`)

	b.WriteString("## Estado Actual de BattOS\n\n")

	// 1. Proyectos
	if projs, err := h.store.ListProjects(ctx); err == nil {
		b.WriteString("### Proyectos:\n")
		if len(projs) == 0 {
			b.WriteString("- (Sin proyectos creados. Sugiérele al usuario iniciar uno usando 'battos project create')\n")
		} else {
			for _, p := range projs {
				desc := ""
				if p.Description.Valid {
					desc = " - " + p.Description.String
				}
				b.WriteString(fmt.Sprintf("- `%s`: %s (Estado: %s)%s\n", p.ID, p.Name, p.Status, desc))
			}
		}
		b.WriteString("\n")
	}

	// 2. Agentes
	if agents, err := h.store.ListAgents(ctx); err == nil {
		b.WriteString("### Agentes:\n")
		if len(agents) == 0 {
			b.WriteString("- (Sin agentes registrados)\n")
		} else {
			for _, a := range agents {
				b.WriteString(fmt.Sprintf("- `%s`: %s (Role: %s, Runtime: %s, Lead: %t)\n", a.ID, a.Name, textValue(a.Role), textValue(a.RuntimeID), a.IsLead != 0))
			}
		}
		b.WriteString("\n")
	}

	// 3. Skills
	if skills, err := h.store.ListSkills(ctx); err == nil {
		b.WriteString("### Skills:\n")
		if len(skills) == 0 {
			b.WriteString("- (Sin skills instaladas)\n")
		} else {
			for _, s := range skills {
				b.WriteString(fmt.Sprintf("- `%s`: %s (Categoría: %s)\n", s.ID, s.Name, s.Category.String))
			}
		}
		b.WriteString("\n")
	}

	// 4. Runtimes
	if runtimes, err := h.store.ListAgentRuntimes(ctx); err == nil {
		b.WriteString("### Runtimes de Agente:\n")
		for _, r := range runtimes {
			b.WriteString(fmt.Sprintf("- `%s`: %s (Tipo: %s, Estado: %s)\n", r.ID, r.Name, r.Kind, r.Status))
		}
		b.WriteString("\n")
	}

	// 5. Providers
	if providers, err := h.store.ListProviders(ctx); err == nil {
		b.WriteString("### Proveedores LLM:\n")
		for _, p := range providers {
			b.WriteString(fmt.Sprintf("- `%s`: %s (Estado: %s)\n", p.ID, p.Name, p.Status))
		}
		b.WriteString("\n")
	}

	// 6. Memory Stats
	if h.memory != nil {
		if stats, err := h.memory.Stats(ctx); err == nil {
			b.WriteString("### Estadísticas de Memoria (SQLite):\n")
			b.WriteString(fmt.Sprintf("- Total observaciones: %d\n", stats.TotalItems))
			b.WriteString(fmt.Sprintf("- Observaciones últimas 24h: %d\n", stats.ItemsLast24h))
			b.WriteString("\n")
		}
	}

	// 7. Work Board — snapshot de tasks para propose_runs
	if tasks, err := h.store.ListTasks(ctx); err == nil && len(tasks) > 0 {
		b.WriteString("### Work Board (Tasks):\n")
		for _, t := range tasks {
			desc := ""
			if t.Description.Valid {
				desc = " — " + t.Description.String
			}
			b.WriteString(fmt.Sprintf("- `%s` [%s] %s (proyecto: %s)%s\n", t.ID, t.Status, t.Title, t.ProjectID, desc))
		}
		b.WriteString("\n")
	}

	// 8. Instrucciones de tools disponibles para Nova
	b.WriteString(`## Tools Disponibles

Podés invocar estas tools embebiendo un bloque con la siguiente sintaxis en tu respuesta.
El sistema ejecutará la tool y reemplazará el bloque por el resultado antes de enviarlo al usuario.

### Tool: propose_runs
Propone un plan de runs para un objetivo, basándote en los agentes y runtimes disponibles del sistema.
Usá esta tool cuando el usuario quiera planificar la ejecución de un agente o runtime.

Sintaxis:
<tool:propose_runs>
{"goal": "descripción del objetivo", "runtimes": ["claude-code", "codex"]}
</tool:propose_runs>

### Tool: launch_run
Crea un run en estado awaiting_approval. El run queda pendiente de aprobación humana antes de ejecutarse.
Usá esta tool cuando el usuario confirme que quiere lanzar un run específico.

Sintaxis:
<tool:launch_run>
{"runtime_id": "id-del-runtime", "execution_mode": "sandbox", "prompt": "instrucción exacta para el agente", "parent_run_id": ""}
</tool:launch_run>

Reglas para usar tools:
1. NUNCA uses una tool sin que el usuario haya expresado intención clara de ejecutar o planificar.
2. Para propose_runs: primero describí el plan en texto, LUEGO incluí el bloque tool.
3. Para launch_run: confirmá siempre con el usuario antes de incluir el bloque. El run nace en awaiting_approval — el usuario debe aprobarlo por separado.
4. Si el sistema no tiene agentes o runtimes registrados, informá al usuario en lugar de invocar la tool.

`)

	return b.String()
}

// processToolCalls detecta bloques <tool:nombre>{...}</tool:nombre> en la
// respuesta del LLM, ejecuta la acción correspondiente y devuelve:
// - la respuesta con los bloques reemplazados por el resultado de la tool
// - un JSON array con los tool calls ejecutados (para persistir en ToolCalls)
func (h *NovaCoreHandler) processToolCalls(ctx context.Context, response string) (string, string) {
	type toolCallRecord struct {
		Name   string `json:"name"`
		Input  string `json:"input"`
		Result string `json:"result"`
	}

	var records []toolCallRecord

	processed := toolCallRE.ReplaceAllStringFunc(response, func(match string) string {
		sub := toolCallRE.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		toolName := sub[1]
		rawJSON := strings.TrimSpace(sub[2])

		var result string
		switch toolName {
		case "propose_runs":
			result = h.executeProposesRuns(ctx, rawJSON)
		case "launch_run":
			result = h.executeLaunchRun(ctx, rawJSON)
		default:
			result = fmt.Sprintf("⚠️ Tool desconocida: `%s`", toolName)
		}

		records = append(records, toolCallRecord{Name: toolName, Input: rawJSON, Result: result})
		return "\n\n" + result + "\n"
	})

	callsJSON := "[]"
	if len(records) > 0 {
		b, err := json.Marshal(records)
		if err == nil {
			callsJSON = string(b)
		}
	}

	return processed, callsJSON
}

// executeProposesRuns lee el Work Board y genera un plan de runs en Markdown.
func (h *NovaCoreHandler) executeProposesRuns(ctx context.Context, rawJSON string) string {
	var in proposeRunsInput
	if err := json.Unmarshal([]byte(rawJSON), &in); err != nil {
		return fmt.Sprintf("⚠️ propose_runs: JSON inválido — %v", err)
	}
	if in.Goal == "" {
		return "⚠️ propose_runs: `goal` es obligatorio."
	}

	agents, _ := h.store.ListAgents(ctx)
	runtimes, _ := h.store.ListAgentRuntimes(ctx)
	tasks, _ := h.store.ListTasks(ctx)
	projects, _ := h.store.ListProjects(ctx)

	// Mapa runtime_id → runtime para lookup rápido
	rtMap := make(map[string]store.AgentRuntime, len(runtimes))
	for _, r := range runtimes {
		rtMap[r.ID] = r
	}

	// Filtrar agentes que tengan alguno de los runtimes pedidos
	var matched []store.Agent
	requestedSet := make(map[string]bool, len(in.Runtimes))
	for _, rt := range in.Runtimes {
		requestedSet[strings.ToLower(rt)] = true
	}

	for _, a := range agents {
		rtID := strings.ToLower(textValue(a.RuntimeID))
		if len(requestedSet) == 0 || requestedSet[rtID] {
			matched = append(matched, a)
		}
	}

	// Buscar tareas pendientes relevantes para contexto
	var pendingTasks []store.Task
	for _, t := range tasks {
		if t.Status == "todo" || t.Status == "in_progress" {
			pendingTasks = append(pendingTasks, t)
		}
	}

	// Primer proyecto disponible como fallback
	firstProjectID := "inbox"
	if len(projects) > 0 {
		firstProjectID = projects[0].ID
	}

	var sb strings.Builder
	sb.WriteString("## Plan de Runs propuesto\n\n")
	sb.WriteString(fmt.Sprintf("**Objetivo:** %s\n\n", in.Goal))

	if len(matched) == 0 {
		sb.WriteString("> ⚠️ No se encontraron agentes con los runtimes solicitados (`")
		sb.WriteString(strings.Join(in.Runtimes, "`, `"))
		sb.WriteString("`). Registrá un agente antes de lanzar runs.\n")
		return sb.String()
	}

	sb.WriteString("### Runs sugeridos\n\n")
	for i, a := range matched {
		rt, hasRT := rtMap[textValue(a.RuntimeID)]
		rtName := textValue(a.RuntimeID)
		if hasRT {
			rtName = rt.Name
		}

		// Primer task pendiente como task_id del run
		taskID := ""
		if len(pendingTasks) > 0 {
			taskID = pendingTasks[0].ID
		}

		sb.WriteString(fmt.Sprintf("#### Run %d — Agente `%s` / Runtime `%s`\n", i+1, a.Name, rtName))
		sb.WriteString(fmt.Sprintf("- **Runtime ID:** `%s`\n", textValue(a.RuntimeID)))
		sb.WriteString(fmt.Sprintf("- **Execution mode:** `sandbox`\n"))
		sb.WriteString(fmt.Sprintf("- **Prompt sugerido:** `%s`\n", in.Goal))
		if taskID != "" {
			sb.WriteString(fmt.Sprintf("- **Task ID:** `%s`\n", taskID))
		}
		sb.WriteString(fmt.Sprintf("- **Project ID:** `%s`\n", firstProjectID))
		sb.WriteString("\n")
	}

	if len(pendingTasks) > 0 {
		sb.WriteString("### Contexto del Work Board\n\n")
		limit := len(pendingTasks)
		if limit > 5 {
			limit = 5
		}
		for _, t := range pendingTasks[:limit] {
			sb.WriteString(fmt.Sprintf("- `%s` [%s] %s\n", t.ID, t.Status, t.Title))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("> Para lanzar alguno de estos runs, confirmame cuál querés y ejecuto `launch_run`.\n")
	return sb.String()
}

// executeLaunchRun crea un run via el store con status awaiting_approval.
func (h *NovaCoreHandler) executeLaunchRun(ctx context.Context, rawJSON string) string {
	var in launchRunInput
	if err := json.Unmarshal([]byte(rawJSON), &in); err != nil {
		return fmt.Sprintf("⚠️ launch_run: JSON inválido — %v", err)
	}
	if in.RuntimeID == "" {
		return "⚠️ launch_run: `runtime_id` es obligatorio."
	}
	if in.Prompt == "" {
		return "⚠️ launch_run: `prompt` es obligatorio."
	}

	execMode := strings.ToLower(strings.TrimSpace(in.ExecutionMode))
	if execMode == "" {
		execMode = "sandbox"
	}
	if !validExecutionMode(execMode) {
		return fmt.Sprintf("⚠️ launch_run: execution_mode `%s` inválido (sandbox|direct|connected).", execMode)
	}

	// Obtener primer agente disponible (o buscar uno que use el runtime pedido)
	agents, errAgents := h.store.ListAgents(ctx)
	if errAgents != nil || len(agents) == 0 {
		return "⚠️ launch_run: no hay agentes registrados en el sistema. Creá uno antes de lanzar runs."
	}

	var targetAgent store.Agent
	found := false
	for _, a := range agents {
		if strings.EqualFold(textValue(a.RuntimeID), in.RuntimeID) {
			targetAgent = a
			found = true
			break
		}
	}
	if !found {
		targetAgent = agents[0]
	}

	// Obtener primer proyecto disponible
	projects, _ := h.store.ListProjects(ctx)
	projectID := "inbox"
	if len(projects) > 0 {
		projectID = projects[0].ID
	}

	// Obtener primera task disponible
	tasks, _ := h.store.ListTasks(ctx)
	taskID := "inbox"
	for _, t := range tasks {
		if t.Status == "todo" || t.Status == "in_progress" {
			taskID = t.ID
			break
		}
	}

	// Metadata con parent_run_id si se proveyó
	metadata := map[string]any{
		"created_by": "nova",
	}
	parentRunID := strings.TrimSpace(in.ParentRunID)
	if parentRunID != "" {
		if _, err := h.store.GetRun(ctx, parentRunID); err == nil {
			metadata["parent_run_id"] = parentRunID
		}
	}
	metaBytes, _ := json.Marshal(metadata)

	run, err := h.store.CreateRun(ctx, store.CreateRunParams{
		ProjectID:        projectID,
		TaskID:           taskID,
		AgentID:          targetAgent.ID,
		RuntimeAdapterID: in.RuntimeID,
		Prompt:           strings.TrimSpace(in.Prompt),
		RequestedNetwork: 0,
		ExecutionMode:    execMode,
		Metadata:         string(metaBytes),
	})
	if err != nil {
		return fmt.Sprintf("⚠️ launch_run: error creando el run — %v", err)
	}

	var sb strings.Builder
	sb.WriteString("## Run creado (awaiting_approval)\n\n")
	sb.WriteString(fmt.Sprintf("- **Run ID:** `%s`\n", run.ID))
	sb.WriteString(fmt.Sprintf("- **Agente:** `%s`\n", targetAgent.Name))
	sb.WriteString(fmt.Sprintf("- **Runtime:** `%s`\n", in.RuntimeID))
	sb.WriteString(fmt.Sprintf("- **Execution mode:** `%s`\n", run.ExecutionMode))
	sb.WriteString(fmt.Sprintf("- **Estado:** `%s`\n", run.Status))
	sb.WriteString(fmt.Sprintf("- **Prompt:** %s\n\n", run.Prompt))
	sb.WriteString("> El run está en `awaiting_approval`. Aprobalo desde el Command Center o con:\n")
	sb.WriteString(fmt.Sprintf("> `battos run approve %s --kind execute --decision approved`\n", run.ID))
	return sb.String()
}

func novacoreConversationDTO(item store.NovacoreConversation) novacoreConversationResponse {
	var ended *time.Time
	if item.EndedAt.Valid {
		ended = &item.EndedAt.Time
	}
	return novacoreConversationResponse{
		ID:                item.ID,
		UserID:            textValue(item.UserID),
		StartedAt:         item.StartedAt,
		EndedAt:           ended,
		MessageCount:      item.MessageCount,
		TotalInputTokens:  item.TotalInputTokens,
		TotalOutputTokens: item.TotalOutputTokens,
		TotalCostUSD:      item.TotalCostUsd,
	}
}

func novacoreMessageDTO(item store.NovacoreMessage) novacoreMessageResponse {
	return novacoreMessageResponse{
		ID:             item.ID,
		ConversationID: item.ConversationID,
		Role:           item.Role,
		Content:        textValue(item.Content),
		TokensIn:       item.TokensIn,
		TokensOut:      item.TokensOut,
		CreatedAt:      item.CreatedAt,
	}
}
