package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestAuthMiddlewareTokenMode(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := authMiddleware("token", "correct-token")(next)

	for _, tc := range []struct {
		name   string
		header string
		want   int
	}{
		{name: "missing", want: http.StatusUnauthorized},
		{name: "wrong", header: "Bearer wrong-token", want: http.StatusUnauthorized},
		{name: "accepted", header: "Bearer correct-token", want: http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/status", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestAuthMiddlewareDisabledAllowsLocalDevelopment(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	authMiddleware("disabled", "")(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/status", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestUnavailableWorkRoutesReturnActionableServiceUnavailable(t *testing.T) {
	r := chi.NewRouter()
	mountUnavailableWorkRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/goals", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Work Board no disponible") {
		t.Fatalf("body=%s, want actionable Work Board message", rec.Body.String())
	}
}

func TestUnavailableKnowledgeRoutesReturnActionableServiceUnavailable(t *testing.T) {
	r := chi.NewRouter()
	mountUnavailableKnowledgeRoutes(r)

	req := httptest.NewRequest(http.MethodGet, "/knowledge/workspaces", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Knowledge Center no disponible") {
		t.Fatalf("body=%s, want actionable Knowledge Center message", rec.Body.String())
	}
}

func TestUnavailableRuntimeRoutesReturnActionableServiceUnavailable(t *testing.T) {
	r := chi.NewRouter()
	mountUnavailableRuntimeRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/runtime-adapters/detect", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Runtime detection no disponible") {
		t.Fatalf("body=%s, want actionable Runtime detection message", rec.Body.String())
	}
}
