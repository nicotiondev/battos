package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type fakeStore struct {
	claimErr    error
	claimedID   pgtype.UUID
	run         store.Run
	logs        []store.AppendRunLogParams
	artifacts   []store.CreateArtifactParams
	artifactErr error
	completed   store.CompleteRunParams
	failed      store.FailRunParams
	completeOK  bool
	failOK      bool
}

func (f *fakeStore) ClaimNextQueuedRun(context.Context) (store.Run, error) {
	if f.claimErr != nil {
		return store.Run{}, f.claimErr
	}
	return f.run, nil
}

func (f *fakeStore) ClaimQueuedRunByID(_ context.Context, id pgtype.UUID) (store.Run, error) {
	f.claimedID = id
	if f.claimErr != nil {
		return store.Run{}, f.claimErr
	}
	return f.run, nil
}

func (f *fakeStore) AppendRunLog(_ context.Context, arg store.AppendRunLogParams) (store.RunLog, error) {
	f.logs = append(f.logs, arg)
	return store.RunLog{ID: int64(len(f.logs)), RunID: arg.RunID, Stream: arg.Stream, Message: arg.Message, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}}, nil
}

func (f *fakeStore) CompleteRun(_ context.Context, arg store.CompleteRunParams) (store.Run, error) {
	f.completed = arg
	f.completeOK = true
	item := f.run
	item.Status = "succeeded"
	return item, nil
}

func (f *fakeStore) FailRun(_ context.Context, arg store.FailRunParams) (store.Run, error) {
	f.failed = arg
	f.failOK = true
	item := f.run
	item.Status = "failed"
	return item, nil
}

func (f *fakeStore) CreateArtifact(_ context.Context, arg store.CreateArtifactParams) (store.Artifact, error) {
	if f.artifactErr != nil {
		return store.Artifact{}, f.artifactErr
	}
	f.artifacts = append(f.artifacts, arg)
	return store.Artifact{
		ID:          pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		ProjectID:   arg.ProjectID,
		TaskID:      arg.TaskID,
		RunID:       arg.RunID,
		Name:        arg.Name,
		Kind:        arg.Kind,
		Content:     arg.Content,
		ManagedPath: arg.ManagedPath,
		ExternalUrl: arg.ExternalUrl,
	}, nil
}

func (f *fakeStore) GetRepository(_ context.Context, id string) (store.Repository, error) {
	return store.Repository{
		ID:        id,
		ProjectID: "web",
		Kind:      "managed_local",
		Name:      "test-repo",
	}, nil
}

func (f *fakeStore) UpdateRunBranchAndMetadata(_ context.Context, arg store.UpdateRunBranchAndMetadataParams) (store.Run, error) {
	item := f.run
	item.BranchName = arg.BranchName
	item.Metadata = arg.Metadata
	return item, nil
}

func (f *fakeStore) CreateUsageEvent(_ context.Context, arg store.CreateUsageEventParams) (store.UsageEvent, error) {
	return store.UsageEvent{
		ID:               pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
		RunID:            arg.RunID,
		ProviderID:       arg.ProviderID,
		ModelID:          arg.ModelID,
		ProjectID:        arg.ProjectID,
		AgentID:          arg.AgentID,
		SkillID:          arg.SkillID,
		InputTokens:      arg.InputTokens,
		OutputTokens:     arg.OutputTokens,
		CachedTokens:     arg.CachedTokens,
		RequestCount:     arg.RequestCount,
		EstimatedCostUsd: arg.EstimatedCostUsd,
	}, nil
}

