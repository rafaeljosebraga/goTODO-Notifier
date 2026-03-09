package ui

import (
	"goTODO/anytype"
	"testing"
	"time"
)

type mockInvalidator struct {
	invalidated bool
}

func (m *mockInvalidator) Invalidate() {
	m.invalidated = true
}

func TestUpdateState(t *testing.T) {
	s := &State{}
	tasks := []anytype.Task{
		{ID: "1", Name: "Task 1"},
		{ID: "2", Name: "Task 2"},
	}
	mock := &mockInvalidator{}

	UpdateState(s, tasks, nil, mock)

	if !s.Done {
		t.Error("Expected Done to be true")
	}
	if s.Loading {
		t.Error("Expected Loading to be false")
	}
	if len(s.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(s.Tasks))
	}
	if len(s.TaskClickables) != 2 {
		t.Errorf("Expected 2 clickables, got %d", len(s.TaskClickables))
	}
	if !mock.invalidated {
		t.Error("Expected Invalidate to be called")
	}
}

func TestSortTasks(t *testing.T) {
	now := time.Now()
	tasks := []anytype.Task{
		{ID: "no-date", Name: "Later"},
		{ID: "soon", Name: "Soon", DueDate: now.Add(1 * time.Hour)},
		{ID: "earlier", Name: "Earlier", DueDate: now.Add(-1 * time.Hour)},
	}

	SortTasks(tasks)

	if tasks[0].ID != "earlier" {
		t.Errorf("Expected first task to be 'earlier', got %s", tasks[0].ID)
	}
	if tasks[1].ID != "soon" {
		t.Errorf("Expected second task to be 'soon', got %s", tasks[1].ID)
	}
	if tasks[2].ID != "no-date" {
		t.Errorf("Expected last task to be 'no-date', got %s", tasks[2].ID)
	}
}
