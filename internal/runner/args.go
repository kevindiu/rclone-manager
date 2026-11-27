package runner

import (
	"strconv"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
	"github.com/kevindiu/rclone-manager/internal/rclone"
)

// BuildRcloneArgs constructs the complete argument list for an rclone job execution.
func BuildRcloneArgs(job domain.Job, settings domain.GlobalSettings, confPath string, dryRun bool, nowLocal time.Time) (args []string, backupBase string, datedBackupDir string, err error) {
	op := job.Operation
	switch op {
	case "copy", "sync", "move", "check", "copyto":
	default:
		op = "sync"
	}

	args = []string{
		"--config", confPath,
		op,
		job.Source,
		job.Destination,
		"--use-json-log",
		"--stats=1s",
		"--stats-log-level=NOTICE",
		"-v",
	}

	if dryRun {
		args = append(args, "--dry-run")
	}

	transfers := settings.Transfers
	if job.Transfers != nil && *job.Transfers > 0 {
		transfers = *job.Transfers
	}
	if transfers > 0 {
		args = append(args, "--transfers", strconv.Itoa(transfers))
	}

	checkers := settings.Checkers
	if job.Checkers != nil && *job.Checkers > 0 {
		checkers = *job.Checkers
	}
	if checkers > 0 {
		args = append(args, "--checkers", strconv.Itoa(checkers))
	}

	bwlimit := strings.TrimSpace(job.BwLimit)
	if bwlimit == "" {
		bwlimit = strings.TrimSpace(settings.BwLimit)
	}
	if bwlimit != "" {
		args = append(args, "--bwlimit", bwlimit)
	}

	retries := settings.Retries
	if job.Retries != nil && *job.Retries >= 0 {
		retries = *job.Retries
	}
	if retries >= 0 {
		args = append(args, "--retries", strconv.Itoa(retries))
	}

	llRetries := settings.LowLevelRetries
	if job.LowLevelRetries != nil && *job.LowLevelRetries >= 0 {
		llRetries = *job.LowLevelRetries
	}
	if llRetries >= 0 {
		args = append(args, "--low-level-retries", strconv.Itoa(llRetries))
	}

	timeout := strings.TrimSpace(job.Timeout)
	if timeout == "" {
		timeout = strings.TrimSpace(settings.Timeout)
	}
	if timeout != "" {
		args = append(args, "--timeout", timeout)
	}

	conTimeout := strings.TrimSpace(job.ConTimeout)
	if conTimeout == "" {
		conTimeout = strings.TrimSpace(settings.ConTimeout)
	}
	if conTimeout != "" {
		args = append(args, "--contimeout", conTimeout)
	}

	if strings.TrimSpace(job.MaxTransfer) != "" && op != "check" {
		args = append(args, "--max-transfer", strings.TrimSpace(job.MaxTransfer))
		cutoff := strings.TrimSpace(job.CutoffMode)
		if cutoff == "" {
			cutoff = "soft"
		}
		args = append(args, "--cutoff-mode="+cutoff)
	}

	if job.MaxDelete != nil && *job.MaxDelete >= 0 && op != "check" {
		args = append(args, "--max-delete", strconv.Itoa(*job.MaxDelete))
	}

	if job.FastList {
		args = append(args, "--fast-list")
	}
	if job.Checksum {
		args = append(args, "--checksum")
	}
	if job.UpdateOnly && op != "check" {
		args = append(args, "--update")
	}
	if job.ServerSideAcrossConfigs {
		args = append(args, "--server-side-across-configs")
	}

	// Incremental --backup-dir versioning
	if job.VersioningEnabled && op != "check" {
		backupBase = strings.TrimSpace(job.BackupDirBase)
		if backupBase == "" {
			backupBase = joinRclonePath(job.Destination, "_versions")
		}
		format := strings.TrimSpace(job.VersionFormat)
		if format == "" {
			format = "2006-01-02"
		}
		datedBackupDir = joinRclonePath(backupBase, nowLocal.Format(format))
		args = append(args, "--backup-dir", datedBackupDir)

		if rel, inside := relativeSubpath(job.Destination, backupBase); inside && rel != "" {
			args = append(args, "--exclude", "/"+rel+"/**")
		}
	}

	for _, pat := range job.ExcludePatterns {
		p := strings.TrimSpace(pat)
		if p != "" {
			args = append(args, "--exclude", p)
		}
	}
	for _, pat := range job.IncludePatterns {
		p := strings.TrimSpace(pat)
		if p != "" {
			args = append(args, "--include", p)
		}
	}
	for _, rule := range job.FilterRules {
		p := strings.TrimSpace(rule)
		if p != "" {
			args = append(args, "--filter", p)
		}
	}

	if strings.TrimSpace(job.CustomFlags) != "" {
		customTokens, err := rclone.ParseAndValidateFlags(job.CustomFlags)
		if err != nil {
			return nil, "", "", err
		}
		args = append(args, customTokens...)
	}

	return args, backupBase, datedBackupDir, nil
}

func joinRclonePath(base, sub string) string {
	sub = strings.Trim(sub, "/")
	if strings.HasSuffix(base, ":") {
		return base + sub
	}
	return strings.TrimRight(base, "/") + "/" + sub
}

func normalizeComparePath(p string) string {
	p = strings.TrimSpace(p)
	if strings.HasSuffix(p, ":") {
		return p
	}
	return strings.TrimRight(p, "/")
}

func relativeSubpath(dest, backupBase string) (string, bool) {
	d := normalizeComparePath(dest)
	b := normalizeComparePath(backupBase)
	if d == "" || b == "" {
		return "", false
	}
	if strings.HasSuffix(d, ":") {
		if strings.HasPrefix(b, d) && len(b) > len(d) {
			rel := strings.TrimPrefix(b, d)
			rel = strings.Trim(rel, "/")
			return rel, rel != ""
		}
		return "", false
	}
	prefix := d + "/"
	if strings.HasPrefix(b, prefix) {
		rel := strings.TrimPrefix(b, prefix)
		rel = strings.Trim(rel, "/")
		return rel, rel != ""
	}
	return "", false
}

func formatCommandPreview(args []string) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		if strings.ContainsAny(a, " \t\n\"'") {
			parts = append(parts, strconv.Quote(a))
		} else {
			parts = append(parts, a)
		}
	}
	return strings.Join(parts, " ")
}
