package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicotion/battos/apps/api/internal/store"
	"github.com/nicotion/battos/apps/api/internal/worker"
)

type RuntimeStore interface {
	ListAgentRuntimes(context.Context) ([]store.AgentRuntime, error)
	UpdateAgentRuntimeDetection(context.Context, store.UpdateAgentRuntimeDetectionParams) (store.AgentRuntime, error)
	ListCLITools(context.Context) ([]store.CliTool, error)
	GetCLITool(context.Context, string) (store.CliTool, error)
	UpsertCLIToolDetection(context.Context, store.UpsertCLIToolDetectionParams) (store.CliTool, error)
	ListProviders(context.Context) ([]store.Provider, error)
	UpdateProviderStatus(context.Context, store.UpdateProviderStatusParams) error
	CreateCLIToolInstall(context.Context, store.CreateCLIToolInstallParams) (store.CliToolInstall, error)
	GetCLIToolInstall(context.Context, string) (store.CliToolInstall, error)
	ListCLIToolInstalls(context.Context, string) ([]store.CliToolInstall, error)
	DecideCLIToolInstall(context.Context, store.DecideCLIToolInstallParams) (store.CliToolInstall, error)
	FinishCLIToolInstall(context.Context, store.FinishCLIToolInstallParams) (store.CliToolInstall, error)
}

type RuntimeHandler struct {
	store          RuntimeStore
	lookPath       func(string) (string, error)
	commandVersion func(context.Context, string) (string, error)
	getenv         func(string) string
	now            func() time.Time
	// installRunner ejecuta un comando de instalación en el host y devuelve su
	// output combinado. Inyectable para tests.
	installRunner func(context.Context, string) (string, error)
	// spawn corre la instalación aprobada; en producción es una goroutine,
	// en tests se reemplaza por ejecución síncrona.
	spawn func(func())
}

func NewRuntimeHandler(q RuntimeStore) *RuntimeHandler {
	return &RuntimeHandler{
		store:          q,
		lookPath:       exec.LookPath,
		commandVersion: detectCommandVersion,
		getenv:         os.Getenv,
		now:            time.Now,
		installRunner:  runHostInstall,
		spawn:          func(f func()) { go f() },
	}
}

type runtimeToolSpec struct {
	ID             string
	Name           string
	Command        string
	Kind           string
	RuntimeID      string
	RiskLevel      string
	RequiresAuth   bool
	ProviderEnv    string
	Capabilities   string
	InstallCommand string
	InstallURL     string
}

type runtimeAdapterResponse struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	Status               string    `json:"status"`
	Version              string    `json:"version,omitempty"`
	Executable           string    `json:"executable,omitempty"`
	Command              string    `json:"command,omitempty"`
	ApprovalRequired     bool      `json:"approval_required"`
	ApprovedForExecution bool      `json:"approved_for_execution"`
	RequiresAuth         bool      `json:"requires_auth"`
	LastDetectedAt       time.Time `json:"last_detected_at,omitempty"`
}

type cliToolResponse struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Command        string    `json:"command"`
	Kind           string    `json:"kind"`
	Status         string    `json:"status"`
	DetectedPath   string    `json:"detected_path,omitempty"`
	Version        string    `json:"version,omitempty"`
	RuntimeID      string    `json:"runtime_id,omitempty"`
	RiskLevel      string    `json:"risk_level"`
	RequiresAuth   bool      `json:"requires_auth"`
	InstallCommand string    `json:"install_command,omitempty"`
	InstallURL     string    `json:"install_url,omitempty"`
	LastDetectedAt time.Time `json:"last_detected_at,omitempty"`
}

type cliToolInstallResponse struct {
	ID             string    `json:"id"`
	CliToolID      string    `json:"cli_tool_id"`
	InstallCommand string    `json:"install_command"`
	Status         string    `json:"status"`
	Reason         string    `json:"reason,omitempty"`
	Output         string    `json:"output,omitempty"`
	RequestedAt    time.Time `json:"requested_at"`
	DecidedAt      time.Time `json:"decided_at,omitempty"`
	CompletedAt    time.Time `json:"completed_at,omitempty"`
}

