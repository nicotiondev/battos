package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nicotion/battos/apps/api/internal/store"
)

type fakeWorkStore struct {
	createDomainParams store.CreateDomainParams
	createTaskParams   store.CreateTaskParams
	ensureInboxCalled  bool
	getTaskResult      store.Task
	updateTaskParams   store.UpdateTaskParams
}

func (f *fakeWorkStore) CreateDomain(_ context.Context, arg store.CreateDomainParams) (store.Domain, error) {
	f.createDomainParams = arg
	return store.Domain{ID: arg.ID, Slug: arg.Slug, Name: arg.Name, Status: arg.Status}, nil
}

func (f *fakeWorkStore) ListDomains(context.Context) ([]store.Domain, error) {
	return nil, nil
}

func (f *fakeWorkStore) GetDomain(context.Context, string) (store.Domain, error) {
	return store.Domain{}, nil
}

func (f *fakeWorkStore) UpdateDomain(context.Context, store.UpdateDomainParams) (store.Domain, error) {
	return store.Domain{}, nil
}

func (f *fakeWorkStore) CreateProject(context.Context, store.CreateProjectParams) (store.Project, error) {
	return store.Project{}, nil
}

func (f *fakeWorkStore) ListProjects(context.Context) ([]store.Project, error) {
	return nil, nil
}

func (f *fakeWorkStore) EnsureInboxProject(context.Context) (store.Project, error) {
	f.ensureInboxCalled = true
	return store.Project{ID: "inbox", Slug: "inbox", Name: "Inbox", Status: "active"}, nil
}

func (f *fakeWorkStore) GetProject(context.Context, string) (store.Project, error) {
	return store.Project{}, nil
}

func (f *fakeWorkStore) UpdateProject(context.Context, store.UpdateProjectParams) (store.Project, error) {
	return store.Project{}, nil
}

func (f *fakeWorkStore) CreateGoal(context.Context, store.CreateGoalParams) (store.Goal, error) {
	return store.Goal{}, nil
}

func (f *fakeWorkStore) ListGoals(context.Context) ([]store.Goal, error) {
	return []store.Goal{{ID: "goal-1", ProjectID: "web", Title: "Publicar landing", Status: "planned"}}, nil
}

func (f *fakeWorkStore) ListGoalsByProject(context.Context, string) ([]store.Goal, error) {
	return nil, nil
}

func (f *fakeWorkStore) GetGoal(context.Context, string) (store.Goal, error) {
	return store.Goal{ID: "goal-1", ProjectID: "web", Title: "Publicar landing", Status: "planned"}, nil
}

func (f *fakeWorkStore) UpdateGoal(context.Context, store.UpdateGoalParams) (store.Goal, error) {
	return store.Goal{}, nil
}

func (f *fakeWorkStore) CreateTask(_ context.Context, arg store.CreateTaskParams) (store.Task, error) {
	f.createTaskParams = arg
	return store.Task{ID: "task-1", ProjectID: arg.ProjectID, Title: arg.Title, Status: arg.Status}, nil
}

func (f *fakeWorkStore) ListTasks(context.Context) ([]store.Task, error) {
	return []store.Task{{ID: "task-1", ProjectID: "web", Title: "Preparar brief", Status: "backlog"}}, nil
}

func (f *fakeWorkStore) ListTasksByProject(context.Context, string) ([]store.Task, error) {
	return nil, nil
}

func (f *fakeWorkStore) GetTask(context.Context, string) (store.Task, error) {
	return f.getTaskResult, nil
}

func (f *fakeWorkStore) UpdateTask(_ context.Context, arg store.UpdateTaskParams) (store.Task, error) {
	f.updateTaskParams = arg
	return store.Task{ID: arg.ID, ProjectID: arg.ProjectID, GoalID: arg.GoalID, Title: arg.Title, Description: arg.Description, Status: arg.Status, BoardPosition: arg.BoardPosition}, nil
}

func TestCreateDomainDefaultsToActive(t *testing.T) {
	q := &fakeWorkStore{}
	h := NewWorkHandler(q)
	req := httptest.NewRequest(http.MethodPost, "/domains", strings.NewReader(`{"slug":"studio","name":"Studio"}`))
	rec := httptest.NewRecorder()

	h.CreateDomain(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if q.createDomainParams.ID != "studio" || q.createDomainParams.Status != "active" {
		t.Fatalf("created domain = %+v, want slug id and active status", q.createDomainParams)
	}
}

func TestCreateTaskDefaultsToBacklogAndNullableGoal(t *testing.T) {
	q := &fakeWorkStore{}
	h := NewWorkHandler(q)
	req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"project_id":"web","title":"Brief"}`))
	rec := httptest.NewRecorder()

	h.CreateTask(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if q.createTaskParams.Status != "backlog" || q.createTaskParams.GoalID != (sql.NullString{}) {
		t.Fatalf("created task = %+v, want backlog without goal", q.createTaskParams)
	}
}

func TestCreateTaskWithoutProjectUsesInbox(t *testing.T) {
	q := &fakeWorkStore{}
	h := NewWorkHandler(q)
	req := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"title":"Idea suelta"}`))
	rec := httptest.NewRecorder()

	h.CreateTask(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !q.ensureInboxCalled || q.createTaskParams.ProjectID != "inbox" {
		t.Fatalf("created task = %+v ensureInbox=%v, want inbox task", q.createTaskParams, q.ensureInboxCalled)
	}
}

func TestListTasksWithoutProjectReturnsGlobalBoard(t *testing.T) {
	h := NewWorkHandler(&fakeWorkStore{})
	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	rec := httptest.NewRecorder()

	h.ListTasks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"project_id":"web"`) {
		t.Fatalf("body=%s, want global tasks payload", rec.Body.String())
	}
}

func TestUpdateTaskPreservesFieldsOmittedByPatch(t *testing.T) {
	q := &fakeWorkStore{getTaskResult: store.Task{
		ID:            "task-1",
		ProjectID:     "web",
		Title:         "Preparar brief",
		Description:   sql.NullString{String: "No borrar", Valid: true},
		Status:        "backlog",
		BoardPosition: 7,
	}}
	h := NewWorkHandler(q)
	req := httptest.NewRequest(http.MethodPatch, "/tasks/task-1", strings.NewReader(`{"status":"ready"}`))
	req = req.WithContext(context.Background())
	rec := httptest.NewRecorder()

	h.UpdateTask(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if q.updateTaskParams.Title != "Preparar brief" ||
		q.updateTaskParams.ProjectID != "web" ||
		q.updateTaskParams.Description.String != "No borrar" ||
		q.updateTaskParams.BoardPosition != 7 ||
		q.updateTaskParams.Status != "ready" {
		t.Fatalf("updated task = %+v, want only status changed", q.updateTaskParams)
	}
}

func TestUpdateTaskRejectsGoalFromAnotherProject(t *testing.T) {
	q := &fakeWorkStore{getTaskResult: store.Task{
		ID:        "task-1",
		ProjectID: "other",
		Title:     "Preparar brief",
		Status:    "backlog",
	}}
	h := NewWorkHandler(q)
	req := httptest.NewRequest(http.MethodPatch, "/tasks/task-1", strings.NewReader(`{"goal_id":"goal-1"}`))
	rec := httptest.NewRecorder()

	h.UpdateTask(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "otro proyecto") {
		t.Fatalf("body=%s, want cross-project goal message", rec.Body.String())
	}
}
