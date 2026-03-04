package domain

import "testing"

func TestTaskTitleValidation(t *testing.T) {
	task := Task{Title: "  hello  ", Status: StatusCreated}
	if err := ValidateTask(&task); err != nil {
		t.Fatalf("expected valid task, got error: %v", err)
	}
	if task.Title != "hello" {
		t.Fatalf("expected title to be trimmed, got %q", task.Title)
	}

	empty := Task{Title: "   ", Status: StatusCreated}
	if err := ValidateTask(&empty); err == nil {
		t.Fatalf("expected empty title error")
	}
}

func TestStatusCycle(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{StatusCreated, StatusInProgress},
		{StatusInProgress, StatusCompleted},
		{StatusCompleted, StatusClosed},
		{StatusClosed, StatusArchived},
		{StatusArchived, StatusCreated},
		{"custom", StatusCompleted},
	}
	for _, tc := range cases {
		got := CycleStatus(tc.input)
		if got != tc.want {
			t.Fatalf("cycle %q: got %q want %q", tc.input, got, tc.want)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	if got, err := NormalizeURL(""); err != nil || got != "" {
		t.Fatalf("expected empty URL to pass, got %q err=%v", got, err)
	}
	if got, err := NormalizeURL("example.com"); err != nil || got != "https://example.com" {
		t.Fatalf("expected normalized URL, got %q err=%v", got, err)
	}
	if _, err := NormalizeURL("http://"); err == nil {
		t.Fatalf("expected invalid URL error")
	}
}

func TestPriorityProgressBounds(t *testing.T) {
	priority := 0
	task := Task{Title: "x", Status: StatusCreated, Priority: &priority}
	if err := ValidateTask(&task); err == nil {
		t.Fatalf("expected priority bounds error")
	}
	priority = 4
	task.Priority = &priority
	if err := ValidateTask(&task); err != nil {
		t.Fatalf("expected valid priority, got %v", err)
	}

	progress := 101
	task.Progress = &progress
	if err := ValidateTask(&task); err == nil {
		t.Fatalf("expected progress bounds error")
	}
	progress = 100
	task.Progress = &progress
	if err := ValidateTask(&task); err != nil {
		t.Fatalf("expected valid progress, got %v", err)
	}
}
