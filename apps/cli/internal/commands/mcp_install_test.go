// mcp_install_test.go — tests para las funciones de merge del subcomando `battos mcp install`.
//
// Estrategia TDD: los tests se enfocan en las funciones puras de merge (no en el comando cobra),
// usando t.TempDir() para aislar I/O. No se toca ~/.codex ni .mcp.json real.
package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- helpers de conveniencia ----

// writeTempFile escribe contenido en un archivo dentro de dir y devuelve la ruta.
func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}
	return path
}

// readFile lee un archivo y devuelve su contenido como string.
func readFileStr(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile %s: %v", path, err)
	}
	return string(b)
}

// ---- config de entrada para los helpers ----

// testEntry es la entrada MCP que usamos en todos los tests.
func testEntry() mcpEntry {
	return mcpEntry{
		Command: "/usr/local/bin/battos",
		Args:    []string{"mcp"},
		Env: map[string]string{
			"BATTOS_API_URL": "http://localhost:8000",
		},
	}
}

// testEntryWithToken incluye también BATTOS_API_TOKEN.
func testEntryWithToken() mcpEntry {
	return mcpEntry{
		Command: "/usr/local/bin/battos",
		Args:    []string{"mcp"},
		Env: map[string]string{
			"BATTOS_API_URL":   "http://localhost:8000",
			"BATTOS_API_TOKEN": "tok-secret",
		},
	}
}

// ===========================================================================
// Claude Code JSON (.mcp.json)
// ===========================================================================

func TestMergeClaudeCodeJSON_CreateWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".mcp.json")

	// El archivo no existe; mergeMCPJSON debe crearlo.
	result, err := mergeMCPJSON(nil, testEntry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Escribimos y luego verificamos.
	if err := os.WriteFile(path, result, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("invalid JSON output: %v\ncontent:\n%s", err, string(result))
	}

	servers, ok := out["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers missing or wrong type; got: %+v", out)
	}
	entry, ok := servers["battos-memory"].(map[string]any)
	if !ok {
		t.Fatalf("battos-memory missing; servers=%+v", servers)
	}
	if entry["command"] != "/usr/local/bin/battos" {
		t.Errorf("want command='/usr/local/bin/battos', got %v", entry["command"])
	}
	args, _ := entry["args"].([]any)
	if len(args) != 1 || args[0] != "mcp" {
		t.Errorf("want args=[\"mcp\"], got %v", args)
	}
	env, _ := entry["env"].(map[string]any)
	if env == nil || env["BATTOS_API_URL"] != "http://localhost:8000" {
		t.Errorf("want env.BATTOS_API_URL=http://localhost:8000, got %v", env)
	}
	_ = path // used above
}

