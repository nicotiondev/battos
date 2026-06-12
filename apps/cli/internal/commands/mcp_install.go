// mcp_install.go — subcomando `battos mcp install`.
//
// Registra el servidor MCP de BattOS (battos mcp) en los archivos de configuración
// de agentes compatibles: Claude Code (.mcp.json) y Codex (~/.codex/config.toml).
//
// Uso:
//
//	battos mcp install                    # registra en todos los agentes
//	battos mcp install --agent claude-code
//	battos mcp install --agent codex
//	battos mcp install --dry-run          # imprime lo que haría, no escribe
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// mcpEntry contiene la información del servidor MCP que se registra en cada
// archivo de configuración. Es la "fuente de verdad" centralizada para los
// helpers de merge.
type mcpEntry struct {
	Command string
	Args    []string
	Env     map[string]string
}

// newMCPInstallCmd construye el subcomando `battos mcp install`.
// getAPIURL y getToken son funciones que leen los valores de los flags persistentes
// del root command (evaluadas en tiempo de ejecución, no en construcción).
func newMCPInstallCmd(getAPIURL func() string, getToken func() string) *cobra.Command {
	var agent string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Registra el servidor MCP de BattOS en Claude Code, Codex, Cursor y/o VS Code",
		Long: `Escribe (o actualiza) la entrada "battos-memory" en los archivos de
configuración MCP de los agentes seleccionados.

Agentes soportados:
  claude-code  →  .mcp.json en el directorio de trabajo actual
  codex        →  ~/.codex/config.toml
  cursor       →  ~/.cursor/mcp.json
  vscode       →  .vscode/mcp.json en el directorio de trabajo actual
  all          →  todos (por defecto)

El comando es idempotente: volver a correrlo actualiza la entrada existente
sin duplicar ni borrar otras configuraciones.

Con --dry-run imprime el contenido final de cada archivo sin tocar el disco.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPInstall(cmd.Context(), mcpInstallConfig{
				agent:   agent,
				dryRun:  dryRun,
				apiURL:  getAPIURL(),
				token:   getToken(),
				outFunc: func(s string) { fmt.Fprintln(cmd.OutOrStdout(), s) },
			})
		},
	}

	cmd.Flags().StringVar(&agent, "agent", "all", "Agente target: claude-code | codex | cursor | vscode | all")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Imprimir el resultado sin escribir ningún archivo")

	return cmd
}

// mcpInstallConfig agrupa los parámetros de runMCPInstall para facilitar tests.
type mcpInstallConfig struct {
	agent   string
	dryRun  bool
	apiURL  string
	token   string
	outFunc func(string)
}

// runMCPInstall ejecuta la lógica principal del comando.
func runMCPInstall(_ context.Context, cfg mcpInstallConfig) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("mcp install: no se pudo determinar el path del binario: %w", err)
	}

	env := map[string]string{
		"BATTOS_API_URL": cfg.apiURL,
	}
	if cfg.token != "" {
		env["BATTOS_API_TOKEN"] = cfg.token
	}

	entry := mcpEntry{
		Command: execPath,
		Args:    []string{"mcp"},
		Env:     env,
	}

	doClaude := cfg.agent == "claude-code" || cfg.agent == "all"
	doCodex := cfg.agent == "codex" || cfg.agent == "all"
	doCursor := cfg.agent == "cursor" || cfg.agent == "all"
	doVSCode := cfg.agent == "vscode" || cfg.agent == "all"

	if !doClaude && !doCodex && !doCursor && !doVSCode {
		return fmt.Errorf("mcp install: --agent debe ser claude-code, codex, cursor, vscode o all (recibido: %q)", cfg.agent)
	}

	if doClaude {
		if err := installClaudeCode(entry, cfg.dryRun, cfg.token, cfg.outFunc); err != nil {
			return err
		}
	}
	if doCodex {
		if err := installCodex(entry, cfg.dryRun, cfg.outFunc); err != nil {
			return err
		}
	}
	if doCursor {
		if err := installCursor(entry, cfg.dryRun, cfg.token, cfg.outFunc); err != nil {
			return err
		}
	}
	if doVSCode {
		if err := installVSCode(entry, cfg.dryRun, cfg.token, cfg.outFunc); err != nil {
			return err
		}
	}
	return nil
}

// installClaudeCode gestiona el archivo .mcp.json en el directorio de trabajo.
func installClaudeCode(entry mcpEntry, dryRun bool, token string, out func(string)) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("mcp install: no se pudo obtener el directorio actual: %w", err)
	}
	path := filepath.Join(cwd, ".mcp.json")

	var existing []byte
	if b, err := os.ReadFile(path); err == nil {
		existing = b
	}

	merged, err := mergeMCPJSON(existing, entry)
	if err != nil {
		return fmt.Errorf("mcp install (claude-code): %w", err)
	}

	if dryRun {
		out(fmt.Sprintf("[dry-run] Claude Code — %s", path))
		out(string(merged))
		return nil
	}

	if err := writeWithBackup(path, merged); err != nil {
		return fmt.Errorf("mcp install (claude-code): %w", err)
	}

	out(fmt.Sprintf("✓ Claude Code: escrito %s", path))
	out("  Próximo paso: reiniciar Claude Code para que tome el nuevo servidor MCP.")

	if token != "" {
		out("")
		out("  ⚠️  ADVERTENCIA: el archivo .mcp.json contiene BATTOS_API_TOKEN (un secreto).")
		out("  Considera agregar .mcp.json a .gitignore para no exponerlo en el repo.")
	}

	return nil
}

// installCodex gestiona ~/.codex/config.toml.
func installCodex(entry mcpEntry, dryRun bool, out func(string)) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("mcp install: no se pudo obtener el directorio home: %w", err)
	}
	dir := filepath.Join(home, ".codex")
	path := filepath.Join(dir, "config.toml")

	var existing []byte
	if b, err := os.ReadFile(path); err == nil {
		existing = b
	}

	merged, err := mergeCodexTOML(existing, entry)
	if err != nil {
		return fmt.Errorf("mcp install (codex): %w", err)
	}

	if dryRun {
		out(fmt.Sprintf("[dry-run] Codex — %s", path))
		out(string(merged))
		return nil
	}

	// Crear directorio si no existe.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mcp install (codex): crear directorio %s: %w", dir, err)
	}

	if err := writeWithBackup(path, merged); err != nil {
		return fmt.Errorf("mcp install (codex): %w", err)
	}

	out(fmt.Sprintf("✓ Codex: escrito %s", path))
	out("  Próximo paso: el config.toml será leído por Codex en el próximo arranque.")

	return nil
}

// installCursor gestiona ~/.cursor/mcp.json.
// Cursor usa el mismo formato JSON que Claude Code.
func installCursor(entry mcpEntry, dryRun bool, token string, out func(string)) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("mcp install: no se pudo obtener el directorio home: %w", err)
	}
	dir := filepath.Join(home, ".cursor")
	path := filepath.Join(dir, "mcp.json")

	var existing []byte
	if b, err := os.ReadFile(path); err == nil {
		existing = b
	}

	merged, err := mergeMCPJSON(existing, entry)
	if err != nil {
		return fmt.Errorf("mcp install (cursor): %w", err)
	}

	if dryRun {
		out(fmt.Sprintf("[dry-run] Cursor — %s", path))
		out(string(merged))
		return nil
	}

	// Crear directorio si no existe.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mcp install (cursor): crear directorio %s: %w", dir, err)
	}

	if err := writeWithBackup(path, merged); err != nil {
		return fmt.Errorf("mcp install (cursor): %w", err)
	}

	out(fmt.Sprintf("✓ Cursor: escrito %s", path))
	out("  Próximo paso: reiniciar Cursor para que tome el nuevo servidor MCP.")

	if token != "" {
		out("")
		out("  ⚠️  ADVERTENCIA: el archivo mcp.json contiene BATTOS_API_TOKEN (un secreto).")
		out("  El archivo vive en tu home (~/.cursor/) y no está en ningún repo.")
	}

	return nil
}

// installVSCode gestiona .vscode/mcp.json en el directorio de trabajo actual.
// VS Code usa el mismo formato JSON que Claude Code.
func installVSCode(entry mcpEntry, dryRun bool, token string, out func(string)) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("mcp install: no se pudo obtener el directorio actual: %w", err)
	}
	dir := filepath.Join(cwd, ".vscode")
	path := filepath.Join(dir, "mcp.json")

	var existing []byte
	if b, err := os.ReadFile(path); err == nil {
		existing = b
	}

	merged, err := mergeMCPJSON(existing, entry)
	if err != nil {
		return fmt.Errorf("mcp install (vscode): %w", err)
	}

	if dryRun {
		out(fmt.Sprintf("[dry-run] VS Code — %s", path))
		out(string(merged))
		return nil
	}

	// Crear directorio si no existe.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mcp install (vscode): crear directorio %s: %w", dir, err)
	}

	if err := writeWithBackup(path, merged); err != nil {
		return fmt.Errorf("mcp install (vscode): %w", err)
	}

	out(fmt.Sprintf("✓ VS Code: escrito %s", path))
	out("  Próximo paso: recargar la ventana de VS Code para que tome el nuevo servidor MCP.")

	if token != "" {
		out("")
		out("  ⚠️  ADVERTENCIA: el archivo .vscode/mcp.json contiene BATTOS_API_TOKEN (un secreto).")
		out("  Considera agregar .vscode/mcp.json a .gitignore para no exponerlo en el repo.")
	}

	return nil
}

// ===========================================================================
// Funciones puras de merge — no hacen I/O, reciben bytes y devuelven bytes.
// ===========================================================================

// mergeMCPJSON recibe el contenido actual del .mcp.json (nil = archivo no existe)
// y la nueva entrada, y devuelve el contenido final del archivo.
// Preserva todas las claves existentes excepto mcpServers["battos-memory"] que
// se añade/reemplaza.
func mergeMCPJSON(existing []byte, entry mcpEntry) ([]byte, error) {
	// Estructura del .mcp.json
	type serverEntry struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}

	// Usamos map[string]any para preservar claves desconocidas en el top level.
	var root map[string]any

	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &root); err != nil {
			return nil, fmt.Errorf("parsear .mcp.json existente: %w", err)
		}
	}
	if root == nil {
		root = make(map[string]any)
	}

	// Obtener o crear el mapa mcpServers.
	var servers map[string]any
	if s, ok := root["mcpServers"]; ok {
		if sm, ok := s.(map[string]any); ok {
			servers = sm
		}
	}
	if servers == nil {
		servers = make(map[string]any)
	}

	// Construir la nueva entrada.
	newEntry := serverEntry{
		Command: entry.Command,
		Args:    entry.Args,
		Env:     entry.Env,
	}

	// Serializar la entrada a map[string]any para consistencia al reinsertar.
	b, err := json.Marshal(newEntry)
	if err != nil {
		return nil, fmt.Errorf("serializar entrada: %w", err)
	}
	var entryMap map[string]any
	if err := json.Unmarshal(b, &entryMap); err != nil {
		return nil, fmt.Errorf("convertir entrada: %w", err)
	}

	servers["battos-memory"] = entryMap
	root["mcpServers"] = servers

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serializar .mcp.json: %w", err)
	}
	// Añadir newline final (convención Unix).
	out = append(out, '\n')
	return out, nil
}

// codexConfig es la estructura del ~/.codex/config.toml que manejamos.
// Usamos map[string]any para mcp_servers para no acoplar al schema exacto de Codex.
type codexConfig struct {
	MCPServers map[string]codexMCPServer `toml:"mcp_servers,omitempty"`
}

// codexMCPServer es una entrada dentro de [mcp_servers.<name>].
type codexMCPServer struct {
	Command string            `toml:"command"`
	Args    []string          `toml:"args,omitempty"`
	Env     map[string]string `toml:"env,omitempty"`
}

// mergeCodexTOML recibe el contenido actual de config.toml (nil = no existe)
// y la nueva entrada, y devuelve el contenido final del archivo.
// Preserva todas las tablas existentes; añade/reemplaza [mcp_servers.battos-memory].
func mergeCodexTOML(existing []byte, entry mcpEntry) ([]byte, error) {
	// Deserializamos en un map genérico para preservar claves desconocidas.
	var root map[string]any

	if len(existing) > 0 {
		if err := toml.Unmarshal(existing, &root); err != nil {
			return nil, fmt.Errorf("parsear config.toml existente: %w", err)
		}
	}
	if root == nil {
		root = make(map[string]any)
	}

	// Obtener o crear la tabla mcp_servers.
	var servers map[string]any
	if s, ok := root["mcp_servers"]; ok {
		if sm, ok := s.(map[string]any); ok {
			servers = sm
		}
	}
	if servers == nil {
		servers = make(map[string]any)
	}

	// Construir la nueva entrada como map[string]any para coherencia con el resto.
	newServer := map[string]any{
		"command": entry.Command,
		"args":    entry.Args,
	}
	if len(entry.Env) > 0 {
		// go-toml/v2 serializa map[string]string correctamente.
		newServer["env"] = entry.Env
	}

	servers["battos-memory"] = newServer
	root["mcp_servers"] = servers

	out, err := toml.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("serializar config.toml: %w", err)
	}
	return out, nil
}

// ===========================================================================
// I/O helpers
// ===========================================================================

// writeWithBackup escribe newContent en path.
// Si el archivo ya existe, crea primero una copia <path>.bak con su contenido.
func writeWithBackup(path string, newContent []byte) error {
	// Crear backup si el archivo ya existe.
	if existing, err := os.ReadFile(path); err == nil {
		if err := os.WriteFile(path+".bak", existing, 0o600); err != nil {
			return fmt.Errorf("crear backup %s.bak: %w", path, err)
		}
	}
	if err := os.WriteFile(path, newContent, 0o600); err != nil {
		return fmt.Errorf("escribir %s: %w", path, err)
	}
	return nil
}
