package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

// CreateRun persists a newly created job run.
func (s *Store) CreateRun(ctx context.Context, run domain.JobRun) error {
	prunedBytes, err := json.Marshal(nonNilSlice(run.PrunedVersions))
	if err != nil {
		return err
	}
	var finishedAt sql.NullString
	if run.FinishedAt != nil {
		finishedAt = sql.NullString{String: formatSQLiteTime(*run.FinishedAt), Valid: true}
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO job_runs (
	id, job_id, job_name, trigger, dry_run, status, started_at, finished_at,
	duration_ms, bytes_transferred, total_bytes, files_transferred, files_checked,
	files_deleted, errors_count, error_message, command_preview, log_file,
	backup_dir_used, pruned_versions
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		run.ID, run.JobID, run.JobName, run.Trigger, boolToInt(run.DryRun), run.Status,
		formatSQLiteTime(run.StartedAt), finishedAt,
		run.DurationMs, run.BytesTransferred, run.TotalBytes, run.FilesTransferred,
		run.FilesChecked, run.FilesDeleted, run.ErrorsCount, run.ErrorMessage,
		run.CommandPreview, run.LogFile, run.BackupDirUsed, string(prunedBytes),
	)
	return err
}

// UpdateRun updates a job run record.
func (s *Store) UpdateRun(ctx context.Context, run domain.JobRun) error {
	prunedBytes, err := json.Marshal(nonNilSlice(run.PrunedVersions))
	if err != nil {
		return err
	}
	var finishedAt sql.NullString
	if run.FinishedAt != nil {
		finishedAt = sql.NullString{String: formatSQLiteTime(*run.FinishedAt), Valid: true}
	}

	_, err = s.db.ExecContext(ctx, `
UPDATE job_runs SET
	status = ?, finished_at = ?, duration_ms = ?, bytes_transferred = ?,
	total_bytes = ?, files_transferred = ?, files_checked = ?, files_deleted = ?,
	errors_count = ?, error_message = ?, command_preview = ?, log_file = ?,
	backup_dir_used = ?, pruned_versions = ?
WHERE id = ?
`,
		run.Status, finishedAt, run.DurationMs, run.BytesTransferred,
		run.TotalBytes, run.FilesTransferred, run.FilesChecked, run.FilesDeleted,
		run.ErrorsCount, run.ErrorMessage, run.CommandPreview, run.LogFile,
		run.BackupDirUsed, string(prunedBytes), run.ID,
	)
	return err
}

// GetRun returns a single job run by ID.
func (s *Store) GetRun(ctx context.Context, id string) (*domain.JobRun, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, job_id, job_name, trigger, dry_run, status, started_at, finished_at,
	duration_ms, bytes_transferred, total_bytes, files_transferred, files_checked,
	files_deleted, errors_count, error_message, command_preview, log_file,
	backup_dir_used, pruned_versions
FROM job_runs WHERE id = ?
`, id)

	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// ListRuns returns job runs ordered newest-first, optionally filtered by jobID, status, and trigger.
func (s *Store) ListRuns(ctx context.Context, jobID, status, trigger string, limit, offset int) ([]domain.JobRun, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	var conditions []string
	var args []any
	if jobID != "" {
		conditions = append(conditions, "job_id = ?")
		args = append(args, jobID)
	}
	if status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}
	if trigger != "" {
		conditions = append(conditions, "trigger = ?")
		args = append(args, trigger)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	countQuery := "SELECT COUNT(*) FROM job_runs " + whereClause
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
SELECT id, job_id, job_name, trigger, dry_run, status, started_at, finished_at,
	duration_ms, bytes_transferred, total_bytes, files_transferred, files_checked,
	files_deleted, errors_count, error_message, command_preview, log_file,
	backup_dir_used, pruned_versions
FROM job_runs ` + whereClause + ` ORDER BY started_at DESC LIMIT ? OFFSET ?`

	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	runs := make([]domain.JobRun, 0)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, 0, err
		}
		runs = append(runs, run)
	}
	return runs, total, rows.Err()
}

