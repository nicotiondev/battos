package gitauth

import "testing"

func TestResolveReadsEnvVar(t *testing.T) {
	t.Setenv("BATTOS_CREDENTIAL_TEST", "ghp_secret123")
	if got := Resolve("BATTOS_CREDENTIAL_TEST"); got != "ghp_secret123" {
		t.Fatalf("Resolve = %q, want ghp_secret123", got)
	}
	if got := Resolve(""); got != "" {
		t.Fatalf("Resolve empty ref = %q, want empty", got)
	}
	if got := Resolve("BATTOS_CREDENTIAL_MISSING"); got != "" {
		t.Fatalf("Resolve missing env = %q, want empty", got)
	}
}

func TestAuthenticatedURLInjectsTokenOnHTTPS(t *testing.T) {
	got := AuthenticatedURL("https://github.com/acme/web.git", "ghp_secret123")
	want := "https://x-access-token:ghp_secret123@github.com/acme/web.git"
	if got != want {
		t.Fatalf("AuthenticatedURL = %q, want %q", got, want)
	}
}

func TestAuthenticatedURLLeavesNonHTTPSAndEmptyUnchanged(t *testing.T) {
	cases := []struct {
		name      string
		remoteURL string
		token     string
		want      string
	}{
		{"ssh url", "git@github.com:acme/web.git", "ghp_x", "git@github.com:acme/web.git"},
		{"empty token", "https://github.com/acme/web.git", "", "https://github.com/acme/web.git"},
		{"empty url", "", "ghp_x", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AuthenticatedURL(tc.remoteURL, tc.token)
			if got != tc.want {
				t.Fatalf("AuthenticatedURL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRedactScrubsToken(t *testing.T) {
	in := "fatal: could not read from https://x-access-token:ghp_secret123@github.com/acme/web.git"
	got := Redact(in, "ghp_secret123")
	if want := "fatal: could not read from https://x-access-token:***@github.com/acme/web.git"; got != want {
		t.Fatalf("Redact = %q, want %q", got, want)
	}
	if got := Redact("no token here", ""); got != "no token here" {
		t.Fatalf("Redact with empty token changed string: %q", got)
	}
}
