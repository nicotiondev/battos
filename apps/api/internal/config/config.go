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
	DBPath  string `mapstructure:"db_path"`
	UseFTS5 bool   `mapstructure:"use_fts5"`
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
	WorkerEnabled        bool   `mapstructure:"worker_enabled"`
	SandboxMode          string `mapstructure:"sandbox_mode"`
	DefaultTimeoutS      int    `mapstructure:"default_timeout_s"`
	PollIntervalS        int    `mapstructure:"poll_interval_s"`
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
}

type NovaCoreConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Provider string `mapstructure:"provider"`
	Model    string `mapstructure:"model"`
}

// Load lee el archivo de config y devuelve la struct tipada.
//
// Orden de búsqueda del archivo:
//  1. Path explícito si BATTOS_CONFIG está seteado.
//  2. ./config/battos.yaml relativo al cwd.
//  3. /app/config/battos.yaml (dentro del contenedor).
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("battos")
	v.SetConfigType("yaml")
	v.AddConfigPath("./config")
	v.AddConfigPath("/app/config")

	// Env var BATTOS_CONFIG=/path/al/archivo permite override total.
	if envPath := v.GetString("BATTOS_CONFIG"); envPath != "" {
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
