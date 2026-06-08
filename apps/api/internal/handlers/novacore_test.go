package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicotion/battos/apps/api/internal/config"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type fakeNovaStore struct {
	CreateNovaConversationFn         func(context.Context, sql.NullString) (store.NovacoreConversation, error)
	GetNovaConversationFn            func(context.Context, string) (store.NovacoreConversation, error)
	ListNovaConversationsFn          func(context.Context) ([]store.NovacoreConversation, error)
	CreateNovaMessageFn              func(context.Context, store.CreateNovaMessageParams) (store.NovacoreMessage, error)
	ListNovaMessagesByConversationFn func(context.Context, string) ([]store.NovacoreMessage, error)
	UpdateNovaConversationStatsFn    func(context.Context, store.UpdateNovaConversationStatsParams) (store.NovacoreConversation, error)

	ListProjectsFn      func(context.Context) ([]store.Project, error)
	ListAgentsFn        func(context.Context) ([]store.Agent, error)
	ListSkillsFn        func(context.Context) ([]store.Skill, error)
	ListAgentRuntimesFn func(context.Context) ([]store.AgentRuntime, error)
	ListProvidersFn     func(context.Context) ([]store.Provider, error)
}

func (f *fakeNovaStore) CreateNovaConversation(ctx context.Context, userID sql.NullString) (store.NovacoreConversation, error) {
	if f.CreateNovaConversationFn != nil {
		return f.CreateNovaConversationFn(ctx, userID)
	}
	return store.NovacoreConversation{}, nil
}

func (f *fakeNovaStore) GetNovaConversation(ctx context.Context, id string) (store.NovacoreConversation, error) {
	if f.GetNovaConversationFn != nil {
		return f.GetNovaConversationFn(ctx, id)
	}
	return store.NovacoreConversation{}, nil
}

func (f *fakeNovaStore) ListNovaConversations(ctx context.Context) ([]store.NovacoreConversation, error) {
	if f.ListNovaConversationsFn != nil {
		return f.ListNovaConversationsFn(ctx)
	}
	return nil, nil
}

func (f *fakeNovaStore) CreateNovaMessage(ctx context.Context, params store.CreateNovaMessageParams) (store.NovacoreMessage, error) {
	if f.CreateNovaMessageFn != nil {
		return f.CreateNovaMessageFn(ctx, params)
	}
	return store.NovacoreMessage{}, nil
}

func (f *fakeNovaStore) ListNovaMessagesByConversation(ctx context.Context, id string) ([]store.NovacoreMessage, error) {
	if f.ListNovaMessagesByConversationFn != nil {
		return f.ListNovaMessagesByConversationFn(ctx, id)
	}
	return nil, nil
}

func (f *fakeNovaStore) UpdateNovaConversationStats(ctx context.Context, params store.UpdateNovaConversationStatsParams) (store.NovacoreConversation, error) {
	if f.UpdateNovaConversationStatsFn != nil {
		return f.UpdateNovaConversationStatsFn(ctx, params)
	}
	return store.NovacoreConversation{}, nil
}

func (f *fakeNovaStore) ListProjects(ctx context.Context) ([]store.Project, error) {
	if f.ListProjectsFn != nil {
		return f.ListProjectsFn(ctx)
	}
	return nil, nil
}

func (f *fakeNovaStore) ListAgents(ctx context.Context) ([]store.Agent, error) {
	if f.ListAgentsFn != nil {
		return f.ListAgentsFn(ctx)
	}
	return nil, nil
}

func (f *fakeNovaStore) ListSkills(ctx context.Context) ([]store.Skill, error) {
	if f.ListSkillsFn != nil {
		return f.ListSkillsFn(ctx)
	}
	return nil, nil
}

func (f *fakeNovaStore) ListAgentRuntimes(ctx context.Context) ([]store.AgentRuntime, error) {
	if f.ListAgentRuntimesFn != nil {
		return f.ListAgentRuntimesFn(ctx)
	}
	return nil, nil
}

