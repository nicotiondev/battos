// credentials.go — handlers HTTP para la Bóveda de credenciales (ADR-0023).
//
// Endpoints:
//
//	GET    /credentials          → lista todas (sin secret_locator)
//	POST   /credentials          → crea una credencial nueva
//	DELETE /credentials/{name}   → elimina por name
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicotion/battos/apps/api/internal/credstore"
	"github.com/nicotion/battos/apps/api/internal/store"
)

// CredentialStore es la interfaz mínima que el handler necesita.
type CredentialStore interface {
	CreateCredential(context.Context, store.CreateCredentialParams) (store.Credential, error)
	ListCredentials(context.Context) ([]store.Credential, error)
	DeleteCredential(context.Context, string) error
}

// CredentialHandler agrupa los endpoints de la Bóveda.
type CredentialHandler struct {
	store    CredentialStore
	resolver *credstore.Resolver
}

// NewCredentialHandler construye el handler con store y resolver inyectados.
func NewCredentialHandler(s CredentialStore, r *credstore.Resolver) *CredentialHandler {
	return &CredentialHandler{store: s, resolver: r}
}

// credentialResponse es el DTO público: NUNCA incluye secret_locator.
type credentialResponse struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Kind         string     `json:"kind"`
	ProviderID   *string    `json:"provider_id,omitempty"`
	SecretSource string     `json:"secret_source"`
	Description  *string    `json:"description,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func credDTO(c store.Credential) credentialResponse {
	dto := credentialResponse{
		ID:           c.ID,
		Name:         c.Name,
		Kind:         c.Kind,
		SecretSource: c.SecretSource,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
	if c.ProviderID.Valid {
		v := c.ProviderID.String
		dto.ProviderID = &v
	}
	if c.Description.Valid {
		v := c.Description.String
		dto.Description = &v
	}
	return dto
}

// List — GET /credentials
func (h *CredentialHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListCredentials(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody(http.StatusInternalServerError, err.Error()))
		return
	}
	out := make([]credentialResponse, 0, len(items))
	for _, c := range items {
		out = append(out, credDTO(c))
	}
	writeJSON(w, http.StatusOK, out)
}

// createCredentialInput es el body esperado en POST /credentials.
type createCredentialInput struct {
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	ProviderID   *string `json:"provider_id,omitempty"`
	Description  *string `json:"description,omitempty"`
	SecretSource string  `json:"secret_source"`
	SecretValue  string  `json:"secret_value"`
}

// Create — POST /credentials
func (h *CredentialHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in createCredentialInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody(http.StatusBadRequest, "JSON inválido: "+err.Error()))
		return
	}

	// Validaciones
	if strings.TrimSpace(in.Name) == "" {
		writeJSON(w, http.StatusBadRequest, errBody(http.StatusBadRequest, "name es obligatorio"))
		return
	}
	validKinds := map[string]bool{"api_key": true, "oauth_token": true, "git_token": true}
	if !validKinds[in.Kind] {
		writeJSON(w, http.StatusBadRequest, errBody(http.StatusBadRequest, "kind debe ser api_key, oauth_token o git_token"))
		return
	}
	validSources := map[string]bool{"env": true, "inline_encrypted": true}
	if !validSources[in.SecretSource] {
		writeJSON(w, http.StatusBadRequest, errBody(http.StatusBadRequest, "secret_source debe ser env o inline_encrypted"))
		return
	}

	// Determinar el secret_locator según la fuente.
	var locator string
	switch in.SecretSource {
	case "inline_encrypted":
		blob, err := h.resolver.Encrypt(in.SecretValue)
		if err != nil {
			// Si falla por falta de master key, el error del credstore ya es descriptivo.
			writeJSON(w, http.StatusBadRequest, errBody(http.StatusBadRequest, err.Error()))
			return
		}
		locator = blob
	case "env":
		locator = in.SecretValue
	}

	params := store.CreateCredentialParams{
		Name:          strings.TrimSpace(in.Name),
		Kind:          in.Kind,
		SecretSource:  in.SecretSource,
		SecretLocator: locator,
	}
	if in.ProviderID != nil && strings.TrimSpace(*in.ProviderID) != "" {
		params.ProviderID = sql.NullString{String: strings.TrimSpace(*in.ProviderID), Valid: true}
	}
	if in.Description != nil && strings.TrimSpace(*in.Description) != "" {
		params.Description = sql.NullString{String: strings.TrimSpace(*in.Description), Valid: true}
	}

	cred, err := h.store.CreateCredential(r.Context(), params)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "UNIQUE constraint failed") {
			writeJSON(w, http.StatusConflict, errBody(http.StatusConflict, "ya existe una credencial con ese nombre"))
			return
		}
		if strings.Contains(msg, "CHECK constraint failed") {
			writeJSON(w, http.StatusBadRequest, errBody(http.StatusBadRequest, "valor inválido para kind o secret_source"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody(http.StatusInternalServerError, msg))
		return
	}

	writeJSON(w, http.StatusCreated, credDTO(cred))
}

// Delete — DELETE /credentials/{name}
func (h *CredentialHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if strings.TrimSpace(name) == "" {
		writeJSON(w, http.StatusBadRequest, errBody(http.StatusBadRequest, "name es obligatorio"))
		return
	}
	if err := h.store.DeleteCredential(r.Context(), name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errBody(http.StatusNotFound, "credencial no encontrada"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody(http.StatusInternalServerError, err.Error()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// errBody construye el envelope de error estándar del proyecto.
func errBody(code int, msg string) map[string]any {
	return map[string]any{"error": map[string]any{"message": msg, "code": code}}
}
