package main

import (
	"goTODO/anytype"
	"goTODO/ui"
	"sync"
	"testing"
	"time"
)

type mockNotifier struct {
	mu       sync.Mutex
	notified bool
}

func (m *mockNotifier) Notify(title, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notified = true
	return nil
}

func TestProcessReminders(t *testing.T) {
	s := &ui.State{
		Tasks: []anytype.Task{
			{
				ID:          "1",
				Name:        "Test Task",
				DueDate:     time.Now().Add(1 * time.Hour),
				IsCompleted: false,
			},
		},
	}
	notifier := &mockNotifier{}

	processReminders(notifier, s)

	if !notifier.notified {
		t.Error("Expected Notify to be called for pending task")
	}

	// Test completed task
	notifier.notified = false
	s.Tasks[0].IsCompleted = true
	processReminders(notifier, s)

	if notifier.notified {
		t.Error("Expected Notify NOT to be called for completed task")
	}

	// Test task due in the future (> 24h)
	notifier.notified = false
	s.Tasks[0].IsCompleted = false
	s.Tasks[0].DueDate = time.Now().Add(48 * time.Hour)
	processReminders(notifier, s)
	if notifier.notified {
		t.Error("Expected Notify NOT to be called for task due in 48h")
	}
}

