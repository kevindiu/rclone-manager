package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// ListJobs returns all configured jobs sorted by creation time ascending, then name.
func (s *Store) ListJobs(ctx context.Context) ([]domain.Job, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT data FROM jobs ORDER BY created_at ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	jobs := make([]domain.Job, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var job domain.Job
		if err := json.Unmarshal([]byte(raw), &job); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// GetJob returns a single job by ID.
func (s *Store) GetJob(ctx context.Context, id string) (*domain.Job, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT data FROM jobs WHERE id = ?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var job domain.Job
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// CreateJob inserts a new job, generating an ID and timestamps if omitted.
func (s *Store) CreateJob(ctx context.Context, job domain.Job) (*domain.Job, error) {
	now := time.Now().UTC()
	if job.ID == "" {
		job.ID = domain.NewID()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	normalizeJobDefaults(&job)

	data, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO jobs (id, name, data, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		job.ID, job.Name, string(data),
		job.CreatedAt.UTC().Format(time.RFC3339Nano),
		job.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// UpdateJob replaces an existing job's definition while preserving CreatedAt.
func (s *Store) UpdateJob(ctx context.Context, id string, job domain.Job) (*domain.Job, error) {
	existing, err := s.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	job.ID = id
	job.CreatedAt = existing.CreatedAt
	job.UpdatedAt = time.Now().UTC()
	normalizeJobDefaults(&job)

	data, err := json.Marshal(job)
	if err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE jobs SET name = ?, data = ?, updated_at = ? WHERE id = ?`,
		job.Name, string(data), job.UpdatedAt.UTC().Format(time.RFC3339Nano), id,
	)
	if err != nil {
		return nil, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, ErrNotFound
	}
	return &job, nil
}

// DeleteJob deletes a job by ID.
func (s *Store) DeleteJob(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// ImportBundle imports settings and jobs inside a single atomic transaction.
// mode must be "merge" (upsert by ID) or "replace" (clear existing jobs first).
func (s *Store) ImportBundle(ctx context.Context, bundle domain.ExportBundle, mode string, defaults domain.GlobalSettings) error {
	if mode != "merge" && mode != "replace" {
		return fmt.Errorf("invalid import mode %q: expected merge or replace", mode)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	settings := bundle.Settings
	normalizeSettings(&settings, defaults)
	settingsBytes, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)

	if _, err := tx.ExecContext(ctx, `
INSERT INTO settings (id, data, updated_at) VALUES (1, ?, ?)
ON CONFLICT(id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at
`, string(settingsBytes), nowStr); err != nil {
		return err
	}

	if mode == "replace" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM jobs`); err != nil {
			return err
		}
	}

	for _, job := range bundle.Jobs {
		if job.ID == "" {
			job.ID = domain.NewID()
		}
		if job.CreatedAt.IsZero() {
			job.CreatedAt = now
		}
		job.UpdatedAt = now
		normalizeJobDefaults(&job)

		data, err := json.Marshal(job)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO jobs (id, name, data, created_at, updated_at) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET name = excluded.name, data = excluded.data, updated_at = excluded.updated_at
`, job.ID, job.Name, string(data), job.CreatedAt.UTC().Format(time.RFC3339Nano), nowStr); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func normalizeJobDefaults(job *domain.Job) {
	if job.Operation != "copy" && job.Operation != "sync" {
		job.Operation = "sync"
	}
	if job.CutoffMode == "" && job.MaxTransfer != "" {
		job.CutoffMode = "soft"
	}
	if job.VersionFormat == "" {
		job.VersionFormat = "2006-01-02"
	}
	if job.VersionRetentionDays < 0 {
		job.VersionRetentionDays = 0
	}
}
