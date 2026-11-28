package rclone

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestUploadFileNestedSubpath(t *testing.T) {
	dir := t.TempDir()
	rootDir := filepath.Join(dir, "remote-root")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatal(err)
	}
	confPath := filepath.Join(dir, "rclone.conf")
	client := NewClient("rclone", confPath)
	ctx := context.Background()

	if err := client.CreateRemote(ctx, "testrem", "alias", map[string]string{"remote": rootDir}); err != nil {
		t.Fatalf("CreateRemote: %v", err)
	}

	if err := client.UploadFile(ctx, "testrem", "folder-a/sub-b/hello.txt", strings.NewReader("nested-ok")); err != nil {
		t.Fatalf("UploadFile nested subpath failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(rootDir, "folder-a", "sub-b", "hello.txt"))
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}
	if string(got) != "nested-ok" {
		t.Fatalf("uploaded content = %q, want %q", string(got), "nested-ok")
	}
}

func TestUploadDir(t *testing.T) {
	dir := t.TempDir()
	rootDir := filepath.Join(dir, "remote-root")
	srcDir := filepath.Join(dir, "staging")
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "nested", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "nested", "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "nested", "sub", "b.txt"), []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}

	confPath := filepath.Join(dir, "rclone.conf")
	client := NewClient("rclone", confPath)
	ctx := context.Background()
	if err := client.CreateRemote(ctx, "testrem", "alias", map[string]string{"remote": rootDir}); err != nil {
		t.Fatalf("CreateRemote: %v", err)
	}

	if err := client.UploadDir(ctx, srcDir, "testrem", "uploads", 16, 32, ""); err != nil {
		t.Fatalf("UploadDir failed: %v", err)
	}

	for rel, want := range map[string]string{
		filepath.Join("uploads", "nested", "a.txt"):        "alpha",
		filepath.Join("uploads", "nested", "sub", "b.txt"): "beta",
	} {
		got, err := os.ReadFile(filepath.Join(rootDir, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if string(got) != want {
			t.Fatalf("%s content = %q, want %q", rel, string(got), want)
		}
	}
}
