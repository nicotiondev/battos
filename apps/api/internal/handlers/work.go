package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nicotion/battos/apps/api/internal/store"
)

// WorkStore is the typed persistence surface used by the Work Board API.
type WorkStore interface {
	CreateDomain(context.Context, store.CreateDomainParams) (store.Domain, error)
	ListDomains(context.Context) ([]store.Domain, error)
	GetDomain(context.Context, string) (store.Domain, error)
	UpdateDomain(context.Context, store.UpdateDomainParams) (store.Domain, error)
	CreateProject(context.Context, store.CreateProjectParams) (store.Project, error)
	ListProjects(context.Context) ([]store.Project, error)
	GetProject(context.Context, string) (store.Project, error)
	UpdateProject(context.Context, store.UpdateProjectParams) (store.Project, error)
	CreateGoal(context.Context, store.CreateGoalParams) (store.Goal, error)
	ListGoalsByProject(context.Context, string) ([]store.Goal, error)
	GetGoal(context.Context, string) (store.Goal, error)
	UpdateGoal(context.Context, store.UpdateGoalParams) (store.Goal, error)
	CreateTask(context.Context, store.CreateTaskParams) (store.Task, error)
	ListTasksByProject(context.Context, string) ([]store.Task, error)
	GetTask(context.Context, string) (store.Task, error)
	UpdateTask(context.Context, store.UpdateTaskParams) (store.Task, error)
}

type WorkHandler struct {
	store WorkStore
}

func NewWorkHandler(q WorkStore) *WorkHandler {
	return &WorkHandler{store: q}
}

type domainInput struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type projectInput struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DomainID    string `json:"domain_id"`
	Status      string `json:"status"`
}

type goalInput struct {
	ProjectID   string `json:"project_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type taskInput struct {
	ProjectID       string `json:"project_id"`
	GoalID          string `json:"goal_id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	AssignedAgentID string `json:"assigned_agent_id"`
	Status          string `json:"status"`
	BoardPosition   int32  `json:"board_position"`
}

type domainPatch struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

type projectPatch struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	DomainID    *string `json:"domain_id"`
	Status      *string `json:"status"`
}

type goalPatch struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

type taskPatch struct {
	GoalID          *string `json:"goal_id"`
	Title           *string `json:"title"`
	Description     *string `json:"description"`
	AssignedAgentID *string `json:"assigned_agent_id"`
	Status          *string `json:"status"`
	BoardPosition   *int32  `json:"board_position"`
}

