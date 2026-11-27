package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

// DraftDryRunResult holds the bounded preview outcome of a non-persisted draft job dry run.
type DraftDryRunResult struct {
	OK         bool     `json:"ok"`
	DurationMs int64    `json:"duration_ms"`
	Command    string   `json:"command"`
	Summary    string   `json:"summary"`
	Lines      []string `json:"lines,omitempty"`
	Error      string   `json:"error,omitempty"`
}

const maxDraftOutputBytes = 256 * 1024
const maxDraftLines = 80

type cappedBuffer struct {
	buf   bytes.Buffer
	limit int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	remaining := c.limit - c.buf.Len()
	if remaining > 0 {
		chunk := p
		if len(chunk) > remaining {
			chunk = chunk[:remaining]
		}
		_, _ = c.buf.Write(chunk)
	}
	return len(p), nil
}

// DryRunDraft executes an ephemeral `rclone --dry-run` for an unsaved or edited job
// without creating run history records or altering files.
func (r *Runner) DryRunDraft(ctx context.Context, job domain.Job, settings domain.GlobalSettings) (DraftDryRunResult, error) {
	start := time.Now()
	dCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	loc, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		loc = time.UTC
	}

	args, _, _, err := BuildRcloneArgs(job, settings, r.client.ConfPath(), true, time.Now().In(loc))
	if err != nil {
		return DraftDryRunResult{}, err
	}
	cmdPreview := r.client.Bin() + " " + FormatCommandPreview(args)

	if job.AbortIfSourceEmpty {
		nonEmpty, checkErr := r.client.CheckSourceNotEmpty(dCtx, job.Source)
		if checkErr != nil {
			return DraftDryRunResult{
				OK:         false,
				DurationMs: time.Since(start).Milliseconds(),
				Command:    cmdPreview,
				Summary:    "Pre-flight check failed",
				Error:      "Pre-flight source check failed: " + checkErr.Error(),
			}, nil
		}
		if !nonEmpty {
			return DraftDryRunResult{
				OK:         false,
				DurationMs: time.Since(start).Milliseconds(),
				Command:    cmdPreview,
				Summary:    "Aborted by empty-source guard",
				Error:      "Pre-flight guard aborted run: source directory is empty",
			}, nil
		}
	}

	if strings.TrimSpace(job.RequiredMarkerFile) != "" {
		exists, markerErr := r.client.CheckMarkerExists(dCtx, job.Source, job.RequiredMarkerFile)
		if markerErr != nil || !exists {
			errMsg := fmt.Sprintf("Pre-flight guard aborted run: required marker file %q not found", job.RequiredMarkerFile)
			if markerErr != nil {
				errMsg += " (" + markerErr.Error() + ")"
			}
			return DraftDryRunResult{
				OK:         false,
				DurationMs: time.Since(start).Milliseconds(),
				Command:    cmdPreview,
				Summary:    "Aborted by marker-file guard",
				Error:      errMsg,
			}, nil
		}
	}

	outBuf := &cappedBuffer{limit: maxDraftOutputBytes}
	cmd := exec.CommandContext(dCtx, r.client.Bin(), args...)
	cmd.Stdout = outBuf
	cmd.Stderr = outBuf
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return cmd.Process.Signal(syscall.SIGINT)
		}
		return nil
	}
	cmd.WaitDelay = 5 * time.Second

	runErr := cmd.Run()
	durationMs := time.Since(start).Milliseconds()

	var lines []string
	var lastErrLine string
	scanner := bufio.NewScanner(bytes.NewReader(outBuf.buf.Bytes()))
	scanner.Buffer(make([]byte, 0, 16*1024), 256*1024)
	for scanner.Scan() {
		rawLine := strings.TrimSpace(scanner.Text())
		if rawLine == "" {
			continue
		}
		var obj struct {
			Level  string `json:"level"`
			Msg    string `json:"msg"`
			Object string `json:"object"`
			Stats  any    `json:"stats"`
		}
		if strings.HasPrefix(rawLine, "{") && json.Unmarshal([]byte(rawLine), &obj) == nil {
			if obj.Stats != nil {
				continue
			}
			msg := strings.TrimSpace(obj.Msg)
			if msg == "" {
				continue
			}
			formatted := msg
			if obj.Object != "" {
				formatted = fmt.Sprintf("[%s] %s", obj.Object, msg)
			}
			if obj.Level == "error" || obj.Level == "critical" {
				lastErrLine = formatted
			}
			if len(lines) < maxDraftLines {
				lines = append(lines, formatted)
			}
		} else if len(lines) < maxDraftLines {
			lines = append(lines, rawLine)
		}
	}

	if runErr != nil {
		errMsg := runErr.Error()
		summary := "Dry run failed"
		if errors.Is(dCtx.Err(), context.DeadlineExceeded) {
			summary = "Dry run timed out after 45s"
			errMsg = "Dry run exceeded 45s preview limit — save the job to run full async dry runs."
		} else if lastErrLine != "" {
			errMsg = lastErrLine
		}
		return DraftDryRunResult{
			OK:         false,
			DurationMs: durationMs,
			Command:    cmdPreview,
			Summary:    summary,
			Lines:      lines,
			Error:      errMsg,
		}, nil
	}

	summary := fmt.Sprintf("Dry run succeeded (%d log message(s))", len(lines))
	if len(lines) == 0 {
		summary = "Dry run succeeded (no changes needed)"
	}
	return DraftDryRunResult{
		OK:         true,
		DurationMs: durationMs,
		Command:    cmdPreview,
		Summary:    summary,
		Lines:      lines,
	}, nil
}
