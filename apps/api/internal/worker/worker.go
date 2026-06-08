package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nicotion/battos/apps/api/internal/gitauth"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type Store interface {
	ClaimNextQueuedRun(context.Context) (store.Run, error)
	ClaimQueuedRunByID(context.Context, string) (store.Run, error)
	AppendRunLog(context.Context, store.AppendRunLogParams) (store.RunLog, error)
	CompleteRun(context.Context, store.CompleteRunParams) (store.Run, error)
	FailRun(context.Context, store.FailRunParams) (store.Run, error)
	CreateArtifact(context.Context, store.CreateArtifactParams) (store.Artifact, error)
	GetRepository(context.Context, string) (store.Repository, error)
	UpdateRunBranchAndMetadata(context.Context, store.UpdateRunBranchAndMetadataParams) (store.Run, error)
	CreateUsageEvent(context.Context, store.CreateUsageEventParams) (store.UsageEvent, error)
}

type Adapter interface {
	Plan(context.Context, store.Run) (ExecutionPlan, error)
}

type Sandbox interface {
	Execute(context.Context, ExecutionPlan, LogFunc) (Result, error)
}

type MemoryContextProvider interface {
	ContextForRun(context.Context, store.Run) (MemoryContext, error)
}

type MemoryContext struct {
	Content string
	Count   int
}

type LogFunc func(stream, message string) error

type ExecutionPlan struct {
	RuntimeID      string
	Command        string
	Args           []string
	EnvKeys        []string
	Mounts         []Mount
	Prompt         string
	WorkDir        string
	NetworkEnabled bool
	Timeout        time.Duration
}

type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

type ProducedArtifact struct {
	Name    string
	Kind    string
	Content string
}

type Result struct {
	Summary          string
	Artifacts        []ProducedArtifact
	TokensIn         int32
	TokensOut        int32
	EstimatedCostUSD float64
}

type Worker struct {
	store           Store
	adapters        map[string]Adapter
	sandbox         Sandbox
	ArtifactsDir    string
	WorkspacesDir   string
	RepositoriesDir string
	Memory          MemoryContextProvider
}

func New(store Store, sandbox Sandbox, adapters map[string]Adapter) *Worker {
	if adapters == nil {
		adapters = map[string]Adapter{}
	}
	if sandbox == nil {
		sandbox = DryRunSandbox{}
	}
	return &Worker{
		store:           store,
		adapters:        adapters,
		sandbox:         sandbox,
		ArtifactsDir:    "data/artifacts",
		WorkspacesDir:   "data/runs/workspaces",
		RepositoriesDir: "data/repositories",
	}
}

func (w *Worker) RunLoop(ctx context.Context, pollInterval time.Duration) error {
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	for {
		processed, err := w.ProcessOne(ctx)
		if err != nil {
			return err
		}
		if processed {
			continue
		}
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return nil
		case <-timer.C:
		}
	}
}

func (w *Worker) ProcessOne(ctx context.Context) (bool, error) {
	run, err := w.store.ClaimNextQueuedRun(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim queued run: %w", err)
	}
	return w.processClaimedRun(ctx, run)
}

func (w *Worker) ProcessRunID(ctx context.Context, id string) (bool, error) {
	run, err := w.store.ClaimQueuedRunByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim queued run by id: %w", err)
	}
	return w.processClaimedRun(ctx, run)
}

