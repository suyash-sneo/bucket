package logging

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type CappedWriter struct {
	mu              sync.Mutex
	file            *os.File
	path            string
	maxBytes        int64
	currentSize     int64
	lastTruncateAt  time.Time
	stderrWarned    bool
	truncationClock func() time.Time
}

func NewCappedWriter(path string, maxBytes int64) (*CappedWriter, error) {
	if maxBytes < 1 {
		return nil, fmt.Errorf("maxBytes must be >= 1")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", path, err)
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat log file %s: %w", path, err)
	}
	return &CappedWriter{
		file:            file,
		path:            path,
		maxBytes:        maxBytes,
		currentSize:     stat.Size(),
		truncationClock: time.Now,
	}, nil
}

func (writer *CappedWriter) Write(payload []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()

	count, err := writer.file.Write(payload)
	writer.currentSize += int64(count)
	if err != nil {
		return count, err
	}

	now := writer.truncationClock()
	if writer.currentSize > writer.maxBytes && now.Sub(writer.lastTruncateAt) >= time.Second {
		writer.lastTruncateAt = now
		if truncateErr := writer.truncateToTailLocked(); truncateErr != nil {
			writer.warnToStderrOnceLocked(truncateErr)
		}
	}
	return count, nil
}

func (writer *CappedWriter) truncateToTailLocked() error {
	if err := writer.file.Close(); err != nil {
		return fmt.Errorf("close log file before truncation: %w", err)
	}

	content, err := os.ReadFile(writer.path)
	if err != nil {
		writer.file, _ = os.OpenFile(writer.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		return fmt.Errorf("read log file for truncation: %w", err)
	}

	keep := writer.maxBytes / 2
	if keep < 1 {
		keep = 1
	}
	if int64(len(content)) > keep {
		content = content[int64(len(content))-keep:]
	}

	if err := os.WriteFile(writer.path, content, 0o600); err != nil {
		writer.file, _ = os.OpenFile(writer.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		return fmt.Errorf("truncate log file: %w", err)
	}

	file, err := os.OpenFile(writer.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("reopen log file after truncation: %w", err)
	}

	writer.file = file
	writer.currentSize = int64(len(content))
	return nil
}

func (writer *CappedWriter) warnToStderrOnceLocked(err error) {
	if writer.stderrWarned {
		return
	}
	writer.stderrWarned = true
	_, _ = io.WriteString(os.Stderr, fmt.Sprintf("bucket logging warning: %v\n", err))
}

func (writer *CappedWriter) Close() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.file == nil {
		return nil
	}
	err := writer.file.Close()
	writer.file = nil
	return err
}

func (writer *CappedWriter) SetClockForTest(clock func() time.Time) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.truncationClock = clock
}

func (writer *CappedWriter) CurrentSize() int64 {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.currentSize
}
