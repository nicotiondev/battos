package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nicotion/battos/apps/api/internal/store"
)

type fakeKnowledgeStore struct {
	workspace            store.KnowledgeWorkspace
	createJournalParams  store.CreateJournalParams
	createArtifactParams store.CreateArtifactParams
}

func (f *fakeKnowledgeStore) CreateKnowledgeWorkspace(context.Context, store.CreateKnowledgeWorkspaceParams) (store.KnowledgeWorkspace, error) {
	return store.KnowledgeWorkspace{}, nil
}

func (f *fakeKnowledgeStore) ListKnowledgeWorkspaces(context.Context) ([]store.KnowledgeWorkspace, error) {
	return nil, nil
}

func (f *fakeKnowledgeStore) GetKnowledgeWorkspace(context.Context, string) (store.KnowledgeWorkspace, error) {
	return f.workspace, nil
}

func (f *fakeKnowledgeStore) CreateJournal(_ context.Context, arg store.CreateJournalParams) (store.Journal, error) {
	f.createJournalParams = arg
	return store.Journal{
		ID:          "4f81b4b5-0df6-4f8a-b0f4-35ef03b56f31",
		WorkspaceID: arg.WorkspaceID,
		ProjectID:   arg.ProjectID,
		Title:       arg.Title,
		Content:     arg.Content,
		JournalDate: arg.JournalDate,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil
}

func (f *fakeKnowledgeStore) ListJournalsByProject(context.Context, string) ([]store.Journal, error) {
	return nil, nil
}

func (f *fakeKnowledgeStore) GetJournal(context.Context, string) (store.Journal, error) {
	return store.Journal{}, nil
}

func (f *fakeKnowledgeStore) CreateArtifact(_ context.Context, arg store.CreateArtifactParams) (store.Artifact, error) {
	f.createArtifactParams = arg
	return store.Artifact{
		ID:        "a08ddf7c-116a-4e49-a604-39965087a979",
		ProjectID: arg.ProjectID,
		TaskID:    arg.TaskID,
		RunID:     arg.RunID,
		Name:      arg.Name,
		Kind:      arg.Kind,
		Content:   arg.Content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (f *fakeKnowledgeStore) ListArtifactsByProject(context.Context, string) ([]store.Artifact, error) {
	return nil, nil
}

func (f *fakeKnowledgeStore) GetArtifact(context.Context, string) (store.Artifact, error) {
	return store.Artifact{}, nil
}

func TestCreateJournalInfersProjectFromWorkspace(t *testing.T) {
	workspaceID := "92373df9-afc6-49b0-8c78-adfac5259df3"
	q := &fakeKnowledgeStore{workspace: store.KnowledgeWorkspace{ID: workspaceID, ProjectID: "web"}}
	h := NewKnowledgeHandler(q, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/journals", strings.NewReader(`{"workspace_id":"92373df9-afc6-49b0-8c78-adfac5259df3","title":"Daily","content":"Notas"}`))
	rec := httptest.NewRecorder()

	h.CreateJournal(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if q.createJournalParams.ProjectID != "web" || q.createJournalParams.WorkspaceID != workspaceID {
		t.Fatalf("created journal = %+v, want project inferred from workspace", q.createJournalParams)
	}
}

func TestCreateJournalRejectsProjectDifferentFromWorkspace(t *testing.T) {
	q := &fakeKnowledgeStore{workspace: store.KnowledgeWorkspace{ID: "92373df9-afc6-49b0-8c78-adfac5259df3", ProjectID: "web"}}
	h := NewKnowledgeHandler(q, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/journals", strings.NewReader(`{"workspace_id":"92373df9-afc6-49b0-8c78-adfac5259df3","project_id":"other","title":"Daily","content":"Notas"}`))
	rec := httptest.NewRecorder()

	h.CreateJournal(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "project_id no coincide") {
		t.Fatalf("body=%s, want mismatch message", rec.Body.String())
	}
}

func TestCreateArtifactRequiresStorageReference(t *testing.T) {
	h := NewKnowledgeHandler(&fakeKnowledgeStore{}, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/artifacts", strings.NewReader(`{"project_id":"web","name":"Brief","kind":"markdown"}`))
	rec := httptest.NewRecorder()

	h.CreateArtifact(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestCreateArtifactWritesManagedContent(t *testing.T) {
	root := t.TempDir()
	q := &fakeKnowledgeStore{}
	h := NewKnowledgeHandler(q, root)
	req := httptest.NewRequest(http.MethodPost, "/artifacts", strings.NewReader(`{"project_id":"web","name":"Brief","kind":"markdown","bucket":"wiki","content":"# Brief"}`))
	rec := httptest.NewRecorder()

	h.CreateArtifact(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if q.createArtifactParams.Content.Valid {
		t.Fatalf("content stored inline = %+v, want managed file only", q.createArtifactParams.Content)
	}
	if !q.createArtifactParams.ManagedPath.Valid || !strings.Contains(q.createArtifactParams.ManagedPath.String, "web/wiki/") {
		t.Fatalf("managed_path = %+v, want web/wiki path", q.createArtifactParams.ManagedPath)
	}
}

func TestCreateArtifactRejectsPathTraversal(t *testing.T) {
	h := NewKnowledgeHandler(&fakeKnowledgeStore{}, t.TempDir())
	req := httptest.NewRequest(http.MethodPost, "/artifacts", strings.NewReader(`{"project_id":"web","name":"Brief","kind":"markdown","managed_path":"../outside.md","content":"bad"}`))
	rec := httptest.NewRecorder()

	h.CreateArtifact(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