func TestProcessOneRegistersProducedArtifacts(t *testing.T) {
	run := testRun("codex")
	store := &fakeStore{run: run}
	sandbox := &fakeSandbox{result: Result{
		Summary: "done",
		Artifacts: []ProducedArtifact{{
			Name:    "report.md",
			Kind:    "markdown",
			Content: "# report",
		}},
	}}
	worker := New(store, sandbox, map[string]Adapter{
		"codex": fakeAdapter{plan: testPlan("codex")},
	})
	worker.ArtifactsDir = t.TempDir()

	processed, err := worker.ProcessOne(context.Background())

	if err != nil {
		t.Fatalf("ProcessOne returned error: %v", err)
	}
	if !processed || !store.completeOK {
		t.Fatalf("processed=%v completeOK=%v, want completed run", processed, store.completeOK)
	}
	if len(store.artifacts) != 1 {
		t.Fatalf("artifacts=%+v, want one registered artifact", store.artifacts)
	}
	artifact := store.artifacts[0]
	if artifact.ProjectID != "web" || artifact.TaskID.String != "task-1" || artifact.RunID != run.ID {
		t.Fatalf("artifact=%+v, want project/task/run association", artifact)
	}
	if !artifact.ManagedPath.Valid || !strings.Contains(artifact.ManagedPath.String, "web/outputs/") {
		t.Fatalf("managed_path=%+v, want managed project outputs path", artifact.ManagedPath)
	}
	content, err := os.ReadFile(filepath.Join(worker.ArtifactsDir, filepath.FromSlash(artifact.ManagedPath.String)))
	if err != nil {
		t.Fatalf("artifact file was not written: %v", err)
	}
	if string(content) != "# report" {
		t.Fatalf("artifact content = %q, want # report", string(content))
	}
	if !strings.Contains(joinLogMessages(store.logs), `artifact "report.md" registered successfully`) {
		t.Fatalf("logs=%+v, want artifact registration log", store.logs)
	}
}

type fakeAdapter struct {
	plan ExecutionPlan
	err  error
}

func (f fakeAdapter) Plan(context.Context, store.Run) (ExecutionPlan, error) {
	return f.plan, f.err
}

type fakeSandbox struct {
	result Result
	err    error
	plan   ExecutionPlan
}

func (f *fakeSandbox) Execute(_ context.Context, plan ExecutionPlan, log LogFunc) (Result, error) {
	f.plan = plan
	if err := log("stdout", "sandbox started"); err != nil {
		return Result{}, err
	}
	return f.result, f.err
}

type fakeMemoryContext struct {
	context MemoryContext
	err     error
}

func (f fakeMemoryContext) ContextForRun(context.Context, store.Run) (MemoryContext, error) {
	return f.context, f.err
}

func TestProcessOneNoQueuedRun(t *testing.T) {
	worker := New(&fakeStore{claimErr: pgx.ErrNoRows}, nil, nil)

	processed, err := worker.ProcessOne(context.Background())

	if err != nil {
		t.Fatalf("ProcessOne returned error: %v", err)
	}
	if processed {
		t.Fatalf("processed = true, want false")
	}
}

func TestProcessRunIDClaimsSpecificQueuedRun(t *testing.T) {
	run := testRun("codex")
	store := &fakeStore{run: run}
	sandbox := &fakeSandbox{result: Result{Summary: "done"}}
	worker := New(store, sandbox, map[string]Adapter{
		"codex": fakeAdapter{plan: testPlan("codex")},
	})

	processed, err := worker.ProcessRunID(context.Background(), run.ID)

	if err != nil {
		t.Fatalf("ProcessRunID returned error: %v", err)
	}
	if !processed || !store.completeOK {
		t.Fatalf("processed=%v completeOK=%v, want completed run", processed, store.completeOK)
	}
	if store.claimedID != run.ID {
		t.Fatalf("claimedID=%+v, want %+v", store.claimedID, run.ID)
	}
}

func TestRunLoopStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	worker := New(&fakeStore{claimErr: pgx.ErrNoRows}, nil, nil)

	if err := worker.RunLoop(ctx, time.Millisecond); err != nil {
		t.Fatalf("RunLoop returned error: %v", err)
	}
}

