package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicotion/battos/apps/api/internal/store"
)

// MessageStore es el subconjunto del store que el mailbox de inter-comunicación
// multi-agente necesita (Fase B).
type MessageStore interface {
	CreateAgentMessage(context.Context, store.CreateAgentMessageParams) (store.AgentMessage, error)
	ListInboxForAgent(context.Context, store.ListInboxForAgentParams) ([]store.AgentMessage, error)
	ListUnreadInboxForAgent(context.Context, string) ([]store.AgentMessage, error)
	MarkAgentMessageRead(context.Context, string) (store.AgentMessage, error)
}

type MessagesHandler struct {
	store MessageStore
}

func NewMessagesHandler(s MessageStore) *MessagesHandler {
	return &MessagesHandler{store: s}
}

type sendMessageInput struct {
	ProjectID   string `json:"project_id"`
	FromAgentID string `json:"from_agent_id"`
	ToAgentID   string `json:"to_agent_id"`
	RunID       string `json:"run_id"`
	Subject     string `json:"subject"`
	Body        string `json:"body"`
}

type agentMessageDTO struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id,omitempty"`
	FromAgentID string `json:"from_agent_id,omitempty"`
	ToAgentID   string `json:"to_agent_id"`
	RunID       string `json:"run_id,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Body        string `json:"body"`
	Read        bool   `json:"read"`
	ReadAt      string `json:"read_at,omitempty"`
	CreatedAt   string `json:"created_at"`
}

func agentMessageToDTO(m store.AgentMessage) agentMessageDTO {
	dto := agentMessageDTO{
		ID:          m.ID,
		ProjectID:   textValue(m.ProjectID),
		FromAgentID: textValue(m.FromAgentID),
		ToAgentID:   m.ToAgentID,
		RunID:       textValue(m.RunID),
		Subject:     textValue(m.Subject),
		Body:        m.Body,
		Read:        m.ReadAt.Valid,
		CreatedAt:   m.CreatedAt.Format(time.RFC3339),
	}
	if m.ReadAt.Valid {
		dto.ReadAt = m.ReadAt.Time.Format(time.RFC3339)
	}
	return dto
}

// SendMessage — POST /agent-messages. Un agente deja un mensaje en el inbox de otro.
func (h *MessagesHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	var in sendMessageInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "JSON invalido", "code": 400}})
		return
	}
	if strings.TrimSpace(in.ToAgentID) == "" || strings.TrimSpace(in.Body) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "to_agent_id y body son requeridos", "code": 400}})
		return
	}
	msg, err := h.store.CreateAgentMessage(r.Context(), store.CreateAgentMessageParams{
		ProjectID:   nullableText(in.ProjectID),
		FromAgentID: nullableText(in.FromAgentID),
		ToAgentID:   strings.TrimSpace(in.ToAgentID),
		RunID:       nullableText(in.RunID),
		Subject:     nullableText(in.Subject),
		Body:        in.Body,
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"message": agentMessageToDTO(msg)})
}

// ListInbox — GET /agents/{id}/messages?unread=true&limit=N. Inbox de un agente.
func (h *MessagesHandler) ListInbox(w http.ResponseWriter, r *http.Request) {
	agentID := strings.TrimSpace(chi.URLParam(r, "id"))
	if agentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "id de agente requerido", "code": 400}})
		return
	}

	var msgs []store.AgentMessage
	var err error
	if r.URL.Query().Get("unread") == "true" {
		msgs, err = h.store.ListUnreadInboxForAgent(r.Context(), agentID)
	} else {
		limit := int64(50)
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, e := strconv.ParseInt(l, 10, 64); e == nil && n > 0 {
				limit = n
			}
		}
		msgs, err = h.store.ListInboxForAgent(r.Context(), store.ListInboxForAgentParams{ToAgentID: agentID, Limit: limit})
	}
	if err != nil {
		writeWorkError(w, err)
		return
	}

	dtos := make([]agentMessageDTO, 0, len(msgs))
	for _, m := range msgs {
		dtos = append(dtos, agentMessageToDTO(m))
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": dtos})
}

// MarkRead — POST /agent-messages/{id}/read.
func (h *MessagesHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "id de mensaje requerido", "code": 400}})
		return
	}
	msg, err := h.store.MarkAgentMessageRead(r.Context(), id)
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": agentMessageToDTO(msg)})
}
