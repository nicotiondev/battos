package worker

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// TeamMCPConfig describe cómo los agentes lanzados por el worker alcanzan el
// servidor MCP de BattOS (battos mcp). Cuando está configurado, los runs cuyo
// adapter declara SupportsMCP reciben un battos-mcp.json en su workspace con
// las tools de equipo (team_spawn_run, team_send_message, memory_*), lo que
// habilita la delegación multi-agente desde adentro de un run.
type TeamMCPConfig struct {
	// CLIPath es el path absoluto al binario battos (el CLI, no la API).
	CLIPath string
	// APIURL es la base URL de esta API, ej. http://127.0.0.1:8000.
	APIURL string
}

// teamMCPConfigJSON compone el archivo de configuración MCP que claude-code
// consume vía --mcp-config. BATTOS_RUN_ID viaja en el env del server para que
// el agente pueda enlazar parent_run_id al delegar.
func teamMCPConfigJSON(cliPath, apiURL, runID string) string {
	type mcpServer struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	}
	cfg := map[string]map[string]mcpServer{
		"mcpServers": {
			"battos": {
				Command: cliPath,
				Args:    []string{"mcp"},
				Env: map[string]string{
					"BATTOS_API_URL": apiURL,
					"BATTOS_RUN_ID":  runID,
				},
			},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(data)
}

// ResolveBattosCLI localiza el binario battos: primero como hermano del
// ejecutable actual (layout del installer y del repo: battos al lado de
// battos-api — en dev garantiza usar el build fresco), después en PATH.
// Devuelve "" si no se encuentra — el caller degrada silenciosamente (runs
// sin tools de equipo).
func ResolveBattosCLI() string {
	exe, err := os.Executable()
	if err != nil {
		exe = ""
	}
	return resolveBattosCLIFrom(exe, exec.LookPath)
}

func resolveBattosCLIFrom(currentExe string, lookPath func(string) (string, error)) string {
	if strings.TrimSpace(currentExe) != "" {
		name := "battos"
		if runtime.GOOS == "windows" {
			name = "battos.exe"
		}
		sibling := filepath.Join(filepath.Dir(currentExe), name)
		if info, err := os.Stat(sibling); err == nil && !info.IsDir() {
			return sibling
		}
	}
	if path, err := lookPath("battos"); err == nil && strings.TrimSpace(path) != "" {
		return path
	}
	return ""
}