func TestProcessOneCompletesRun(t *testing.T) {
	run := testRun("codex")
	store := &fakeStore{run: run}
	sandbox := &fakeSandbox{result: Result{Summary: "done"}}
	worker := New(store, sandbox, map[string]Adapter{
		"codex": fakeAdapter{plan: testPlan("codex")},
	})

	processed, err := worker.ProcessOne(context.Background())

	if err != nil {
		t.Fatalf("ProcessOne returned error: %v", err)
	}
	if !processed || !store.completeOK {
		t.Fatalf("processed=%v completeOK=%v, want completed run", processed, store.completeOK)
	}
	if store.completed.ResultSummary.String != "done" {
		t.Fatalf("summary = %q, want done", store.completed.ResultSummary.String)
	}
	logs := joinLogMessages(store.logs)
	if store.logs[1].Stream != "stdout" ||
		!strings.Contains(logs, "run claimed by worker") ||
		!strings.Contains(logs, "sandbox started") ||
		!strings.Contains(logs, "run completed successfully") {
		t.Fatalf("logs = %+v, want claim/stdout/completed", store.logs)
	}
	if sandbox.plan.Command != "codex" {
		t.Fatalf("sandbox plan = %+v, want codex command from fake adapter", sandbox.plan)
	}
}

func TestProcessOneInjectsMemoryContextIntoPrompt(t *testing.T) {
	run := testRun("codex")
	store := &fakeStore{run: run}
	sandbox := &fakeSandbox{result: Result{Summary: "done"}}
	worker := New(store, sandbox, map[string]Adapter{
		"codex": fakeAdapter{plan: testPlan("codex")},
	})
	worker.Memory = fakeMemoryContext{context: MemoryContext{
		Content: "# BattOS Memory Context\n\n## [decision] Use Go\n\nPrefer Go for workers.",
		Count:   1,
	}}

	processed, err := worker.ProcessOne(context.Background())

	if err != nil {
		t.Fatalf("ProcessOne returned error: %v", err)
	}
	if !processed || !store.completeOK {
		t.Fatalf("processed=%v completeOK=%v, want completed run", processed, store.completeOK)
	}
	if !strings.Contains(sandbox.plan.Prompt, "# BattOS Memory Context") || !strings.Contains(sandbox.plan.Prompt, "# User Prompt") || !strings.Contains(sandbox.plan.Prompt, "build it") {
		t.Fatalf("prompt=%q, want memory context plus original prompt", sandbox.plan.Prompt)
	}
	if !strings.Contains(joinLogMessages(store.logs), "memory context injected (1 items)") {
		t.Fatalf("logs=%+v, want memory context injected log", store.logs)
	}
}

func TestProcessOneFailsMissingAdapter(t *testing.T) {
	store := &fakeStore{run: testRun("claude-code")}
	worker := New(store, nil, nil)

	processed, err := worker.ProcessOne(context.Background())

	if err != nil {
		t.Fatalf("ProcessOne returned error: %v", err)
	}
	if !processed || !store.failOK {
		t.Fatalf("processed=%v failOK=%v, want failed run", processed, store.failOK)
	}
	if store.failed.ErrorMessage.String == "" {
		t.Fatalf("missing adapter did not persist error message")
	}
}

func TestProcessOneFailsAdapterError(t *testing.T) {
	store := &fakeStore{run: testRun("codex")}
	worker := New(store, nil, map[string]Adapter{
		"codex": fakeAdapter{err: errors.New("boom")},
	})

	processed, err := worker.ProcessOne(context.Background())

	if err != nil {
		t.Fatalf("ProcessOne returned error: %v", err)
	}
	if !processed || !store.failOK || store.failed.ErrorMessage.String != "boom" {
		t.Fatalf("processed=%v failed=%+v, want boom failure", processed, store.failed)
	}
}

