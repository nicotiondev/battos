package worker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestClaudeCodeAdapterWantsMCP: el adapter claude-code declara soporte MCP en
// el plan y su script incluye la expansión condicional de BATTOS_MCP_CONFIG
// (forma --mcp-config=... porque el flag de claude es variádico y de otro modo
// se comería el prompt posicional).
func TestClaudeCodeAdapterWantsMCP(t *testing.T) {
	adapters := ApprovedDryRunAdapters()
	adapter, ok := adapters["claude-code"].(CommandAdapter)
	if !ok {
		t.Fatal("claude-code adapter is not a CommandAdapter")
	}
	if !adapter.SupportsMCP {
		t.Fatal("claude-code adapter should declare SupportsMCP")
	}
	script := strings.Join(adapter.BaseArgs, " ")
	if !strings.Contains(script, `${BATTOS_MCP_CONFIG:+--strict-mcp-config --mcp-config="$BATTOS_MCP_CONFIG"}`) {
		t.Fatalf("claude-code script missing conditional mcp-config expansion: %s", script)
	}

	plan, err := adapter.Plan(context.Background(), testRun("claude-code"))
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !plan.WantsMCP {
		t.Fatal("plan.WantsMCP = false, want true for claude-code")
	}
}

// TestProcessOneComposesTeamMCPConfig: con TeamMCP configurado y un adapter que
// quiere MCP, el worker compone el JSON del server battos (cli path + api url +
// run id) y lo deja en el plan que recibe el sandbox.
func TestProcessOneComposesTeamMCPConfig(t *testing.T) {
	run := testRun("claude-code")
	st := &fakeStore{run: run}
	sandbox := &fakeSandbox{result: Result{Summary: "done"}}
	plan := testPlan("claude-code")
	plan.WantsMCP = true
	w := New(st, sandbox, map[string]Adapter{
		"claude-code": fakeAdapter{plan: plan},
	})
	w.ArtifactsDir = t.TempDir()
	w.TeamMCP = &TeamMCPConfig{CLIPath: `C:\bin\battos.exe`, APIURL: "http://127.0.0.1:8000"}

	processed, err := w.ProcessOne(context.Background())
	if err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}
	if !processed {
		t.Fatal("processed = false")
	}
	if sandbox.plan.MCPConfigJSON == "" {
		t.Fatal("plan.MCPConfigJSON is empty, want battos mcp server config")
	}

	var cfg struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(sandbox.plan.MCPConfigJSON), &cfg); err != nil {
		t.Fatalf("MCPConfigJSON is not valid JSON: %v\n%s", err, sandbox.plan.MCPConfigJSON)
	}
	server, ok := cfg.MCPServers["battos"]
	if !ok {
		t.Fatalf("mcpServers missing battos entry: %s", sandbox.plan.MCPConfigJSON)
	}
	if server.Command != `C:\bin\battos.exe` {
		t.Errorf("command = %q, want battos CLI path", server.Command)
	}
	if len(server.Args) != 1 || server.Args[0] != "mcp" {
		t.Errorf("args = %v, want [mcp]", server.Args)
	}
	if server.Env["BATTOS_API_URL"] != "http://127.0.0.1:8000" {
		t.Errorf("env BATTOS_API_URL = %q", server.Env["BATTOS_API_URL"])
	}
	if server.Env["BATTOS_RUN_ID"] != run.ID {
		t.Errorf("env BATTOS_RUN_ID = %q, want %q", server.Env["BATTOS_RUN_ID"], run.ID)
	}
}

// TestProcessOneSkipsTeamMCPWhenUnconfigured: sin TeamMCP el plan no lleva
// config MCP aunque el adapter la quiera (degradación silenciosa).
func TestProcessOneSkipsTeamMCPWhenUnconfigured(t *testing.T) {
	run := testRun("claude-code")
	st := &fakeStore{run: run}
	sandbox := &fakeSandbox{result: Result{Summary: "done"}}
	plan := testPlan("claude-code")
	plan.WantsMCP = true
	w := New(st, sandbox, map[string]Adapter{
		"claude-code": fakeAdapter{plan: plan},
	})
	w.ArtifactsDir = t.TempDir()

	if _, err := w.ProcessOne(context.Background()); err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}
	if sandbox.plan.MCPConfigJSON != "" {
		t.Fatalf("MCPConfigJSON = %q, want empty when TeamMCP is nil", sandbox.plan.MCPConfigJSON)
	}
}

// TestDirectSandboxWritesMCPConfig: el sandbox direct materializa
// battos-mcp.json en el workspace, exporta BATTOS_MCP_CONFIG con su path, y el
// archivo NO se recolecta como artifact del run.
func TestDirectSandboxWritesMCPConfig(t *testing.T) {
	workspaces := t.TempDir()

	var command string
	var args []string
	if runtime.GOOS == "windows" {
		// Sin comillas internas: cmd /c con comillas anidadas se mangla por las
		// reglas de quote-stripping; t.TempDir() no tiene espacios.
		command = "cmd"
		args = []string{"/c", `copy %BATTOS_MCP_CONFIG% mcp-copy.json >nul & echo MCPMARKER`}
	} else {
		command = "sh"
		args = []string{"-c", `cp "$BATTOS_MCP_CONFIG" mcp-copy.json && echo MCPMARKER`}
	}

	logFn, _ := collectLog()
	res, err := DirectSandbox{WorkspacesDir: workspaces}.Execute(
		context.Background(),
		ExecutionPlan{
			RuntimeID:     "test-direct",
			Command:       command,
			Args:          args,
			Prompt:        "do the thing",
			Timeout:       30 * time.Second,
			MCPConfigJSON: `{"mcpServers":{"battos":{"command":"battos","args":["mcp"]}}}`,
		},
		logFn,
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var copied *ProducedArtifact
	for i := range res.Artifacts {
		if res.Artifacts[i].Name == "battos-mcp.json" {
			t.Errorf("battos-mcp.json was collected as artifact, want excluded")
		}
		if res.Artifacts[i].Name == "mcp-copy.json" {
			copied = &res.Artifacts[i]
		}
	}
	if copied == nil {
		t.Fatalf("mcp-copy.json artifact not found: BATTOS_MCP_CONFIG did not point to a readable file (artifacts=%v)", res.Artifacts)
	}
	if !strings.Contains(copied.Content, `"battos"`) {
		t.Errorf("copied mcp config content = %q", copied.Content)
	}
}

// TestResolveBattosCLISiblingFallback: si battos no está en PATH se usa el
// binario hermano del ejecutable actual (layout del installer: battos.exe al
// lado de battos-api.exe).
func TestResolveBattosCLISiblingFallback(t *testing.T) {
	dir := t.TempDir()
	name := "battos"
	if runtime.GOOS == "windows" {
		name = "battos.exe"
	}
	sibling := filepath.Join(dir, name)
	if err := os.WriteFile(sibling, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := resolveBattosCLIFrom(filepath.Join(dir, "battos-api.exe"), func(string) (string, error) {
		return "", os.ErrNotExist
	})
	if got != sibling {
		t.Errorf("resolveBattosCLIFrom = %q, want sibling %q", got, sibling)
	}

	got = resolveBattosCLIFrom(filepath.Join(t.TempDir(), "battos-api.exe"), func(string) (string, error) {
		return "", os.ErrNotExist
	})
	if got != "" {
		t.Errorf("resolveBattosCLIFrom = %q, want empty when nothing resolvable", got)
	}
}
