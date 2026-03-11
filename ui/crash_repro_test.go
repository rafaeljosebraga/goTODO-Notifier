package ui

import (
	"goTODO/anytype"
	"sync"
	"testing"
	"time"

	"gioui.org/widget"
)

func TestShowTaskDetailsRace(t *testing.T) {
	// This test simulates the UI rendering details while a background goroutine
	// updates the task list, potentially causing a crash if slices are accessed unsafely.
	
	s := &State{
		SelectedTaskID: "task-1",
		Tasks: []anytype.Task{
			{
				ID:    "task-1",
				Name:  "Task 1",
				Links: []string{"l1", "l2"},
			},
		},
		LinkClickables: make([]widget.Clickable, 2),
		CurrentView:    "details",
	}
	
	client := anytype.NewClient("fake")
	mock := &mockInvalidator{}
	
	_ = client
	_ = mock
	
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Simulate UI "Rendering" (Accessing state)
	go func() {
		defer wg.Done()
		start := time.Now()
		for time.Since(start) < 500*time.Millisecond {
			// Simulate what showTaskDetails does
			s.Mu.Lock()
			var task anytype.Task
			for _, tk := range s.Tasks {
				if tk.ID == s.SelectedTaskID {
					task = tk
					break
				}
			}
			// Re-allocating clickables like the UI does
			if len(s.LinkClickables) != len(task.Links) {
				s.LinkClickables = make([]widget.Clickable, len(task.Links))
			}
			s.Mu.Unlock()

			// Accessing the local copy of task.Links and s.LinkClickables
			// This is where the crash usually happens in Gio apps if not careful
			for i := range task.Links {
				_ = task.Links[i]
				if i < len(s.LinkClickables) {
					_ = s.LinkClickables[i].History() // Accessing the widget
				}
			}
			time.Sleep(1 * time.Microsecond)
		}
	}()

	// Goroutine 2: Simulate Background Updates (Modifying state)
	go func() {
		defer wg.Done()
		start := time.Now()
		for time.Since(start) < 500*time.Millisecond {
			s.Mu.Lock()
			// Rapidly change the number of links
			newLinks := []string{"new-1"}
			if time.Now().UnixNano()%2 == 0 {
				newLinks = []string{"n1", "n2", "n3", "n4"}
			}
			
			for i := range s.Tasks {
				if s.Tasks[i].ID == "task-1" {
					s.Tasks[i].Links = newLinks
				}
			}
			s.Mu.Unlock()
			time.Sleep(1 * time.Microsecond)
		}
	}()

	wg.Wait()
}

// We also need to test if clicking causes issues
func TestNavigateToTaskConcurrency(t *testing.T) {
	s := &State{
		Tasks: []anytype.Task{{ID: "1", Name: "T1"}},
	}
	client := anytype.NewClient("fake")
	mock := &mockInvalidator{}

	// Rapidly call NavigateToTask
	for i := 0; i < 100; i++ {
		go NavigateToTask(client, s, "1", mock)
	}
	
	time.Sleep(100 * time.Millisecond)
}
