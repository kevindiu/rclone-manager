package runner

import (
	"testing"
	"time"
)

func TestParseVersionFolderDate(t *testing.T) {
	loc := time.UTC

	cases := []struct {
		name   string
		format string
		wantOK bool
		wantY  int
		wantM  time.Month
		wantD  int
	}{
		{"2026-11-26", "2006-01-02", true, 2026, time.November, 26},
		{"2026-11-26_150405", "2006-01-02_150405", true, 2026, time.November, 26},
		{"2026-11-26_custom", "", true, 2026, time.November, 26},
		{"2026-11-26T10-00", "", true, 2026, time.November, 26},
		{"notes", "2006-01-02", false, 0, 0, 0},
		{"archive", "", false, 0, 0, 0},
		{"2026-99-99", "2006-01-02", false, 0, 0, 0},
		{"2026-11-26extra", "2006-01-02", false, 0, 0, 0},
	}

	for _, tc := range cases {
		got, ok := parseVersionFolderDate(tc.name, tc.format, loc)
		if ok != tc.wantOK {
			t.Errorf("parseVersionFolderDate(%q, %q) ok=%v, want %v", tc.name, tc.format, ok, tc.wantOK)
			continue
		}
		if ok && (got.Year() != tc.wantY || got.Month() != tc.wantM || got.Day() != tc.wantD) {
			t.Errorf("parseVersionFolderDate(%q, %q) = %v, want %04d-%02d-%02d", tc.name, tc.format, got, tc.wantY, tc.wantM, tc.wantD)
		}
	}
}
