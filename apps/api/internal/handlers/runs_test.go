package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nicotion/battos/apps/api/internal/memory"
	"github.com/nicotion/battos/apps/api/internal/store"
)

type fakeRunStore struct {
	createRunParams        store.CreateRunParams
	createApprovalParams   store.CreateRunApprovalParams
	current                store.Run
	updatedStatus          string
	networkEnabled         bool
	hostSessionEnabled     bool
	executionModeApprovals int64
	repo                   *store.Repository
}

func (f *fakeRunStore) GetRepository(_ context.Context, id string) (store.Repository, error) {
	if f.repo != nil {
		return *f.repo, nil
	}
	return store.Repository{ID: id, Kind: "managed_local"}, nil
}

func (f *fakeRunStore) CreateRun(_ context.Context, arg store.CreateRunParams) (store.Run, error) {
	f.createRunParams = arg
	run := testRun("awaiting_approval", arg.RequestedNetwork != 0)
	run.ExecutionMode = arg.ExecutionMode
	return run, nil
}

func (f *fakeRunStore) ListRuns(context.Context) ([]store.Run, error) {
	return []store.Run{testRun("queued", false)}, nil
}

func (f *fakeRunStore) ListRunsByProject(context.Context, string) ([]store.Run, error) {
	return []store.Run{testRun("queued", false)}, nil
}

func (f *fakeRunStore) GetRun(context.Context, string) (store.Run, error) {
	if f.current.ID == "" {
		return testRun("awaiting_approval", true), nil
	}
	return f.current, nil
}

func (f *fakeRunStore) CreateRunApproval(_ context.Context, arg store.CreateRunApprovalParams) (store.RunApproval, error) {
	f.createApprovalParams = arg
	if arg.Kind == "execution_mode" && arg.Decision == "approved" {
		f.executionModeApprovals++
	}
	return store.RunApproval{
		ID:        "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		RunID:     arg.RunID,
		Kind:      arg.Kind,
		Decision:  arg.Decision,
		Reason:    arg.Reason,
		DecidedAt: time.Now(),
	}, nil
}

func (f *fakeRunStore) GetCredentialByName(_ context.Context, _ string) (store.Credential, error) {
	// No managed credentials in tests: the broker falls back to os.Getenv.
	return store.Credential{}, sql.ErrNoRows
}

func (f *fakeRunStore) CountApprovedRunApproval(_ context.Context, arg store.CountApprovedRunApprovalParams) (int64, error) {
	if arg.Kind == "execution_mode" {
		return f.executionModeApprovals, nil
	}
	return 0, nil
}

func (f *fakeRunStore) UpdateRunStatus(_ context.Context, arg store.UpdateRunStatusParams) (store.Run, error) {
	f.updatedStatus = arg.Status
	item := testRun(arg.Status, true)
	item.ID = arg.ID
	return item, nil
}

func (f *fakeRunStore) EnableRunNetwork(_ context.Context, id string) (store.Run, error) {
	f.networkEnabled = true
	item := testRun("awaiting_approval", true)
	item.ID = id
	item.NetworkEnabled = 1
	return item, nil
}

func (f *fakeRunStore) EnableRunHostSession(_ context.Context, id string) (store.Run, error) {
	f.hostSessionEnabled = true
	item := testRun("awaiting_approval", true)
	item.ID = id
	item.HostSessionEnabled = 1
	return item, nil
}

func (f *fakeRunStore) CancelRun(_ context.Context, id string) (store.Run, error) {
	if f.current.Status == "succeeded" {
		return store.Run{}, sql.ErrNoRows
	}
	item := testRun("cancelled", false)
	item.ID = id
	return item, nil
}

func (f *fakeRunStore) ListRunLogs(context.Context, string) ([]store.RunLog, error) {
	return []store.RunLog{{ID: 1, RunID: "11111111-1111-1111-1111-111111111111", Stream: "system", Message: "queued", CreatedAt: time.Now()}}, nil
}

func (f *fakeRunStore) GetArtifactByRunAndKind(_ context.Context, arg store.GetArtifactByRunAndKindParams) (store.Artifact, error) {
	return store.Artifact{
		ID:        "22222222-2222-2222-2222-222222222222",
		ProjectID: "web",
		RunID:     arg.RunID,
		Name:      "run-diff",
		Kind:      "diff",
		Content:   sql.NullString{String: "some diff", Valid: true},
	}, nil
}

func (f *fakeRunStore) ListArtifactsByRun(_ context.Context, runID sql.NullString) ([]store.Artifact, error) {
	return []store.Artifact{{
		ID:        "22222222-2222-2222-2222-222222222222",
		ProjectID: "web",
		RunID:     runID,
		Name:      "run-diff",
		Kind:      "diff",
		Content:   sql.NullString{String: "some diff", Valid: true},
	}}, nil
}