// GetLastRunsByJobIDs returns the most recent run per job_id.
func (s *Store) GetLastRunsByJobIDs(ctx context.Context) (map[string]domain.JobRun, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT r.id, r.job_id, r.job_name, r.trigger, r.dry_run, r.status, r.started_at, r.finished_at,
	r.duration_ms, r.bytes_transferred, r.total_bytes, r.files_transferred, r.files_checked,
	r.files_deleted, r.errors_count, r.error_message, r.command_preview, r.log_file,
	r.backup_dir_used, r.pruned_versions
FROM job_runs r
INNER JOIN (
	SELECT job_id, MAX(started_at) AS max_started
	FROM job_runs
	GROUP BY job_id
) latest ON r.job_id = latest.job_id AND r.started_at = latest.max_started
`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]domain.JobRun)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		result[run.JobID] = run
	}
	return result, rows.Err()
}

// DeleteRun removes a run record and its associated log file on disk if present.
func (s *Store) DeleteRun(ctx context.Context, id string) error {
	run, err := s.GetRun(ctx, id)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM job_runs WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	if run.LogFile != "" {
		_ = os.Remove(run.LogFile)
	}
	return nil
}

// MarkInterruptedRunsFailed recovers from an unclean shutdown by marking any
// queued or running job runs as failed.
func (s *Store) MarkInterruptedRunsFailed(ctx context.Context) (int64, error) {
	now := formatSQLiteTime(time.Now().UTC())
	res, err := s.db.ExecContext(ctx, `
UPDATE job_runs
SET status = 'failed',
	finished_at = ?,
	error_message = CASE
		WHEN error_message = '' THEN 'Interrupted by server restart'
		ELSE error_message
	END
WHERE status IN ('running', 'queued')
`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PruneOldRuns deletes finished run records older than retentionDays and removes
// their log files from disk. If retentionDays <= 0, no runs are pruned.
func (s *Store) PruneOldRuns(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := formatSQLiteTime(time.Now().UTC().AddDate(0, 0, -retentionDays))

	rows, err := s.db.QueryContext(ctx, `
SELECT id, log_file FROM job_runs
WHERE status NOT IN ('running', 'queued') AND started_at < ?
`, cutoff)
	if err != nil {
		return 0, err
	}

	type target struct {
		id      string
		logFile string
	}
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.logFile); err != nil {
			_ = rows.Close()
			return 0, err
		}
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	_ = rows.Close()

	if len(targets) == 0 {
		return 0, nil
	}

	res, err := s.db.ExecContext(ctx, `
DELETE FROM job_runs
WHERE status NOT IN ('running', 'queued') AND started_at < ?
`, cutoff)
	if err != nil {
		return 0, err
	}
	deleted, _ := res.RowsAffected()

	for _, t := range targets {
		if t.logFile != "" {
			_ = os.Remove(t.logFile)
		}
	}
	return deleted, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRun(row scanner) (domain.JobRun, error) {
	var run domain.JobRun
	var dryRunInt int
	var startedStr string
	var finishedStr sql.NullString
	var prunedStr string

	err := row.Scan(
		&run.ID, &run.JobID, &run.JobName, &run.Trigger, &dryRunInt, &run.Status,
		&startedStr, &finishedStr, &run.DurationMs, &run.BytesTransferred,
		&run.TotalBytes, &run.FilesTransferred, &run.FilesChecked, &run.FilesDeleted,
		&run.ErrorsCount, &run.ErrorMessage, &run.CommandPreview, &run.LogFile,
		&run.BackupDirUsed, &prunedStr,
	)
	if err != nil {
		return run, err
	}

	run.DryRun = dryRunInt != 0
	if t, err := parseSQLiteTime(startedStr); err == nil {
		run.StartedAt = t
	}
	if finishedStr.Valid && finishedStr.String != "" {
		if t, err := parseSQLiteTime(finishedStr.String); err == nil {
			run.FinishedAt = &t
		}
	}
	if prunedStr != "" {
		_ = json.Unmarshal([]byte(prunedStr), &run.PrunedVersions)
	}
	return run, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nonNilSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
