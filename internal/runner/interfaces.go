package runner

import (
	"context"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

// RunStore defines the persistence operations required by Runner.
type RunStore interface {
	GetSettings(ctx context.Context, defaults domain.GlobalSettings) (domain.GlobalSettings, error)
	CreateRun(ctx context.Context, run domain.JobRun) error
	UpdateRun(ctx context.Context, run domain.JobRun) error
}

// RcloneOps defines the rclone CLI operations required by Runner.
type RcloneOps interface {
	Bin() string
	ConfPath() string
	CheckSourceNotEmpty(ctx context.Context, source string) (bool, error)
	CheckMarkerExists(ctx context.Context, source, marker string) (bool, error)
	ListDirs(ctx context.Context, targetPath string) ([]string, error)
	PurgeDir(ctx context.Context, targetPath string) error
}

// Broadcaster publishes live execution events to subscribers.
type Broadcaster interface {
	Broadcast(eventType string, data any)
}
