package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kevindiu/rclone-manager/internal/rclone"
)

// ListLocalDirs lists subdirectories inside an absolute local filesystem path.
// Regular files are omitted because backup sources, destinations, and restore
// targets in PathPickerModal operate on directories.
func (s *AppService) ListLocalDirs(dirPath string) (string, []rclone.BrowseEntry, error) {
	trimmed := strings.TrimSpace(dirPath)
	if trimmed == "" {
		trimmed = "/"
	}
	if !filepath.IsAbs(trimmed) {
		return "", nil, fmt.Errorf("%w: local path must be absolute (start with /)", ErrValidation)
	}
	clean := filepath.Clean(trimmed)

	entries, err := os.ReadDir(clean)
	if err != nil {
		return "", nil, err
	}

	dirs := make([]rclone.BrowseEntry, 0, len(entries))
	for _, de := range entries {
		name := de.Name()
		isDir := de.IsDir()
		if !isDir && (de.Type()&os.ModeSymlink) != 0 {
			if info, statErr := os.Stat(filepath.Join(clean, name)); statErr == nil && info.IsDir() {
				isDir = true
			}
		}
		if !isDir {
			continue
		}
		dirs = append(dirs, rclone.BrowseEntry{
			Path:  filepath.Join(clean, name),
			Name:  name,
			IsDir: true,
		})
	}

	sort.Slice(dirs, func(i, j int) bool {
		hi := strings.HasPrefix(dirs[i].Name, ".")
		hj := strings.HasPrefix(dirs[j].Name, ".")
		if hi != hj {
			return !hi
		}
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	return clean, dirs, nil
}