func (w *Worker) processClaimedRun(ctx context.Context, run store.Run) (bool, error) {
	if err := w.log(ctx, run.ID, "system", "run claimed by worker"); err != nil {
		return true, err
	}

	adapter := w.adapters[run.RuntimeAdapterID]
	if adapter == nil {
		return true, w.fail(ctx, run.ID, "runtime adapter unavailable", fmt.Sprintf("runtime adapter %q is not registered in this worker", run.RuntimeAdapterID))
	}

	plan, err := adapter.Plan(ctx, run)
	if err != nil {
		failErr := w.fail(ctx, run.ID, "adapter plan failed", err.Error())
		if failErr != nil {
			return true, failErr
		}
		return true, nil
	}
	if err := validatePlan(plan, run); err != nil {
		failErr := w.fail(ctx, run.ID, "invalid execution plan", err.Error())
		if failErr != nil {
			return true, failErr
		}
		return true, nil
	}
	if w.Memory != nil {
		memoryContext, err := w.Memory.ContextForRun(ctx, run)
		if err != nil {
			_ = w.log(ctx, run.ID, "system", fmt.Sprintf("memory context unavailable: %v", err))
		} else if strings.TrimSpace(memoryContext.Content) != "" {
			plan.Prompt = injectMemoryContext(plan.Prompt, memoryContext.Content)
			_ = w.log(ctx, run.ID, "system", fmt.Sprintf("memory context injected (%d items)", memoryContext.Count))
		}
	}

	var workDir string
	var hasRepo bool
	var repo store.Repository
	var branchName string

	if run.RepositoryID.Valid && run.RepositoryID.String != "" {
		var errRepo error
		repo, errRepo = w.store.GetRepository(ctx, run.RepositoryID.String)
		if errRepo != nil {
			_ = w.log(ctx, run.ID, "system", fmt.Sprintf("error fetching repository details: %v", errRepo))
		} else {
			hasRepo = true
			var errDir error
			workDir, errDir = os.MkdirTemp(w.WorkspacesDir, "run-*")
			if errDir != nil {
				failErr := w.fail(ctx, run.ID, "workspace creation failed", errDir.Error())
				if failErr != nil {
					return true, failErr
				}
				return true, nil
			}
			_ = os.Chmod(workDir, 0o777)

			repoPath := filepath.Join(w.RepositoriesDir, repo.ID)
			cloneSource := repoPath
			var gitToken string
			if repo.Kind == "github" {
				remoteURL := strings.TrimSpace(repo.RemoteUrl.String)
				if !repo.RemoteUrl.Valid || remoteURL == "" {
					_ = os.RemoveAll(workDir)
					failErr := w.fail(ctx, run.ID, "git clone failed", "repositorio github sin remote_url configurado")
					if failErr != nil {
						return true, failErr
					}
					return true, nil
				}
				gitToken = gitauth.Resolve(repo.CredentialRef.String)
				cloneSource = gitauth.AuthenticatedURL(remoteURL, gitToken)
			}
			_ = w.log(ctx, run.ID, "system", fmt.Sprintf("git: cloning repository %s", repo.Name))
			cmdClone := exec.Command("git", "clone", cloneSource, workDir)
			if outClone, errClone := cmdClone.CombinedOutput(); errClone != nil {
				_ = os.RemoveAll(workDir)
				failErr := w.fail(ctx, run.ID, "git clone failed", gitauth.Redact(fmt.Sprintf("error: %v, output: %s", errClone, string(outClone)), gitToken))
				if failErr != nil {
					return true, failErr
				}
				return true, nil
			}
			// No dejar el token persistido en .git/config del workspace temporal:
			// restauramos el remote limpio. El push re-inyecta el token al vuelo.
			if repo.Kind == "github" && gitToken != "" {
				cmdSetURL := exec.Command("git", "remote", "set-url", "origin", strings.TrimSpace(repo.RemoteUrl.String))
				cmdSetURL.Dir = workDir
				_ = cmdSetURL.Run()
			}

			branchName = fmt.Sprintf("battos-run-%s", run.ID)
			_ = w.log(ctx, run.ID, "system", fmt.Sprintf("git: creating and switching to branch %s", branchName))
			cmdCheckout := exec.Command("git", "checkout", "-b", branchName)
			cmdCheckout.Dir = workDir
			if outCheckout, errCheckout := cmdCheckout.CombinedOutput(); errCheckout != nil {
				_ = os.RemoveAll(workDir)
				failErr := w.fail(ctx, run.ID, "git checkout failed", fmt.Sprintf("error: %v, output: %s", errCheckout, string(outCheckout)))
				if failErr != nil {
					return true, failErr
				}
				return true, nil
			}

			plan.WorkDir = workDir
		}
	}

	result, err := w.sandbox.Execute(ctx, plan, func(stream, message string) error {
		return w.log(ctx, run.ID, normalizeStream(stream), message)
	})
	if err != nil {
		if hasRepo && workDir != "" {
			_ = os.RemoveAll(workDir)
		}
		w.recordUsage(ctx, run, Result{})
		failErr := w.fail(ctx, run.ID, "run failed", err.Error())
		if failErr != nil {
			return true, failErr
		}
		return true, nil
	}

	for _, art := range result.Artifacts {
		bucket := "outputs"
		relPath := filepath.ToSlash(filepath.Join(safePathSegment(run.ProjectID), bucket, managedArtifactFilename(art.Name, art.Kind)))

		if err := w.writeManagedArtifact(relPath, art.Content); err != nil {
			_ = w.log(ctx, run.ID, "system", fmt.Sprintf("error saving physical artifact %q: %v", art.Name, err))
			continue
		}

		_, errArt := w.store.CreateArtifact(ctx, store.CreateArtifactParams{
			ProjectID:   run.ProjectID,
			TaskID:      nullableText(run.TaskID),
			RunID:       sql.NullString{String: run.ID, Valid: true},
			Name:        art.Name,
			Kind:        art.Kind,
			Content:     sql.NullString{String: art.Content, Valid: true},
			ManagedPath: sql.NullString{String: relPath, Valid: true},
			ExternalUrl: sql.NullString{},
			Metadata:    "{}",
		})
		if errArt != nil {
			_ = w.log(ctx, run.ID, "system", fmt.Sprintf("error registering artifact %q in database: %v", art.Name, errArt))
		} else {
			_ = w.log(ctx, run.ID, "system", fmt.Sprintf("artifact %q registered successfully", art.Name))
		}
	}

	var metadataMap map[string]any
	if len(run.Metadata) > 0 {
		_ = json.Unmarshal([]byte(run.Metadata), &metadataMap)
	}
	if metadataMap == nil {
		metadataMap = make(map[string]any)
	}

	if hasRepo {
		_ = w.log(ctx, run.ID, "system", "git: calculating differences (git diff)")
		cmdDiff := exec.Command("git", "diff", repo.DefaultBranch)
		cmdDiff.Dir = workDir
		diffBytes, errDiff := cmdDiff.CombinedOutput()
		if errDiff != nil {
			_ = w.log(ctx, run.ID, "system", fmt.Sprintf("error calculating git diff: %v, output: %s", errDiff, string(diffBytes)))
		} else {
			diffStr := string(diffBytes)
			_, errArt := w.store.CreateArtifact(ctx, store.CreateArtifactParams{
				ProjectID:   run.ProjectID,
				TaskID:      nullableText(run.TaskID),
				RunID:       sql.NullString{String: run.ID, Valid: true},
				Name:        "run-diff",
				Kind:        "diff",
				Content:     sql.NullString{String: diffStr, Valid: true},
				ManagedPath: sql.NullString{},
				ExternalUrl: sql.NullString{},
				Metadata:    "{}",
			})
			if errArt != nil {
				_ = w.log(ctx, run.ID, "system", fmt.Sprintf("error registering diff artifact: %v", errArt))
			} else {
				_ = w.log(ctx, run.ID, "system", "diff artifact registered successfully")
			}
		}

		metadataMap["work_dir"] = workDir
		metadataBytes, _ := json.Marshal(metadataMap)
		_, errUpdate := w.store.UpdateRunBranchAndMetadata(ctx, store.UpdateRunBranchAndMetadataParams{
			ID:         run.ID,
			BranchName: sql.NullString{String: branchName, Valid: true},
			Metadata:   string(metadataBytes),
		})
		if errUpdate != nil {
			_ = w.log(ctx, run.ID, "system", fmt.Sprintf("error updating run branch/metadata: %v", errUpdate))
		}
	}

	w.recordUsage(ctx, run, result)
	if _, err := w.store.CompleteRun(ctx, store.CompleteRunParams{ID: run.ID, ResultSummary: nullableText(result.Summary)}); err != nil {
		return true, fmt.Errorf("complete run: %w", err)
	}
	if err := w.log(ctx, run.ID, "system", "run completed successfully"); err != nil {
		return true, err
	}
	return true, nil
}

