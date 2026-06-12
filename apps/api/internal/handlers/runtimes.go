package handlers

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"net/http"

	"github.com/nicotion/battos/apps/api/internal/store"
)

type RuntimeStore interface {
	ListAgentRuntimes(context.Context) ([]store.AgentRuntime, error)
	UpdateAgentRuntimeDetection(context.Context, store.UpdateAgentRuntimeDetectionParams) (store.AgentRuntime, error)
	ListCLITools(context.Context) ([]store.CliTool, error)
	UpsertCLIToolDetection(context.Context, store.UpsertCLIToolDetectionParams) (store.CliTool, error)
	ListProviders(context.Context) ([]store.Provider, error)
	UpdateProviderStatus(context.Context, store.UpdateProviderStatusParams) error
}

type RuntimeHandler struct {
	store          RuntimeStore
	lookPath       func(string) (string, error)
	commandVersion func(context.Context, string) (string, error)
	getenv         func(string) string
	now            func() time.Time
}

func NewRuntimeHandler(q RuntimeStore) *RuntimeHandler {
	return &RuntimeHandler{
		store:          q,
		lookPath:       exec.LookPath,
		commandVersion: detectCommandVersion,
		getenv:         os.Getenv,
		now:            time.Now,
	}
}

type runtimeToolSpec struct {
	ID           string
	Name         string
	Command      string
	Kind         string
	RuntimeID    string
	RiskLevel    string
	RequiresAuth bool
	ProviderEnv  string
	Capabilities string
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
	LastDetectedAt time.Time `json:"last_detected_at,omitempty"`
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
		ID:           "claude-code",
		Name:         "Claude Code",
		Command:      "claude",
		Kind:         "coding_agent",
		RuntimeID:    "claude-code",
		RiskLevel:    "high",
		RequiresAuth: true,
		ProviderEnv:  "ANTHROPIC_API_KEY",
		Capabilities: `["code_editing","file_reading","terminal_commands","mcp"]`,
	},
	{
		ID:           "claude-code-host-session",
		Name:         "Claude Code (host session)",
		Command:      "claude",
		Kind:         "coding_agent",
		RuntimeID:    "claude-code-host-session",
		RiskLevel:    "high",
		RequiresAuth: true,
		Capabilities: `["code_editing","file_reading","terminal_commands","mcp","host_session"]`,
	},
	{
		ID:           "codex",
		Name:         "Codex CLI",
		Command:      "codex",
		Kind:         "coding_agent",
		RuntimeID:    "codex",
		RiskLevel:    "high",
		RequiresAuth: true,
		ProviderEnv:  "OPENAI_API_KEY",
		Capabilities: `["code_generation","repo_editing","tests"]`,
	},
	{
		ID:           "codex-host-session",
		Name:         "Codex CLI (host session)",
		Command:      "codex",
		Kind:         "coding_agent",
		RuntimeID:    "codex-host-session",
		RiskLevel:    "high",
		RequiresAuth: true,
		Capabilities: `["code_generation","repo_editing","tests","host_session"]`,
	},
	{
		ID:           "gemini",
		Name:         "Gemini CLI",
		Command:      "gemini",
		Kind:         "coding_agent",
		RuntimeID:    "gemini",
		RiskLevel:    "medium",
		RequiresAuth: true,
		ProviderEnv:  "GEMINI_API_KEY",
		Capabilities: `["long_context","multimodal","code_generation"]`,
	},
	{
		ID:           "pi",
		Name:         "Pi",
		Command:      "pi",
		Kind:         "coding_agent",
		RuntimeID:    "pi",
		RiskLevel:    "medium",
		RequiresAuth: true,
		Capabilities: `["code_editing","terminal","skills"]`,
	},
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
		ID:           spec.ID,
		Name:         spec.Name,
		Command:      spec.Command,
		Kind:         spec.Kind,
		DetectedPath: nullableText(path),
		Version:      nullableText(version),
		RuntimeID:    nullableText(spec.RuntimeID),
		Status:       cliStatusFromRuntimeStatus(status),
		RiskLevel:    spec.RiskLevel,
		RequiresAuth: boolInt(spec.RequiresAuth),
		Capabilities: spec.Capabilities,
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
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
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
		LastDetectedAt: timeValue(item.LastDetectedAt),
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