func TestProcessOneRejectsNetworkWithoutApproval(t *testing.T) {
	store := &fakeStore{run: testRun("codex")}
	worker := New(store, nil, map[string]Adapter{
		"codex": fakeAdapter{plan: ExecutionPlan{RuntimeID: "codex", Command: "codex", NetworkEnabled: true, Timeout: time.Minute}},
	})

	processed, err := worker.ProcessOne(context.Background())

	if err != nil {
		t.Fatalf("ProcessOne returned error: %v", err)
	}
	if !processed || !store.failOK || store.failed.ResultSummary.String != "invalid execution plan" {
		t.Fatalf("processed=%v failed=%+v, want invalid plan failure", processed, store.failed)
	}
}

func TestProcessOneRejectsInvalidEnvKey(t *testing.T) {
	store := &fakeStore{run: testRun("codex")}
	worker := New(store, nil, map[string]Adapter{
		"codex": fakeAdapter{plan: ExecutionPlan{RuntimeID: "codex", Command: "codex", EnvKeys: []string{"OPENAI_API_KEY=bad"}, Timeout: time.Minute}},
	})

	processed, err := worker.ProcessOne(context.Background())

	if err != nil {
		t.Fatalf("ProcessOne returned error: %v", err)
	}
	if !processed || !store.failOK || store.failed.ResultSummary.String != "invalid execution plan" {
		t.Fatalf("processed=%v failed=%+v, want invalid env key failure", processed, store.failed)
	}
}