func (f *fakeNovaStore) ListProviders(ctx context.Context) ([]store.Provider, error) {
	if f.ListProvidersFn != nil {
		return f.ListProvidersFn(ctx)
	}
	return nil, nil
}

func TestListConversations(t *testing.T) {
	convID := "01020304-0506-0708-090a-0b0c0d0e0f10"
	started := time.Now().Add(-10 * time.Minute)

	q := &fakeNovaStore{
		ListNovaConversationsFn: func(ctx context.Context) ([]store.NovacoreConversation, error) {
			return []store.NovacoreConversation{
				{
					ID:                convID,
					StartedAt:         started,
					MessageCount:      5,
					TotalInputTokens:  100,
					TotalOutputTokens: 200,
					TotalCostUsd:      0.00125,
				},
			}, nil
		},
	}

	h := NewNovaCoreHandler(q, nil, &config.Config{})
	req := httptest.NewRequest(http.MethodGet, "/novacore/conversations", nil)
	rec := httptest.NewRecorder()

	h.ListConversations(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var res []novacoreConversationResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(res) != 1 {
		t.Fatalf("len = %d, want 1", len(res))
	}
	if res[0].MessageCount != 5 {
		t.Errorf("MessageCount = %d, want 5", res[0].MessageCount)
	}
	if res[0].TotalCostUSD != 0.00125 {
		t.Errorf("TotalCostUSD = %f, want 0.00125", res[0].TotalCostUSD)
	}
}

func TestGetConversationMessages(t *testing.T) {
	convIDStr := "01020304-0506-0708-090a-0b0c0d0e0f10"
	started := time.Now().Add(-10 * time.Minute)

	t.Run("success", func(t *testing.T) {
		q := &fakeNovaStore{
			GetNovaConversationFn: func(ctx context.Context, id string) (store.NovacoreConversation, error) {
				return store.NovacoreConversation{
					ID:        id,
					StartedAt: started,
				}, nil
			},
			ListNovaMessagesByConversationFn: func(ctx context.Context, id string) ([]store.NovacoreMessage, error) {
				return []store.NovacoreMessage{
					{
						ID:             "msg-1",
						ConversationID: id,
						Role:           "user",
						Content:        sql.NullString{String: "Hola", Valid: true},
						CreatedAt:      started,
					},
					{
						ID:             "msg-2",
						ConversationID: id,
						Role:           "assistant",
						Content:        sql.NullString{String: "Mundo", Valid: true},
						CreatedAt:      started.Add(time.Second),
					},
				}, nil
			},
		}

		h := NewNovaCoreHandler(q, nil, &config.Config{})
		req := httptest.NewRequest(http.MethodGet, "/novacore/conversations/"+convIDStr+"/messages", nil)
		rec := httptest.NewRecorder()

		// Configurar chi.Router context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", convIDStr)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		h.GetConversationMessages(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var res []novacoreMessageResponse
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if len(res) != 2 {
			t.Fatalf("len = %d, want 2", len(res))
		}
		if res[0].Content != "Hola" || res[0].Role != "user" {
			t.Errorf("first message: %+v", res[0])
		}
		if res[1].Content != "Mundo" || res[1].Role != "assistant" {
			t.Errorf("second message: %+v", res[1])
		}
	})

	t.Run("not found", func(t *testing.T) {
		q := &fakeNovaStore{
			GetNovaConversationFn: func(ctx context.Context, id string) (store.NovacoreConversation, error) {
				return store.NovacoreConversation{}, sql.ErrNoRows
			},
		}

		h := NewNovaCoreHandler(q, nil, &config.Config{})
		req := httptest.NewRequest(http.MethodGet, "/novacore/conversations/"+convIDStr+"/messages", nil)
		rec := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", convIDStr)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		h.GetConversationMessages(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
		}
	})

	t.Run("text id", func(t *testing.T) {
		h := NewNovaCoreHandler(&fakeNovaStore{}, nil, &config.Config{})
		req := httptest.NewRequest(http.MethodGet, "/novacore/conversations/text-id/messages", nil)
		rec := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "text-id")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		h.GetConversationMessages(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})
}

