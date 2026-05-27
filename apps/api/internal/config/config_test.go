package config

import "testing"

func TestValidateAuthRejectsDisabledAuthOnPublicBind(t *testing.T) {
	cfg := &Config{
		API:  APIConfig{Host: "0.0.0.0"},
		Auth: AuthConfig{Mode: "disabled"},
	}
	if err := validateAuth(cfg); err == nil {
		t.Fatal("expected disabled auth on public bind to fail")
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