type cliToolInstallDecisionInput struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

type providerResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	EnvKey      string    `json:"env_key"`
	Status      string    `json:"status"`
	LastCheckAt time.Time `json:"last_check_at,omitempty"`
}

var approvedRuntimeTools = []runtimeToolSpec{
	{
		ID:             "claude-code",
		Name:           "Claude Code",
		Command:        "claude",
		Kind:           "coding_agent",
		RuntimeID:      "claude-code",
		RiskLevel:      "high",
		RequiresAuth:   true,
		ProviderEnv:    "ANTHROPIC_API_KEY",
		Capabilities:   `["code_editing","file_reading","terminal_commands","mcp"]`,
		InstallCommand: "npm install -g @anthropic-ai/claude-code",
		InstallURL:     "https://docs.claude.com/en/docs/claude-code/setup",
	},
	{
		ID:             "claude-code-host-session",
		Name:           "Claude Code (host session)",
		Command:        "claude",
		Kind:           "coding_agent",
		RuntimeID:      "claude-code-host-session",
		RiskLevel:      "high",
		RequiresAuth:   true,
		Capabilities:   `["code_editing","file_reading","terminal_commands","mcp","host_session"]`,
		InstallCommand: "npm install -g @anthropic-ai/claude-code",
		InstallURL:     "https://docs.claude.com/en/docs/claude-code/setup",
	},
	{
		ID:             "codex",
		Name:           "Codex CLI",
		Command:        "codex",
		Kind:           "coding_agent",
		RuntimeID:      "codex",
		RiskLevel:      "high",
		RequiresAuth:   true,
		ProviderEnv:    "OPENAI_API_KEY",
		Capabilities:   `["code_generation","repo_editing","tests"]`,
		InstallCommand: "npm install -g @openai/codex",
		InstallURL:     "https://github.com/openai/codex",
	},
	{
		ID:             "codex-host-session",
		Name:           "Codex CLI (host session)",
		Command:        "codex",
		Kind:           "coding_agent",
		RuntimeID:      "codex-host-session",
		RiskLevel:      "high",
		RequiresAuth:   true,
		Capabilities:   `["code_generation","repo_editing","tests","host_session"]`,
		InstallCommand: "npm install -g @openai/codex",
		InstallURL:     "https://github.com/openai/codex",
	},
	{
		ID:             "gemini",
		Name:           "Gemini CLI",
		Command:        "gemini",
		Kind:           "coding_agent",
		RuntimeID:      "gemini",
		RiskLevel:      "medium",
		RequiresAuth:   true,
		ProviderEnv:    "GEMINI_API_KEY",
		Capabilities:   `["long_context","multimodal","code_generation"]`,
		InstallCommand: "npm install -g @google/gemini-cli",
		InstallURL:     "https://github.com/google-gemini/gemini-cli",
	},
	{
		ID:             "pi",
		Name:           "Pi",
		Command:        "pi",
		Kind:           "coding_agent",
		RuntimeID:      "pi",
		RiskLevel:      "medium",
		RequiresAuth:   true,
		Capabilities:   `["code_editing","terminal","skills"]`,
		InstallCommand: "npm install -g @earendil-works/pi-coding-agent",
		InstallURL:     "https://www.npmjs.com/package/@earendil-works/pi-coding-agent",
	},
	{
		ID:             "opencode",
		Name:           "OpenCode",
		Command:        "opencode",
		Kind:           "coding_agent",
		RuntimeID:      "opencode",
		RiskLevel:      "medium",
		RequiresAuth:   true,
		Capabilities:   `["code_editing","local_agent","terminal_workflows"]`,
		InstallCommand: "npm install -g opencode-ai",
		InstallURL:     "https://opencode.ai",
	},
}