func TestMergeClaudeCodeJSON_MergeIntoExisting_PreservesOtherKeys(t *testing.T) {
	existing := `{
  "mcpServers": {
    "other-server": {
      "command": "other",
      "args": ["serve"]
    }
  },
  "someOtherKey": "preserved"
}`

	result, err := mergeMCPJSON([]byte(existing), testEntry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(result, &out); err != nil {
		t.Fatalf("invalid JSON: %v\ncontent:\n%s", err, string(result))
	}

	// La clave de nivel superior debe preservarse.
	if out["someOtherKey"] != "preserved" {
		t.Errorf("someOtherKey should be preserved, got: %v", out["someOtherKey"])
	}

	servers := out["mcpServers"].(map[string]any)

	// El server existente sigue ahí.
	if _, ok := servers["other-server"]; !ok {
		t.Errorf("other-server should be preserved; servers=%+v", servers)
	}

	// battos-memory fue añadido.
	if _, ok := servers["battos-memory"]; !ok {
		t.Errorf("battos-memory should be added; servers=%+v", servers)
	}
}

func TestMergeClaudeCodeJSON_Idempotent(t *testing.T) {
	// Primer merge.
	first, err := mergeMCPJSON(nil, testEntry())
	if err != nil {
		t.Fatalf("first merge: %v", err)
	}

	// Segundo merge sobre el resultado anterior — misma entrada.
	second, err := mergeMCPJSON(first, testEntry())
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}

	var out map[string]any
	if err := json.Unmarshal(second, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	servers := out["mcpServers"].(map[string]any)

	// Solo debe haber un battos-memory, no dos.
	count := 0
	for k := range servers {
		if k == "battos-memory" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 battos-memory, got %d; servers=%+v", count, servers)
	}
}

func TestMergeClaudeCodeJSON_UpdatesExistingEntry(t *testing.T) {
	// Primer merge con una URL.
	e1 := mcpEntry{
		Command: "/old/path/battos",
		Args:    []string{"mcp"},
		Env:     map[string]string{"BATTOS_API_URL": "http://old:8000"},
	}
	first, err := mergeMCPJSON(nil, e1)
	if err != nil {
		t.Fatalf("first merge: %v", err)
	}

	// Segundo merge con URL nueva.
	e2 := mcpEntry{
		Command: "/new/path/battos",
		Args:    []string{"mcp"},
		Env:     map[string]string{"BATTOS_API_URL": "http://new:9000"},
	}
	second, err := mergeMCPJSON(first, e2)
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}

	var out map[string]any
	json.Unmarshal(second, &out)
	servers := out["mcpServers"].(map[string]any)
	entry := servers["battos-memory"].(map[string]any)

	if entry["command"] != "/new/path/battos" {
		t.Errorf("command should be updated to /new/path/battos, got %v", entry["command"])
	}
	env := entry["env"].(map[string]any)
	if env["BATTOS_API_URL"] != "http://new:9000" {
		t.Errorf("BATTOS_API_URL should be updated to http://new:9000, got %v", env["BATTOS_API_URL"])
	}
}

// ===========================================================================
// Codex TOML (~/.codex/config.toml)
// ===========================================================================

func TestMergeCodexTOML_CreateWhenMissing(t *testing.T) {
	result, err := mergeCodexTOML(nil, testEntry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(result)
	if !strings.Contains(content, "[mcp_servers.battos-memory]") {
		t.Errorf("expected [mcp_servers.battos-memory] in output:\n%s", content)
	}
	if !strings.Contains(content, `/usr/local/bin/battos`) {
		t.Errorf("expected command value in output:\n%s", content)
	}
	if !strings.Contains(content, `BATTOS_API_URL`) {
		t.Errorf("expected BATTOS_API_URL in output:\n%s", content)
	}
	if !strings.Contains(content, `http://localhost:8000`) {
		t.Errorf("expected http://localhost:8000 in output:\n%s", content)
	}
}

func TestMergeCodexTOML_MergePreservesOtherTables(t *testing.T) {
	// go-toml/v2 puede cambiar el estilo de comillas al re-serializar,
	// así que usamos un round-trip: parseamos el existing y verificamos los valores.
	existing := `[model]
name = "o4-mini"
temperature = 0.7

[other_table]
key = "value"
`
	result, err := mergeCodexTOML([]byte(existing), testEntry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(result)

	// Las tablas existentes se preservan — verificamos los valores (no el estilo de comilla).
	if !strings.Contains(content, `o4-mini`) {
		t.Errorf("model.name should be preserved:\n%s", content)
	}
	if !strings.Contains(content, `0.7`) {
		t.Errorf("model.temperature should be preserved:\n%s", content)
	}
	if !strings.Contains(content, `value`) {
		t.Errorf("other_table.key should be preserved:\n%s", content)
	}

	// battos-memory fue añadido.
	if !strings.Contains(content, "[mcp_servers.battos-memory]") {
		t.Errorf("battos-memory should be added:\n%s", content)
	}
}

func TestMergeCodexTOML_Idempotent(t *testing.T) {
	first, err := mergeCodexTOML(nil, testEntry())
	if err != nil {
		t.Fatalf("first merge: %v", err)
	}

	second, err := mergeCodexTOML(first, testEntry())
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}

	content := string(second)
	count := strings.Count(content, "[mcp_servers.battos-memory]")
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of [mcp_servers.battos-memory], got %d:\n%s", count, content)
	}
}

