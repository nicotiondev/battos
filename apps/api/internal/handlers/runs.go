package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nicotion/battos/apps/api/internal/gitauth"
	"github.com/nicotion/battos/apps/api/internal/memory"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type RunStore interface {
	CreateRun(context.Context, store.CreateRunParams) (store.Run, error)
	ListRuns(context.Context) ([]store.Run, error)
	ListRunsByProject(context.Context, string) ([]store.Run, error)
	GetRun(context.Context, pgtype.UUID) (store.Run, error)
	CreateRunApproval(context.Context, store.CreateRunApprovalParams) (store.RunApproval, error)
	UpdateRunStatus(context.Context, store.UpdateRunStatusParams) (store.Run, error)
	EnableRunNetwork(context.Context, pgtype.UUID) (store.Run, error)
	CancelRun(context.Context, pgtype.UUID) (store.Run, error)
	ListRunLogs(context.Context, pgtype.UUID) ([]store.RunLog, error)
	GetArtifactByRunAndKind(context.Context, store.GetArtifactByRunAndKindParams) (store.Artifact, error)
	UpdateRunBranchAndMetadata(context.Context, store.UpdateRunBranchAndMetadataParams) (store.Run, error)
	GetRepository(context.Context, string) (store.Repository, error)
}

type RunHandler struct {
	store  RunStore
	memory *memory.Core
}

func NewRunHandler(q RunStore, mem *memory.Core) *RunHandler {
	return &RunHandler{store: q, memory: mem}
}

type runProposalInput struct {
	ProjectID        string `json:"project_id"`
	TaskID           string `json:"task_id"`
	AgentID          string `json:"agent_id"`
	SkillID          string `json:"skill_id"`
	RuntimeAdapterID string `json:"runtime_adapter_id"`
	RepositoryID     string `json:"repository_id"`
	Prompt           string `json:"prompt"`
	RequestedNetwork bool   `json:"requested_network"`
}

type runApprovalInput struct {
	Kind     string `json:"kind"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

type runResponse struct {
	ID               string    `json:"id"`
	ProjectID        string    `json:"project_id"`
	TaskID           string    `json:"task_id"`
	AgentID          string    `json:"agent_id"`
	SkillID          string    `json:"skill_id,omitempty"`
	RuntimeAdapterID string    `json:"runtime_adapter_id"`
	RepositoryID     string    `json:"repository_id,omitempty"`
	Prompt           string    `json:"prompt"`
	RequestedNetwork bool      `json:"requested_network"`
	NetworkEnabled   bool      `json:"network_enabled"`
	Status           string    `json:"status"`
	BranchName       string    `json:"branch_name,omitempty"`
	ResultSummary    string    `json:"result_summary,omitempty"`
	ErrorMessage     string    `json:"error_message,omitempty"`
	EstimatedCostUSD float64   `json:"estimated_cost_usd"`
	StartedAt        time.Time `json:"started_at,omitempty"`
	CompletedAt      time.Time `json:"completed_at,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type runApprovalResponse struct {
	ID        string    `json:"id"`
	RunID     string    `json:"run_id"`
	Kind      string    `json:"kind"`
	Decision  string    `json:"decision"`
	Reason    string    `json:"reason,omitempty"`
	DecidedAt time.Time `json:"decided_at"`
}

