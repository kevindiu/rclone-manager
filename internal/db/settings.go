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
	fillUploadBatchDefaults(&settings, domain.GlobalSettings{})
	settings.WebhookURL = strings.TrimSpace(settings.WebhookURL)
	settings.WebhookOn = strings.ToLower(strings.TrimSpace(settings.WebhookOn))
	if settings.WebhookOn == "" {
		settings.WebhookOn = "failure"
	}
	settings.WebhookType = strings.ToLower(strings.TrimSpace(settings.WebhookType))
	if settings.WebhookType == "" {
		settings.WebhookType = "generic"
	}
	settings.TelegramBotToken = strings.TrimSpace(settings.TelegramBotToken)
	settings.TelegramChatID = strings.TrimSpace(settings.TelegramChatID)
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
	fillUploadBatchDefaults(s, defaults)
	s.WebhookURL = strings.TrimSpace(s.WebhookURL)
	s.WebhookOn = strings.ToLower(strings.TrimSpace(s.WebhookOn))
	if s.WebhookOn == "" {
		s.WebhookOn = "failure"
	}
	s.WebhookType = strings.ToLower(strings.TrimSpace(s.WebhookType))
	if s.WebhookType == "" {
		s.WebhookType = "generic"
	}
	s.TelegramBotToken = strings.TrimSpace(s.TelegramBotToken)
	s.TelegramChatID = strings.TrimSpace(s.TelegramChatID)
}

func fillUploadBatchDefaults(s *domain.GlobalSettings, defaults domain.GlobalSettings) {
	if s.UploadBatchTransfers <= 0 {
		s.UploadBatchTransfers = max(defaults.UploadBatchTransfers, 0)
		if s.UploadBatchTransfers == 0 {
			s.UploadBatchTransfers = 16
		}
	}
	if s.UploadBatchCheckers <= 0 {
		s.UploadBatchCheckers = max(defaults.UploadBatchCheckers, 0)
		if s.UploadBatchCheckers == 0 {
			s.UploadBatchCheckers = 32
		}
	}
	if s.UploadBatchMaxFiles <= 0 {
		s.UploadBatchMaxFiles = max(defaults.UploadBatchMaxFiles, 0)
		if s.UploadBatchMaxFiles == 0 {
			s.UploadBatchMaxFiles = 80
		}
	}
	if s.UploadBatchMaxMB <= 0 {
		s.UploadBatchMaxMB = max(defaults.UploadBatchMaxMB, 0)
		if s.UploadBatchMaxMB == 0 {
			s.UploadBatchMaxMB = 64
		}
	}
	if s.UploadSmallFileMaxMB <= 0 {
		s.UploadSmallFileMaxMB = max(defaults.UploadSmallFileMaxMB, 0)
		if s.UploadSmallFileMaxMB == 0 {
			s.UploadSmallFileMaxMB = 16
		}
	}
}
