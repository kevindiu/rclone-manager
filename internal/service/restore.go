package service

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/kevindiu/rclone-manager/internal/domain"
	"github.com/kevindiu/rclone-manager/internal/rclone"
)

// StartRestore launches an asynchronous restore job via Runner so progress streams over SSE
// and execution history/logs are persisted in SQLite.
func (s *AppService) StartRestore(ctx context.Context, req rclone.RestoreRequest) (*domain.JobRun, error) {
	if s.browser != nil {
		if err := s.browser.EnsureRemoteExists(ctx, req.SourceRemote); err != nil {
			return nil, err
		}
	}
	srcTarget, err := rclone.FormatRemoteTarget(req.SourceRemote, req.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	target := strings.TrimSpace(req.TargetPath)
	if target == "" {
		return nil, fmt.Errorf("%w: target_path is required", ErrValidation)
	}
	if strings.HasPrefix(target, "-") {
		return nil, fmt.Errorf("%w: target_path cannot start with '-'", ErrValidation)
	}

	var customFlags string
	switch strings.TrimSpace(req.ConflictMode) {
	case "", "overwrite":
	case "skip":
		customFlags = "--ignore-existing"
	case "newer_only":
		customFlags = "--update"
	default:
		return nil, fmt.Errorf("%w: invalid conflict_mode %q", ErrValidation, req.ConflictMode)
	}

	op := "copyto"
	if req.IsDir {
		op = "copy"
	}

	cleanSub, _ := rclone.CleanSubpath(req.SourcePath)
	label := path.Base(cleanSub)
	if label == "" || label == "." {
		label = req.SourceRemote + ":"
	}

	restoreJob := domain.Job{
		ID:          "restore-" + domain.NewID(),
		Name:        "Restore: " + label,
		Operation:   op,
		Source:      srcTarget,
		Destination: target,
		CustomFlags: customFlags,
	}

	trigger := "restore"
	if req.DryRun {
		trigger = "restore_dry_run"
	}
	return s.runner.StartJob(restoreJob, trigger, req.DryRun)
}

// ListFileVersions discovers historical backup-dir versions of a path on remote.
func (s *AppService) ListFileVersions(ctx context.Context, remote, subpath string) ([]rclone.FileVersion, error) {
	if s.browser == nil {
		return []rclone.FileVersion{}, nil
	}
	cleanSub, err := rclone.CleanSubpath(subpath)
	if err != nil || cleanSub == "" {
		return nil, fmt.Errorf("%w: valid file or folder path is required", ErrValidation)
	}

	var extra []rclone.VersionRootCandidate
	if jobs, err := s.store.ListJobs(ctx); err == nil {
		for _, job := range jobs {
			if !job.VersioningEnabled {
				continue
			}
			destRemote, destSub, ok := splitRemotePath(job.Destination)
			if !ok || destRemote != remote {
				continue
			}
			rel, inside := relativeSub(destSub, cleanSub)
			if !inside {
				continue
			}
			backupRoot := path.Join(destSub, "_versions")
			if strings.TrimSpace(job.BackupDirBase) != "" {
				bRem, bSub, ok := splitRemotePath(job.BackupDirBase)
				if !ok || bRem != remote {
					continue
				}
				backupRoot = bSub
			}
			extra = append(extra, rclone.VersionRootCandidate{
				BackupRoot: backupRoot,
				RelPath:    rel,
			})
		}
	}

	return s.browser.ListVersions(ctx, remote, cleanSub, extra)
}

func splitRemotePath(p string) (remote string, subpath string, ok bool) {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" || strings.HasPrefix(trimmed, "/") {
		return "", "", false
	}
	idx := strings.IndexByte(trimmed, ':')
	if idx <= 0 {
		return "", "", false
	}
	remote = trimmed[:idx]
	sub, err := rclone.CleanSubpath(trimmed[idx+1:])
	if err != nil {
		return "", "", false
	}
	return remote, sub, true
}

func relativeSub(baseSub, targetSub string) (string, bool) {
	if baseSub == "" {
		return targetSub, targetSub != ""
	}
	prefix := baseSub + "/"
	if strings.HasPrefix(targetSub, prefix) {
		rel := strings.TrimPrefix(targetSub, prefix)
		return rel, rel != ""
	}
	return "", false
}