// specForCLITool busca el spec aprobado para un cli_tool id. La instalación
// solo re-detecta tools que están en esta lista.
func specForCLITool(id string) (runtimeToolSpec, bool) {
	for _, spec := range approvedRuntimeTools {
		if spec.ID == id {
			return spec, true
		}
	}
	return runtimeToolSpec{}, false
}

func (h *RuntimeHandler) ListRuntimeAdapters(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListAgentRuntimes(r.Context())
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]runtimeAdapterResponse, 0, len(items))
	for _, item := range items {
		out = append(out, runtimeAdapterDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *RuntimeHandler) DetectRuntimeAdapters(w http.ResponseWriter, r *http.Request) {
	out := make([]runtimeAdapterResponse, 0, len(approvedRuntimeTools))
	for _, spec := range approvedRuntimeTools {
		runtime, err := h.detectRuntime(r.Context(), spec)
		if err != nil {
			writeWorkError(w, err)
			return
		}
		out = append(out, runtimeAdapterDTO(runtime))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *RuntimeHandler) ListCLITools(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListCLITools(r.Context())
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]cliToolResponse, 0, len(items))
	for _, item := range items {
		out = append(out, cliToolDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *RuntimeHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListProviders(r.Context())
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]providerResponse, 0, len(items))
	for _, item := range items {
		out = append(out, providerDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *RuntimeHandler) DetectProviders(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ListProviders(r.Context())
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]providerResponse, 0, len(items))
	for _, item := range items {
		status := "not_configured"
		if strings.TrimSpace(item.EnvKey) != "" && strings.TrimSpace(h.getenv(item.EnvKey)) != "" {
			status = "configured"
		}
		if err := h.store.UpdateProviderStatus(r.Context(), store.UpdateProviderStatusParams{ID: item.ID, Status: status}); err != nil {
			writeWorkError(w, err)
			return
		}
		item.Status = status
		item.LastCheckAt = sql.NullTime{Time: h.now(), Valid: true}
		out = append(out, providerDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

// RequestCLIToolInstall crea una solicitud de instalación para una CLI
// registrada. No ejecuta nada: queda pending_approval hasta el approve
// explícito (mismo modelo HITL que run_approvals/host_session).
//
//	POST /cli-tools/{id}/install
func (h *RuntimeHandler) RequestCLIToolInstall(w http.ResponseWriter, r *http.Request) {
	toolID := strings.TrimSpace(chi.URLParam(r, "id"))
	if toolID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "id de CLI requerido", "code": 400}})
		return
	}
	tool, err := h.store.GetCLITool(r.Context(), toolID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]any{"message": "CLI no registrada; corre la deteccion primero", "code": 404}})
			return
		}
		writeWorkError(w, err)
		return
	}
	installCommand := strings.TrimSpace(textValue(tool.InstallCommand))
	if installCommand == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "esta CLI no tiene comando de instalacion registrado", "code": 400}})
		return
	}
	install, err := h.store.CreateCLIToolInstall(r.Context(), store.CreateCLIToolInstallParams{
		CliToolID:      tool.ID,
		InstallCommand: installCommand,
	})
	if err != nil {
		writeWorkError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"install": cliToolInstallDTO(install)})
}

// ApproveCLIToolInstall decide una solicitud de instalación. approved dispara
// la ejecución del install_command en el host en background; rejected solo
// registra la decisión. La respuesta vuelve inmediata con status running.
//
//	POST /cli-tools/installs/{installId}/approve
func (h *RuntimeHandler) ApproveCLIToolInstall(w http.ResponseWriter, r *http.Request) {
	installID := strings.TrimSpace(chi.URLParam(r, "installId"))
	if installID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "id de instalacion requerido", "code": 400}})
		return
	}
	var in cliToolInstallDecisionInput
	if !decodeWorkInput(w, r, &in) || !required(w, in.Decision, "decision") {
		return
	}
	decision := strings.ToLower(strings.TrimSpace(in.Decision))
	if !validApprovalDecision(decision) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "decision invalida; valores permitidos: approved, rejected", "code": 400}})
		return
	}
	status := "rejected"
	if decision == "approved" {
		status = "running"
	}
	install, err := h.store.DecideCLIToolInstall(r.Context(), store.DecideCLIToolInstallParams{
		ID:     installID,
		Status: status,
		Reason: nullableText(in.Reason),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "la solicitud no existe o ya fue decidida", "code": 400}})
			return
		}
		writeWorkError(w, err)
		return
	}
	if decision == "approved" {
		h.spawn(func() { h.executeInstall(install) })
	}
	writeJSON(w, http.StatusOK, map[string]any{"install": cliToolInstallDTO(install)})
}

