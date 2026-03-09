package ui

import (
	"fmt"
	"goTODO/anytype"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNavigateToTaskFullFlow(t *testing.T) {
	// 1. Setup mock server to handle object details and name resolution
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Handle details request
		if r.URL.Path == "/v1/spaces/faculdade-space-id/objects/task-1" {
			fmt.Fprint(w, `{"object":{"markdown":"# Task 1\n- [ ] Sub 1\n[Link](anytype://object?id=link-1)"}}`)
			return
		}
		// Handle name resolution request
		if r.URL.Path == "/v1/spaces/faculdade-space-id/objects/link-1" {
			fmt.Fprint(w, `{"object":{"name":"Resolved Link Name"}}`)
			return
		}
		// Fallback for spaces
		if r.URL.Path == "/v1/spaces" {
			fmt.Fprint(w, `{"data":[{"id":"faculdade-space-id","name":"Faculdade"}]}`)
			return
		}
	}))
	defer server.Close()

	client := anytype.NewClient("fake")
	client.BaseURL = server.URL + "/v1"

	s := &State{
		Tasks: []anytype.Task{
			{ID: "task-1", Name: "Task 1", Links: []string{"link-1"}, LinkNames: make(map[string]string)},
		},
	}
	mock := &mockInvalidator{}

	// 2. Trigger Navigation
	NavigateToTask(client, s, "task-1", mock)

	// Wait for background goroutines to complete
	time.Sleep(500 * time.Millisecond)

	// 3. Assertions
	s.Mu.Lock()
	defer s.Mu.Unlock()

	// Check Markdown Cleaning
	expectedMd := "# Task 1\n- [ ] Sub 1\nLink"
	if s.Tasks[0].Markdown != expectedMd {
		t.Errorf("Markdown not cleaned correctly.\nGot: %q\nExp: %q", s.Tasks[0].Markdown, expectedMd)
	}

	// Check Link Name Resolution
	if s.Tasks[0].LinkNames["link-1"] != "Resolved Link Name" {
		t.Errorf("Link name not resolved correctly. Got: %s", s.Tasks[0].LinkNames["link-1"])
	}

	if s.CurrentView != "details" {
		t.Errorf("Expected view 'details', got %s", s.CurrentView)
	}
}

func TestNavigateToTaskStubCreation(t *testing.T) {
	// Scenario: Navigate to a task ID that does NOT exist in s.Tasks list
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/spaces/faculdade-space-id/objects/new-task" {
			fmt.Fprint(w, `{"object":{"markdown":"New Task Markdown"}}`)
			return
		}
		if r.URL.Path == "/v1/spaces" {
			fmt.Fprint(w, `{"data":[{"id":"faculdade-space-id","name":"Faculdade"}]}`)
			return
		}
	}))
	defer server.Close()

	client := anytype.NewClient("fake")
	client.BaseURL = server.URL + "/v1"
	s := &State{Tasks: []anytype.Task{}} // Empty initial list
	mock := &mockInvalidator{}

	NavigateToTask(client, s, "new-task", mock)
	time.Sleep(500 * time.Millisecond)

	s.Mu.Lock()
	defer s.Mu.Unlock()

	if len(s.Tasks) != 1 {
		t.Fatalf("Expected 1 task (the stub), got %d", len(s.Tasks))
	}
	if s.Tasks[0].ID != "new-task" {
		t.Errorf("Stub ID mismatch. Got %s", s.Tasks[0].ID)
	}
	if s.Tasks[0].Markdown != "New Task Markdown" {
		t.Errorf("Stub Markdown mismatch. Got %q", s.Tasks[0].Markdown)
	}
}

func TestNavigateToTaskApiFailure(t *testing.T) {
	// Scenario: API returns error during navigation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := anytype.NewClient("fake")
	client.BaseURL = server.URL + "/v1"
	s := &State{Tasks: []anytype.Task{{ID: "task-1"}}}
	mock := &mockInvalidator{}

	// Should not panic
	NavigateToTask(client, s, "task-1", mock)
	time.Sleep(200 * time.Millisecond)

	// State should still be in details view even if fetch failed
	if s.CurrentView != "details" {
		t.Errorf("Expected view 'details', got %s", s.CurrentView)
	}
}
