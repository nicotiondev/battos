// Package config carga la configuración de BattOS desde:
//  1. config/battos.yaml en el cwd (o el path indicado por BATTOS_CONFIG)
//  2. variables de entorno con prefijo BATTOS_
//
// Las variables sensibles (API keys, passwords) NO viven en battos.yaml:
// se leen vía env (infra/.env cargado por docker-compose, o el shell en dev).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config es la representación tipada del archivo battos.yaml.
type Config struct {
	API        APIConfig        `mapstructure:"api"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Memory     MemoryConfig     `mapstructure:"memory"`
	Knowledge  KnowledgeConfig  `mapstructure:"knowledge"`
	Logs       LogsConfig       `mapstructure:"logs"`
	Sysmetrics SysmetricsConfig `mapstructure:"sysmetrics"`
	Registries RegistriesConfig `mapstructure:"registries"`
	Execution  ExecutionConfig  `mapstructure:"execution"`
	NovaCore   NovaCoreConfig   `mapstructure:"novacore"`

	APIToken string `mapstructure:"-"`
}

type APIConfig struct {
	Host        string   `mapstructure:"host"`
	Port        int      `mapstructure:"port"`
	CORSOrigins []string `mapstructure:"cors_origins"`
}

type AuthConfig struct {
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type MemoryConfig struct {
	DBPath    string `mapstructure:"db_path"`
	UseFTS5   bool   `mapstructure:"use_fts5"`
	Provider  string `mapstructure:"provider"`   // "builtin" (default) o "engram"
	EngramURL string `mapstructure:"engram_url"` // default: "http://localhost:7437"
}

type KnowledgeConfig struct {
	ArtifactsDir string `mapstructure:"artifacts_dir"`
}

type LogsConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Dir    string `mapstructure:"dir"`
}

type SysmetricsConfig struct {
	SampleIntervalS int `mapstructure:"sample_interval_s"`
	HistorySize     int `mapstructure:"history_size"`
}

type RegistriesConfig struct {
	AgentsDir string `mapstructure:"agents_dir"`
	SkillsDir string `mapstructure:"skills_dir"`
}

type ExecutionConfig struct {
	WorkerEnabled   bool   `mapstructure:"worker_enabled"`
	SandboxMode     string `mapstructure:"sandbox_mode"`
	DefaultTimeoutS int    `mapstructure:"default_timeout_s"`
	PollIntervalS   int    `mapstructure:"poll_interval_s"`
	// WorkerConcurrency es la cantidad de runs procesados en paralelo por el
	// worker (RunPool, Etapa 3). 1 = loop secuencial clásico. Para team-runs
	// (lead + delegados simultáneos) se necesita >= 3.
	WorkerConcurrency    int    `mapstructure:"worker_concurrency"`
	DockerImage          string `mapstructure:"docker_image"`
	WorkspacesDir        string `mapstructure:"workspaces_dir"`
	RepositoriesDir      string `mapstructure:"repositories_dir"`
	HostSessionEnabled   bool   `mapstructure:"host_session_enabled"`
	CodexCredentialsDir  string `mapstructure:"codex_credentials_dir"`
	ClaudeCredentialsDir string `mapstructure:"claude_credentials_dir"`
	// EgressNetwork es la red Docker interna por la que corren los runs host_session+network.
	// La red es internal:true, el contenedor no tiene ruta directa a internet.
	// Ver ADR-0022.
	EgressNetwork   string `mapstructure:"egress_network"`
	EgressProxyAddr string `mapstructure:"egress_proxy_addr"`
	// ConnectedRuntimes mapea un runtime adapter id a la config del servicio
	// always-on al que el tier "connected" reenvía el run (Hermes, OpenClaw, …).
	// Ver ConnectedSandbox / Fase A3.
	ConnectedRuntimes map[string]ConnectedRuntimeConfig `mapstructure:"connected_runtimes"`
}

// ConnectedRuntimeConfig describe cómo alcanzar un servicio always-on para el
// tier "connected". kind: "local-cli" (corre `command` en el host con los args,
// soporta placeholders {{prompt}}/{{prompt_file}}) o "http" (POST al endpoint).
type ConnectedRuntimeConfig struct {
	Kind     string   `mapstructure:"kind"`
	Endpoint string   `mapstructure:"endpoint"`
	Command  string   `mapstructure:"command"`
	Args     []string `mapstructure:"args"`
}

type NovaCoreConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Provider  string `mapstructure:"provider"`
	Model     string `mapstructure:"model"`
	BaseURL   string `mapstructure:"base_url"`   // override URL del LLM API; vacío = default del provider
	APIKeyEnv string `mapstructure:"api_key_env"` // env var con la API key; vacío = default del provider
}

// Load lee el archivo de config y devuelve la struct tipada.
//
// Orden de búsqueda del archivo:
//  1. Path explícito si BATTOS_CONFIG está seteado.
//  2. ./config/battos.yaml relativo al cwd.
//  3. $APPDATA/battos/battos.yaml   (Windows, instalación de usuario).
//  4. $HOME/.config/battos/battos.yaml (Linux/macOS, XDG).
//  5. /app/config/battos.yaml (dentro del contenedor).
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("battos")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	if appData := os.Getenv("APPDATA"); appData != "" {
		v.AddConfigPath(appData + "/battos")
	}
	if home := os.Getenv("HOME"); home != "" {
		v.AddConfigPath(home + "/.config/battos")
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		v.AddConfigPath(xdg + "/battos")
	}
	v.AddConfigPath("/app/config")

	// Env var BATTOS_CONFIG=/path/al/archivo permite override total.
	if envPath := os.Getenv("BATTOS_CONFIG"); envPath != "" {
		v.SetConfigFile(envPath)
	}

	// BATTOS_API_PORT, BATTOS_LOGS_LEVEL, etc. mapean a api.port, logs.level...
	v.SetEnvPrefix("BATTOS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	for _, key := range []string{
		"api.host",
		"api.port",
		"auth.mode",
		"database.path",
		"logs.level",
		"logs.format",
		"logs.dir",
		"memory.db_path",
		"memory.use_fts5",
		"memory.provider",
		"memory.engram_url",
		"knowledge.artifacts_dir",
		"sysmetrics.sample_interval_s",
		"sysmetrics.history_size",
		"registries.agents_dir",
		"registries.skills_dir",
		"execution.worker_enabled",
		"execution.sandbox_mode",
		"execution.default_timeout_s",
		"execution.poll_interval_s",
		"execution.docker_image",
		"execution.workspaces_dir",
		"execution.repositories_dir",
		"execution.host_session_enabled",
		"execution.codex_credentials_dir",
		"execution.claude_credentials_dir",
		"execution.egress_network",
		"execution.egress_proxy_addr",
		"novacore.enabled",
		"novacore.provider",
		"novacore.model",
		"novacore.base_url",
		"novacore.api_key_env",
	} {
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("binding env %s: %w", key, err)
		}
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("leyendo battos.yaml: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parseando config: %w", err)
	}

	// Instalación de usuario (APPDATA/XDG/~/.config): los paths relativos del
	// YAML se anclan al directorio del config, NO al cwd del proceso. Sin esto,
	// arrancar battos desde distintos directorios crea bases de datos y
	// artifacts distintos (estado partido — bug encontrado en E2E 2026-06-12).
	// En dev (./config) y docker (/app/config) se mantienen cwd-relativos.
	if base := userInstallBase(v.ConfigFileUsed()); base != "" {
		anchorRelativePaths(&cfg, base)
	}

	cfg.APIToken = os.Getenv("BATTOS_API_TOKEN")

	switch cfg.Auth.Mode {
	case "", "disabled":
		cfg.Auth.Mode = "disabled"
	case "token":
		if strings.TrimSpace(cfg.APIToken) == "" {
			return nil, fmt.Errorf("auth: BATTOS_API_TOKEN es obligatorio cuando auth.mode=token")
		}
	default:
		return nil, fmt.Errorf("auth: mode invalido %q (use disabled o token)", cfg.Auth.Mode)
	}
	if err := validateAuth(&cfg); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Memory.Provider) == "" {
		cfg.Memory.Provider = "builtin"
	}
	switch cfg.Memory.Provider {
	case "builtin", "engram":
	default:
		return nil, fmt.Errorf("memory: provider invalido %q (use builtin o engram)", cfg.Memory.Provider)
	}
	if strings.TrimSpace(cfg.Memory.EngramURL) == "" {
		cfg.Memory.EngramURL = "http://localhost:7437"
	}
	if strings.TrimSpace(cfg.Knowledge.ArtifactsDir) == "" {
		cfg.Knowledge.ArtifactsDir = "data/artifacts"
	}
	if strings.TrimSpace(cfg.Database.Path) == "" {
		cfg.Database.Path = "data/battos.db"
	}
	if strings.TrimSpace(cfg.Execution.SandboxMode) == "" {
		cfg.Execution.SandboxMode = "dry_run"
	}
	switch cfg.Execution.SandboxMode {
	case "dry_run", "docker":
	default:
		return nil, fmt.Errorf("execution: sandbox_mode invalido %q (use dry_run o docker)", cfg.Execution.SandboxMode)
	}
	if cfg.Execution.DefaultTimeoutS <= 0 {
		cfg.Execution.DefaultTimeoutS = 1800
	}
	if cfg.Execution.PollIntervalS <= 0 {
		cfg.Execution.PollIntervalS = 2
	}
	if cfg.Execution.WorkerConcurrency <= 0 {
		cfg.Execution.WorkerConcurrency = 1
	}
	if strings.TrimSpace(cfg.Execution.DockerImage) == "" {
		cfg.Execution.DockerImage = "alpine:3.20"
	}
	if strings.TrimSpace(cfg.Execution.WorkspacesDir) == "" {
		cfg.Execution.WorkspacesDir = "data/runs/workspaces"
	}
	if strings.TrimSpace(cfg.Execution.RepositoriesDir) == "" {
		cfg.Execution.RepositoriesDir = "data/repositories"
	}
	if strings.TrimSpace(cfg.Execution.EgressNetwork) == "" {
		cfg.Execution.EgressNetwork = "battos-egress"
	}
	if strings.TrimSpace(cfg.Execution.EgressProxyAddr) == "" {
		cfg.Execution.EgressProxyAddr = "battos-egress-proxy:8888"
	}
	if strings.TrimSpace(cfg.NovaCore.Provider) == "" {
		cfg.NovaCore.Provider = "anthropic"
	}
	if strings.TrimSpace(cfg.NovaCore.Model) == "" {
		if cfg.NovaCore.Provider == "openai" {
			cfg.NovaCore.Model = "gpt-4o-mini"
		} else {
			cfg.NovaCore.Model = "claude-3-haiku-20240307"
		}
	}

	return &cfg, nil
}

// userInstallBase devuelve el directorio del config cuando se cargó desde una
// ubicación de instalación de usuario (APPDATA\battos, $XDG_CONFIG_HOME/battos
// o ~/.config/battos). Para ./config (dev), /app/config (docker) o BATTOS_CONFIG
// explícito devuelve "" — esos casos siguen siendo cwd-relativos.
func userInstallBase(configFile string) string {
	if strings.TrimSpace(configFile) == "" || os.Getenv("BATTOS_CONFIG") != "" {
		return ""
	}
	configDir := filepath.Clean(filepath.Dir(configFile))
	candidates := []string{}
	if appData := os.Getenv("APPDATA"); appData != "" {
		candidates = append(candidates, filepath.Join(appData, "battos"))
	}
	if home := os.Getenv("HOME"); home != "" {
		candidates = append(candidates, filepath.Join(home, ".config", "battos"))
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		candidates = append(candidates, filepath.Join(xdg, "battos"))
	}
	for _, c := range candidates {
		if strings.EqualFold(configDir, filepath.Clean(c)) {
			return configDir
		}
	}
	return ""
}

// anchorRelativePaths resuelve los paths relativos de datos contra base, de
// modo que la instalación de usuario tenga una única DB/artifacts sin importar
// desde qué directorio se arranque el proceso.
func anchorRelativePaths(cfg *Config, base string) {
	anchor := func(p *string) {
		if strings.TrimSpace(*p) == "" || filepath.IsAbs(*p) {
			return
		}
		*p = filepath.Join(base, *p)
	}
	anchor(&cfg.Database.Path)
	anchor(&cfg.Memory.DBPath)
	anchor(&cfg.Knowledge.ArtifactsDir)
	anchor(&cfg.Logs.Dir)
	anchor(&cfg.Registries.AgentsDir)
	anchor(&cfg.Registries.SkillsDir)
	anchor(&cfg.Execution.WorkspacesDir)
	anchor(&cfg.Execution.RepositoriesDir)
}

func validateAuth(cfg *Config) error {
	if cfg.Auth.Mode != "disabled" {
		return nil
	}
	switch cfg.API.Host {
	case "127.0.0.1", "localhost", "::1":
		return nil
	default:
		return fmt.Errorf("auth: mode disabled solo puede usarse con api.host local")
	}
}
