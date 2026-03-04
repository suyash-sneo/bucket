package domain

import (
	"testing"
	"time"
)

func TestParseDueInput(t *testing.T) {
	location := time.Local
	due, err := ParseDueInput("2026-03-04", location)
	if err != nil {
		t.Fatalf("expected date parse success, got %v", err)
	}
	if due.Hour() != 0 || due.Minute() != 0 {
		t.Fatalf("expected local midnight, got %v", due)
	}
	if due.Format(DueDateLayout) != "2026-03-04" {
		t.Fatalf("unexpected parsed date %v", due)
	}

	dueWithTime, err := ParseDueInput("2026-03-04 13:45", location)
	if err != nil {
		t.Fatalf("expected date-time parse success, got %v", err)
	}
	if dueWithTime.Hour() != 13 || dueWithTime.Minute() != 45 {
		t.Fatalf("expected 13:45 local, got %v", dueWithTime)
	}

	if _, err := ParseDueInput("03/04/2026", location); err == nil {
		t.Fatalf("expected invalid format error")
	}
}

func TestEstimatedParsing(t *testing.T) {
	if got, err := ParseEstimatedMinutes(""); err != nil || got != nil {
		t.Fatalf("expected empty estimated to be nil, got %v err=%v", got, err)
	}
	minutes, err := ParseEstimatedMinutes("90")
	if err != nil || minutes == nil || *minutes != 90 {
		t.Fatalf("expected 90 minutes, got %v err=%v", minutes, err)
	}
	duration, err := ParseEstimatedMinutes("1h30m")
	if err != nil || duration == nil || *duration != 90 {
		t.Fatalf("expected 1h30m => 90, got %v err=%v", duration, err)
	}
	if _, err := ParseEstimatedMinutes("abc"); err == nil {
		t.Fatalf("expected invalid estimate error")
	}
}
