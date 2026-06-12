package worker

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/nicotion/battos/apps/api/internal/store"
)

// poolStore es un Store thread-safe con una cola real: cada run se entrega
// exactamente una vez (claim atómico bajo mutex, como el UPDATE de SQLite).
type poolStore struct {
	mu        sync.Mutex
	queue     []store.Run
	claimed   map[string]int
	completed map[string]int
	logs      []store.AppendRunLogParams
}

func newPoolStore(runs ...store.Run) *poolStore {
	return &poolStore{
		queue:     runs,
		claimed:   map[string]int{},
		completed: map[string]int{},
	}
}

func (p *poolStore) ClaimNextQueuedRun(context.Context) (store.Run, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.queue) == 0 {
		return store.Run{}, sql.ErrNoRows
	}
	run := p.queue[0]
	p.queue = p.queue[1:]
	p.claimed[run.ID]++
	return run, nil
}

func (p *poolStore) ClaimQueuedRunByID(_ context.Context, id string) (store.Run, error) {
	return store.Run{}, sql.ErrNoRows
}

func (p *poolStore) AppendRunLog(_ context.Context, arg store.AppendRunLogParams) (store.RunLog, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.logs = append(p.logs, arg)
	return store.RunLog{ID: int64(len(p.logs)), RunID: arg.RunID, Stream: arg.Stream, Message: arg.Message}, nil
}

func (p *poolStore) ListRunLogs(_ context.Context, runID string) ([]store.RunLog, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []store.RunLog
	for i, item := range p.logs {
		if item.RunID == runID {
			out = append(out, store.RunLog{ID: int64(i + 1), RunID: item.RunID, Stream: item.Stream, Message: item.Message})
		}
	}
	return out, nil
}

func (p *poolStore) CompleteRun(_ context.Context, arg store.CompleteRunParams) (store.Run, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completed[arg.ID]++
	return store.Run{ID: arg.ID, Status: "succeeded"}, nil
}

func (p *poolStore) FailRun(_ context.Context, arg store.FailRunParams) (store.Run, error) {
	return store.Run{ID: arg.ID, Status: "failed"}, nil
}

func (p *poolStore) CreateArtifact(_ context.Context, arg store.CreateArtifactParams) (store.Artifact, error) {
	return store.Artifact{ID: "artifact", ProjectID: arg.ProjectID}, nil
}

func (p *poolStore) GetRepository(_ context.Context, id string) (store.Repository, error) {
	return store.Repository{}, sql.ErrNoRows
}

func (p *poolStore) GetCredentialByName(_ context.Context, _ string) (store.Credential, error) {
	return store.Credential{}, sql.ErrNoRows
}

func (p *poolStore) UpdateRunBranchAndMetadata(_ context.Context, arg store.UpdateRunBranchAndMetadataParams) (store.Run, error) {
	return store.Run{ID: arg.ID}, nil
}

func (p *poolStore) CreateUsageEvent(_ context.Context, arg store.CreateUsageEventParams) (store.UsageEvent, error) {
	return store.UsageEvent{ID: "usage"}, nil
}

func (p *poolStore) snapshot() (claimed, completed map[string]int, pending int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	claimed = make(map[string]int, len(p.claimed))
	for k, v := range p.claimed {
		claimed[k] = v
	}
	completed = make(map[string]int, len(p.completed))
	for k, v := range p.completed {
		completed[k] = v
	}
	return claimed, completed, len(p.queue)
}

