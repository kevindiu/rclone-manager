package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kevindiu/rclone-manager/internal/rclone"
)

const localBrowseRoot = "/"

func isWithinBase(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

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

	baseAbs, err := filepath.Abs(localBrowseRoot)
	if err != nil {
		return "", nil, err
	}
	cleanAbs, err := filepath.Abs(clean)
	if err != nil {
		return "", nil, err
	}
	if !isWithinBase(baseAbs, cleanAbs) {
		return "", nil, fmt.Errorf("%w: local path escapes allowed browse root", ErrValidation)
	}

	entries, err := os.ReadDir(cleanAbs)
	if err != nil {
		return "", nil, err
	}

	dirs := make([]rclone.BrowseEntry, 0, len(entries))
	for _, de := range entries {
		name := de.Name()
		isDir := de.IsDir()
		if !isDir && (de.Type()&os.ModeSymlink) != 0 {
			if info, statErr := os.Stat(filepath.Join(cleanAbs, name)); statErr == nil && info.IsDir() {
				isDir = true
			}
		}
		if !isDir {
			continue
		}
		dirs = append(dirs, rclone.BrowseEntry{
			Path:  filepath.Join(cleanAbs, name),
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
