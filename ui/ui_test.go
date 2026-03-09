package ui

import (
	"goTODO/anytype"
	"testing"
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

func TestFetchTasksWithErrors(t *testing.T) {
	client := anytype.NewClient("fake")
	client.BaseURL = "http://invalid-url"
	s := &State{}
	mock := &mockInvalidator{}

	// Should not panic and should update state with error
	FetchTasks(client, s, mock)

	if !s.Done {
		t.Error("Expected Done to be true even on error")
	}
	if s.Err == nil {
		t.Error("Expected error for invalid URL")
	}
}

