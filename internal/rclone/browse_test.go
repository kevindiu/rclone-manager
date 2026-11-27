package rclone

import "testing"

func TestCleanSubpathAndFormatRemoteTarget(t *testing.T) {
	valid := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{".", ""},
		{"///", ""},
		{"docs/report.txt", "docs/report.txt"},
		{"/docs/sub//report.txt/", "docs/sub/report.txt"},
		{`photos\2026\a.jpg`, "photos/2026/a.jpg"},
		{"./folder/file", "folder/file"},
	}
	for _, tc := range valid {
		got, err := CleanSubpath(tc.in)
		if err != nil {
			t.Errorf("CleanSubpath(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("CleanSubpath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	for _, bad := range []string{"..", "../secret", "a/../../b", `a\..\b`, "docs/../etc/passwd"} {
		if _, err := CleanSubpath(bad); err == nil {
			t.Errorf("CleanSubpath(%q) expected traversal error, got nil", bad)
		}
	}

	if target, err := FormatRemoteTarget("r2", ""); err != nil || target != "r2:" {
		t.Fatalf("FormatRemoteTarget root = (%q, %v), want (\"r2:\", nil)", target, err)
	}
	if target, err := FormatRemoteTarget("r2", "/docs/file.txt"); err != nil || target != "r2:docs/file.txt" {
		t.Fatalf("FormatRemoteTarget file = (%q, %v)", target, err)
	}
	for _, badRem := range []string{"", "bad..name", "-flag", "a:b", "a/b"} {
		if _, err := FormatRemoteTarget(badRem, "file.txt"); err == nil {
			t.Errorf("FormatRemoteTarget(%q) expected error, got nil", badRem)
		}
	}
}

func TestBuildVersionCandidates(t *testing.T) {
	// Nested path produces innermost-to-root ancestor _versions candidates.
	got := buildVersionCandidates("a/b/c.txt", nil)
	want := []VersionRootCandidate{
		{BackupRoot: "a/b/_versions", RelPath: "c.txt"},
		{BackupRoot: "a/_versions", RelPath: "b/c.txt"},
		{BackupRoot: "_versions", RelPath: "a/b/c.txt"},
	}
	if len(got) != len(want) {
		t.Fatalf("nested candidates = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidate[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	// Extra custom backup roots appear first and duplicates are deduplicated.
	extra := []VersionRootCandidate{
		{BackupRoot: "archive/vers", RelPath: "c.txt"},
		{BackupRoot: "_versions", RelPath: "c.txt"},
		{BackupRoot: "archive/vers", RelPath: "c.txt"},
	}
	gotRoot := buildVersionCandidates("c.txt", extra)
	if len(gotRoot) != 2 {
		t.Fatalf("expected 2 deduplicated candidates, got %+v", gotRoot)
	}
	if gotRoot[0].BackupRoot != "archive/vers" || gotRoot[1].BackupRoot != "_versions" {
		t.Fatalf("unexpected candidate ordering: %+v", gotRoot)
	}
}
