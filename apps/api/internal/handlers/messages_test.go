package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type fakeMessageStore struct {
	messages  []store.AgentMessage
	createErr error
}

func (f *fakeMessageStore) CreateAgentMessage(_ context.Context, arg store.CreateAgentMessageParams) (store.AgentMessage, error) {
	if f.createErr != nil {
		return store.AgentMessage{}, f.createErr
	}
	m := store.AgentMessage{
		ID:          "msg-" + strconv.Itoa(len(f.messages)+1),
		ProjectID:   arg.ProjectID,
		FromAgentID: arg.FromAgentID,
		ToAgentID:   arg.ToAgentID,
		RunID:       arg.RunID,
		Subject:     arg.Subject,
		Body:        arg.Body,
		CreatedAt:   time.Now(),
	}
	f.messages = append(f.messages, m)
	return m, nil
}

func (f *fakeMessageStore) ListInboxForAgent(_ context.Context, arg store.ListInboxForAgentParams) ([]store.AgentMessage, error) {
	var out []store.AgentMessage
	for _, m := range f.messages {
		if m.ToAgentID == arg.ToAgentID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeMessageStore) ListUnreadInboxForAgent(_ context.Context, agentID string) ([]store.AgentMessage, error) {
	var out []store.AgentMessage
	for _, m := range f.messages {
		if m.ToAgentID == agentID && !m.ReadAt.Valid {
			out = append(out, m)
		}
	}
	return out, nil
}

func (f *fakeMessageStore) MarkAgentMessageRead(_ context.Context, id string) (store.AgentMessage, error) {
	for i := range f.messages {
		if f.messages[i].ID == id {
			f.messages[i].ReadAt = sql.NullTime{Time: time.Now(), Valid: true}
			return f.messages[i], nil
		}
	}
	return store.AgentMessage{}, sql.ErrNoRows
}

func msgRequest(method, target, id, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestSendMessageRequiresToAndBody(t *testing.T) {
	h := NewMessagesHandler(&fakeMessageStore{})
	req := httptest.NewRequest(http.MethodPost, "/agent-messages", strings.NewReader(`{"body":"hola"}`))
	rec := httptest.NewRecorder()
	h.SendMessage(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (falta to_agent_id); body=%s", rec.Code, rec.Body.String())
	}
}

func TestSendAndListInbox(t *testing.T) {
	store := &fakeMessageStore{}
	h := NewMessagesHandler(store)

	for _, body := range []string{"primer encargo", "segundo encargo"} {
		req := httptest.NewRequest(http.MethodPost, "/agent-messages",
			strings.NewReader(`{"from_agent_id":"lead","to_agent_id":"worker","subject":"tarea","body":"`+body+`"}`))
		rec := httptest.NewRecorder()
		h.SendMessage(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("SendMessage status=%d, want 201; body=%s", rec.Code, rec.Body.String())
		}
	}

	// Inbox del worker lista los 2.
	req := msgRequest(http.MethodGet, "/agents/worker/messages", "worker", "")
	rec := httptest.NewRecorder()
	h.ListInbox(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ListInbox status=%d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "primer encargo") || !strings.Contains(bodyStr, "segundo encargo") {
		t.Fatalf("inbox no contiene los mensajes: %s", bodyStr)
	}

	// El lead no tiene inbox (los mensajes son dirigidos).
	reqLead := msgRequest(http.MethodGet, "/agents/lead/messages", "lead", "")
	recLead := httptest.NewRecorder()
	h.ListInbox(recLead, reqLead)
	if strings.Contains(recLead.Body.String(), "encargo") {
		t.Fatalf("el lead no deberia tener mensajes dirigidos a worker: %s", recLead.Body.String())
	}
}

func TestMarkReadDropsFromUnread(t *testing.T) {
	st := &fakeMessageStore{}
	h := NewMessagesHandler(st)

	send := httptest.NewRequest(http.MethodPost, "/agent-messages",
		strings.NewReader(`{"to_agent_id":"worker","body":"leeme"}`))
	sendRec := httptest.NewRecorder()
	h.SendMessage(sendRec, send)
	if sendRec.Code != http.StatusCreated {
		t.Fatalf("send status=%d", sendRec.Code)
	}
	id := st.messages[0].ID

	// Marcar leído.
	mr := msgRequest(http.MethodPost, "/agent-messages/"+id+"/read", id, "")
	mrRec := httptest.NewRecorder()
	h.MarkRead(mrRec, mr)
	if mrRec.Code != http.StatusOK {
		t.Fatalf("MarkRead status=%d; body=%s", mrRec.Code, mrRec.Body.String())
	}

	// Unread ahora vacío.
	un := msgRequest(http.MethodGet, "/agents/worker/messages?unread=true", "worker", "")
	// query param unread no viaja por msgRequest (usa el target), seteo a mano:
	un.URL.RawQuery = "unread=true"
	unRec := httptest.NewRecorder()
	h.ListInbox(unRec, un)
	if strings.Contains(unRec.Body.String(), "leeme") {
		t.Fatalf("el mensaje leido no deberia aparecer en unread: %s", unRec.Body.String())
	}
}
