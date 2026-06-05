package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/nicotion/battos/apps/cli/internal/client"
)

func TestAgentCreatePostsAgentInput(t *testing.T) {
	t.Setenv("BATTOS_NO_BANNER", "1")
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"builder-web","slug":"builder-web","name":"Builder Web","role":"web_builder","runtime_id":"codex","risk_level":"medium","is_lead":false,"is_meta":false,"status":"active"}`))
	}))
	defer srv.Close()

	cmd := NewAgentCmd(func() *client.Client { return client.New(srv.URL, "") })
	cmd.SetArgs([]string{"create", "builder-web", "--name", "Builder Web", "--runtime", "codex", "--role", "web_builder", "--system-prompt", "Build safely"})
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent create returned error: %v", err)
	}
	if gotPath != "/agents" {
		t.Fatalf("path = %q, want /agents", gotPath)
	}
	if gotBody["slug"] != "builder-web" || gotBody["name"] != "Builder Web" || gotBody["runtime_id"] != "codex" {
		t.Fatalf("body = %#v, want slug/name/runtime", gotBody)
	}
	if gotBody["system_prompt"] != "Build safely" {
		t.Fatalf("system_prompt = %#v, want Build safely", gotBody["system_prompt"])
	}
}

func TestAgentCreateRequiresNameAndRuntime(t *testing.T) {
	cmd := NewAgentCmd(func() *client.Client { return client.New("http://127.0.0.1:1", "") })
	cmd.SetArgs([]string{"create", "builder-web", "--name", "Builder Web"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("agent create returned nil, want validation error")
	}
	if got, want := err.Error(), "--name y --runtime son obligatorios"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestAgentListReadsAgents(t *testing.T) {
	t.Setenv("BATTOS_NO_BANNER", "1")
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"novacore","slug":"novacore","name":"NovaCore","runtime_id":"direct-api","risk_level":"medium","is_lead":true,"is_meta":true,"status":"active"}]`))
	}))
	defer srv.Close()

	cmd := NewAgentCmd(func() *client.Client { return client.New(srv.URL, "") })
	cmd.SetArgs([]string{"list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent list returned error: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/agents" {
		t.Fatalf("request = %s %s, want GET /agents", gotMethod, gotPath)
	}
}