func (f *fakeRunStore) UpdateRunBranchAndMetadata(_ context.Context, arg store.UpdateRunBranchAndMetadataParams) (store.Run, error) {
	item := testRun("succeeded", true)
	item.ID = arg.ID
	item.BranchName = arg.BranchName
	item.Metadata = arg.Metadata
	return item, nil
}

func TestCreateRunStartsAwaitingApproval(t *testing.T) {
	q := &fakeRunStore{}
	h := NewRunHandler(q, nil)
	req := httptest.NewRequest(http.MethodPost, "/runs", strings.NewReader(`{"project_id":"web","task_id":"task-1","agent_id":"agent-1","runtime_adapter_id":"codex","prompt":"build it","requested_network":true}`))
	rec := httptest.NewRecorder()

	h.CreateRun(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if q.createRunParams.ProjectID != "web" || q.createRunParams.RuntimeAdapterID != "codex" || q.createRunParams.RequestedNetwork == 0 {
		t.Fatalf("create params = %+v, want proposed run params", q.createRunParams)
	}
	if !strings.Contains(rec.Body.String(), `"status":"awaiting_approval"`) {
		t.Fatalf("response = %s, want awaiting_approval", rec.Body.String())
	}
}

func TestApproveExecuteQueuesRun(t *testing.T) {
	q := &fakeRunStore{current: testRun("awaiting_approval", true)}
	h := NewRunHandler(q, nil)
	req := runRequest(http.MethodPost, "/runs/11111111-1111-1111-1111-111111111111/approvals", `{"kind":"execute","decision":"approved","reason":"ok"}`)
	rec := httptest.NewRecorder()

	h.ApproveRunAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if q.updatedStatus != "queued" || q.createApprovalParams.Kind != "execute" {
		t.Fatalf("approval=%+v status=%q, want execute approval and queued", q.createApprovalParams, q.updatedStatus)
	}
}

func TestApproveExecuteOnDirectRunRequiresExecutionModeApproval(t *testing.T) {
	run := testRun("awaiting_approval", false)
	run.ExecutionMode = "direct"
	q := &fakeRunStore{current: run}
	h := NewRunHandler(q, nil)

	// Approving execute without first approving the execution_mode must be rejected.
	req := runRequest(http.MethodPost, "/runs/11111111-1111-1111-1111-111111111111/approvals", `{"kind":"execute","decision":"approved"}`)
	rec := httptest.NewRecorder()
	h.ApproveRunAction(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if q.updatedStatus == "queued" {
		t.Fatal("direct run must not be queued without execution_mode approval")
	}

	// Approve the execution_mode, then execute should queue it.
	reqMode := runRequest(http.MethodPost, "/runs/11111111-1111-1111-1111-111111111111/approvals", `{"kind":"execution_mode","decision":"approved"}`)
	recMode := httptest.NewRecorder()
	h.ApproveRunAction(recMode, reqMode)
	if recMode.Code != http.StatusOK {
		t.Fatalf("execution_mode approval status = %d, want 200; body=%s", recMode.Code, recMode.Body.String())
	}

	reqExec := runRequest(http.MethodPost, "/runs/11111111-1111-1111-1111-111111111111/approvals", `{"kind":"execute","decision":"approved"}`)
	recExec := httptest.NewRecorder()
	h.ApproveRunAction(recExec, reqExec)
	if recExec.Code != http.StatusOK {
		t.Fatalf("execute status = %d, want 200; body=%s", recExec.Code, recExec.Body.String())
	}
	if q.updatedStatus != "queued" {
		t.Fatalf("updatedStatus=%q, want queued after both approvals", q.updatedStatus)
	}
}

func TestApproveExecutionModeOnSandboxRunIsRejected(t *testing.T) {
	run := testRun("awaiting_approval", false)
	run.ExecutionMode = "sandbox"
	q := &fakeRunStore{current: run}
	h := NewRunHandler(q, nil)
	req := runRequest(http.MethodPost, "/runs/11111111-1111-1111-1111-111111111111/approvals", `{"kind":"execution_mode","decision":"approved"}`)
	rec := httptest.NewRecorder()
	h.ApproveRunAction(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (sandbox needs no mode approval); body=%s", rec.Code, rec.Body.String())
	}
}

func TestApproveHostSessionEnablesHostSession(t *testing.T) {
	q := &fakeRunStore{current: testRun("awaiting_approval", true)}
	h := NewRunHandler(q, nil)
	req := runRequest(http.MethodPost, "/runs/11111111-1111-1111-1111-111111111111/approvals", `{"kind":"host_session","decision":"approved"}`)
	rec := httptest.NewRecorder()

	h.ApproveRunAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !q.hostSessionEnabled {
		t.Fatalf("host_session approval did not enable host_session")
	}
	if !strings.Contains(rec.Body.String(), `"host_session_enabled":true`) {
		t.Fatalf("response does not reflect host_session_enabled=true: %s", rec.Body.String())
	}
}

func TestApproveNetworkEnablesNetwork(t *testing.T) {
	q := &fakeRunStore{current: testRun("awaiting_approval", true)}
	h := NewRunHandler(q, nil)
	req := runRequest(http.MethodPost, "/runs/11111111-1111-1111-1111-111111111111/approvals", `{"kind":"network","decision":"approved"}`)
	rec := httptest.NewRecorder()

	h.ApproveRunAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !q.networkEnabled {
		t.Fatalf("network approval did not enable network")
	}
}

func TestApproveExecuteTerminalRunIsRejectedBeforeApproval(t *testing.T) {
	q := &fakeRunStore{current: testRun("succeeded", true)}
	h := NewRunHandler(q, nil)
	req := runRequest(http.MethodPost, "/runs/11111111-1111-1111-1111-111111111111/approvals", `{"kind":"execute","decision":"approved"}`)
	rec := httptest.NewRecorder()

	h.ApproveRunAction(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if q.createApprovalParams.Kind != "" {
		t.Fatalf("approval was recorded for invalid transition: %+v", q.createApprovalParams)
	}
}

func TestCancelTerminalRunRejected(t *testing.T) {
	q := &fakeRunStore{current: testRun("succeeded", false)}
	h := NewRunHandler(q, nil)
	req := runRequest(http.MethodPost, "/runs/11111111-1111-1111-1111-111111111111/cancel", `{}`)
	rec := httptest.NewRecorder()

	h.CancelRun(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestStreamRunEventsEmitsSnapshotLogAndDone(t *testing.T) {
	q := &fakeRunStore{current: testRun("succeeded", false)}
	h := NewRunHandler(q, nil)
	req := runRequest(http.MethodGet, "/events/runs/11111111-1111-1111-1111-111111111111", "")
	rec := httptest.NewRecorder()

	h.StreamRunEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("content-type = %q, want text/event-stream", got)
	}
	body := rec.Body.String()
	for _, want := range []string{"event: run.snapshot", "event: run.log", "event: run.done", `"message":"queued"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("SSE body missing %q:\n%s", want, body)
		}
	}
}

func runRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "11111111-1111-1111-1111-111111111111")
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}

func testRun(status string, requestedNetwork bool) store.Run {
	now := time.Now()
	return store.Run{
		ID:               "11111111-1111-1111-1111-111111111111",
		ProjectID:        "web",
		TaskID:           "task-1",
		AgentID:          "agent-1",
		RuntimeAdapterID: "codex",
		Prompt:           "build it",
		RequestedNetwork: boolInt(requestedNetwork),
		ExecutionMode:    "sandbox",
		Status:           status,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func TestApproveCommitSuccess(t *testing.T) {
	// Crear un repositorio temporal real para simular workDir
	workDir := t.TempDir()
	setupTestGitRepo(t, workDir)

	// Crear un archivo nuevo modificado para tener algo que commitear
	newFile := filepath.Join(workDir, "new_file.txt")
	if err := os.WriteFile(newFile, []byte("cambio de codigo"), 0o644); err != nil {
		t.Fatalf("failed to write modified file: %v", err)
	}

	runItem := testRun("succeeded", false)
	runItem.RepositoryID = sql.NullString{String: "repo-1", Valid: true}
	runItem.Metadata = `{"work_dir":"` + filepath.ToSlash(workDir) + `"}`

	q := &fakeRunStore{current: runItem}
	h := NewRunHandler(q, nil)

	req := runRequest(http.MethodPost, "/runs/11111111-1111-1111-1111-111111111111/approvals", `{"kind":"commit","decision":"approved"}`)
	rec := httptest.NewRecorder()

	h.ApproveRunAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Validar que el commit se hizo
	cmdLog := exec.Command("git", "log", "-n", "1", "--oneline")
	cmdLog.Dir = workDir
	outLog, errLog := cmdLog.CombinedOutput()
	if errLog != nil {
		t.Fatalf("failed to run git log: %v, output: %s", errLog, string(outLog))
	}
	if !strings.Contains(string(outLog), "battos run 11111111-1111-1111-1111-111111111111") {
		t.Fatalf("expected commit message not found in git log: %s", string(outLog))
	}
}

func TestApprovePushSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("git local push usa Git Bash y puede ser bloqueado por Windows Application Control")
	}

	// Crear un repositorio "origin" que sirva de origen remoto de push
	originDir := t.TempDir()
	setupTestGitRepo(t, originDir)

	// Habilitar la recepcion de pushes en el repo origin (ya que es un repo local no-bare)
	cmdConfig := exec.Command("git", "config", "receive.denyCurrentBranch", "ignore")
	cmdConfig.Dir = originDir
	if err := cmdConfig.Run(); err != nil {
		t.Fatalf("failed to config denyCurrentBranch: %v", err)
	}

	// Crear un repositorio de trabajo temporal. Evitamos `git clone` local en
	// Windows porque Git Bash puede fallar con rutas bajo politicas restrictivas.
	workDir := t.TempDir()
	setupTestGitRepo(t, workDir)
	cmdRemote := exec.Command("git", "remote", "add", "origin", originDir)
	cmdRemote.Dir = workDir
	if out, err := cmdRemote.CombinedOutput(); err != nil {
		t.Fatalf("failed to add origin: %v, output: %s", err, string(out))
	}

	// Crear una rama de prueba
	branchName := "battos-run-11111111-1111-1111-1111-111111111111"
	cmdCheckout := exec.Command("git", "checkout", "-b", branchName)
	cmdCheckout.Dir = workDir
	if err := cmdCheckout.Run(); err != nil {
		t.Fatalf("failed to checkout branch: %v", err)
	}

	// Modificar un archivo y commitearlo
	errWrite := os.WriteFile(filepath.Join(workDir, "push_file.txt"), []byte("data push"), 0o644)
	if errWrite != nil {
		t.Fatalf("failed to write test file: %v", errWrite)
	}
	cmdAdd := exec.Command("git", "add", ".")
	cmdAdd.Dir = workDir
	_ = cmdAdd.Run()

	cmdCommit := exec.Command("git",
		"-c", "user.name=Test",
		"-c", "user.email=test@example.com",
		"commit", "-m", "commit de push",
	)
	cmdCommit.Dir = workDir
	if err := cmdCommit.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	runItem := testRun("succeeded", false)
	runItem.RepositoryID = sql.NullString{String: "repo-1", Valid: true}
	runItem.BranchName = sql.NullString{String: branchName, Valid: true}
	runItem.Metadata = `{"work_dir":"` + filepath.ToSlash(workDir) + `"}`

	q := &fakeRunStore{current: runItem}
	h := NewRunHandler(q, nil)

	req := runRequest(http.MethodPost, "/runs/11111111-1111-1111-1111-111111111111/approvals", `{"kind":"push","decision":"approved"}`)
	rec := httptest.NewRecorder()

	h.ApproveRunAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Validar que el directorio temporal workDir fue eliminado
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Fatalf("expected workDir to be removed, but it still exists")
	}

	// Validar que el commit fue subido al origin en la rama
	cmdBranch := exec.Command("git", "branch", "-a")
	cmdBranch.Dir = originDir
	outBranch, _ := cmdBranch.CombinedOutput()

	cmdShowRef := exec.Command("git", "show-ref", "refs/heads/"+branchName)
	cmdShowRef.Dir = originDir
	errShowRef := cmdShowRef.Run()
	if errShowRef != nil {
		t.Fatalf("expected branch %s to exist in origin repository, show-ref failed: %v, output: %s", branchName, errShowRef, string(outBranch))
	}
}

func TestApprovePushGithubMissingCredentialRejected(t *testing.T) {
	workDir := t.TempDir()
	setupTestGitRepo(t, workDir)

	runItem := testRun("succeeded", false)
	runItem.RepositoryID = sql.NullString{String: "repo-1", Valid: true}
	runItem.BranchName = sql.NullString{String: "battos-run-11111111-1111-1111-1111-111111111111", Valid: true}
	runItem.Metadata = `{"work_dir":"` + filepath.ToSlash(workDir) + `"}`

	q := &fakeRunStore{
		current: runItem,
		repo: &store.Repository{
			ID:            "repo-1",
			Kind:          "github",
			RemoteUrl:     sql.NullString{String: "https://github.com/acme/web.git", Valid: true},
			CredentialRef: sql.NullString{String: "BATTOS_CREDENTIAL_ABSENT_FOR_TEST", Valid: true},
		},
	}
	h := NewRunHandler(q, nil)

	req := runRequest(http.MethodPost, "/runs/11111111-1111-1111-1111-111111111111/approvals", `{"kind":"push","decision":"approved"}`)
	rec := httptest.NewRecorder()

	h.ApproveRunAction(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "credential_ref") {
		t.Fatalf("expected credential_ref error, got: %s", rec.Body.String())
	}
	// El push se aborta antes de limpiar: el workspace debe seguir existiendo.
	if _, err := os.Stat(workDir); err != nil {
		t.Fatalf("workDir should remain after rejected push: %v", err)
	}
}

func setupTestGitRepo(t *testing.T, dir string) {
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git init: %v", err)
	}

	// Desactivar configs globales de usuario que podrian faltar en el entorno de pruebas
	cmdUser := exec.Command("git", "config", "user.name", "Test")
	cmdUser.Dir = dir
	_ = cmdUser.Run()
	cmdEmail := exec.Command("git", "config", "user.email", "test@example.com")
	cmdEmail.Dir = dir
	_ = cmdEmail.Run()

	err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Repo"), 0o644)
	if err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	cmdAdd := exec.Command("git", "add", ".")
	cmdAdd.Dir = dir
	_ = cmdAdd.Run()

	cmdCommit := exec.Command("git",
		"-c", "user.name=Test",
		"-c", "user.email=test@example.com",
		"commit", "-m", "initial commit",
	)
	cmdCommit.Dir = dir
	if err := cmdCommit.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
}

func TestCreateRunExecutionModeDirectAccepted(t *testing.T) {
	q := &fakeRunStore{}
	h := NewRunHandler(q, nil)
	req := httptest.NewRequest(http.MethodPost, "/runs", strings.NewReader(`{"project_id":"web","task_id":"task-1","agent_id":"agent-1","runtime_adapter_id":"codex","prompt":"build it","execution_mode":"direct"}`))
	rec := httptest.NewRecorder()

	h.CreateRun(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if q.createRunParams.ExecutionMode != "direct" {
		t.Fatalf("execution_mode = %q, want direct", q.createRunParams.ExecutionMode)
	}
	if !strings.Contains(rec.Body.String(), `"execution_mode":"direct"`) {
		t.Fatalf("response missing execution_mode=direct: %s", rec.Body.String())
	}
}

func TestCreateRunExecutionModeInvalidRejected(t *testing.T) {
	q := &fakeRunStore{}
	h := NewRunHandler(q, nil)
	req := httptest.NewRequest(http.MethodPost, "/runs", strings.NewReader(`{"project_id":"web","task_id":"task-1","agent_id":"agent-1","runtime_adapter_id":"codex","prompt":"build it","execution_mode":"unknown_mode"}`))
	rec := httptest.NewRecorder()

	h.CreateRun(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "execution_mode") {
		t.Fatalf("expected execution_mode error in body: %s", rec.Body.String())
	}
}

func TestCreateRunExecutionModeDefaultsSandbox(t *testing.T) {
	q := &fakeRunStore{}
	h := NewRunHandler(q, nil)
	req := httptest.NewRequest(http.MethodPost, "/runs", strings.NewReader(`{"project_id":"web","task_id":"task-1","agent_id":"agent-1","runtime_adapter_id":"codex","prompt":"build it"}`))
	rec := httptest.NewRecorder()

	h.CreateRun(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if q.createRunParams.ExecutionMode != "sandbox" {
		t.Fatalf("execution_mode = %q, want sandbox (default)", q.createRunParams.ExecutionMode)
	}
	if !strings.Contains(rec.Body.String(), `"execution_mode":"sandbox"`) {
		t.Fatalf("response missing execution_mode=sandbox: %s", rec.Body.String())
	}
}

func TestApproveRememberSuccess(t *testing.T) {
	memCore, err := memory.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open memory core: %v", err)
	}
	defer memCore.Close()

	runItem := testRun("succeeded", false)
	q := &fakeRunStore{current: runItem}
	h := NewRunHandler(q, memCore)

	req := runRequest(http.MethodPost, "/runs/11111111-1111-1111-1111-111111111111/approvals", `{"kind":"remember","decision":"approved"}`)
	rec := httptest.NewRecorder()

	h.ApproveRunAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	recent, errRecent := memCore.Recent(context.Background(), 1)
	if errRecent != nil {
		t.Fatalf("failed to query recent memories: %v", errRecent)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 saved memory, got %d", len(recent))
	}
	if recent[0].Title != "Run 11111111 succeeded" {
		t.Fatalf("unexpected saved memory title: %s", recent[0].Title)
	}
	if !strings.Contains(recent[0].Content, "- Run: 11111111-1111-1111-1111-111111111111") {
		t.Fatalf("unexpected saved memory content: %s", recent[0].Content)
	}
}
