package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthorizeAddsConfiguredBearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	New("http://localhost:8000", "admin-token").Authorize(req)
	if got := req.Header.Get("Authorization"); got != "Bearer admin-token" {
		t.Fatalf("authorization = %q", got)
	}
}

func TestAuthorizeLeavesHeaderEmptyWithoutToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	New("http://localhost:8000", "").Authorize(req)
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("authorization = %q", got)
	}
}