func (w *Worker) fail(ctx context.Context, id string, summary, message string) error {
	if logErr := w.log(ctx, id, "stderr", message); logErr != nil {
		return logErr
	}
	if _, err := w.store.FailRun(ctx, store.FailRunParams{
		ID:            id,
		ResultSummary: nullableText(summary),
		ErrorMessage:  nullableText(message),
	}); err != nil {
		return fmt.Errorf("fail run: %w", err)
	}
	return nil
}

func (w *Worker) log(ctx context.Context, id string, stream, message string) error {
	if message == "" {
		return nil
	}
	if _, err := w.store.AppendRunLog(ctx, store.AppendRunLogParams{RunID: id, Stream: stream, Message: message}); err != nil {
		return fmt.Errorf("append run log: %w", err)
	}
	return nil
}

func normalizeStream(value string) string {
	switch value {
	case "stdout", "stderr", "system":
		return value
	default:
		return "system"
	}
}

func nullableText(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

func validatePlan(plan ExecutionPlan, run store.Run) error {
	if strings.TrimSpace(plan.RuntimeID) == "" {
		return fmt.Errorf("runtime id is required")
	}
	if plan.RuntimeID != run.RuntimeAdapterID {
		return fmt.Errorf("plan runtime %q does not match run runtime %q", plan.RuntimeID, run.RuntimeAdapterID)
	}
	if strings.TrimSpace(plan.Command) == "" {
		return fmt.Errorf("command is required")
	}
	for _, key := range plan.EnvKeys {
		if !validEnvKey(key) {
			return fmt.Errorf("invalid env key %q", key)
		}
	}
	for _, mount := range plan.Mounts {
		if strings.TrimSpace(mount.Source) == "" || strings.TrimSpace(mount.Target) == "" {
			return fmt.Errorf("invalid host_session mount")
		}
		if !mount.ReadOnly {
			return fmt.Errorf("host_session mounts must be read-only")
		}
	}
	if len(plan.Mounts) > 0 && run.HostSessionEnabled == 0 {
		return fmt.Errorf("host_session mounts require host_session approval")
	}
	if plan.NetworkEnabled && run.NetworkEnabled == 0 {
		return fmt.Errorf("network was not approved for this run")
	}
	if plan.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	return nil
}

func validEnvKey(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if r == '_' || ('A' <= r && r <= 'Z') || (i > 0 && '0' <= r && r <= '9') {
			continue
		}
		return false
	}
	return true
}

