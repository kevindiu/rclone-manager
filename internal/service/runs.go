package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

// ListRuns lists historical job runs.
func (s *AppService) ListRuns(ctx context.Context, jobID, status string, limit, offset int) ([]domain.JobRun, int, error) {
	return s.store.ListRuns(ctx, jobID, status, limit, offset)
}

// GetRun returns a single job run.
func (s *AppService) GetRun(ctx context.Context, id string) (*domain.JobRun, error) {
	return s.store.GetRun(ctx, id)
}

// DeleteRun removes a completed run record and its log file.
func (s *AppService) DeleteRun(ctx context.Context, id string) error {
	run, err := s.store.GetRun(ctx, id)
	if err != nil {
		return err
	}
	if run.Status == "running" || run.Status == "queued" {
		return fmt.Errorf("%w: cannot delete an active run", ErrConflict)
	}
	return s.store.DeleteRun(ctx, id)
}

// GetRunLogFilePath returns the log file path for a run after verifying the run exists.
func (s *AppService) GetRunLogFilePath(ctx context.Context, id string) (string, error) {
	run, err := s.store.GetRun(ctx, id)
	if err != nil {
		return "", err
	}
	return run.LogFile, nil
}

// ReadRunLogs loads and parses the tail of a run's log file without loading huge files entirely into memory.
func (s *AppService) ReadRunLogs(ctx context.Context, id string, maxBytes int64, maxLines int) (LogReadResult, error) {
	if maxBytes <= 0 {
		maxBytes = 2 * 1024 * 1024
	}
	if maxLines <= 0 {
		maxLines = 1500
	}

	run, err := s.store.GetRun(ctx, id)
	if err != nil {
		return LogReadResult{}, err
	}
	if run.LogFile == "" {
		return LogReadResult{Lines: []ParsedLogLine{}}, nil
	}

	f, err := os.Open(run.LogFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LogReadResult{Lines: []ParsedLogLine{}}, nil
		}
		return LogReadResult{}, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return LogReadResult{}, err
	}
	totalBytes := info.Size()
	truncated := false

	if totalBytes > maxBytes {
		if _, err := f.Seek(totalBytes-maxBytes, io.SeekStart); err != nil {
			return LogReadResult{}, err
		}
		truncated = true
	}

	rawBytes, err := io.ReadAll(io.LimitReader(f, maxBytes))
	if err != nil {
		return LogReadResult{}, err
	}
	raw := string(rawBytes)
	if truncated {
		// Drop partial first line caused by seeking into the middle of a line.
		if idx := strings.IndexByte(raw, '\n'); idx >= 0 {
			raw = raw[idx+1:]
		}
	}

	lines := make([]ParsedLogLine, 0)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj struct {
			Time   string `json:"time"`
			Level  string `json:"level"`
			Msg    string `json:"msg"`
			Object string `json:"object"`
			Size   *int64 `json:"size"`
			Stats  any    `json:"stats"`
		}
		if strings.HasPrefix(line, "{") && json.Unmarshal([]byte(line), &obj) == nil {
			lines = append(lines, ParsedLogLine{
				Time:    obj.Time,
				Level:   obj.Level,
				Msg:     strings.TrimSpace(obj.Msg),
				Object:  obj.Object,
				Size:    obj.Size,
				IsStats: obj.Stats != nil,
			})
		} else {
			lines = append(lines, ParsedLogLine{
				Level: "info",
				Msg:   line,
			})
		}
	}

	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
		truncated = true
		rawLines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
		if len(rawLines) > maxLines {
			raw = strings.Join(rawLines[len(rawLines)-maxLines:], "\n")
		}
	}

	return LogReadResult{
		Lines:      lines,
		Raw:        raw,
		Truncated:  truncated,
		TotalBytes: totalBytes,
	}, nil
}
