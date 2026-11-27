package runner

import (
	"context"
	"fmt"
	"time"
)

func (r *Runner) pruneExpiredVersions(
	ctx context.Context,
	backupBase string,
	versionFormat string,
	retentionDays int,
	loc *time.Location,
	writeLog func(level, msg string),
) ([]string, error) {
	dirs, err := r.client.ListDirs(ctx, backupBase)
	if err != nil {
		return nil, err
	}
	if len(dirs) == 0 {
		return nil, nil
	}

	nowLocal := time.Now().In(loc)
	cutoffDay := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -retentionDays)

	var pruned []string
	for _, dirName := range dirs {
		folderDate, ok := parseVersionFolderDate(dirName, versionFormat, loc)
		if !ok {
			continue
		}
		if folderDate.Before(cutoffDay) {
			target := joinRclonePath(backupBase, dirName)
			writeLog("info", fmt.Sprintf("Pruning expired version folder %q (dated %s, retention %dd)", target, folderDate.Format("2006-01-02"), retentionDays))
			if err := r.client.PurgeDir(ctx, target); err != nil {
				return pruned, err
			}
			pruned = append(pruned, dirName)
		}
	}
	return pruned, nil
}

func parseVersionFolderDate(name, versionFormat string, loc *time.Location) (time.Time, bool) {
	if versionFormat != "" {
		if t, err := time.ParseInLocation(versionFormat, name, loc); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc), true
		}
	}
	// Fallback: strict YYYY-MM-DD or YYYY-MM-DD_...
	if len(name) >= 10 {
		prefix := name[:10]
		if len(name) == 10 || name[10] == '_' || name[10] == '-' || name[10] == 'T' {
			if t, err := time.ParseInLocation("2006-01-02", prefix, loc); err == nil {
				return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc), true
			}
		}
	}
	return time.Time{}, false
}
