package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCappedWriterTruncatesAndPreservesTail(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "log.txt")
	writer, err := NewCappedWriter(path, 64)
	if err != nil {
		t.Fatalf("new capped writer: %v", err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	payload := strings.Repeat("a", 80) + "TAIL"
	if _, err := writer.Write([]byte(payload)); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if int64(len(content)) > 64 {
		t.Fatalf("expected log <= max bytes, got %d", len(content))
	}
	if !strings.Contains(string(content), "TAIL") {
		t.Fatalf("expected truncated log to keep tail content: %q", string(content))
	}
}

func TestCappedWriterTruncationThrottled(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "log.txt")
	writer, err := NewCappedWriter(path, 32)
	if err != nil {
		t.Fatalf("new capped writer: %v", err)
	}
	t.Cleanup(func() { _ = writer.Close() })

	base := time.Now()
	current := base
	writer.SetClockForTest(func() time.Time { return current })

	if _, err := writer.Write([]byte(strings.Repeat("x", 40))); err != nil {
		t.Fatalf("write #1: %v", err)
	}
	sizeAfterFirst := writer.CurrentSize()

	if _, err := writer.Write([]byte(strings.Repeat("y", 40))); err != nil {
		t.Fatalf("write #2: %v", err)
	}
	sizeAfterSecond := writer.CurrentSize()
	if sizeAfterSecond <= sizeAfterFirst {
		t.Fatalf("expected no second truncation in same second; size1=%d size2=%d", sizeAfterFirst, sizeAfterSecond)
	}

	current = base.Add(2 * time.Second)
	if _, err := writer.Write([]byte(strings.Repeat("z", 40))); err != nil {
		t.Fatalf("write #3: %v", err)
	}
	sizeAfterThird := writer.CurrentSize()
	if sizeAfterThird > sizeAfterSecond {
		t.Fatalf("expected truncation after throttle window; size2=%d size3=%d", sizeAfterSecond, sizeAfterThird)
	}
}
