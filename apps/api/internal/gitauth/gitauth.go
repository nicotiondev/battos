// Package gitauth resuelve credenciales para operaciones Git sobre remotos
// autenticados (GitHub).
//
// Modelo de secretos por referencia (ADR-0013): la base de datos guarda
// `credential_ref`, que es el NOMBRE de una variable de entorno; el token real
// vive en el entorno del proceso (infra/.env o shell) y nunca se persiste.
//
// Estas funciones son puras (salvo Resolve, que lee env) para poder testearlas
// sin tocar la red ni Git.
package gitauth

import (
	"net/url"
	"os"
	"strings"
)

// Resolve devuelve el valor del token referenciado por `ref` (un nombre de env
// var). Si `ref` está vacío o la env var no existe, devuelve "".
func Resolve(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	return strings.TrimSpace(os.Getenv(ref))
}

// AuthenticatedURL inyecta el token como credencial en una URL https para que
// `git clone`/`git push` puedan autenticarse sin un credential helper.
//
// Usa el usuario `x-access-token`, compatible con Personal Access Tokens y con
// installation tokens de GitHub. Para URLs no-https (ssh, file://) o token
// vacío devuelve la URL original sin cambios: ahí la auth la maneja la llave
// SSH o no hace falta.
func AuthenticatedURL(remoteURL, token string) string {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" || token == "" {
		return remoteURL
	}
	u, err := url.Parse(remoteURL)
	if err != nil || u.Scheme != "https" {
		// Forma scp/ssh (git@host:path) o cualquier no-https: la auth la maneja
		// la llave SSH, no inyectamos token.
		return remoteURL
	}
	u.User = url.UserPassword("x-access-token", token)
	return u.String()
}

// Redact reemplaza el token por un placeholder en `s`. Defensa en profundidad
// para no filtrar el secreto si una URL autenticada aparece en stdout/stderr de
// Git al fallar.
func Redact(s, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return s
	}
	return strings.ReplaceAll(s, token, "***")
}
