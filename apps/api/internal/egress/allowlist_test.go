package egress

import (
	"testing"
)

func TestAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		host      string
		allowlist []string
		want      bool
	}{
		// --- Match exacto ---
		{
			name:      "exact match",
			host:      "anthropic.com",
			allowlist: []string{"anthropic.com"},
			want:      true,
		},
		{
			name:      "exact match with port stripped",
			host:      "anthropic.com:443",
			allowlist: []string{"anthropic.com"},
			want:      true,
		},
		{
			name:      "exact match api subdomain",
			host:      "api.anthropic.com",
			allowlist: []string{"api.anthropic.com"},
			want:      true,
		},

		// --- Match por sufijo de subdominio ---
		{
			name:      "subdomain match",
			host:      "api.anthropic.com",
			allowlist: []string{"anthropic.com"},
			want:      true,
		},
		{
			name:      "deep subdomain match",
			host:      "chat.api.anthropic.com",
			allowlist: []string{"anthropic.com"},
			want:      true,
		},
		{
			// Trailing-dot FQDN: el proxy falla CERRADO (rechaza) por seguridad.
			// Lockea el comportamiento para que un cambio no lo abra por accidente.
			name:      "trailing dot subdomain fails closed",
			host:      "api.anthropic.com.",
			allowlist: []string{"anthropic.com"},
			want:      false,
		},
		{
			name:      "trailing dot exact fails closed",
			host:      "anthropic.com.",
			allowlist: []string{"anthropic.com"},
			want:      false,
		},
		{
			name:      "subdomain with port stripped",
			host:      "api.anthropic.com:443",
			allowlist: []string{"anthropic.com"},
			want:      true,
		},

		// --- Negativos criticos ---
		{
			name:      "evil prefix - evilanthropic.com must NOT match anthropic.com",
			host:      "evilanthropic.com",
			allowlist: []string{"anthropic.com"},
			want:      false,
		},
		{
			name:      "suffix attack - anthropic.com.attacker.net must NOT match anthropic.com",
			host:      "anthropic.com.attacker.net",
			allowlist: []string{"anthropic.com"},
			want:      false,
		},
		{
			name:      "no match different domain",
			host:      "openai.com",
			allowlist: []string{"anthropic.com"},
			want:      false,
		},
		{
			name:      "partial hostname prefix",
			host:      "notanthropicatall.com",
			allowlist: []string{"thropic.com"},
			want:      false,
		},

		// --- Allowlist vacia ---
		{
			name:      "empty allowlist blocks everything",
			host:      "api.anthropic.com",
			allowlist: []string{},
			want:      false,
		},
		{
			name:      "nil allowlist blocks everything",
			host:      "api.anthropic.com",
			allowlist: nil,
			want:      false,
		},

		// --- Case-insensitivity ---
		{
			name:      "case insensitive host uppercase",
			host:      "API.ANTHROPIC.COM",
			allowlist: []string{"anthropic.com"},
			want:      true,
		},
		{
			name:      "case insensitive entry uppercase",
			host:      "api.anthropic.com",
			allowlist: []string{"ANTHROPIC.COM"},
			want:      true,
		},
		{
			name:      "case insensitive mixed",
			host:      "Api.Anthropic.Com",
			allowlist: []string{"Anthropic.Com"},
			want:      true,
		},

		// --- Multiple entries ---
		{
			name:      "match second entry in list",
			host:      "api.openai.com",
			allowlist: []string{"anthropic.com", "openai.com"},
			want:      true,
		},
		{
			name:      "no match any entry",
			host:      "attacker.net",
			allowlist: []string{"anthropic.com", "openai.com"},
			want:      false,
		},

		// --- Entries con espacios (trim) ---
		{
			name:      "entry with surrounding spaces",
			host:      "api.anthropic.com",
			allowlist: []string{"  anthropic.com  "},
			want:      true,
		},

		// --- Entry vacia ignorada ---
		{
			name:      "empty entry in list is skipped",
			host:      "api.anthropic.com",
			allowlist: []string{"", "openai.com"},
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := allowed(tc.host, tc.allowlist)
			if got != tc.want {
				t.Errorf("allowed(%q, %v) = %v, want %v", tc.host, tc.allowlist, got, tc.want)
			}
		})
	}
}
