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
	"sync"
	"time"
)

// TestRemote checks connectivity and authentication for a remote using `rclone lsf --max-depth 1`.
func (c *Client) TestRemote(ctx context.Context, name string) (RemoteTestResult, error) {
	if err := c.ensureRemoteExists(ctx, name); err != nil {
		return RemoteTestResult{Remote: name, OK: false, Error: err.Error()}, err
	}
	return testRemoteWithConf(ctx, c.bin, c.confPath, name, name), nil
}

func testRemoteWithConf(ctx context.Context, bin, confPath, remoteName, displayName string) RemoteTestResult {
	tCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(tCtx, bin, "--config", confPath, "lsf", "--max-depth", "1", remoteName+":")
	out, err := cmd.CombinedOutput()
	latency := time.Since(start).Milliseconds()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return RemoteTestResult{
			Remote:    displayName,
			OK:        false,
			LatencyMs: latency,
			Error:     msg,
		}
	}

	lines := 0
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(l) != "" {
			lines++
		}
	}
	return RemoteTestResult{
		Remote:    displayName,
		OK:        true,
		LatencyMs: latency,
		Message:   fmt.Sprintf("Connected (%d top-level item(s) listed)", lines),
	}
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

// MakeDir creates a directory at remote:subpath using `rclone mkdir`.
func (c *Client) MakeDir(ctx context.Context, remote, subpath string) error {
	if err := c.ensureRemoteExists(ctx, remote); err != nil {
		return err
	}
	cleanSub, err := CleanSubpath(subpath)
	if err != nil {
		return err
	}
	if cleanSub == "" {
		return errors.New("directory path is required")
	}
	target, err := FormatRemoteTarget(remote, cleanSub)
	if err != nil {
		return err
	}
	out, err := exec.CommandContext(ctx, c.bin, "--config", c.confPath, "mkdir", target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mkdir failed: %s", firstNonEmpty(strings.TrimSpace(string(out)), err.Error()))
	}
	return nil
}

// UploadFile streams body directly to remote:subpath via `rclone rcat`.
func (c *Client) UploadFile(ctx context.Context, remote, subpath string, body io.Reader) error {
	if err := c.ensureRemoteExists(ctx, remote); err != nil {
		return err
	}
	cleanSub, err := CleanSubpath(subpath)
	if err != nil {
		return err
	}
	if cleanSub == "" {
		return errors.New("destination file path is required")
	}
	target, err := FormatRemoteTarget(remote, cleanSub)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, c.bin, "--config", c.confPath, "rcat", target)
	cmd.Stdin = body
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("upload failed: %s", firstNonEmpty(strings.TrimSpace(string(out)), err.Error()))
	}
	return nil
}

// RenamePath renames or moves a file or folder within remote using `rclone moveto`.
func (c *Client) RenamePath(ctx context.Context, remote, srcSubpath, dstSubpath string) error {
	if err := c.ensureRemoteExists(ctx, remote); err != nil {
		return err
	}
	cleanSrc, err := CleanSubpath(srcSubpath)
	if err != nil {
		return err
	}
	cleanDst, err := CleanSubpath(dstSubpath)
	if err != nil {
		return err
	}
	if cleanSrc == "" || cleanDst == "" {
		return errors.New("source and destination paths are required")
	}
	if cleanSrc == cleanDst {
		return errors.New("new name matches existing name")
	}
	srcTarget, err := FormatRemoteTarget(remote, cleanSrc)
	if err != nil {
		return err
	}
	dstTarget, err := FormatRemoteTarget(remote, cleanDst)
	if err != nil {
		return err
	}
	out, err := exec.CommandContext(ctx, c.bin, "--config", c.confPath, "moveto", srcTarget, dstTarget).CombinedOutput()
	if err != nil {
		return fmt.Errorf("rename failed: %s", firstNonEmpty(strings.TrimSpace(string(out)), err.Error()))
	}
	return nil
}

// DeletePath deletes a single file (`deletefile`) or recursive directory (`purge`) on remote.
func (c *Client) DeletePath(ctx context.Context, remote, subpath string, isDir bool) error {
	if err := c.ensureRemoteExists(ctx, remote); err != nil {
		return err
	}
	cleanSub, err := CleanSubpath(subpath)
	if err != nil {
		return err
	}
	if cleanSub == "" {
		return errors.New("refusing to delete remote root path")
	}
	target, err := FormatRemoteTarget(remote, cleanSub)
	if err != nil {
		return err
	}
	verb := "deletefile"
	if isDir {
		verb = "purge"
	}
	out, err := exec.CommandContext(ctx, c.bin, "--config", c.confPath, verb, target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("delete failed: %s", firstNonEmpty(strings.TrimSpace(string(out)), err.Error()))
	}
	return nil
}

// DirSize calculates file count and byte total for remote:subpath via `rclone size --json`.
func (c *Client) DirSize(ctx context.Context, remote, subpath string) (DirSize, error) {
	if err := c.ensureRemoteExists(ctx, remote); err != nil {
		return DirSize{}, err
	}
	target, err := FormatRemoteTarget(remote, subpath)
	if err != nil {
		return DirSize{}, err
	}
	sCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(sCtx, c.bin, "--config", c.confPath, "size", "--json", target)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return DirSize{}, fmt.Errorf("size check failed: %s", firstNonEmpty(strings.TrimSpace(stderr.String()), err.Error()))
	}
	var res DirSize
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		return DirSize{}, fmt.Errorf("parse size output: %w", err)
	}
	return res, nil
}

