package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

// GetSettings returns persisted global settings, seeding from defaults if none exist yet.
func (s *Store) GetSettings(ctx context.Context, defaults domain.GlobalSettings) (domain.GlobalSettings, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT data FROM settings WHERE id = 1`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		if saveErr := s.SaveSettings(ctx, defaults); saveErr != nil {
			return defaults, saveErr
		}
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}

	out := defaults
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return defaults, err
	}
	normalizeSettings(&out, defaults)
	return out, nil
}

// SaveSettings persists global settings.
func (s *Store) SaveSettings(ctx context.Context, settings domain.GlobalSettings) error {
	if settings.MaxConcurrentJobs < 1 {
		settings.MaxConcurrentJobs = 1
	}
	if settings.Timezone == "" {
		settings.Timezone = "UTC"
	}
	if settings.LogRetentionDays < 0 {
		settings.LogRetentionDays = 0
	}
	settings.WebhookURL = strings.TrimSpace(settings.WebhookURL)
	settings.WebhookOn = strings.ToLower(strings.TrimSpace(settings.WebhookOn))
	if settings.WebhookOn == "" {
		settings.WebhookOn = "failure"
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `
INSERT INTO settings (id, data, updated_at) VALUES (1, ?, ?)
ON CONFLICT(id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at
`, string(data), now)
	return err
}

func normalizeSettings(s *domain.GlobalSettings, defaults domain.GlobalSettings) {
	if s.MaxConcurrentJobs < 1 {
		s.MaxConcurrentJobs = defaults.MaxConcurrentJobs
		if s.MaxConcurrentJobs < 1 {
			s.MaxConcurrentJobs = 1
		}
	}
	if s.Timezone == "" {
		s.Timezone = defaults.Timezone
		if s.Timezone == "" {
			s.Timezone = "UTC"
		}
	}
	if s.Timeout == "" {
		s.Timeout = defaults.Timeout
	}
	if s.ConTimeout == "" {
		s.ConTimeout = defaults.ConTimeout
	}
	s.WebhookURL = strings.TrimSpace(s.WebhookURL)
	s.WebhookOn = strings.ToLower(strings.TrimSpace(s.WebhookOn))
	if s.WebhookOn == "" {
		s.WebhookOn = "failure"
	}
}