type domainResponse struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type projectResponse struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	DomainID    string    `json:"domain_id,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type goalResponse struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type taskResponse struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	GoalID          string    `json:"goal_id,omitempty"`
	Title           string    `json:"title"`
	Description     string    `json:"description,omitempty"`
	AssignedAgentID string    `json:"assigned_agent_id,omitempty"`
	Status          string    `json:"status"`
	BoardPosition   int32     `json:"board_position"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (h *WorkHandler) ListDomains(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListDomains(r.Context())
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]domainResponse, 0, len(items))
	for _, item := range items {
		out = append(out, domainDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *WorkHandler) CreateDomain(w http.ResponseWriter, r *http.Request) {
	var in domainInput
	if !decodeWorkInput(w, r, &in) || !required(w, in.Slug, "slug") || !required(w, in.Name, "name") {
		return
	}
	item, err := h.store.CreateDomain(r.Context(), store.CreateDomainParams{
		ID:          in.Slug,
		Slug:        in.Slug,
		Name:        in.Name,
		Description: nullableText(in.Description),
		Status:      defaultString(in.Status, "active"),
		Metadata:    []byte("{}"),
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, domainDTO(item))
}

func (h *WorkHandler) GetDomain(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetDomain(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, domainDTO(item))
}

func (h *WorkHandler) UpdateDomain(w http.ResponseWriter, r *http.Request) {
	var in domainPatch
	if !decodeWorkInput(w, r, &in) {
		return
	}
	if in.Name != nil && !required(w, *in.Name, "name") {
		return
	}
	current, err := h.store.GetDomain(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeWorkError(w, err)
		return
	}
	item, err := h.store.UpdateDomain(r.Context(), store.UpdateDomainParams{
		ID:          chi.URLParam(r, "id"),
		Name:        patchedString(in.Name, current.Name),
		Description: nullableText(patchedString(in.Description, textValue(current.Description))),
		Status:      patchedString(in.Status, current.Status),
		Metadata:    []byte("{}"),
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, domainDTO(item))
}

func (h *WorkHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListProjects(r.Context())
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]projectResponse, 0, len(items))
	for _, item := range items {
		out = append(out, projectDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *WorkHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var in projectInput
	if !decodeWorkInput(w, r, &in) || !required(w, in.Slug, "slug") || !required(w, in.Name, "name") {
		return
	}
	item, err := h.store.CreateProject(r.Context(), store.CreateProjectParams{
		ID:          in.Slug,
		Slug:        in.Slug,
		Name:        in.Name,
		Description: nullableText(in.Description),
		DomainID:    nullableText(in.DomainID),
		Status:      defaultString(in.Status, "active"),
		Metadata:    []byte("{}"),
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, projectDTO(item))
}

func (h *WorkHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetProject(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projectDTO(item))
}

func (h *WorkHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	var in projectPatch
	if !decodeWorkInput(w, r, &in) {
		return
	}
	if in.Name != nil && !required(w, *in.Name, "name") {
		return
	}
	current, err := h.store.GetProject(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeWorkError(w, err)
		return
	}
	item, err := h.store.UpdateProject(r.Context(), store.UpdateProjectParams{
		ID:          chi.URLParam(r, "id"),
		Name:        patchedString(in.Name, current.Name),
		Description: nullableText(patchedString(in.Description, textValue(current.Description))),
		DomainID:    nullableText(patchedString(in.DomainID, textValue(current.DomainID))),
		Status:      patchedString(in.Status, current.Status),
		Metadata:    []byte("{}"),
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projectDTO(item))
}

func (h *WorkHandler) ListGoals(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if !required(w, projectID, "project_id") {
		return
	}
	items, err := h.store.ListGoalsByProject(r.Context(), projectID)
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]goalResponse, 0, len(items))
	for _, item := range items {
		out = append(out, goalDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *WorkHandler) CreateGoal(w http.ResponseWriter, r *http.Request) {
	var in goalInput
	if !decodeWorkInput(w, r, &in) || !required(w, in.ProjectID, "project_id") || !required(w, in.Title, "title") {
		return
	}
	item, err := h.store.CreateGoal(r.Context(), store.CreateGoalParams{
		ProjectID:   in.ProjectID,
		Title:       in.Title,
		Description: nullableText(in.Description),
		Status:      defaultString(in.Status, "planned"),
		Metadata:    []byte("{}"),
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, goalDTO(item))
}

func (h *WorkHandler) GetGoal(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetGoal(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, goalDTO(item))
}

func (h *WorkHandler) UpdateGoal(w http.ResponseWriter, r *http.Request) {
	var in goalPatch
	if !decodeWorkInput(w, r, &in) {
		return
	}
	if in.Title != nil && !required(w, *in.Title, "title") {
		return
	}
	current, err := h.store.GetGoal(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeWorkError(w, err)
		return
	}
	item, err := h.store.UpdateGoal(r.Context(), store.UpdateGoalParams{
		ID:          chi.URLParam(r, "id"),
		Title:       patchedString(in.Title, current.Title),
		Description: nullableText(patchedString(in.Description, textValue(current.Description))),
		Status:      patchedString(in.Status, current.Status),
		Metadata:    []byte("{}"),
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, goalDTO(item))
}

func (h *WorkHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if !required(w, projectID, "project_id") {
		return
	}
	items, err := h.store.ListTasksByProject(r.Context(), projectID)
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]taskResponse, 0, len(items))
	for _, item := range items {
		out = append(out, taskDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *WorkHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var in taskInput
	if !decodeWorkInput(w, r, &in) || !required(w, in.ProjectID, "project_id") || !required(w, in.Title, "title") {
		return
	}
	item, err := h.store.CreateTask(r.Context(), store.CreateTaskParams{
		ProjectID:       in.ProjectID,
		GoalID:          nullableText(in.GoalID),
		Title:           in.Title,
		Description:     nullableText(in.Description),
		AssignedAgentID: nullableText(in.AssignedAgentID),
		Status:          defaultString(in.Status, "backlog"),
		BoardPosition:   in.BoardPosition,
		Metadata:        []byte("{}"),
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, taskDTO(item))
}

func (h *WorkHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	item, err := h.store.GetTask(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, taskDTO(item))
}

func (h *WorkHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	var in taskPatch
	if !decodeWorkInput(w, r, &in) {
		return
	}
	if in.Title != nil && !required(w, *in.Title, "title") {
		return
	}
	current, err := h.store.GetTask(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeWorkError(w, err)
		return
	}
	item, err := h.store.UpdateTask(r.Context(), store.UpdateTaskParams{
		ID:              chi.URLParam(r, "id"),
		GoalID:          nullableText(patchedString(in.GoalID, textValue(current.GoalID))),
		Title:           patchedString(in.Title, current.Title),
		Description:     nullableText(patchedString(in.Description, textValue(current.Description))),
		AssignedAgentID: nullableText(patchedString(in.AssignedAgentID, textValue(current.AssignedAgentID))),
		Status:          patchedString(in.Status, current.Status),
		BoardPosition:   patchedInt32(in.BoardPosition, current.BoardPosition),
		Metadata:        []byte("{}"),
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, taskDTO(item))
}

func decodeWorkInput(w http.ResponseWriter, r *http.Request, out any) bool {
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "JSON invalido: " + err.Error(), "code": 400}})
		return false
	}
	return true
}

func required(w http.ResponseWriter, value, field string) bool {
	if strings.TrimSpace(value) != "" {
		return true
	}
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": field + " es obligatorio", "code": 400}})
	return false
}

func nullableText(value string) pgtype.Text {
	value = strings.TrimSpace(value)
	return pgtype.Text{String: value, Valid: value != ""}
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func patchedString(value *string, current string) string {
	if value != nil {
		return *value
	}
	return current
}

func patchedInt32(value *int32, current int32) int32 {
	if value != nil {
		return *value
	}
	return current
}

func writeWorkError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "recurso no encontrado", "code": 404}})
		return
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"message": "el recurso ya existe", "code": 409}})
			return
		case "23503", "23514":
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "referencia o estado invalido", "code": 400}})
			return
		}
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "error persistiendo work board", "code": 500}})
}

func textValue(value pgtype.Text) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func domainDTO(item store.Domain) domainResponse {
	return domainResponse{item.ID, item.Slug, item.Name, textValue(item.Description), item.Status, item.CreatedAt.Time, item.UpdatedAt.Time}
}

func projectDTO(item store.Project) projectResponse {
	return projectResponse{item.ID, item.Slug, item.Name, textValue(item.Description), textValue(item.DomainID), item.Status, item.CreatedAt.Time, item.UpdatedAt.Time}
}

func goalDTO(item store.Goal) goalResponse {
	return goalResponse{item.ID, item.ProjectID, item.Title, textValue(item.Description), item.Status, item.CreatedAt.Time, item.UpdatedAt.Time}
}

func taskDTO(item store.Task) taskResponse {
	return taskResponse{item.ID, item.ProjectID, textValue(item.GoalID), item.Title, textValue(item.Description), textValue(item.AssignedAgentID), item.Status, item.BoardPosition, item.CreatedAt.Time, item.UpdatedAt.Time}
}
