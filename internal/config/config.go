package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

// Config holds startup environment configuration and default settings seeds.
type Config struct {
	Host         string
	Port         string
	DataDir      string
	DBPath       string
	RcloneConfig string
	LogDir       string
	RcloneBin    string
	LogLevel     string
	LogFormat    string
	CORSOrigins  []string
	Defaults     domain.GlobalSettings
}

// Load reads configuration from environment variables with sensible defaults
// and ensures required directories exist.
func Load() (*Config, error) {
	dataDir := getEnv("RCLONE_MANAGER_DATA_DIR", "./data")
	port := getEnv("PORT", getEnv("RCLONE_MANAGER_PORT", "8080"))
	host := getEnv("RCLONE_MANAGER_HOST", "0.0.0.0")
	dbPath := getEnv("RCLONE_MANAGER_DB_PATH", filepath.Join(dataDir, "rclone-manager.db"))
	rcloneConf := getEnv("RCLONE_CONFIG", filepath.Join(dataDir, "rclone.conf"))
	logDir := getEnv("RCLONE_MANAGER_LOG_DIR", filepath.Join(dataDir, "logs"))
	rcloneBin := getEnv("RCLONE_BIN", "rclone")
	logLevel := strings.ToLower(strings.TrimSpace(getEnv("RCLONE_MANAGER_LOG_LEVEL", "info")))
	logFormat := strings.ToLower(strings.TrimSpace(getEnv("RCLONE_MANAGER_LOG_FORMAT", "text")))

	tz := getEnv("RCLONE_TIMEZONE", getEnv("TZ", "UTC"))
	if strings.TrimSpace(tz) == "" {
		tz = "UTC"
	}

	var corsOrigins []string
	if raw := strings.TrimSpace(getEnv("RCLONE_MANAGER_CORS_ORIGINS", "")); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if p := strings.TrimSpace(part); p != "" {
				corsOrigins = append(corsOrigins, p)
			}
		}
	}

	cfg := &Config{
		Host:         host,
		Port:         port,
		DataDir:      dataDir,
		DBPath:       dbPath,
		RcloneConfig: rcloneConf,
		LogDir:       logDir,
		RcloneBin:    rcloneBin,
		LogLevel:     logLevel,
		LogFormat:    logFormat,
		CORSOrigins:  corsOrigins,
		Defaults: domain.GlobalSettings{
			Transfers:         getEnvInt("RCLONE_DEFAULT_TRANSFERS", 4),
			Checkers:          getEnvInt("RCLONE_DEFAULT_CHECKERS", 8),
			BwLimit:           getEnv("RCLONE_DEFAULT_BWLIMIT", ""),
			Retries:           getEnvInt("RCLONE_DEFAULT_RETRIES", 3),
			LowLevelRetries:   getEnvInt("RCLONE_DEFAULT_LOW_LEVEL_RETRIES", 10),
			Timeout:           getEnv("RCLONE_DEFAULT_TIMEOUT", "5m"),
			ConTimeout:        getEnv("RCLONE_DEFAULT_CONTIMEOUT", "1m"),
			LogRetentionDays:  getEnvInt("RCLONE_LOG_RETENTION_DAYS", 30),
			Timezone:          tz,
			MaxConcurrentJobs: maxInt(1, getEnvInt("RCLONE_MAX_CONCURRENT_JOBS", 2)),
		},
	}

	if err := cfg.EnsureDirs(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// EnsureDirs creates the data, log, DB parent, and rclone config parent directories.
func (c *Config) EnsureDirs() error {
	dirs := []string{
		c.DataDir,
		c.LogDir,
		filepath.Dir(c.DBPath),
		filepath.Dir(c.RcloneConfig),
	}
	for _, d := range dirs {
		if d == "" || d == "." {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// Addr returns the HTTP listen address.
func (c *Config) Addr() string {
	return c.Host + ":" + c.Port
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return parsed
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
