package config

import (
	"path/filepath"
	"testing"
)

func TestValidateAuthRejectsDisabledAuthOnPublicBind(t *testing.T) {
	cfg := &Config{
		API:  APIConfig{Host: "0.0.0.0"},
		Auth: AuthConfig{Mode: "disabled"},
	}
	if err := validateAuth(cfg); err == nil {
		t.Fatal("expected disabled auth on public bind to fail")
	}
}

// TestUserInstallBaseDetectsAppData: config cargado desde APPDATA\battos (o
// ~/.config/battos) ancla; ./config de dev y BATTOS_CONFIG explícito no.
func TestUserInstallBaseDetectsAppData(t *testing.T) {
	appData := t.TempDir()
	home := t.TempDir()
	t.Setenv("APPDATA", appData)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("BATTOS_CONFIG", "")

	if got := userInstallBase(filepath.Join(appData, "battos", "battos.yaml")); got != filepath.Join(appData, "battos") {
		t.Errorf("appdata config: base = %q, want %q", got, filepath.Join(appData, "battos"))
	}
	if got := userInstallBase(filepath.Join(home, ".config", "battos", "battos.yaml")); got != filepath.Join(home, ".config", "battos") {
		t.Errorf("xdg-home config: base = %q", got)
	}
	if got := userInstallBase(filepath.Join("config", "battos.yaml")); got != "" {
		t.Errorf("dev config: base = %q, want empty", got)
	}
	t.Setenv("BATTOS_CONFIG", filepath.Join(appData, "battos", "battos.yaml"))
	if got := userInstallBase(filepath.Join(appData, "battos", "battos.yaml")); got != "" {
		t.Errorf("explicit BATTOS_CONFIG: base = %q, want empty (no anchoring)", got)
	}
}

// TestAnchorRelativePaths: paths relativos se anclan a base; absolutos y
// vacíos quedan intactos.
func TestAnchorRelativePaths(t *testing.T) {
	base := t.TempDir()
	abs := filepath.Join(t.TempDir(), "fixed.db")
	cfg := &Config{}
	cfg.Database.Path = "data/battos.db"
	cfg.Memory.DBPath = abs
	cfg.Logs.Dir = ""
	cfg.Knowledge.ArtifactsDir = "data/artifacts"

	anchorRelativePaths(cfg, base)

	if cfg.Database.Path != filepath.Join(base, "data", "battos.db") {
		t.Errorf("database.path = %q", cfg.Database.Path)
	}
	if cfg.Memory.DBPath != abs {
		t.Errorf("absolute path was rewritten: %q", cfg.Memory.DBPath)
	}
	if cfg.Logs.Dir != "" {
		t.Errorf("empty path was rewritten: %q", cfg.Logs.Dir)
	}
	if cfg.Knowledge.ArtifactsDir != filepath.Join(base, "data", "artifacts") {
		t.Errorf("artifacts_dir = %q", cfg.Knowledge.ArtifactsDir)
	}
}

func TestValidateAuthAllowsLocalDisabledAndTokenPublic(t *testing.T) {
	cases := []*Config{
		{API: APIConfig{Host: "127.0.0.1"}, Auth: AuthConfig{Mode: "disabled"}},
		{API: APIConfig{Host: "0.0.0.0"}, Auth: AuthConfig{Mode: "token"}},
	}
	for _, cfg := range cases {
		if err := validateAuth(cfg); err != nil {
			t.Fatalf("validate auth: %v", err)
		}
	}
}
