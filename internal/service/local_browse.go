package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kevindiu/rclone-manager/internal/rclone"
)

func isWithinBase(base, candidate string) bool {
	rel, err := filepath.Rel(base, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

// ListLocalDirs lists subdirectories inside an absolute local filesystem path.
// Regular files are omitted because backup sources, destinations, and restore
// targets in PathPickerModal operate on directories.
func (s *AppService) ListLocalDirs(dirPath string) (string, []rclone.BrowseEntry, error) {
	const localBrowseBase = "/"

	trimmed := strings.TrimSpace(dirPath)
	if trimmed == "" {
		trimmed = localBrowseBase
	}
	if !filepath.IsAbs(trimmed) {
		return "", nil, fmt.Errorf("%w: local path must be absolute (start with /)", ErrValidation)
	}
	clean := filepath.Clean(trimmed)

	baseAbs, err := filepath.Abs(localBrowseBase)
	if err != nil {
		return "", nil, err
	}
	cleanAbs, err := filepath.Abs(clean)
	if err != nil {
		return "", nil, err
	}
	if !isWithinBase(baseAbs, cleanAbs) {
		return "", nil, fmt.Errorf("%w: local path is outside allowed directory", ErrValidation)
	}

	entries, err := os.ReadDir(cleanAbs)
	if err != nil {
		return "", nil, err
	}

	dirs := make([]rclone.BrowseEntry, 0, len(entries))
	for _, de := range entries {
		name := de.Name()
		isDir := de.IsDir()
		childPath := filepath.Join(cleanAbs, name)
		if !isWithinBase(baseAbs, childPath) {
			continue
		}
		if !isDir && (de.Type()&os.ModeSymlink) != 0 {
			if info, statErr := os.Stat(childPath); statErr == nil && info.IsDir() {
				isDir = true
			}
		}
		if !isDir {
			continue
		}
		dirs = append(dirs, rclone.BrowseEntry{
			Path:  childPath,
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
	return cleanAbs, dirs, nil
}