func TestChatDisabled(t *testing.T) {
	cfg := &config.Config{
		NovaCore: config.NovaCoreConfig{
			Enabled: false,
		},
	}
	h := NewNovaCoreHandler(&fakeNovaStore{}, nil, cfg)
	req := httptest.NewRequest(http.MethodPost, "/novacore/chat", strings.NewReader(`{"content":"hola"}`))
	rec := httptest.NewRecorder()

	h.Chat(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "deshabilitado") {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestChatValidation(t *testing.T) {
	cfg := &config.Config{
		NovaCore: config.NovaCoreConfig{
			Enabled:  true,
			Provider: "anthropic",
			Model:    "claude-3-haiku-20240307",
		},
	}
	h := NewNovaCoreHandler(&fakeNovaStore{}, nil, cfg)

	t.Run("empty content", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/novacore/chat", strings.NewReader(`{"content":""}`))
		rec := httptest.NewRecorder()
		h.Chat(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/novacore/chat", strings.NewReader(`invalid json`))
		rec := httptest.NewRecorder()
		h.Chat(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})
}

func TestChatSuccessAnthropic(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "dummy-key")

	cfg := &config.Config{
		NovaCore: config.NovaCoreConfig{
			Enabled:  true,
			Provider: "anthropic",
			Model:    "claude-3-haiku-20240307",
		},
	}

	convID := "conv-1"
	msgID := "msg-1"

	var createdMessages []store.CreateNovaMessageParams
	var updatedStats []store.UpdateNovaConversationStatsParams

	q := &fakeNovaStore{
		CreateNovaConversationFn: func(ctx context.Context, userID sql.NullString) (store.NovacoreConversation, error) {
			return store.NovacoreConversation{
				ID: convID,
			}, nil
		},
		CreateNovaMessageFn: func(ctx context.Context, params store.CreateNovaMessageParams) (store.NovacoreMessage, error) {
			createdMessages = append(createdMessages, params)
			return store.NovacoreMessage{
				ID:             msgID,
				ConversationID: params.ConversationID,
				Role:           params.Role,
				Content:        params.Content,
			}, nil
		},
		ListNovaMessagesByConversationFn: func(ctx context.Context, id string) ([]store.NovacoreMessage, error) {
			// Devolver los mensajes guardados hasta ahora en el test
			var list []store.NovacoreMessage
			for _, m := range createdMessages {
				list = append(list, store.NovacoreMessage{
					ConversationID: id,
					Role:           m.Role,
					Content:        m.Content,
				})
			}
			return list, nil
		},
		UpdateNovaConversationStatsFn: func(ctx context.Context, params store.UpdateNovaConversationStatsParams) (store.NovacoreConversation, error) {
			updatedStats = append(updatedStats, params)
			return store.NovacoreConversation{ID: params.ID}, nil
		},
		ListProjectsFn: func(ctx context.Context) ([]store.Project, error) {
			return []store.Project{
				{
					ID:     "proj-1",
					Name:   "Mi Proyecto",
					Status: "active",
				},
			}, nil
		},
	}

	origTransport := http.DefaultTransport
	defer func() { http.DefaultTransport = origTransport }()

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.anthropic.com" {
			return nil, fmt.Errorf("unexpected host %s", req.URL.Host)
		}
		if req.Header.Get("x-api-key") != "dummy-key" {
			return nil, fmt.Errorf("missing or invalid x-api-key header")
		}

		// Leer el cuerpo para verificar que contenga el contexto
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		bodyStr := string(bodyBytes)
		if !strings.Contains(bodyStr, "Mi Proyecto") {
			return nil, fmt.Errorf("system prompt did not include projects context: %s", bodyStr)
		}

		respJSON := `{
			"content": [{"type": "text", "text": "Hola, soy NovaCore."}],
			"usage": {"input_tokens": 100, "output_tokens": 50}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(respJSON)),
			Header:     make(http.Header),
		}, nil
	})

	h := NewNovaCoreHandler(q, nil, cfg)
	req := httptest.NewRequest(http.MethodPost, "/novacore/chat", strings.NewReader(`{"content":"Hola asistente"}`))
	rec := httptest.NewRecorder()

	h.Chat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var res novaChatResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if res.Content != "Hola, soy NovaCore." {
		t.Errorf("content = %q, want 'Hola, soy NovaCore.'", res.Content)
	}

	// Verificar mensajes creados (1 del usuario y 1 del asistente)
	if len(createdMessages) != 2 {
		t.Fatalf("createdMessages len = %d, want 2", len(createdMessages))
	}
	if createdMessages[0].Role != "user" || createdMessages[0].Content.String != "Hola asistente" {
		t.Errorf("first message check failed: %+v", createdMessages[0])
	}
	if createdMessages[1].Role != "assistant" || createdMessages[1].Content.String != "Hola, soy NovaCore." {
		t.Errorf("second message check failed: %+v", createdMessages[1])
	}

	// Verificar estadísticas actualizadas
	if len(updatedStats) != 1 {
		t.Fatalf("updatedStats len = %d, want 1", len(updatedStats))
	}
	if updatedStats[0].TotalInputTokens != 100 || updatedStats[0].TotalOutputTokens != 50 {
		t.Errorf("updated stats check failed: %+v", updatedStats[0])
	}
}

func TestChatSuccessOpenAI(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "dummy-openai-key")

	cfg := &config.Config{
		NovaCore: config.NovaCoreConfig{
			Enabled:  true,
			Provider: "openai",
			Model:    "gpt-4o-mini",
		},
	}

	convID := "conv-1"
	msgID := "msg-1"

	var createdMessages []store.CreateNovaMessageParams

	q := &fakeNovaStore{
		CreateNovaConversationFn: func(ctx context.Context, userID sql.NullString) (store.NovacoreConversation, error) {
			return store.NovacoreConversation{ID: convID}, nil
		},
		CreateNovaMessageFn: func(ctx context.Context, params store.CreateNovaMessageParams) (store.NovacoreMessage, error) {
			createdMessages = append(createdMessages, params)
			return store.NovacoreMessage{
				ID:             msgID,
				ConversationID: params.ConversationID,
				Role:           params.Role,
				Content:        params.Content,
			}, nil
		},
		ListNovaMessagesByConversationFn: func(ctx context.Context, id string) ([]store.NovacoreMessage, error) {
			var list []store.NovacoreMessage
			for _, m := range createdMessages {
				list = append(list, store.NovacoreMessage{
					ConversationID: id,
					Role:           m.Role,
					Content:        m.Content,
				})
			}
			return list, nil
		},
		UpdateNovaConversationStatsFn: func(ctx context.Context, params store.UpdateNovaConversationStatsParams) (store.NovacoreConversation, error) {
			return store.NovacoreConversation{ID: params.ID}, nil
		},
	}

	origTransport := http.DefaultTransport
	defer func() { http.DefaultTransport = origTransport }()

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.openai.com" {
			return nil, fmt.Errorf("unexpected host %s", req.URL.Host)
		}
		if req.Header.Get("Authorization") != "Bearer dummy-openai-key" {
			return nil, fmt.Errorf("missing or invalid Authorization header")
		}

		respJSON := `{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "Hola desde OpenAI."
				}
			}],
			"usage": {"prompt_tokens": 80, "completion_tokens": 40}
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(respJSON)),
			Header:     make(http.Header),
		}, nil
	})

	h := NewNovaCoreHandler(q, nil, cfg)
	req := httptest.NewRequest(http.MethodPost, "/novacore/chat", strings.NewReader(`{"content":"Hola chatbot"}`))
	rec := httptest.NewRecorder()

	h.Chat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var res novaChatResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if res.Content != "Hola desde OpenAI." {
		t.Errorf("content = %q, want 'Hola desde OpenAI.'", res.Content)
	}
}