// ListCLIToolInstallHistory lista las solicitudes de instalación de una CLI,
// más reciente primero. El dashboard pollea esto hasta estado terminal.
//
//	GET /cli-tools/{id}/installs
func (h *RuntimeHandler) ListCLIToolInstallHistory(w http.ResponseWriter, r *http.Request) {
	toolID := strings.TrimSpace(chi.URLParam(r, "id"))
	if toolID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "id de CLI requerido", "code": 400}})
		return
	}
	items, err := h.store.ListCLIToolInstalls(r.Context(), toolID)
	if err != nil {
		writeWorkError(w, err)
		return
	}
	out := make([]cliToolInstallResponse, 0, len(items))
	for _, item := range items {
		out = append(out, cliToolInstallDTO(item))
	}
	writeJSON(w, http.StatusOK, out)
}

// executeInstall corre el install_command en el host y persiste el resultado.
// Corre fuera del request (contexto propio): una instalación npm puede tardar
// minutos. Al terminar con éxito re-detecta la CLI para refrescar path/versión.
func (h *RuntimeHandler) executeInstall(install store.CliToolInstall) {
	ctx := context.Background()
	output, runErr := h.installRunner(ctx, install.InstallCommand)
	output = worker.RedactKnownSecrets(output)
	const maxOutputBytes = 64 * 1024
	if len(output) > maxOutputBytes {
		output = "[truncated]\n" + output[len(output)-maxOutputBytes:]
	}
	status := "succeeded"
	if runErr != nil {
		status = "failed"
		output = strings.TrimSpace(output + "\n[error] " + worker.RedactKnownSecrets(runErr.Error()))
	}
	if _, err := h.store.FinishCLIToolInstall(ctx, store.FinishCLIToolInstallParams{
		ID:     install.ID,
		Status: status,
		Output: nullableText(output),
	}); err != nil {
		slog.Error("cli install: persistir resultado", "install_id", install.ID, "error", err)
		return
	}
	if status != "succeeded" {
		return
	}
	if spec, ok := specForCLITool(install.CliToolID); ok {
		if _, err := h.detectRuntime(ctx, spec); err != nil {
			slog.Error("cli install: re-deteccion post-instalacion", "cli_tool_id", install.CliToolID, "error", err)
		}
	}
}

// runHostInstall ejecuta el comando de instalación con el shell del host.
// Timeout duro de 10 minutos: npm/go install pueden ser lentos pero no eternos.
func runHostInstall(ctx context.Context, command string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	var cmd *exec.Cmd
	if goruntime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		err = fmt.Errorf("timeout de instalacion (10m): %w", ctx.Err())
	}
	return string(out), err
}

