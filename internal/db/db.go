package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database connection and persistence operations.
type Store struct {
	db *sql.DB
}

var migrations = []string{
	// Version 1: initial schema
	`
CREATE TABLE IF NOT EXISTS settings (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	data TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS jobs (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	data TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_jobs_name ON jobs(name);

CREATE TABLE IF NOT EXISTS job_runs (
	id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL,
	job_name TEXT NOT NULL,
	trigger TEXT NOT NULL,
	dry_run INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL,
	started_at TEXT NOT NULL,
	finished_at TEXT,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	bytes_transferred INTEGER NOT NULL DEFAULT 0,
	total_bytes INTEGER NOT NULL DEFAULT 0,
	files_transferred INTEGER NOT NULL DEFAULT 0,
	files_checked INTEGER NOT NULL DEFAULT 0,
	files_deleted INTEGER NOT NULL DEFAULT 0,
	errors_count INTEGER NOT NULL DEFAULT 0,
	error_message TEXT NOT NULL DEFAULT '',
	command_preview TEXT NOT NULL DEFAULT '',
	log_file TEXT NOT NULL DEFAULT '',
	backup_dir_used TEXT NOT NULL DEFAULT '',
	pruned_versions TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX IF NOT EXISTS idx_job_runs_job_started ON job_runs(job_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_runs_started ON job_runs(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_runs_status ON job_runs(status);
`,
}

// Open opens (or creates) the SQLite database at dbPath, configures WAL mode
// and pragmas, and applies ordered schema migrations.
func Open(dbPath string) (*Store, error) {
	dsn := dbPath
	if !strings.Contains(dsn, "?") {
		dsn += "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	}

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite handles concurrent writers best with a single pooled connection in-process.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: sqlDB}
	if err := s.migrate(context.Background()); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate sqlite: %w", err)
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	var currentVersion int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&currentVersion); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}

	for i := currentVersion; i < len(migrations); i++ {
		targetVersion := i + 1
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", targetVersion, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", targetVersion)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("update user_version to %d: %w", targetVersion, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", targetVersion, err)
		}
	}
	return nil
}