// CleanupRemote empties trash / expired versions on backends supporting `rclone cleanup`.
func (c *Client) CleanupRemote(ctx context.Context, name string) error {
	if err := c.ensureRemoteExists(ctx, name); err != nil {
		return err
	}
	target, err := FormatRemoteTarget(name, "")
	if err != nil {
		return err
	}
	cCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	out, err := exec.CommandContext(cCtx, c.bin, "--config", c.confPath, "cleanup", target).CombinedOutput()
	if err != nil {
		return fmt.Errorf("cleanup failed: %s", firstNonEmpty(strings.TrimSpace(string(out)), err.Error()))
	}
	return nil
}

// DedupeRemote deduplicates identically named files on backends such as Google Drive.
func (c *Client) DedupeRemote(ctx context.Context, name, subpath, mode string, dryRun bool) (DedupeResult, error) {
	if err := c.ensureRemoteExists(ctx, name); err != nil {
		return DedupeResult{}, err
	}
	cleanMode := strings.ToLower(strings.TrimSpace(mode))
	if cleanMode == "" {
		cleanMode = "newest"
	}
	switch cleanMode {
	case "newest", "oldest", "largest", "smallest", "rename", "skip":
	default:
		return DedupeResult{}, fmt.Errorf("invalid dedupe mode %q", mode)
	}
	target, err := FormatRemoteTarget(name, subpath)
	if err != nil {
		return DedupeResult{}, err
	}

	args := []string{"--config", c.confPath, "dedupe", "--dedupe-mode", cleanMode}
	if dryRun {
		args = append(args, "--dry-run")
	}
	args = append(args, target)

	dCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	out, err := exec.CommandContext(dCtx, c.bin, args...).CombinedOutput()
	trimmedOut := strings.TrimSpace(string(out))
	if err != nil {
		return DedupeResult{}, fmt.Errorf("dedupe failed: %s", firstNonEmpty(trimmedOut, err.Error()))
	}
	return DedupeResult{
		Remote: name,
		Mode:   cleanMode,
		DryRun: dryRun,
		Output: trimmedOut,
	}, nil
}

// EnsureRemoteExists verifies that the named remote exists in rclone.conf.
func (c *Client) EnsureRemoteExists(ctx context.Context, name string) error {
	return c.ensureRemoteExists(ctx, name)
}

// ListVersions scans default _versions ancestor folders and any extra job-configured
// backup roots on remote to find historical versions of subpath.
func (c *Client) ListVersions(ctx context.Context, remote, subpath string, extraCandidates []VersionRootCandidate) ([]FileVersion, error) {
	if err := c.ensureRemoteExists(ctx, remote); err != nil {
		return nil, err
	}
	cleanPath, err := CleanSubpath(subpath)
	if err != nil {
		return nil, err
	}
	if cleanPath == "" {
		return []FileVersion{}, nil
	}

	candidates := buildVersionCandidates(cleanPath, extraCandidates)
	versions := make([]FileVersion, 0)
	var mu sync.Mutex

	for _, cand := range candidates {
		rootTarget, err := FormatRemoteTarget(remote, cand.BackupRoot)
		if err != nil {
			continue
		}
		dates, err := c.ListDirs(ctx, rootTarget)
		if err != nil || len(dates) == 0 {
			continue
		}

		sem := make(chan struct{}, 6)
		var wg sync.WaitGroup
		for _, dateDir := range dates {
			dateDir := dateDir
			versionSub := path.Join(cand.BackupRoot, dateDir, cand.RelPath)
			wg.Add(1)
			go func() {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}

				stat, err := c.StatFile(ctx, remote, versionSub)
				if err != nil || stat == nil {
					return
				}
				mu.Lock()
				versions = append(versions, FileVersion{
					Date:       dateDir,
					SourcePath: versionSub,
					Size:       stat.Size,
					ModTime:    stat.ModTime,
					IsDir:      stat.IsDir,
					BackupRoot: cand.BackupRoot,
				})
				mu.Unlock()
			}()
		}
		wg.Wait()
	}

	sort.Slice(versions, func(i, j int) bool {
		if versions[i].Date != versions[j].Date {
			return versions[i].Date > versions[j].Date
		}
		return versions[i].ModTime > versions[j].ModTime
	})
	return versions, nil
}

func buildVersionCandidates(cleanPath string, extra []VersionRootCandidate) []VersionRootCandidate {
	seen := make(map[string]struct{})
	out := make([]VersionRootCandidate, 0, len(extra)+4)

	add := func(root, rel string) {
		r, err1 := CleanSubpath(root)
		rl, err2 := CleanSubpath(rel)
		if err1 != nil || err2 != nil || rl == "" {
			return
		}
		key := r + "|" + rl
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, VersionRootCandidate{BackupRoot: r, RelPath: rl})
	}

	for _, c := range extra {
		add(c.BackupRoot, c.RelPath)
	}

	// Default ancestor _versions folders (e.g. a/b/c.txt -> a/b/_versions + c.txt, a/_versions + b/c.txt, _versions + a/b/c.txt)
	dir := path.Dir(cleanPath)
	for {
		if dir == "." || dir == "/" || dir == "" {
			add("_versions", cleanPath)
			break
		}
		rel := strings.TrimPrefix(cleanPath, dir+"/")
		add(path.Join(dir, "_versions"), rel)
		dir = path.Dir(dir)
	}

	return out
}