func TestMergeCodexTOML_UpdatesExistingEntry(t *testing.T) {
	e1 := mcpEntry{
		Command: "/old/battos",
		Args:    []string{"mcp"},
		Env:     map[string]string{"BATTOS_API_URL": "http://old:8000"},
	}
	first, err := mergeCodexTOML(nil, e1)
	if err != nil {
		t.Fatalf("first merge: %v", err)
	}

	e2 := mcpEntry{
		Command: "/new/battos",
		Args:    []string{"mcp"},
		Env:     map[string]string{"BATTOS_API_URL": "http://new:9000"},
	}
	second, err := mergeCodexTOML(first, e2)
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}

	content := string(second)
	if !strings.Contains(content, `/new/battos`) {
		t.Errorf("command should be updated to /new/battos:\n%s", content)
	}
	if !strings.Contains(content, `http://new:9000`) {
		t.Errorf("BATTOS_API_URL should be updated to http://new:9000:\n%s", content)
	}
	// El valor viejo NO debe aparecer.
	if strings.Contains(content, `http://old:8000`) {
		t.Errorf("old BATTOS_API_URL should not appear:\n%s", content)
	}
}

func TestMergeCodexTOML_WithToken(t *testing.T) {
	result, err := mergeCodexTOML(nil, testEntryWithToken())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(result)
	if !strings.Contains(content, "BATTOS_API_TOKEN") {
		t.Errorf("BATTOS_API_TOKEN should appear in output:\n%s", content)
	}
	if !strings.Contains(content, "tok-secret") {
		t.Errorf("token value should appear in output:\n%s", content)
	}
}

// ===========================================================================
// dry-run: no toca disco
// ===========================================================================

func TestDryRunChangesNothing(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, ".mcp.json")
	tomlPath := filepath.Join(dir, "config.toml")

	// Archivos vacíos de referencia.
	_ = os.WriteFile(jsonPath, []byte(`{"mcpServers":{}}`), 0o600)
	_ = os.WriteFile(tomlPath, []byte("[model]\nname = \"o4-mini\""), 0o600)

	beforeJSON := readFileStr(t, jsonPath)
	beforeTOML := readFileStr(t, tomlPath)

	// dry-run: sólo calcula output, NO escribe.
	entry := testEntry()
	_, err := mergeMCPJSON([]byte(beforeJSON), entry)
	if err != nil {
		t.Fatalf("mergeMCPJSON: %v", err)
	}
	_, err = mergeCodexTOML([]byte(beforeTOML), entry)
	if err != nil {
		t.Fatalf("mergeCodexTOML: %v", err)
	}

	// Los archivos en disco no deben haber cambiado.
	afterJSON := readFileStr(t, jsonPath)
	afterTOML := readFileStr(t, tomlPath)

	if afterJSON != beforeJSON {
		t.Errorf("dry-run should not change .mcp.json; before=%q after=%q", beforeJSON, afterJSON)
	}
	if afterTOML != beforeTOML {
		t.Errorf("dry-run should not change config.toml; before=%q after=%q", beforeTOML, afterTOML)
	}
}

// ===========================================================================
// backup
// ===========================================================================

func TestWriteWithBackup_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.json")
	original := `{"original": true}`

	// Escribir archivo original.
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	newContent := []byte(`{"updated": true}`)
	if err := writeWithBackup(path, newContent); err != nil {
		t.Fatalf("writeWithBackup: %v", err)
	}

	// El archivo principal tiene el nuevo contenido.
	got := readFileStr(t, path)
	if got != string(newContent) {
		t.Errorf("main file should have new content, got: %s", got)
	}

	// El .bak tiene el original.
	bak := readFileStr(t, path+".bak")
	if bak != original {
		t.Errorf("backup should have original content, got: %s", bak)
	}
}

func TestWriteWithBackup_NoBackupWhenFileNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.json")

	newContent := []byte(`{"new": true}`)
	if err := writeWithBackup(path, newContent); err != nil {
		t.Fatalf("writeWithBackup: %v", err)
	}

	// El archivo principal fue creado.
	got := readFileStr(t, path)
	if got != string(newContent) {
		t.Errorf("file should have new content, got: %s", got)
	}

	// No debe existir backup cuando no había archivo previo.
	if _, err := os.Stat(path + ".bak"); err == nil {
		t.Errorf(".bak should not exist when original file did not exist")
	}
}
