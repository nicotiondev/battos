package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nicotion/battos/apps/api/internal/store"
)

type KnowledgeStore interface {
	CreateKnowledgeWorkspace(context.Context, store.CreateKnowledgeWorkspaceParams) (store.KnowledgeWorkspace, error)
	ListKnowledgeWorkspaces(context.Context) ([]store.KnowledgeWorkspace, error)
	GetKnowledgeWorkspace(context.Context, string) (store.KnowledgeWorkspace, error)
	CreateJournal(context.Context, store.CreateJournalParams) (store.Journal, error)
	ListJournalsByProject(context.Context, string) ([]store.Journal, error)
	GetJournal(context.Context, string) (store.Journal, error)
	CreateArtifact(context.Context, store.CreateArtifactParams) (store.Artifact, error)
	ListArtifactsByProject(context.Context, string) ([]store.Artifact, error)
	GetArtifact(context.Context, string) (store.Artifact, error)
}

type KnowledgeHandler struct {
	store        KnowledgeStore
	artifactsDir string
}

func NewKnowledgeHandler(q KnowledgeStore, artifactsDir string) *KnowledgeHandler {
	return &KnowledgeHandler{store: q, artifactsDir: defaultString(artifactsDir, "data/artifacts")}
}

type knowledgeWorkspaceInput struct {
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Layout    string `json:"layout"`
	Status    string `json:"status"`
}

type journalInput struct {
	WorkspaceID string `json:"workspace_id"`
	ProjectID   string `json:"project_id"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	JournalDate string `json:"journal_date"`
}

type artifactInput struct {
	ProjectID   string `json:"project_id"`
	TaskID      string `json:"task_id"`
	RunID       string `json:"run_id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Bucket      string `json:"bucket"`
	Content     string `json:"content"`
	ManagedPath string `json:"managed_path"`
	ExternalURL string `json:"external_url"`
}

type knowledgeWorkspaceResponse struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Name      string    `json:"name"`
	Layout    string    `json:"layout"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type journalResponse struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	ProjectID   string    `json:"project_id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	JournalDate string    `json:"journal_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type artifactResponse struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	TaskID      string    `json:"task_id,omitempty"`
	RunID       string    `json:"run_id,omitempty"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	Content     string    `json:"content,omitempty"`
	ManagedPath string    `json:"managed_path,omitempty"`
	ExternalURL string    `json:"external_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (h *KnowledgeHandler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListKnowledgeWorkspaces(r.Context())
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]knowledgeWorkspaceResponse, 0, len(items))
	for _, item := range items {
		out = append(out, knowledgeWorkspaceDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *KnowledgeHandler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var in knowledgeWorkspaceInput
	if !decodeWorkInput(w, r, &in) || !required(w, in.ProjectID, "project_id") || !required(w, in.Name, "name") {
		return
	}
	item, err := h.store.CreateKnowledgeWorkspace(r.Context(), store.CreateKnowledgeWorkspaceParams{
		ProjectID: in.ProjectID,
		Name:      in.Name,
		Layout:    defaultString(in.Layout, "raw_wiki_outputs"),
		Status:    defaultString(in.Status, "active"),
		Metadata:  "{}",
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, knowledgeWorkspaceDTO(item))
}

func (h *KnowledgeHandler) ListJournals(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if !required(w, projectID, "project_id") {
		return
	}
	items, err := h.store.ListJournalsByProject(r.Context(), projectID)
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]journalResponse, 0, len(items))
	for _, item := range items {
		out = append(out, journalDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *KnowledgeHandler) CreateJournal(w http.ResponseWriter, r *http.Request) {
	var in journalInput
	if !decodeWorkInput(w, r, &in) || !required(w, in.WorkspaceID, "workspace_id") || !required(w, in.Title, "title") || !required(w, in.Content, "content") {
		return
	}
	workspaceID, ok := parseIDInput(w, in.WorkspaceID, "workspace_id")
	if !ok {
		return
	}
	projectID := strings.TrimSpace(in.ProjectID)
	workspace, err := h.store.GetKnowledgeWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeWorkError(w, err)
		return
	}
	if projectID == "" {
		projectID = workspace.ProjectID
	} else if projectID != workspace.ProjectID {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "project_id no coincide con el workspace", "code": 400}})
		return
	}
	journalDate, ok := parseDateInput(w, in.JournalDate)
	if !ok {
		return
	}
	item, err := h.store.CreateJournal(r.Context(), store.CreateJournalParams{
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
		Title:       in.Title,
		Content:     in.Content,
		JournalDate: journalDate,
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, journalDTO(item))
}

func (h *KnowledgeHandler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if !required(w, projectID, "project_id") {
		return
	}
	items, err := h.store.ListArtifactsByProject(r.Context(), projectID)
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]artifactResponse, 0, len(items))
	for _, item := range items {
		out = append(out, artifactDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *KnowledgeHandler) CreateArtifact(w http.ResponseWriter, r *http.Request) {
	var in artifactInput
	if !decodeWorkInput(w, r, &in) || !required(w, in.ProjectID, "project_id") || !required(w, in.Name, "name") || !required(w, in.Kind, "kind") {
		return
	}
	if strings.TrimSpace(in.Content) == "" && strings.TrimSpace(in.ManagedPath) == "" && strings.TrimSpace(in.ExternalURL) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "content, managed_path o external_url es obligatorio", "code": 400}})
		return
	}
	managedPath, ok := h.resolveManagedArtifact(w, in)
	if !ok {
		return
	}
	runID := sql.NullString{}
	if strings.TrimSpace(in.RunID) != "" {
		parsed, ok := parseIDInput(w, in.RunID, "run_id")
		if !ok {
			return
		}
		runID = sql.NullString{String: parsed, Valid: true}
	}
	item, err := h.store.CreateArtifact(r.Context(), store.CreateArtifactParams{
		ProjectID:   in.ProjectID,
		TaskID:      nullableText(in.TaskID),
		RunID:       runID,
		Name:        in.Name,
		Kind:        in.Kind,
		Content:     nullableText(contentForArtifact(in, managedPath)),
		ManagedPath: nullableText(managedPath),
		ExternalUrl: nullableText(in.ExternalURL),
		Metadata:    "{}",
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, artifactDTO(item))
}

