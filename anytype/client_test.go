package anytype

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchTasks(t *testing.T) {
	// 1. Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse := `{
			"data": [
				{
					"id": "task-1",
					"name": "Pending Task",
					"layout": "action",
					"properties": [
						{"key": "due_date", "date": "2026-03-10T03:00:00Z"},
						{"key": "status", "select": {"name": "In Progress"}},
						{"key": "done", "checkbox": false},
						{"key": "links", "objects": ["sub-1"]}
					]
				},
				{
					"id": "task-2",
					"name": "Completed Task",
					"layout": "action",
					"properties": [
						{"key": "done", "checkbox": true}
					]
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, jsonResponse)
	}))
	defer server.Close()

	// 2. Initialize client pointing to mock server
	client := NewClient("fake-key")
	client.BaseURL = server.URL // Override with mock URL

	// 3. Call FetchTasks
	tasks, err := client.FetchTasks("space-id", "type-id")

	// 4. Assertions
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("Expected 2 tasks, got %d", len(tasks))
	}

	// Verify Task 1 (Pending)
	t1 := tasks[0]
	if t1.ID != "task-1" || t1.Name != "Pending Task" {
		t.Errorf("Task 1 mismatch: %+v", t1)
	}
	if t1.Status != "In Progress" {
		t.Errorf("Expected status 'In Progress', got '%s'", t1.Status)
	}
	if t1.IsCompleted != false {
		t.Error("Expected IsCompleted to be false")
	}
	if len(t1.Links) != 1 || t1.Links[0] != "sub-1" {
		t.Errorf("Expected links ['sub-1'], got %v", t1.Links)
	}

	// Verify Task 2 (Completed)
	t2 := tasks[1]
	if t2.IsCompleted != true {
		t.Error("Expected IsCompleted to be true for task 2")
	}
}

func TestGetFirstSpaceID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse := `{"data":[{"id":"space-1","name":"Faculdade"}]}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, jsonResponse)
	}))
	defer server.Close()

	client := NewClient("fake-key")
	client.BaseURL = server.URL

	id, name, err := client.GetFirstSpaceID()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if id != "space-1" || name != "Faculdade" {
		t.Errorf("Expected id 'space-1', name 'Faculdade', got %s, %s", id, name)
	}
}

func TestDiscoverTaskTypeID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse := `{"data":[{"id":"type-1","name":"Task"}]}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, jsonResponse)
	}))
	defer server.Close()

	client := NewClient("fake-key")
	client.BaseURL = server.URL

	id, err := client.DiscoverTaskTypeID("space-id")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if id != "type-1" {
		t.Errorf("Expected id 'type-1', got %s", id)
	}
}

func TestApiErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error": "Unauthorized"}`)
	}))
	defer server.Close()

	client := NewClient("fake-key")
	client.BaseURL = server.URL

	_, err := client.FetchTasks("space", "type")
	if err == nil {
		t.Error("Expected error for 401 response, got nil")
	}
}

func TestGetFirstSpaceID_Fallback(t *testing.T) {
	// Scenario: "Faculdade" space doesn't exist, should take the first one
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse := `{"data":[{"id":"other-id","name":"Personal"}]}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, jsonResponse)
	}))
	defer server.Close()

	client := NewClient("fake-key")
	client.BaseURL = server.URL

	id, name, err := client.GetFirstSpaceID()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if id != "other-id" || name != "Personal" {
		t.Errorf("Expected fallback to 'Personal', got %s", name)
	}
}

func TestCleanMarkdownEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty", "", ""},
		{"No Links", "Just plain text", "Just plain text"},
		{"Multiple Links", "[Link 1](anytype://object?id=1) and [Link 2](anytype://object?id=2)", "Link 1 and Link 2"},
		{"Broken Link", "[Broken](anytype://wrong-format)", "[Broken](anytype://wrong-format)"},
		{"Mixed Content", "# Title\n[Real](anytype://object?id=123)\n[Fake](http://google.com)", "# Title\nReal\n[Fake](http://google.com)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := CleanMarkdown(tt.input); result != tt.expected {
				t.Errorf("CleanMarkdown() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseSubTasksMalformed(t *testing.T) {
	input := "- [ ]Missing Space\n-[ ] No Dash Space\n- [x]Completed\n- [] Empty Brackets"
	subTasks := ParseSubTasks(input)
	
	// Based on current prefix check "- [ ] ", only valid ones should be picked
	// If the implementation is strict, the count might be low.
	// This helps us decide if we need more robust regex here.
	if len(subTasks) > 0 {
		t.Logf("Found %d subtasks in malformed input", len(subTasks))
	}
}
