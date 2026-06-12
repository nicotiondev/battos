package credstore

import (
	"context"
	"database/sql"
	"testing"

	"github.com/nicotion/battos/apps/api/internal/store"
)

type fakeReader struct {
	creds map[string]store.Credential
}

func (f fakeReader) GetCredentialByName(_ context.Context, name string) (store.Credential, error) {
	c, ok := f.creds[name]
	if !ok {
		return store.Credential{}, sql.ErrNoRows
	}
	return c, nil
}

func TestResolveEmptyRef(t *testing.T) {
	r := New(fakeReader{})
	v, err := r.Resolve(context.Background(), "  ")
	if err != nil || v != "" {
		t.Fatalf("got (%q,%v), want empty", v, err)
	}
}

func TestResolveFallsBackToEnv(t *testing.T) {
	t.Setenv("SOME_TOKEN", "from-env")
	r := New(fakeReader{}) // no managed credential named SOME_TOKEN
	v, err := r.Resolve(context.Background(), "SOME_TOKEN")
	if err != nil {
		t.Fatal(err)
	}
	if v != "from-env" {
		t.Errorf("v=%q, want from-env (os.Getenv fallback)", v)
	}
}

func TestResolveEnvSource(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-123")
	r := New(fakeReader{creds: map[string]store.Credential{
		"openai": {Name: "openai", SecretSource: "env", SecretLocator: "OPENAI_API_KEY"},
	}})
	v, err := r.Resolve(context.Background(), "openai")
	if err != nil {
		t.Fatal(err)
	}
	if v != "sk-test-123" {
		t.Errorf("v=%q, want sk-test-123", v)
	}
}

func TestInlineEncryptedRoundTrip(t *testing.T) {
	t.Setenv(MasterKeyEnv, "a-strong-master-passphrase")
	r := New(fakeReader{})

	blob, err := r.Encrypt("sk-secret-value")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if blob == "sk-secret-value" || blob == "" {
		t.Fatalf("blob not encrypted: %q", blob)
	}

	// Resolve a credential whose locator is the encrypted blob.
	r2 := New(fakeReader{creds: map[string]store.Credential{
		"minimax": {Name: "minimax", SecretSource: "inline_encrypted", SecretLocator: blob},
	}})
	// r2 must share the same master key (same env) to decrypt.
	v, err := r2.Resolve(context.Background(), "minimax")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v != "sk-secret-value" {
		t.Errorf("decrypted=%q, want sk-secret-value", v)
	}
}

func TestInlineEncryptWithoutMasterKeyFails(t *testing.T) {
	t.Setenv(MasterKeyEnv, "")
	r := New(fakeReader{})
	if _, err := r.Encrypt("x"); err == nil {
		t.Fatal("expected error encrypting without master key")
	}
}

func TestKeychainSourceNotSupported(t *testing.T) {
	r := New(fakeReader{creds: map[string]store.Credential{
		"kc": {Name: "kc", SecretSource: "keychain", SecretLocator: "some-id"},
	}})
	_, err := r.Resolve(context.Background(), "kc")
	if err == nil || !contains(err.Error(), "keychain") {
		t.Fatalf("expected keychain-not-supported error, got %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
