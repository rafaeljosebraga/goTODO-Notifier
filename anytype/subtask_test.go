package anytype

import (
	"testing"
)

func TestParseSubTasks(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		expected int
	}{
		{
			"Standard subtasks",
			"- [ ] Subtask 1\n- [x] Subtask 2",
			2,
		},
		{
			"Mixed content",
			"# Title\nSome description\n- [ ] Task A\nMore text\n- [x] Task B",
			2,
		},
		{
			"No subtasks",
			"Just some text without checkboxes",
			0,
		},
		{
			"Malformed checkboxes",
			"-[ ] Missing space\n- [  ] Extra space\n* [ ] Wrong bullet",
			0,
		},
		{
			"Empty input",
			"",
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests if ParseSubTasks crashes with various inputs
			subtasks := ParseSubTasks(tt.markdown)
			if len(subtasks) != tt.expected {
				t.Errorf("Expected %d subtasks, got %d", tt.expected, len(subtasks))
			}

			// Check consistency of values if any were found
			for _, st := range subtasks {
				if st.Name == "" {
					t.Errorf("Subtask name should not be empty")
				}
			}
		})
	}
}

func TestSubTaskCrashSimulation(t *testing.T) {
	// Trying to trigger potential nil pointer or out-of-bounds
	inputs := []string{
		"- [ ] ",    // Empty name
		"- [x] ",    // Empty name completed
		"- [ ]\n",   // Newline after
		" - [ ] ",   // Leading space
	}

	for _, input := range inputs {
		// Should not panic
		_ = ParseSubTasks(input)
	}
}