func (w *Worker) recordUsage(ctx context.Context, run store.Run, res Result) {
	var providerID string
	var modelID string

	// Intentar inferir del CommandAdapter
	if adapter, ok := w.adapters[run.RuntimeAdapterID].(CommandAdapter); ok {
		if adapter.ProviderEnv == "ANTHROPIC_API_KEY" {
			providerID = "anthropic"
			modelID = "claude-3-5-sonnet"
		} else if adapter.ProviderEnv == "OPENAI_API_KEY" {
			providerID = "openai"
			modelID = "gpt-4o"
		}
	}
	if providerID == "" {
		if run.RuntimeAdapterID == "claude-code" || run.RuntimeAdapterID == "claude-code-host-session" {
			providerID = "anthropic"
			modelID = "claude-3-5-sonnet"
		} else if run.RuntimeAdapterID == "codex" || run.RuntimeAdapterID == "codex-host-session" {
			providerID = "openai"
			modelID = "gpt-4o"
		} else {
			providerID = "unknown"
			modelID = "unknown"
		}
	}

	_, errUsage := w.store.CreateUsageEvent(ctx, store.CreateUsageEventParams{
		RunID:            sql.NullString{String: run.ID, Valid: run.ID != ""},
		ProviderID:       sql.NullString{String: providerID, Valid: providerID != ""},
		ModelID:          sql.NullString{String: modelID, Valid: modelID != ""},
		ProjectID:        sql.NullString{String: run.ProjectID, Valid: run.ProjectID != ""},
		AgentID:          sql.NullString{String: run.AgentID, Valid: run.AgentID != ""},
		SkillID:          run.SkillID,
		InputTokens:      int64(res.TokensIn),
		OutputTokens:     int64(res.TokensOut),
		CachedTokens:     0,
		RequestCount:     1,
		EstimatedCostUsd: res.EstimatedCostUSD,
	})
	if errUsage != nil {
		_ = w.log(ctx, run.ID, "system", fmt.Sprintf("error registering usage event: %v", errUsage))
	} else {
		_ = w.log(ctx, run.ID, "system", fmt.Sprintf("usage event registered: In=%d, Out=%d, Cost=%.6f", res.TokensIn, res.TokensOut, res.EstimatedCostUSD))
	}
}