func TestDryRunSandboxDoesNotExecuteHostCommand(t *testing.T) {
	var logs []string
	result, err := DryRunSandbox{}.Execute(context.Background(), testPlan("codex"), func(stream, message string) error {
		logs = append(logs, stream+":"+message)
		return nil
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Summary == "" || len(logs) < 3 {
		t.Fatalf("result=%+v logs=%+v, want dry-run summary and logs", result, logs)
	}
}

func TestApprovedDryRunAdaptersCreatePlans(t *testing.T) {
	adapters := ApprovedDryRunAdapters()
	plan, err := adapters["codex"].Plan(context.Background(), testRun("codex"))
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}
	if plan.RuntimeID != "codex" || plan.Command != "sh" || plan.Timeout <= 0 || !contains(plan.EnvKeys, "OPENAI_API_KEY") {
		t.Fatalf("plan=%+v, want codex shell plan with provider env", plan)
	}
	if !strings.Contains(strings.Join(plan.Args, " "), "codex exec") || !strings.Contains(strings.Join(plan.Args, " "), "BATTOS_PROMPT_FILE") {
		t.Fatalf("codex args=%q, want prompt-file exec script", strings.Join(plan.Args, " "))
	}
	claude, err := adapters["claude-code"].Plan(context.Background(), testRun("claude-code"))
	if err != nil {
		t.Fatalf("claude Plan returned error: %v", err)
	}
	if claude.RuntimeID != "claude-code" || claude.Command != "sh" || !contains(claude.EnvKeys, "ANTHROPIC_API_KEY") {
		t.Fatalf("claude plan=%+v, want shell plan with provider env", claude)
	}
	if !strings.Contains(strings.Join(claude.Args, " "), "claude --bare --print") || !strings.Contains(strings.Join(claude.Args, " "), "BATTOS_PROMPT_FILE") {
		t.Fatalf("claude args=%q, want prompt-file print script", strings.Join(claude.Args, " "))
	}
	smoke, err := adapters["sandbox-smoke"].Plan(context.Background(), testRun("sandbox-smoke"))
	if err != nil {
		t.Fatalf("smoke Plan returned error: %v", err)
	}
	if smoke.Command != "sh" || len(smoke.Args) == 0 {
		t.Fatalf("smoke plan=%+v, want fixed shell smoke command", smoke)
	}
	memorySmoke, err := adapters["sandbox-memory-smoke"].Plan(context.Background(), testRun("sandbox-memory-smoke"))
	if err != nil {
		t.Fatalf("memory smoke Plan returned error: %v", err)
	}
	if memorySmoke.Command != "sh" || !strings.Contains(strings.Join(memorySmoke.Args, " "), "BattOS Memory Context") {
		t.Fatalf("memory smoke plan=%+v, want prompt memory context assertion", memorySmoke)
	}
}

type fakeRunner struct {
	name    string
	args    []string
	out     CommandOutput
	err     error
	inspect func(name string, args []string) error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (CommandOutput, error) {
	f.name = name
	f.args = append([]string{}, args...)
	if f.inspect != nil {
		if err := f.inspect(name, f.args); err != nil {
			return CommandOutput{}, err
		}
	}
	return f.out, f.err
}

func TestDockerSandboxBuildsIsolatedNoNetworkCommand(t *testing.T) {
	root := t.TempDir()
	runner := &fakeRunner{out: CommandOutput{Stdout: "ok"}}
	sandbox := DockerSandbox{Image: "alpine:3.20", WorkspacesDir: root, Runner: runner}
	var logs []string

	result, err := sandbox.Execute(context.Background(), testPlan("codex"), func(stream, message string) error {
		logs = append(logs, stream+":"+message)
		return nil
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Summary != "docker sandbox completed" {
		t.Fatalf("summary = %q, want docker sandbox completed", result.Summary)
	}
	joinedArgs := strings.Join(runner.args, " ")
	if runner.name != "docker" || !strings.Contains(joinedArgs, "--network none") || !strings.Contains(joinedArgs, "alpine:3.20 codex") {
		t.Fatalf("runner=%s args=%q, want docker run with no network and command", runner.name, joinedArgs)
	}
	if len(logs) < 3 || !strings.Contains(strings.Join(logs, "\n"), "stdout:ok") {
		t.Fatalf("logs=%+v, want docker lifecycle and stdout", logs)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("workspace root still has entries: %+v", entries)
	}
}

func TestDockerSandboxMakesWorkspaceWritableForContainerUser(t *testing.T) {
	root := t.TempDir()
	var inspected bool
	runner := &fakeRunner{inspect: func(_ string, args []string) error {
		workspaceMount := ""
		for i, arg := range args {
			if arg == "-v" && i+1 < len(args) {
				workspaceMount = args[i+1]
				break
			}
		}
		if workspaceMount == "" {
			return errors.New("missing workspace mount")
		}
		workspace := strings.TrimSuffix(workspaceMount, ":/workspace")
		info, err := os.Stat(workspace)
		if err != nil {
			return err
		}
		if info.Mode().Perm() != 0o777 {
			return fmt.Errorf("workspace mode = %v, want 0777", info.Mode().Perm())
		}
		inspected = true
		return nil
	}}
	plan := testPlan("codex")

	_, err := (DockerSandbox{Image: "battos-runtime-agents:dev", WorkspacesDir: root, Runner: runner}).Execute(context.Background(), plan, func(string, string) error { return nil })

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !inspected {
		t.Fatalf("fake runner did not inspect workspace")
	}
}

func TestDockerSandboxPassesOnlyDeclaredEnvKeys(t *testing.T) {
	runner := &fakeRunner{}
	plan := testPlan("codex")
	plan.EnvKeys = []string{"OPENAI_API_KEY"}

	_, err := (DockerSandbox{Image: "alpine:3.20", WorkspacesDir: t.TempDir(), Runner: runner}).Execute(context.Background(), plan, func(string, string) error { return nil })

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	joined := strings.Join(runner.args, " ")
	if !strings.Contains(joined, "-e OPENAI_API_KEY") || strings.Contains(joined, "ANTHROPIC_API_KEY") {
		t.Fatalf("args=%q, want only declared provider env", joined)
	}
}

func TestDockerSandboxRedactsKnownSecretsFromLogs(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test-secret-value")
	runner := &fakeRunner{out: CommandOutput{Stdout: "token sk-test-secret-value"}}
	var logs []string

	_, err := (DockerSandbox{Image: "alpine:3.20", WorkspacesDir: t.TempDir(), Runner: runner}).Execute(context.Background(), testPlan("codex"), func(stream, message string) error {
		logs = append(logs, stream+":"+message)
		return nil
	})

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	joined := strings.Join(logs, "\n")
	if strings.Contains(joined, "sk-test-secret-value") || !strings.Contains(joined, "[redacted:OPENAI_API_KEY]") {
		t.Fatalf("logs=%q, want redacted secret", joined)
	}
}

func TestDockerSandboxAllowsNetworkOnlyWhenPlanAllowsIt(t *testing.T) {
	root := t.TempDir()
	runner := &fakeRunner{}
	plan := testPlan("codex")
	plan.NetworkEnabled = true

	_, err := (DockerSandbox{Image: "alpine:3.20", WorkspacesDir: root, Runner: runner}).Execute(context.Background(), plan, func(string, string) error { return nil })

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(strings.Join(runner.args, " "), "--network bridge") {
		t.Fatalf("args=%q, want bridge network when approved", strings.Join(runner.args, " "))
	}
}

func TestDockerSandboxReportsRunnerFailure(t *testing.T) {
	runner := &fakeRunner{err: errors.New("docker failed")}

	_, err := (DockerSandbox{Image: "alpine:3.20", WorkspacesDir: t.TempDir(), Runner: runner}).Execute(context.Background(), testPlan("codex"), func(string, string) error { return nil })

	if err == nil || !strings.Contains(err.Error(), "docker sandbox command failed") {
		t.Fatalf("err=%v, want docker sandbox command failed", err)
	}
}

func TestDockerSandboxCollectsWorkspaceArtifacts(t *testing.T) {
	root := t.TempDir()
	runner := &fakeRunner{inspect: func(_ string, args []string) error {
		workspace, err := workspaceFromDockerArgs(args)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(workspace, "outputs"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(workspace, "outputs", "smoke.md"), []byte("# ok"), 0o600); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(workspace, "trace.diff"), []byte("diff --git a/x b/x"), 0o600); err != nil {
			return err
		}
		return nil
	}}

	result, err := (DockerSandbox{Image: "alpine:3.20", WorkspacesDir: root, Runner: runner}).Execute(context.Background(), testPlan("codex"), func(string, string) error { return nil })

	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(result.Artifacts) != 2 {
		t.Fatalf("artifacts=%+v, want two workspace artifacts", result.Artifacts)
	}
	if !hasArtifact(result.Artifacts, "outputs/smoke.md", "markdown", "# ok") {
		t.Fatalf("artifacts=%+v, want markdown smoke artifact", result.Artifacts)
	}
	if !hasArtifact(result.Artifacts, "trace.diff", "diff", "diff --git a/x b/x") {
		t.Fatalf("artifacts=%+v, want diff artifact", result.Artifacts)
	}
}

func testRun(runtimeID string) store.Run {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	return store.Run{
		ID:               pgtype.UUID{Bytes: id, Valid: true},
		ProjectID:        "web",
		TaskID:           "task-1",
		AgentID:          "agent-1",
		RuntimeAdapterID: runtimeID,
		Prompt:           "build it",
		Status:           "running",
	}
}

func testPlan(runtimeID string) ExecutionPlan {
	return ExecutionPlan{
		RuntimeID: runtimeID,
		Command:   runtimeID,
		Prompt:    "build it",
		Timeout:   time.Minute,
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func joinLogMessages(logs []store.AppendRunLogParams) string {
	var values []string
	for _, log := range logs {
		values = append(values, log.Message)
	}
	return strings.Join(values, "\n")
}

func workspaceFromDockerArgs(args []string) (string, error) {
	for i, arg := range args {
		if arg == "-v" && i+1 < len(args) {
			return strings.TrimSuffix(args[i+1], ":/workspace"), nil
		}
	}
	return "", errors.New("missing workspace mount")
}

func hasArtifact(artifacts []ProducedArtifact, name, kind, content string) bool {
	for _, artifact := range artifacts {
		if artifact.Name == name && artifact.Kind == kind && artifact.Content == content {
			return true
		}
	}
	return false
}