func (h *KnowledgeHandler) resolveManagedArtifact(w http.ResponseWriter, in artifactInput) (string, bool) {
	content := strings.TrimSpace(in.Content)
	managedPath := strings.TrimSpace(in.ManagedPath)
	if managedPath != "" {
		cleaned, ok := cleanManagedArtifactPath(w, managedPath)
		if !ok {
			return "", false
		}
		if content != "" {
			if err := h.writeManagedArtifact(cleaned, content); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "no se pudo escribir artifact gestionado", "code": 500}})
				return "", false
			}
		}
		return cleaned, true
	}
	if content == "" || strings.TrimSpace(in.ExternalURL) != "" {
		return "", true
	}
	bucket, ok := normalizeArtifactBucket(w, in.Bucket)
	if !ok {
		return "", false
	}
	path := filepath.ToSlash(filepath.Join(safePathSegment(in.ProjectID), bucket, managedArtifactFilename(in.Name, in.Kind)))
	if err := h.writeManagedArtifact(path, content); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "no se pudo escribir artifact gestionado", "code": 500}})
		return "", false
	}
	return path, true
}

func (h *KnowledgeHandler) writeManagedArtifact(relativePath, content string) error {
	root, err := filepath.Abs(h.artifactsDir)
	if err != nil {
		return fmt.Errorf("artifacts root: %w", err)
	}
	target := filepath.Join(root, filepath.FromSlash(relativePath))
	if !pathWithin(root, target) {
		return fmt.Errorf("artifact path outside root")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("artifact mkdir: %w", err)
	}
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		return fmt.Errorf("artifact write: %w", err)
	}
	return nil
}

func cleanManagedArtifactPath(w http.ResponseWriter, value string) (string, bool) {
	cleaned := filepath.Clean(filepath.FromSlash(strings.TrimSpace(value)))
	if cleaned == "." || filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "managed_path debe ser relativo y permanecer dentro de artifacts_dir", "code": 400}})
		return "", false
	}
	for _, part := range strings.Split(cleaned, string(filepath.Separator)) {
		if part == ".." {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "managed_path no puede contener traversal", "code": 400}})
			return "", false
		}
	}
	return filepath.ToSlash(cleaned), true
}

func normalizeArtifactBucket(w http.ResponseWriter, value string) (string, bool) {
	bucket := strings.ToLower(defaultString(value, "raw"))
	switch bucket {
	case "raw", "wiki", "outputs":
		return bucket, true
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "bucket debe ser raw, wiki u outputs", "code": 400}})
		return "", false
	}
}

func contentForArtifact(in artifactInput, managedPath string) string {
	if managedPath != "" && strings.TrimSpace(in.Content) != "" {
		return ""
	}
	return in.Content
}

func managedArtifactFilename(name, kind string) string {
	return fmt.Sprintf("%s-%s%s", time.Now().UTC().Format("20060102T150405"), safePathSegment(name), artifactExtension(kind))
}

func artifactExtension(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "image":
		return ".bin"
	case "link":
		return ".url"
	case "diff":
		return ".diff"
	case "build_report":
		return ".md"
	default:
		return ".md"
	}
}

var unsafePathChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safePathSegment(value string) string {
	cleaned := strings.Trim(unsafePathChars.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-"), ".-")
	if cleaned == "" {
		return "artifact"
	}
	return cleaned
}

func pathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return relative == "." || (!strings.HasPrefix(relative, ".."+string(filepath.Separator)) && relative != "..")
}

func parseIDInput(w http.ResponseWriter, value, field string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": field + " es obligatorio", "code": 400}})
		return "", false
	}
	return value, true
}

func parseDateInput(w http.ResponseWriter, value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now(), true
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "journal_date debe usar formato YYYY-MM-DD", "code": 400}})
		return time.Time{}, false
	}
	return parsed, true
}

func dateValue(value time.Time) string {
	return value.Format("2006-01-02")
}

func knowledgeWorkspaceDTO(item store.KnowledgeWorkspace) knowledgeWorkspaceResponse {
	return knowledgeWorkspaceResponse{item.ID, item.ProjectID, item.Name, item.Layout, item.Status, item.CreatedAt, item.UpdatedAt}
}

func journalDTO(item store.Journal) journalResponse {
	return journalResponse{item.ID, item.WorkspaceID, item.ProjectID, item.Title, item.Content, dateValue(item.JournalDate), item.CreatedAt, item.UpdatedAt}
}

func artifactDTO(item store.Artifact) artifactResponse {
	return artifactResponse{
		ID:          item.ID,
		ProjectID:   item.ProjectID,
		TaskID:      textValue(item.TaskID),
		RunID:       textValue(item.RunID),
		Name:        item.Name,
		Kind:        item.Kind,
		Content:     textValue(item.Content),
		ManagedPath: textValue(item.ManagedPath),
		ExternalURL: textValue(item.ExternalUrl),
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}
