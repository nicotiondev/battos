package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type RepositoriesStore interface {
	CreateRepository(context.Context, store.CreateRepositoryParams) (store.Repository, error)
	ListRepositories(context.Context) ([]store.Repository, error)
	ListRepositoriesByProject(context.Context, string) ([]store.Repository, error)
	GetRepository(context.Context, string) (store.Repository, error)
	DeleteRepository(context.Context, string) (store.Repository, error)
	GetProject(context.Context, string) (store.Project, error)
}

type RepositoriesHandler struct {
	store           RepositoriesStore
	repositoriesDir string
}

func NewRepositoriesHandler(q RepositoriesStore, reposDir string) *RepositoriesHandler {
	return &RepositoriesHandler{
		store:           q,
		repositoriesDir: defaultString(reposDir, "data/repositories"),
	}
}

type repositoryInput struct {
	ProjectID     string `json:"project_id"`
	Kind          string `json:"kind"` // managed_local | github
	Name          string `json:"name"`
	RemoteURL     string `json:"remote_url"`
	CredentialRef string `json:"credential_ref"`
}

type repositoryResponse struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	Kind          string    `json:"kind"`
	Name          string    `json:"name"`
	RemoteURL     string    `json:"remote_url,omitempty"`
	CredentialRef string    `json:"credential_ref,omitempty"`
	DefaultBranch string    `json:"default_branch"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (h *RepositoriesHandler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	var (
		items []store.Repository
		err   error
	)
	if strings.TrimSpace(projectID) == "" {
		items, err = h.store.ListRepositories(r.Context())
	} else {
		items, err = h.store.ListRepositoriesByProject(r.Context(), projectID)
	}
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]repositoryResponse, 0, len(items))
	for _, item := range items {
		out = append(out, repositoryDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *RepositoriesHandler) ConnectRepository(w http.ResponseWriter, r *http.Request) {
	var in repositoryInput
	if !decodeWorkInput(w, r, &in) || !required(w, in.ProjectID, "project_id") || !required(w, in.Kind, "kind") || !required(w, in.Name, "name") {
		return
	}

	if in.Kind != "managed_local" && in.Kind != "github" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "kind debe ser managed_local o github", "code": 400}})
		return
	}

	// Un repo github necesita remote_url para clonar/pushear; credential_ref es
	// opcional al conectar (puede definirse luego), pero sin remote no hay destino.
	if in.Kind == "github" && strings.TrimSpace(in.RemoteURL) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "un repositorio github requiere remote_url", "code": 400}})
		return
	}

	// Verificar si el proyecto existe
	_, errProj := h.store.GetProject(r.Context(), in.ProjectID)
	if errProj != nil {
		if errors.Is(errProj, pgx.ErrNoRows) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "el project_id especificado no existe", "code": 400}})
			return
		}
		writeWorkError(w, errProj)
		return
	}

	// Crear el registro de repositorio
	id := fmt.Sprintf("repo-%s-%s", safePathSegment(in.ProjectID), safePathSegment(in.Name))
	
	// Si es managed_local, inicializamos el repo git
	if in.Kind == "managed_local" {
		if err := h.initLocalRepo(id); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "no se pudo inicializar el repositorio git local: " + err.Error(), "code": 500}})
			return
		}
	}

	item, err := h.store.CreateRepository(r.Context(), store.CreateRepositoryParams{
		ID:            id,
		ProjectID:     in.ProjectID,
		Kind:          in.Kind,
		Name:          in.Name,
		RemoteUrl:     nullableText(in.RemoteURL),
		CredentialRef: nullableText(in.CredentialRef),
		DefaultBranch: "master",
		Metadata:      []byte("{}"),
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, repositoryDTO(item))
}

func (h *RepositoriesHandler) initLocalRepo(id string) error {
	root, err := filepath.Abs(h.repositoriesDir)
	if err != nil {
		return fmt.Errorf("resolve repos root: %w", err)
	}
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	// 1. git init
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git init: %w", err)
	}

	// 2. README.md
	readmePath := filepath.Join(dir, "README.md")
	readmeContent := fmt.Sprintf("# BattOS Managed Repository: %s\n\nRepositorio local gestionado de forma automatica por el agente.\n", id)
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0o644); err != nil {
		return fmt.Errorf("write readme: %w", err)
	}

	// 3. git add .
	cmdAdd := exec.Command("git", "add", ".")
	cmdAdd.Dir = dir
	if err := cmdAdd.Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// 4. git commit
	cmdCommit := exec.Command("git", 
		"-c", "user.name=BattOS", 
		"-c", "user.email=battos@local", 
		"commit", "-m", "initial commit",
	)
	cmdCommit.Dir = dir
	if err := cmdCommit.Run(); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

func repositoryDTO(item store.Repository) repositoryResponse {
	return repositoryResponse{
		ID:            item.ID,
		ProjectID:     item.ProjectID,
		Kind:          item.Kind,
		Name:          item.Name,
		RemoteURL:     textValue(item.RemoteUrl),
		CredentialRef: textValue(item.CredentialRef),
		DefaultBranch: item.DefaultBranch,
		CreatedAt:     item.CreatedAt.Time,
		UpdatedAt:     item.UpdatedAt.Time,
	}
}