// TestRunPoolProcessesEachRunExactlyOnce verifica la garantía central de B2:
// N goroutines concurrentes sobre la misma cola, cada run claimed y
// completado exactamente una vez. Correr con -race.
func TestRunPoolProcessesEachRunExactlyOnce(t *testing.T) {
	const totalRuns = 12
	const concurrency = 4

	runs := make([]store.Run, 0, totalRuns)
	for i := 0; i < totalRuns; i++ {
		run := testRun("codex")
		run.ID = fmt.Sprintf("run-%02d", i)
		runs = append(runs, run)
	}
	ps := newPoolStore(runs...)

	w := New(ps, &fakeSandbox{result: Result{Summary: "done"}}, map[string]Adapter{
		"codex": fakeAdapter{plan: testPlan("codex")},
	})
	w.ArtifactsDir = t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.RunPool(ctx, concurrency, 10*time.Millisecond) }()

	deadline := time.After(10 * time.Second)
	for {
		_, completed, pending := ps.snapshot()
		if len(completed) == totalRuns && pending == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout: completados %d/%d", len(completed), totalRuns)
		case <-time.After(20 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("RunPool devolvió error: %v", err)
	}

	claimed, completed, _ := ps.snapshot()
	for id, count := range claimed {
		if count != 1 {
			t.Errorf("run %s claimed %d veces, want 1", id, count)
		}
	}
	for id, count := range completed {
		if count != 1 {
			t.Errorf("run %s completado %d veces, want 1", id, count)
		}
	}
	if len(completed) != totalRuns {
		t.Fatalf("completados %d runs, want %d", len(completed), totalRuns)
	}
}

func TestRunPoolWithConcurrencyOneFallsBackToRunLoop(t *testing.T) {
	run := testRun("codex")
	run.ID = "solo-run"
	ps := newPoolStore(run)
	w := New(ps, &fakeSandbox{result: Result{Summary: "done"}}, map[string]Adapter{
		"codex": fakeAdapter{plan: testPlan("codex")},
	})
	w.ArtifactsDir = t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.RunPool(ctx, 1, 10*time.Millisecond) }()

	deadline := time.After(5 * time.Second)
	for {
		_, completed, _ := ps.snapshot()
		if len(completed) == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout esperando el run")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("RunPool devolvió error: %v", err)
	}
}

type fakePromoter struct {
	mu    sync.Mutex
	runs  []store.Run
	nLogs []int
}

func (p *fakePromoter) PromoteRunSummary(_ context.Context, run store.Run, logs []store.RunLog) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.runs = append(p.runs, run)
	p.nLogs = append(p.nLogs, len(logs))
	return nil
}

// TestAutoRememberPromotesRunSummary verifica B3: un run propuesto con
// auto_remember=true en metadata promueve su resumen al terminar.
func TestAutoRememberPromotesRunSummary(t *testing.T) {
	run := testRun("codex")
	run.Metadata = `{"auto_remember":true}`
	fs := &fakeStore{run: run}
	promoter := &fakePromoter{}
	w := New(fs, &fakeSandbox{result: Result{Summary: "done"}}, map[string]Adapter{
		"codex": fakeAdapter{plan: testPlan("codex")},
	})
	w.ArtifactsDir = t.TempDir()
	w.MemoryPromote = promoter

	processed, err := w.ProcessOne(context.Background())
	if err != nil || !processed {
		t.Fatalf("ProcessOne = (%v, %v)", processed, err)
	}
	if len(promoter.runs) != 1 {
		t.Fatalf("promociones = %d, want 1", len(promoter.runs))
	}
	if promoter.runs[0].Status != "succeeded" {
		t.Fatalf("run promovido con status %q, want succeeded", promoter.runs[0].Status)
	}
	if promoter.nLogs[0] == 0 {
		t.Fatal("la promoción debe recibir los logs del run")
	}
}

func TestNoAutoRememberDoesNotPromote(t *testing.T) {
	run := testRun("codex") // sin metadata auto_remember
	fs := &fakeStore{run: run}
	promoter := &fakePromoter{}
	w := New(fs, &fakeSandbox{result: Result{Summary: "done"}}, map[string]Adapter{
		"codex": fakeAdapter{plan: testPlan("codex")},
	})
	w.ArtifactsDir = t.TempDir()
	w.MemoryPromote = promoter

	if _, err := w.ProcessOne(context.Background()); err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}
	if len(promoter.runs) != 0 {
		t.Fatalf("promociones = %d, want 0", len(promoter.runs))
	}
}