func (h *RuntimeHandler) detectRuntime(ctx context.Context, spec runtimeToolSpec) (store.AgentRuntime, error) {
	path, lookErr := h.lookPath(spec.Command)
	status := "unavailable"
	version := ""
	if lookErr == nil {
		versionOut, err := h.commandVersion(ctx, path)
		switch {
		case err == nil:
			status = "detected"
			if strings.TrimSpace(h.getenv(spec.ProviderEnv)) != "" {
				status = "configured"
			}
			version = versionOut
		case errors.Is(err, context.DeadlineExceeded):
			status = "blocked"
		default:
			status = "blocked"
		}
	}

	_, err := h.store.UpsertCLIToolDetection(ctx, store.UpsertCLIToolDetectionParams{
		ID:             spec.ID,
		Name:           spec.Name,
		Command:        spec.Command,
		Kind:           spec.Kind,
		DetectedPath:   nullableText(path),
		Version:        nullableText(version),
		RuntimeID:      nullableText(spec.RuntimeID),
		Status:         cliStatusFromRuntimeStatus(status),
		RiskLevel:      spec.RiskLevel,
		RequiresAuth:   boolInt(spec.RequiresAuth),
		Capabilities:   spec.Capabilities,
		InstallCommand: nullableText(spec.InstallCommand),
		InstallUrl:     nullableText(spec.InstallURL),
	})
	if err != nil {
		return store.AgentRuntime{}, err
	}
	return h.store.UpdateAgentRuntimeDetection(ctx, store.UpdateAgentRuntimeDetectionParams{
		ID:         spec.RuntimeID,
		Status:     status,
		BinaryPath: nullableText(path),
		Version:    nullableText(version),
	})
}

func detectCommandVersion(ctx context.Context, path string) (string, error) {
	// 10s: los shims npm de Windows arrancan node en frío y pueden superar
	// largamente los 2s (gemini/pi tardaban >2s y quedaban "broken").
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(string(out))
	if len(version) > 200 {
		version = version[:200]
	}
	return version, nil
}

func cliStatusFromRuntimeStatus(status string) string {
	switch status {
	case "detected", "configured":
		return "detected"
	case "blocked":
		return "broken"
	default:
		return "not_detected"
	}
}

func runtimeAdapterDTO(item store.AgentRuntime) runtimeAdapterResponse {
	return runtimeAdapterResponse{
		ID:                   item.ID,
		Name:                 item.Name,
		Status:               item.Status,
		Version:              textValue(item.Version),
		Executable:           textValue(item.BinaryPath),
		Command:              commandForRuntime(item.ID),
		ApprovalRequired:     true,
		ApprovedForExecution: false,
		RequiresAuth:         item.RequiresAuth != 0,
		LastDetectedAt:       timeValue(item.DetectedAt),
	}
}

func cliToolDTO(item store.CliTool) cliToolResponse {
	return cliToolResponse{
		ID:             item.ID,
		Name:           item.Name,
		Command:        item.Command,
		Kind:           item.Kind,
		Status:         item.Status,
		DetectedPath:   textValue(item.DetectedPath),
		Version:        textValue(item.Version),
		RuntimeID:      textValue(item.RuntimeID),
		RiskLevel:      item.RiskLevel,
		RequiresAuth:   item.RequiresAuth != 0,
		InstallCommand: textValue(item.InstallCommand),
		InstallURL:     textValue(item.InstallUrl),
		LastDetectedAt: timeValue(item.LastDetectedAt),
	}
}

func cliToolInstallDTO(item store.CliToolInstall) cliToolInstallResponse {
	return cliToolInstallResponse{
		ID:             item.ID,
		CliToolID:      item.CliToolID,
		InstallCommand: item.InstallCommand,
		Status:         item.Status,
		Reason:         textValue(item.Reason),
		Output:         textValue(item.Output),
		RequestedAt:    item.RequestedAt,
		DecidedAt:      timeValue(item.DecidedAt),
		CompletedAt:    timeValue(item.CompletedAt),
	}
}

func providerDTO(item store.Provider) providerResponse {
	return providerResponse{
		ID:          item.ID,
		Name:        item.Name,
		Kind:        item.Kind,
		EnvKey:      item.EnvKey,
		Status:      item.Status,
		LastCheckAt: timeValue(item.LastCheckAt),
	}
}

func commandForRuntime(id string) string {
	for _, spec := range approvedRuntimeTools {
		if spec.RuntimeID == id {
			return spec.Command
		}
	}
	return ""
}

func timeValue(value sql.NullTime) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time
}