type runLogResponse struct {
	ID        int64     `json:"id"`
	RunID     string    `json:"run_id"`
	Stream    string    `json:"stream"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *RunHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	var (
		items []store.Run
		err   error
	)
	if projectID == "" {
		items, err = h.store.ListRuns(r.Context())
	} else {
		items, err = h.store.ListRunsByProject(r.Context(), projectID)
	}
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]runResponse, 0, len(items))
	for _, item := range items {
		out = append(out, runDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *RunHandler) CreateRun(w http.ResponseWriter, r *http.Request) {
	var in runProposalInput
	if !decodeWorkInput(w, r, &in) ||
		!required(w, in.ProjectID, "project_id") ||
		!required(w, in.TaskID, "task_id") ||
		!required(w, in.AgentID, "agent_id") ||
		!required(w, in.RuntimeAdapterID, "runtime_adapter_id") ||
		!required(w, in.Prompt, "prompt") {
		return
	}
	item, err := h.store.CreateRun(r.Context(), store.CreateRunParams{
		ProjectID:        strings.TrimSpace(in.ProjectID),
		TaskID:           strings.TrimSpace(in.TaskID),
		AgentID:          strings.TrimSpace(in.AgentID),
		SkillID:          nullableText(in.SkillID),
		RuntimeAdapterID: strings.TrimSpace(in.RuntimeAdapterID),
		RepositoryID:     nullableText(in.RepositoryID),
		Prompt:           strings.TrimSpace(in.Prompt),
		RequestedNetwork: in.RequestedNetwork,
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, runDTO(item))
}

func (h *RunHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	runID, ok := runIDFromPath(w, r)
	if !ok {
		return
	}
	item, err := h.store.GetRun(r.Context(), runID)
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runDTO(item))
}

func (h *RunHandler) ApproveRunAction(w http.ResponseWriter, r *http.Request) {
	runID, ok := runIDFromPath(w, r)
	if !ok {
		return
	}
	current, err := h.store.GetRun(r.Context(), runID)
	if err != nil {
		writeWorkError(w, err)
		return
	}
	var in runApprovalInput
	if !decodeWorkInput(w, r, &in) ||
		!required(w, in.Kind, "kind") ||
		!required(w, in.Decision, "decision") {
		return
	}
	kind := strings.ToLower(strings.TrimSpace(in.Kind))
	decision := strings.ToLower(strings.TrimSpace(in.Decision))
	if !validApprovalKind(kind) || !validApprovalDecision(decision) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "kind o decision invalido", "code": 400}})
		return
	}
	if decision == "approved" {
		if kind == "execute" && current.Status != "draft" && current.Status != "awaiting_approval" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "solo se puede aprobar execute desde draft o awaiting_approval", "code": 400}})
			return
		}
		if kind == "network" && !current.RequestedNetwork {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "este run no solicito red", "code": 400}})
			return
		}
	}
	approval, err := h.store.CreateRunApproval(r.Context(), store.CreateRunApprovalParams{
		RunID:    runID,
		Kind:     kind,
		Decision: decision,
		Reason:   nullableText(in.Reason),
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	updated := current
	if decision == "approved" {
		switch kind {
		case "execute":
			updated, err = h.store.UpdateRunStatus(r.Context(), store.UpdateRunStatusParams{ID: runID, Status: "queued"})
		case "network":
			updated, err = h.store.EnableRunNetwork(r.Context(), runID)
		case "remember":
			if !isTerminalRunStatus(current.Status) {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": fmt.Sprintf("solo se puede aprobar remember en runs terminales; estado actual: %s", current.Status), "code": 400}})
				return
			}
			if h.memory == nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "Memory Core no esta disponible en este handler", "code": 500}})
				return
			}
			runLogs, errLogs := h.store.ListRunLogs(r.Context(), current.ID)
			if errLogs != nil {
				writeWorkError(w, errLogs)
				return
			}

			// Renderizar resumen Markdown
			content := renderRunMemorySummaryBackend(current, runLogs, true, false, 20)
			runUUIDStr := uuidValue(current.ID)

			_, errSave := h.memory.Save(r.Context(), memory.Observation{
				Type:      memory.TypeLearning,
				Title:     fmt.Sprintf("Run %s %s", shortIDValue(runUUIDStr), current.Status),
				Content:   content,
				TopicKey:  fmt.Sprintf("%s/runs/%s/summary", current.ProjectID, runUUIDStr),
				ProjectID: current.ProjectID,
				AgentID:   current.AgentID,
				Scope:     memory.ScopeProject,
			})
			if errSave != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "no se pudo guardar la memoria en el core: " + errSave.Error(), "code": 500}})
				return
			}
		case "commit":
			if current.Status != "succeeded" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "solo se puede aprobar commit en runs exitosos (succeeded)", "code": 400}})
				return
			}
			if !current.RepositoryID.Valid || current.RepositoryID.String == "" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "este run no esta asociado a un repositorio", "code": 400}})
				return
			}
			var meta map[string]any
			if len(current.Metadata) > 0 {
				_ = json.Unmarshal(current.Metadata, &meta)
			}
			if meta == nil {
				meta = make(map[string]any)
			}
			workDir, _ := meta["work_dir"].(string)
			if workDir == "" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "no se encontro directorio de trabajo en la metadata del run o ya fue procesado", "code": 400}})
				return
			}

			// git add .
			cmdAdd := exec.Command("git", "add", ".")
			cmdAdd.Dir = workDir
			if out, errAdd := cmdAdd.CombinedOutput(); errAdd != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": fmt.Sprintf("git add fallo: %v, output: %s", errAdd, string(out)), "code": 500}})
				return
			}

			// git commit -m "battos run <id>"
			runUUIDStr := uuidValue(current.ID)
			cmdCommit := exec.Command("git",
				"-c", "user.name=BattOS",
				"-c", "user.email=battos@local",
				"commit", "-m", fmt.Sprintf("battos run %s", runUUIDStr),
			)
			cmdCommit.Dir = workDir
			if out, errCommit := cmdCommit.CombinedOutput(); errCommit != nil {
				outStr := string(out)
				if !strings.Contains(outStr, "nothing to commit") && !strings.Contains(outStr, "nothing added to commit") {
					writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": fmt.Sprintf("git commit fallo: %v, output: %s", errCommit, outStr), "code": 500}})
					return
				}
			}

		case "push":
			if current.Status != "succeeded" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "solo se puede aprobar push en runs exitosos (succeeded)", "code": 400}})
				return
			}
			if !current.RepositoryID.Valid || current.RepositoryID.String == "" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "este run no esta asociado a un repositorio", "code": 400}})
				return
			}
			var meta map[string]any
			if len(current.Metadata) > 0 {
				_ = json.Unmarshal(current.Metadata, &meta)
			}
			if meta == nil {
				meta = make(map[string]any)
			}
			workDir, _ := meta["work_dir"].(string)
			if workDir == "" {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "no se encontro directorio de trabajo en la metadata del run o ya fue procesado", "code": 400}})
				return
			}

			branchName := current.BranchName.String
			if branchName == "" {
				branchName = fmt.Sprintf("battos-run-%s", uuidValue(current.ID))
			}

			// Destino del push: por defecto el remoto `origin` del workspace
			// (repos managed_local). Para repos github, resolvemos un remoto
			// https autenticado con el token referenciado por credential_ref.
			pushTarget := "origin"
			var gitToken string
			repo, errRepo := h.store.GetRepository(r.Context(), current.RepositoryID.String)
			if errRepo != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "no se pudo leer el repositorio: " + errRepo.Error(), "code": 500}})
				return
			}
			if repo.Kind == "github" {
				remoteURL := strings.TrimSpace(textValue(repo.RemoteUrl))
				if remoteURL == "" {
					writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "el repositorio github no tiene remote_url configurado", "code": 400}})
					return
				}
				gitToken = gitauth.Resolve(textValue(repo.CredentialRef))
				if gitToken == "" {
					writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "credential_ref no resuelve un token; define la variable de entorno referenciada antes de aprobar push", "code": 400}})
					return
				}
				pushTarget = gitauth.AuthenticatedURL(remoteURL, gitToken)
			}

			// git push <target> <branchName>
			cmdPush := exec.Command("git", "push", pushTarget, branchName)
			cmdPush.Dir = workDir
			if out, errPush := cmdPush.CombinedOutput(); errPush != nil {
				msg := gitauth.Redact(fmt.Sprintf("git push fallo: %v, output: %s", errPush, string(out)), gitToken)
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": msg, "code": 500}})
				return
			}

			// Eliminar carpeta temporal
			_ = os.RemoveAll(workDir)

			// Eliminar work_dir de metadata y actualizar
			delete(meta, "work_dir")
			newMetaBytes, errMarshal := json.Marshal(meta)
			if errMarshal == nil {
				updated, err = h.store.UpdateRunBranchAndMetadata(r.Context(), store.UpdateRunBranchAndMetadataParams{
					ID:         current.ID,
					BranchName: current.BranchName,
					Metadata:   newMetaBytes,
				})
				if err != nil {
					writeWorkError(w, err)
					return
				}
			}
		}
		if err != nil {
			writeWorkError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": runDTO(updated), "approval": runApprovalDTO(approval)})
}

func (h *RunHandler) CancelRun(w http.ResponseWriter, r *http.Request) {
	runID, ok := runIDFromPath(w, r)
	if !ok {
		return
	}
	current, err := h.store.GetRun(r.Context(), runID)
	if err != nil {
		writeWorkError(w, err)
		return
	}
	if current.Status == "succeeded" || current.Status == "failed" || current.Status == "cancelled" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "run ya esta en estado terminal", "code": 400}})
		return
	}
	item, err := h.store.CancelRun(r.Context(), runID)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "run no cancelable", "code": 400}})
			return
		}
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runDTO(item))
}

func (h *RunHandler) ListRunLogs(w http.ResponseWriter, r *http.Request) {
	runID, ok := runIDFromPath(w, r)
	if !ok {
		return
	}
	items, err := h.store.ListRunLogs(r.Context(), runID)
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]runLogResponse, 0, len(items))
	for _, item := range items {
		out = append(out, runLogDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *RunHandler) GetRunDiff(w http.ResponseWriter, r *http.Request) {
	runID, ok := runIDFromPath(w, r)
	if !ok {
		return
	}
	art, err := h.store.GetArtifactByRunAndKind(r.Context(), store.GetArtifactByRunAndKindParams{
		RunID: runID,
		Kind:  "diff",
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("(sin cambios de codigo o diff no registrado)"))
			return
		}
		writeWorkError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if art.Content.Valid {
		w.Write([]byte(art.Content.String))
	} else {
		w.Write([]byte("(diff vacio o formato invalido)"))
	}
}

func (h *RunHandler) StreamRunEvents(w http.ResponseWriter, r *http.Request) {
	runID, ok := runIDFromPath(w, r)
	if !ok {
		return
	}
	current, err := h.store.GetRun(r.Context(), runID)
	if err != nil {
		writeWorkError(w, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{"message": "SSE no disponible en este servidor", "code": 500}})
		return
	}

	lastID := parseLastEventID(r)
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	lastStatus := current.Status
	_ = writeSSEEvent(w, "run.snapshot", runDTO(current))
	flusher.Flush()

	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()

	for {
		logs, err := h.store.ListRunLogs(r.Context(), runID)
		if err != nil {
			_ = writeSSEEvent(w, "run.error", map[string]string{"message": err.Error()})
			flusher.Flush()
			return
		}
		for _, item := range logs {
			if item.ID <= lastID {
				continue
			}
			lastID = item.ID
			_ = writeSSEEvent(w, "run.log", runLogDTO(item))
		}

		current, err = h.store.GetRun(r.Context(), runID)
		if err != nil {
			_ = writeSSEEvent(w, "run.error", map[string]string{"message": err.Error()})
			flusher.Flush()
			return
		}
		if current.Status != lastStatus {
			lastStatus = current.Status
			_ = writeSSEEvent(w, "run.snapshot", runDTO(current))
		}
		if isTerminalRunStatus(current.Status) {
			_ = writeSSEEvent(w, "run.done", runDTO(current))
			flusher.Flush()
			return
		}
		flusher.Flush()

		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func runIDFromPath(w http.ResponseWriter, r *http.Request) (pgtype.UUID, bool) {
	return parseUUIDInput(w, chi.URLParam(r, "id"), "id")
}

func validApprovalKind(value string) bool {
	switch value {
	case "execute", "network", "commit", "push", "remember":
		return true
	default:
		return false
	}
}

func validApprovalDecision(value string) bool {
	return value == "approved" || value == "rejected"
}

func isTerminalRunStatus(value string) bool {
	return value == "succeeded" || value == "failed" || value == "cancelled"
}

func parseLastEventID(r *http.Request) int64 {
	raw := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if raw == "" {
		raw = strings.TrimSpace(r.URL.Query().Get("after"))
	}
	if raw == "" {
		return 0
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 0 {
		return 0
	}
	return id
}

func writeSSEEvent(w http.ResponseWriter, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if log, ok := payload.(runLogResponse); ok {
		if _, err := fmt.Fprintf(w, "id: %d\n", log.ID); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	return err
}

func runDTO(item store.Run) runResponse {
	return runResponse{
		ID:               uuidValue(item.ID),
		ProjectID:        item.ProjectID,
		TaskID:           item.TaskID,
		AgentID:          item.AgentID,
		SkillID:          textValue(item.SkillID),
		RuntimeAdapterID: item.RuntimeAdapterID,
		RepositoryID:     textValue(item.RepositoryID),
		Prompt:           item.Prompt,
		RequestedNetwork: item.RequestedNetwork,
		NetworkEnabled:   item.NetworkEnabled,
		Status:           item.Status,
		BranchName:       textValue(item.BranchName),
		ResultSummary:    textValue(item.ResultSummary),
		ErrorMessage:     textValue(item.ErrorMessage),
		EstimatedCostUSD: 0,
		StartedAt:        item.StartedAt.Time,
		CompletedAt:      item.CompletedAt.Time,
		CreatedAt:        item.CreatedAt.Time,
		UpdatedAt:        item.UpdatedAt.Time,
	}
}

func runApprovalDTO(item store.RunApproval) runApprovalResponse {
	return runApprovalResponse{
		ID:        uuidValue(item.ID),
		RunID:     uuidValue(item.RunID),
		Kind:      item.Kind,
		Decision:  item.Decision,
		Reason:    textValue(item.Reason),
		DecidedAt: item.DecidedAt.Time,
	}
}

func runLogDTO(item store.RunLog) runLogResponse {
	return runLogResponse{
		ID:        item.ID,
		RunID:     uuidValue(item.RunID),
		Stream:    item.Stream,
		Message:   item.Message,
		CreatedAt: item.CreatedAt.Time,
	}
}

func renderRunMemorySummaryBackend(run store.Run, logs []store.RunLog, includeLogs, includePrompt bool, logLimit int) string {
	var b strings.Builder
	b.WriteString("# BattOS Run Summary\n\n")
	runIDStr := uuidValue(run.ID)
	b.WriteString(fmt.Sprintf("- Run: %s\n", runIDStr))
	b.WriteString(fmt.Sprintf("- Project: %s\n", run.ProjectID))
	b.WriteString(fmt.Sprintf("- Task: %s\n", run.TaskID))
	b.WriteString(fmt.Sprintf("- Agent: %s\n", run.AgentID))
	b.WriteString(fmt.Sprintf("- Runtime: %s\n", run.RuntimeAdapterID))
	b.WriteString(fmt.Sprintf("- Status: %s\n", run.Status))
	if run.ResultSummary.Valid && run.ResultSummary.String != "" {
		b.WriteString(fmt.Sprintf("- Result: %s\n", run.ResultSummary.String))
	}
	if run.ErrorMessage.Valid && run.ErrorMessage.String != "" {
		b.WriteString(fmt.Sprintf("- Error: %s\n", run.ErrorMessage.String))
	}
	if includePrompt && run.Prompt != "" {
		b.WriteString("\n## Prompt\n\n")
		b.WriteString(strings.TrimSpace(run.Prompt))
		b.WriteString("\n")
	}
	if includeLogs && len(logs) > 0 {
		b.WriteString("\n## Logs\n\n")
		start := 0
		if logLimit > 0 && len(logs) > logLimit {
			start = len(logs) - logLimit
		}
		for i := start; i < len(logs); i++ {
			b.WriteString(fmt.Sprintf("- `%s` %s\n", logs[i].Stream, strings.TrimSpace(logs[i].Message)))
		}
	}
	return b.String()
}

func shortIDValue(value string) string {
	if len(value) <= 8 {
		return value
	}
	return value[:8]
}
