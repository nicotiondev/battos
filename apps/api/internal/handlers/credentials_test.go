package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicotion/battos/apps/api/internal/credstore"
	"github.com/nicotion/battos/apps/api/internal/store"
)

// fakeCredentialStore es un stub en memoria para los tests.
type fakeCredentialStore struct {
	items []store.Credential
}

func (f *fakeCredentialStore) CreateCredential(_ context.Context, arg store.CreateCredentialParams) (store.Credential, error) {
	c := store.Credential{
		ID:            "test-id-" + arg.Name,
		Name:          arg.Name,
		Kind:          arg.Kind,
		ProviderID:    arg.ProviderID,
		SecretSource:  arg.SecretSource,
		SecretLocator: arg.SecretLocator,
		Description:   arg.Description,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	f.items = append(f.items, c)
	return c, nil
}

func (f *fakeCredentialStore) ListCredentials(_ context.Context) ([]store.Credential, error) {
	return f.items, nil
}

func (f *fakeCredentialStore) DeleteCredential(_ context.Context, name string) error {
	for i, c := range f.items {
		if c.Name == name {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return sql.ErrNoRows
}

// fakeCredReader implementa credstore.CredentialReader con cero registros
// (todos los lookups devuelven sql.ErrNoRows → fallback env).
type fakeCredReader struct{}

func (fakeCredReader) GetCredentialByName(_ context.Context, _ string) (store.Credential, error) {
	return store.Credential{}, sql.ErrNoRows
}

// newTestCredHandler construye un CredentialHandler sin master key
// (solo acepta secret_source=env en tests).
func newTestCredHandler(s *fakeCredentialStore) *CredentialHandler {
	resolver := credstore.New(fakeCredReader{})
	return NewCredentialHandler(s, resolver)
}

// newChiCtx construye una request con chi URLParam {name}.
func newChiCtx(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- Test 1: List con almacén vacío ---

func TestCredentialList_Empty(t *testing.T) {
	h := newTestCredHandler(&fakeCredentialStore{})
	req := httptest.NewRequest(http.MethodGet, "/credentials", nil)
	rec := httptest.NewRecorder()

	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var out []credentialResponse
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty list, got %d items", len(out))
	}
}

// --- Test 2: Create + List ---

func TestCredentialCreate_And_List(t *testing.T) {
	store := &fakeCredentialStore{}
	h := newTestCredHandler(store)

	body := `{"name":"MY_API_KEY","kind":"api_key","secret_source":"env","secret_value":"SOME_ENV_VAR"}`
	req := httptest.NewRequest(http.MethodPost, "/credentials", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d — body: %s", rec.Code, rec.Body.String())
	}
	var created credentialResponse
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Name != "MY_API_KEY" {
		t.Fatalf("expected name MY_API_KEY, got %q", created.Name)
	}
	if created.Kind != "api_key" {
		t.Fatalf("expected kind api_key, got %q", created.Kind)
	}
	// secret_locator NO debe aparecer en la respuesta
	raw := rec.Body.Bytes()
	if bytes.Contains(raw, []byte("secret_locator")) {
		t.Fatal("secret_locator must not appear in the response")
	}

	// Verificar que aparece en el listado
	req2 := httptest.NewRequest(http.MethodGet, "/credentials", nil)
	rec2 := httptest.NewRecorder()
	h.List(rec2, req2)

	var list []credentialResponse
	if err := json.NewDecoder(rec2.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 item in list, got %d", len(list))
	}
	if list[0].Name != "MY_API_KEY" {
		t.Fatalf("list[0].Name: got %q", list[0].Name)
	}
}

// --- Test 3: Delete ---

func TestCredentialDelete(t *testing.T) {
	fs := &fakeCredentialStore{}
	h := newTestCredHandler(fs)

	// Seed directo en el fake store
	fs.items = append(fs.items, store.Credential{
		ID:            "cred-abc",
		Name:          "MY_TOKEN",
		Kind:          "git_token",
		SecretSource:  "env",
		SecretLocator: "GITHUB_TOKEN",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	})

	req := httptest.NewRequest(http.MethodDelete, "/credentials/MY_TOKEN", nil)
	req = newChiCtx(req, "name", "MY_TOKEN")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d — body: %s", rec.Code, rec.Body.String())
	}
	if len(fs.items) != 0 {
		t.Fatalf("expected store to be empty after delete, got %d items", len(fs.items))
	}
}

// --- Test 4: Delete nombre inexistente → 404 ---

func TestCredentialDelete_NotFound(t *testing.T) {
	h := newTestCredHandler(&fakeCredentialStore{})

	req := httptest.NewRequest(http.MethodDelete, "/credentials/nope", nil)
	req = newChiCtx(req, "name", "nope")
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --- Test 5: Create con kind inválido ---

func TestCredentialCreate_InvalidKind(t *testing.T) {
	h := newTestCredHandler(&fakeCredentialStore{})
	body := `{"name":"foo","kind":"bad_kind","secret_source":"env","secret_value":"X"}`
	req := httptest.NewRequest(http.MethodPost, "/credentials", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
