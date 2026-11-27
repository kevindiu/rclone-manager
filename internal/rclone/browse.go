package rclone

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"sort"
	"strings"
	"time"
)

// TestRemote checks connectivity and authentication for a remote using `rclone lsf --max-depth 1`.
func (c *Client) TestRemote(ctx context.Context, name string) (RemoteTestResult, error) {
	if err := c.ensureRemoteExists(ctx, name); err != nil {
		return RemoteTestResult{Remote: name, OK: false, Error: err.Error()}, err
	}

	tCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(tCtx, c.bin, "--config", c.confPath, "lsf", "--max-depth", "1", name+":")
	out, err := cmd.CombinedOutput()
	latency := time.Since(start).Milliseconds()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return RemoteTestResult{
			Remote:    name,
			OK:        false,
			LatencyMs: latency,
			Error:     msg,
		}, nil
	}

	lines := 0
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(l) != "" {
			lines++
		}
	}
	return RemoteTestResult{
		Remote:    name,
		OK:        true,
		LatencyMs: latency,
		Message:   fmt.Sprintf("Connected (%d top-level item(s) listed)", lines),
	}, nil
}

// AboutRemote fetches quota/space usage via `rclone about <name>: --json`.
func (c *Client) AboutRemote(ctx context.Context, name string) (RemoteAbout, error) {
	if err := c.ensureRemoteExists(ctx, name); err != nil {
		return RemoteAbout{Remote: name, Supported: false, Error: err.Error()}, err
	}

	tCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(tCtx, c.bin, "--config", c.confPath, "about", name+":", "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText == "" {
			errText = err.Error()
		}
		return RemoteAbout{
			Remote:    name,
			Supported: false,
			Error:     errText,
		}, nil
	}

	var parsed struct {
		Total   *int64 `json:"total"`
		Used    *int64 `json:"used"`
		Free    *int64 `json:"free"`
		Trashed *int64 `json:"trashed"`
		Other   *int64 `json:"other"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return RemoteAbout{
			Remote:    name,
			Supported: false,
			Error:     err.Error(),
		}, nil
	}
	return RemoteAbout{
		Remote:    name,
		Supported: true,
		Total:     parsed.Total,
		Used:      parsed.Used,
		Free:      parsed.Free,
		Trashed:   parsed.Trashed,
		Other:     parsed.Other,
	}, nil
}

// FormatRemoteTarget builds a safe `remote:subpath` string, rejecting path traversal.
func FormatRemoteTarget(remote, subpath string) (string, error) {
	if err := ValidateRemoteName(remote); err != nil {
		return "", err
	}
	clean, err := CleanSubpath(subpath)
	if err != nil {
		return "", err
	}
	if clean == "" {
		return remote + ":", nil
	}
	return remote + ":" + clean, nil
}

// CleanSubpath normalizes a relative remote subpath and rejects ".." traversal.
func CleanSubpath(subpath string) (string, error) {
	s := strings.ReplaceAll(strings.TrimSpace(subpath), "\\", "/")
	s = strings.TrimLeft(s, "/")
	if s == "" || s == "." {
		return "", nil
	}
	parts := strings.Split(s, "/")
	for _, p := range parts {
		if p == ".." {
			return "", errors.New("path traversal ('..') is not allowed")
		}
	}
	cleaned := path.Clean("/" + s)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." {
		return "", nil
	}
	return cleaned, nil
}

// Browse lists files and directories at remote:subpath via `rclone lsjson`.
func (c *Client) Browse(ctx context.Context, remote, subpath string) ([]BrowseEntry, error) {
	if err := c.ensureRemoteExists(ctx, remote); err != nil {
		return nil, err
	}
	target, err := FormatRemoteTarget(remote, subpath)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, c.bin, "--config", c.confPath, "lsjson", target)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("browse failed: %s", firstNonEmpty(strings.TrimSpace(stderr.String()), err.Error()))
	}

	var entries []BrowseEntry
	if len(bytes.TrimSpace(stdout.Bytes())) > 0 {
		if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
			return nil, fmt.Errorf("parse lsjson: %w", err)
		}
	}
	if entries == nil {
		entries = []BrowseEntry{}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

// StatFile retrieves metadata for a single file or directory via `rclone lsjson --stat`.
func (c *Client) StatFile(ctx context.Context, remote, subpath string) (*BrowseEntry, error) {
	if err := c.ensureRemoteExists(ctx, remote); err != nil {
		return nil, err
	}
	target, err := FormatRemoteTarget(remote, subpath)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, c.bin, "--config", c.confPath, "lsjson", "--stat", target)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("stat failed: %s", firstNonEmpty(strings.TrimSpace(stderr.String()), err.Error()))
	}
	var entry BrowseEntry
	if err := json.Unmarshal(stdout.Bytes(), &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// StreamFile streams a file's contents using `rclone cat` directly to w.
func (c *Client) StreamFile(ctx context.Context, remote, subpath string, w io.Writer) error {
	if err := c.ensureRemoteExists(ctx, remote); err != nil {
		return err
	}
	target, err := FormatRemoteTarget(remote, subpath)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, c.bin, "--config", c.confPath, "cat", target)
	cmd.Stdout = w
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stream failed: %s", firstNonEmpty(strings.TrimSpace(stderr.String()), err.Error()))
	}
	return nil
}

// Restore copies a file or directory from source_remote:source_path to target_path.
func (c *Client) Restore(ctx context.Context, req RestoreRequest) (string, error) {
	if err := c.ensureRemoteExists(ctx, req.SourceRemote); err != nil {
		return "", err
	}
	srcTarget, err := FormatRemoteTarget(req.SourceRemote, req.SourcePath)
	if err != nil {
		return "", err
	}
	target := strings.TrimSpace(req.TargetPath)
	if target == "" {
		return "", errors.New("target_path is required")
	}

	op := "copyto"
	if req.IsDir {
		op = "copy"
	}
	cmd := exec.CommandContext(ctx, c.bin, "--config", c.confPath, op, srcTarget, target, "-v")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("restore failed: %s (%v)", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}
