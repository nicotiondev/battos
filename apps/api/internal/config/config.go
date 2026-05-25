// Package config carga la configuración de BattOS desde:
//   1. config/battos.yaml en el cwd (o el path indicado por BATTOS_CONFIG)
//   2. variables de entorno con prefijo BATTOS_
//
// Las variables sensibles (API keys, passwords) NO viven en battos.yaml:
// se leen vía env (infra/.env cargado por docker-compose, o el shell en dev).
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config es la representación tipada del archivo battos.yaml.
type Config struct {
	API        APIConfig        `mapstructure:"api"`
	Memory     MemoryConfig     `mapstructure:"memory"`
	Logs       LogsConfig       `mapstructure:"logs"`
	Sysmetrics SysmetricsConfig `mapstructure:"sysmetrics"`
	Registries RegistriesConfig `mapstructure:"registries"`

	// Database — viene del env (DATABASE_URL), no de battos.yaml.
	DatabaseURL string `mapstructure:"-"`
}

type APIConfig struct {
	Host        string   `mapstructure:"host"`
	Port        int      `mapstructure:"port"`
	CORSOrigins []string `mapstructure:"cors_origins"`
}

type MemoryConfig struct {
	DBPath  string `mapstructure:"db_path"`
	UseFTS5 bool   `mapstructure:"use_fts5"`
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

// Load lee el archivo de config y devuelve la struct tipada.
//
// Orden de búsqueda del archivo:
//   1. Path explícito si BATTOS_CONFIG está seteado.
//   2. ./config/battos.yaml relativo al cwd.
//   3. /app/config/battos.yaml (dentro del contenedor).
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

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("leyendo battos.yaml: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parseando config: %w", err)
	}

	// DATABASE_URL viene del entorno, no del YAML.
	cfg.DatabaseURL = viper.New().GetString("DATABASE_URL")
	// viper.New() no leyó env todavía — uso os directamente para evitar confusión.
	cfg.DatabaseURL = getEnv("DATABASE_URL", "")

	return &cfg, nil
}

func getEnv(key, fallback string) string {
	v := viper.New()
	v.AutomaticEnv()
	if val := v.GetString(key); val != "" {
		return val
	}
	return fallback
}
