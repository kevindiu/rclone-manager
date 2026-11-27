package rclone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// CheckSourceNotEmpty verifies that a source path contains at least one item.
func (c *Client) CheckSourceNotEmpty(ctx context.Context, source string) (bool, error) {
	cmd := exec.CommandContext(ctx, c.bin, "--config", c.confPath, "lsjson", "--max-depth", "1", source)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("source check failed: %s", firstNonEmpty(strings.TrimSpace(stderr.String()), err.Error()))
	}
	var entries []BrowseEntry
	if err := json.Unmarshal(stdout.Bytes(), &entries); err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}

// CheckMarkerExists verifies that a required marker file exists under source.
func (c *Client) CheckMarkerExists(ctx context.Context, source, marker string) (bool, error) {
	cleanMarker, err := CleanSubpath(marker)
	if err != nil || cleanMarker == "" {
		return false, fmt.Errorf("invalid marker path %q", marker)
	}
	markerPath := joinRclonePath(source, cleanMarker)
	cmd := exec.CommandContext(ctx, c.bin, "--config", c.confPath, "lsjson", "--stat", markerPath)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

// ListDirs returns directory names directly under targetPath.
func (c *Client) ListDirs(ctx context.Context, targetPath string) ([]string, error) {
	cmd := exec.CommandContext(ctx, c.bin, "--config", c.confPath, "lsf", "--dirs-only", "--max-depth", "1", targetPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// If the backup dir hasn't been created yet (nothing deleted/changed), treat as empty.
		errStr := strings.ToLower(stderr.String())
		if strings.Contains(errStr, "directory not found") || strings.Contains(errStr, "no such file") || strings.Contains(errStr, "doesn't exist") {
			return nil, nil
		}
		return nil, fmt.Errorf("list dirs failed: %s", firstNonEmpty(strings.TrimSpace(stderr.String()), err.Error()))
	}
	var dirs []string
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		d := strings.TrimSuffix(strings.TrimSpace(line), "/")
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	return dirs, nil
}

// PurgeDir recursively deletes a directory on local or remote storage.
func (c *Client) PurgeDir(ctx context.Context, targetPath string) error {
	cmd := exec.CommandContext(ctx, c.bin, "--config", c.confPath, "purge", targetPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("purge %s failed: %s (%v)", targetPath, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func joinRclonePath(base, sub string) string {
	sub = strings.TrimLeft(sub, "/")
	if strings.HasSuffix(base, ":") {
		return base + sub
	}
	return strings.TrimRight(base, "/") + "/" + sub
}
