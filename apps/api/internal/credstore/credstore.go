// Package credstore is the BattOS credential broker (la Bóveda funcional, ADR-0023).
//
// It resolves a credential reference to a secret value without that secret ever
// being stored in clear text. Three sources are supported:
//
//   - env: the stored locator is the NAME of an environment variable; the value
//     lives in the process env (infra/.env or shell). This is the legacy
//     secrets-by-reference model (ADR-0013), now unified behind the broker.
//   - inline_encrypted: the locator is an AES-256-GCM blob encrypted at rest with
//     the host master key (BATTOS_MASTER_KEY); decrypted only at resolution time.
//   - keychain: the OS keychain (DPAPI / wincred / Keychain). Not wired yet —
//     returns a clear "not supported in this build" error.
//
// Resolve falls back to os.Getenv when the reference is not a known credential,
// so existing callers (e.g. gitauth) keep working unchanged.
package credstore

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nicotion/battos/apps/api/internal/store"
)

// MasterKeyEnv is the environment variable holding the passphrase from which the
// at-rest encryption key is derived (SHA-256). Empty => inline encryption is
// unavailable and inline credentials cannot be created or resolved.
const MasterKeyEnv = "BATTOS_MASTER_KEY"

// CredentialReader is the subset of the store the broker needs.
type CredentialReader interface {
	GetCredentialByName(ctx context.Context, name string) (store.Credential, error)
}

// Resolver resolves credential references to secret values.
type Resolver struct {
	store     CredentialReader
	masterKey []byte // 32 bytes, or nil when BATTOS_MASTER_KEY is unset
}

// New builds a Resolver. The master key (if BATTOS_MASTER_KEY is set) is derived
// via SHA-256 so any passphrase length yields a valid AES-256 key.
func New(reader CredentialReader) *Resolver {
	var key []byte
	if pass := strings.TrimSpace(os.Getenv(MasterKeyEnv)); pass != "" {
		sum := sha256.Sum256([]byte(pass))
		key = sum[:]
	}
	return &Resolver{store: reader, masterKey: key}
}

// Resolve returns the secret value for ref. If ref names a stored credential its
// configured source is used; otherwise ref is treated as an environment variable
// name (backwards-compatible fallback). An empty ref returns "".
func (r *Resolver) Resolve(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", nil
	}

	cred, err := r.store.GetCredentialByName(ctx, ref)
	if errors.Is(err, sql.ErrNoRows) {
		// Not a managed credential: legacy env-var-name behaviour.
		return strings.TrimSpace(os.Getenv(ref)), nil
	}
	if err != nil {
		return "", fmt.Errorf("credstore: lookup %q: %w", ref, err)
	}

	switch cred.SecretSource {
	case "env":
		return strings.TrimSpace(os.Getenv(cred.SecretLocator)), nil
	case "inline_encrypted":
		secret, err := r.decrypt(cred.SecretLocator)
		if err != nil {
			return "", fmt.Errorf("credstore: decrypt %q: %w", ref, err)
		}
		return secret, nil
	case "keychain":
		return "", fmt.Errorf("credstore: keychain source for %q is not supported in this build", ref)
	default:
		return "", fmt.Errorf("credstore: unknown secret_source %q for %q", cred.SecretSource, ref)
	}
}

// Encrypt seals plaintext into a base64 AES-256-GCM blob for storage as an
// inline_encrypted credential. Requires BATTOS_MASTER_KEY.
func (r *Resolver) Encrypt(plaintext string) (string, error) {
	if r.masterKey == nil {
		return "", fmt.Errorf("credstore: %s is not set; cannot encrypt inline credentials", MasterKeyEnv)
	}
	block, err := aes.NewCipher(r.masterKey)
	if err != nil {
		return "", fmt.Errorf("credstore: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("credstore: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("credstore: nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (r *Resolver) decrypt(blob string) (string, error) {
	if r.masterKey == nil {
		return "", fmt.Errorf("%s is not set", MasterKeyEnv)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(blob))
	if err != nil {
		return "", fmt.Errorf("base64: %w", err)
	}
	block, err := aes.NewCipher(r.masterKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	return string(plain), nil
}
