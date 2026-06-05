package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nicotion/battos/apps/api/internal/config"
	"github.com/nicotion/battos/apps/api/internal/memory"
	"github.com/nicotion/battos/apps/api/internal/novacore"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type NovaStore interface {
	CreateNovaConversation(context.Context, pgtype.Text) (store.NovacoreConversation, error)
	GetNovaConversation(context.Context, pgtype.UUID) (store.NovacoreConversation, error)
	ListNovaConversations(context.Context) ([]store.NovacoreConversation, error)
	CreateNovaMessage(context.Context, store.CreateNovaMessageParams) (store.NovacoreMessage, error)
	ListNovaMessagesByConversation(context.Context, pgtype.UUID) ([]store.NovacoreMessage, error)
	UpdateNovaConversationStats(context.Context, store.UpdateNovaConversationStatsParams) (store.NovacoreConversation, error)

	ListProjects(context.Context) ([]store.Project, error)
	ListAgents(context.Context) ([]store.Agent, error)
	ListSkills(context.Context) ([]store.Skill, error)
	ListAgentRuntimes(context.Context) ([]store.AgentRuntime, error)
	ListProviders(context.Context) ([]store.Provider, error)
}

type NovaCoreHandler struct {
	store  NovaStore
	memory *memory.Core
	cfg    *config.Config
}

func NewNovaCoreHandler(q NovaStore, mem *memory.Core, cfg *config.Config) *NovaCoreHandler {
	return &NovaCoreHandler{
		store:  q,
		memory: mem,
		cfg:    cfg,
	}
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
	MessageCount       int32      `json:"message_count"`
	TotalInputTokens   int32      `json:"total_input_tokens"`
	TotalOutputTokens  int32      `json:"total_output_tokens"`
	TotalCostUSD       float64    `json:"total_cost_usd"`
}

type novacoreMessageResponse struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	TokensIn       int32     `json:"tokens_in"`
	TokensOut      int32     `json:"tokens_out"`
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
	convID, ok := parseUUIDInput(w, convIDStr, "id")
	if !ok {
		return
	}

	// Verificar si la conversacion existe
	_, errConv := h.store.GetNovaConversation(r.Context(), convID)
	if errConv != nil {
		if errors.Is(errConv, pgx.ErrNoRows) {
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

	var convID pgtype.UUID
	if in.ConversationID == "" {
		conv, errCreate := h.store.CreateNovaConversation(r.Context(), pgtype.Text{Valid: false})
		if errCreate != nil {
			writeWorkError(w, errCreate)
			return
		}
		convID = conv.ID
	} else {
		var ok bool
		convID, ok = parseUUIDInput(w, in.ConversationID, "conversation_id")
		if !ok {
			return
		}
		// Verificar existencia
		_, errGet := h.store.GetNovaConversation(r.Context(), convID)
		if errGet != nil {
			if errors.Is(errGet, pgx.ErrNoRows) {
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
		Content:        pgtype.Text{String: in.Content, Valid: true},
		ToolCalls:      []byte("[]"),
		ToolResult:     []byte("{}"),
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
	responseStr, usage, errLLM := client.Generate(r.Context(), systemPrompt, llmHistory)
	if errLLM != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "Error al invocar el LLM: " + errLLM.Error(), "code": 500}})
		return
	}

	// 5. Guardar respuesta del asistente
	_, errAssistMsg := h.store.CreateNovaMessage(r.Context(), store.CreateNovaMessageParams{
		ConversationID: convID,
		Role:           "assistant",
		Content:        pgtype.Text{String: responseStr, Valid: true},
		ToolCalls:      []byte("[]"),
		ToolResult:     []byte("{}"),
		TokensIn:       int32(usage.InputTokens),
		TokensOut:      int32(usage.OutputTokens),
	})
	if errAssistMsg != nil {
		writeWorkError(w, errAssistMsg)
		return
	}

	// 6. Actualizar estadisticas de la conversacion (message_count y tokens)
	// Para el costo, guardamos tokens y lo multiplicamos por la estimacion en decimal
	costNumeric := pgtype.Numeric{}
	_ = costNumeric.Scan(fmt.Sprintf("%.6f", usage.CostUSD))
	
	_, errStats := h.store.UpdateNovaConversationStats(r.Context(), store.UpdateNovaConversationStatsParams{
		ID:                convID,
		TotalInputTokens:  int32(usage.InputTokens),
		TotalOutputTokens: int32(usage.OutputTokens),
		TotalCostUsd:      costNumeric,
	})
	if errStats != nil {
		// Logueamos pero no fallamos el request si ya guardamos el mensaje
		_ = errStats
	}

	writeJSON(w, http.StatusOK, novaChatResponse{
		ConversationID: uuidValue(convID),
		Role:           "assistant",
		Content:        responseStr,
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
				b.WriteString(fmt.Sprintf("- `%s`: %s (Role: %s, Runtime: %s, Lead: %t)\n", a.ID, a.Name, textValue(a.Role), textValue(a.RuntimeID), a.IsLead))
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

	return b.String()
}

func novacoreConversationDTO(item store.NovacoreConversation) novacoreConversationResponse {
	var ended *time.Time
	if item.EndedAt.Valid {
		ended = &item.EndedAt.Time
	}
	cost, _ := item.TotalCostUsd.Float64Value()
	return novacoreConversationResponse{
		ID:                uuidValue(item.ID),
		UserID:            textValue(item.UserID),
		StartedAt:         item.StartedAt.Time,
		EndedAt:           ended,
		MessageCount:      item.MessageCount,
		TotalInputTokens:  item.TotalInputTokens,
		TotalOutputTokens: item.TotalOutputTokens,
		TotalCostUSD:      cost.Float64,
	}
}

func novacoreMessageDTO(item store.NovacoreMessage) novacoreMessageResponse {
	return novacoreMessageResponse{
		ID:             uuidValue(item.ID),
		ConversationID: uuidValue(item.ConversationID),
		Role:           item.Role,
		Content:        textValue(item.Content),
		TokensIn:       item.TokensIn,
		TokensOut:      item.TokensOut,
		CreatedAt:      item.CreatedAt.Time,
	}
}
