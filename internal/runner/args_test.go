package runner

import (
	"slices"
	"testing"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

func containsFlagPair(args []string, flag, val string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == val {
			return true
		}
	}
	return false
}

func TestBuildRcloneArgsOverridesAndDefaults(t *testing.T) {
	ovTransfers := 12
	ovCheckers := 24
	ovRetries := 7
	ovLowRetries := 15
	ovMaxDelete := 25

	defaults := domain.GlobalSettings{
		Transfers:       4,
		Checkers:        8,
		BwLimit:         "5M",
		Retries:         3,
		LowLevelRetries: 10,
		Timeout:         "5m",
		ConTimeout:      "1m",
	}

	job := domain.Job{
		Operation:       "copyto",
		Source:          "r2:src/file.txt",
		Destination:     "/dst/file.txt",
		Transfers:       &ovTransfers,
		Checkers:        &ovCheckers,
		BwLimit:         "20M",
		Retries:         &ovRetries,
		LowLevelRetries: &ovLowRetries,
		Timeout:         "10m",
		ConTimeout:      "30s",
		MaxTransfer:     "50G",
		CutoffMode:      "hard",
		MaxDelete:       &ovMaxDelete,
		ExcludePatterns: []string{"*.tmp"},
		IncludePatterns: []string{"*.pdf"},
		FilterRules:     []string{"- .DS_Store"},
		CustomFlags:     "--fast-list --check-first",
	}

	args, _, _, err := BuildRcloneArgs(job, defaults, "/cfg/rclone.conf", true, time.Now())
	if err != nil {
		t.Fatalf("BuildRcloneArgs failed: %v", err)
	}

	for _, pair := range [][2]string{
		{"--config", "/cfg/rclone.conf"},
		{"--transfers", "12"},
		{"--checkers", "24"},
		{"--bwlimit", "20M"},
		{"--retries", "7"},
		{"--low-level-retries", "15"},
		{"--timeout", "10m"},
		{"--contimeout", "30s"},
		{"--max-transfer", "50G"},
		{"--max-delete", "25"},
		{"--exclude", "*.tmp"},
		{"--include", "*.pdf"},
		{"--filter", "- .DS_Store"},
	} {
		if !containsFlagPair(args, pair[0], pair[1]) {
			t.Errorf("expected flag pair %s %s in args: %v", pair[0], pair[1], args)
		}
	}

	for _, single := range []string{"copyto", "--dry-run", "--cutoff-mode=hard", "--fast-list", "--check-first"} {
		if !slices.Contains(args, single) {
			t.Errorf("expected %q in args: %v", single, args)
		}
	}

	// Empty overrides fall back to global defaults and invalid operation defaults to sync.
	baseArgs, _, _, err := BuildRcloneArgs(domain.Job{
		Operation:   "invalid",
		Source:      "/a",
		Destination: "/b",
	}, defaults, "/cfg/rclone.conf", false, time.Now())
	if err != nil {
		t.Fatalf("fallback BuildRcloneArgs failed: %v", err)
	}
	if !slices.Contains(baseArgs, "sync") || !containsFlagPair(baseArgs, "--transfers", "4") || !containsFlagPair(baseArgs, "--bwlimit", "5M") {
		t.Fatalf("unexpected fallback args: %v", baseArgs)
	}
}

func TestBuildRcloneArgsVersioningSelfExclusion(t *testing.T) {
	now := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	defaults := domain.GlobalSettings{Transfers: 4, Checkers: 8}

	// Default backup dir inside Destination gets /_versions/** excluded automatically.
	argsDefault, baseDefault, datedDefault, err := BuildRcloneArgs(domain.Job{
		Operation:         "sync",
		Source:            "/src",
		Destination:       "r2:bucket/docs",
		VersioningEnabled: true,
	}, defaults, "/cfg/rclone.conf", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if baseDefault != "r2:bucket/docs/_versions" || datedDefault != "r2:bucket/docs/_versions/2026-11-27" {
		t.Fatalf("unexpected default backup dirs: base=%q dated=%q", baseDefault, datedDefault)
	}
	if !containsFlagPair(argsDefault, "--backup-dir", "r2:bucket/docs/_versions/2026-11-27") ||
		!containsFlagPair(argsDefault, "--exclude", "/_versions/**") {
		t.Fatalf("expected auto self-exclude for default _versions: %v", argsDefault)
	}

	// Nested custom backup dir inside remote root Destination excludes relative subpath.
	argsNested, _, _, err := BuildRcloneArgs(domain.Job{
		Operation:         "sync",
		Source:            "/src",
		Destination:       "r2:",
		VersioningEnabled: true,
		BackupDirBase:     "r2:sub/old",
		VersionFormat:     "2006-01-02_1504",
	}, defaults, "/cfg/rclone.conf", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if !containsFlagPair(argsNested, "--backup-dir", "r2:sub/old/2026-11-27_0900") ||
		!containsFlagPair(argsNested, "--exclude", "/sub/old/**") {
		t.Fatalf("expected nested relative self-exclude: %v", argsNested)
	}

	// Custom backup dir OUTSIDE Destination must NOT inject an --exclude rule.
	argsOutside, _, _, err := BuildRcloneArgs(domain.Job{
		Operation:         "sync",
		Source:            "/src",
		Destination:       "r2:docs",
		VersioningEnabled: true,
		BackupDirBase:     "r2:archive/vers",
	}, defaults, "/cfg/rclone.conf", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if !containsFlagPair(argsOutside, "--backup-dir", "r2:archive/vers/2026-11-27") {
		t.Fatalf("expected external backup-dir set: %v", argsOutside)
	}
	if slices.Contains(argsOutside, "--exclude") {
		t.Fatalf("did not expect --exclude for external backup_dir_base: %v", argsOutside)
	}
}

func TestBuildRcloneArgsRejectsManagedCustomFlags(t *testing.T) {
	for _, badFlag := range []string{
		"--config /etc/evil.conf",
		"--dry-run=false",
		"-nv",
		"--password-command=id",
	} {
		_, _, _, err := BuildRcloneArgs(domain.Job{
			Operation:   "copy",
			Source:      "/src",
			Destination: "/dst",
			CustomFlags: badFlag,
		}, domain.GlobalSettings{}, "/cfg/rclone.conf", true, time.Now())
		if err == nil {
			t.Fatalf("expected managed flag rejection for %q", badFlag)
		}
	}

	for _, badJob := range []domain.Job{
		{Operation: "copy", Source: "--dry-run=false", Destination: "/dst"},
		{Operation: "copy", Source: "/src", Destination: "--log-file=/tmp/x"},
		{Operation: "copy", Source: "/src", Destination: "/dst", BackupDirBase: "-rf"},
	} {
		if _, _, _, err := BuildRcloneArgs(badJob, domain.GlobalSettings{}, "/cfg/rclone.conf", true, time.Now()); err == nil {
			t.Fatalf("expected hyphen-prefixed path rejection for %+v", badJob)
		}
	}
}

func TestBuildRcloneArgsMoveCheckAndCommonFlags(t *testing.T) {
	defaults := domain.GlobalSettings{Transfers: 4, Checkers: 8}
	now := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)

	moveArgs, _, _, err := BuildRcloneArgs(domain.Job{
		Operation:               "move",
		Source:                  "/src",
		Destination:             "r2:dst",
		FastList:                true,
		Checksum:                true,
		UpdateOnly:              true,
		ServerSideAcrossConfigs: true,
	}, defaults, "/cfg/rclone.conf", false, now)
	if err != nil {
		t.Fatalf("BuildRcloneArgs move failed: %v", err)
	}
	for _, flag := range []string{"move", "--fast-list", "--checksum", "--update", "--server-side-across-configs"} {
		if !slices.Contains(moveArgs, flag) {
			t.Errorf("expected %q in move args: %v", flag, moveArgs)
		}
	}

	maxDel := 10
	checkArgs, backupBase, datedBackupDir, err := BuildRcloneArgs(domain.Job{
		Operation:               "check",
		Source:                  "/src",
		Destination:             "r2:dst",
		FastList:                true,
		Checksum:                true,
		UpdateOnly:              true,
		ServerSideAcrossConfigs: true,
		VersioningEnabled:       true,
		MaxDelete:               &maxDel,
		MaxTransfer:             "10G",
		ExcludePatterns:         []string{"*.tmp"},
	}, defaults, "/cfg/rclone.conf", false, now)
	if err != nil {
		t.Fatalf("BuildRcloneArgs check failed: %v", err)
	}
	if backupBase != "" || datedBackupDir != "" {
		t.Fatalf("check should not emit backup dirs, got base=%q dated=%q", backupBase, datedBackupDir)
	}
	for _, flag := range []string{"check", "--fast-list", "--checksum", "--server-side-across-configs", "--exclude"} {
		if !slices.Contains(checkArgs, flag) {
			t.Errorf("expected %q in check args: %v", flag, checkArgs)
		}
	}
	for _, forbidden := range []string{"--backup-dir", "--max-delete", "--max-transfer", "--update"} {
		if slices.Contains(checkArgs, forbidden) {
			t.Errorf("check must omit %q: %v", forbidden, checkArgs)
		}
	}
}

func TestBuildRcloneArgsBisyncFiltersAndTiming(t *testing.T) {
	defaults := domain.GlobalSettings{Transfers: 4, Checkers: 8}
	now := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	maxDel := 5

	bisyncArgs, _, _, err := BuildRcloneArgs(domain.Job{
		Operation:             "bisync",
		Source:                "/local/docs",
		Destination:           "r2:docs",
		BisyncResync:          true,
		BisyncConflictResolve: "newer",
		SymlinkMode:           "links",
		SizeOnly:              true,
		UpdateOnly:            true,
		IgnoreExisting:        true,
		VersioningEnabled:     true,
		MaxDelete:             &maxDel,
		MinAge:                "1h",
		MaxAge:                "30d",
		MinSize:               "10k",
		MaxSize:               "1G",
	}, defaults, "/cfg/rclone.conf", false, now)
	if err != nil {
		t.Fatalf("BuildRcloneArgs bisync failed: %v", err)
	}
	for _, want := range []string{"bisync", "--resync", "--links", "--size-only"} {
		if !slices.Contains(bisyncArgs, want) {
			t.Errorf("expected %q in bisync args: %v", want, bisyncArgs)
		}
	}
	for _, pair := range [][2]string{
		{"--conflict-resolve", "newer"},
		{"--min-age", "1h"},
		{"--max-age", "30d"},
		{"--min-size", "10k"},
		{"--max-size", "1G"},
	} {
		if !containsFlagPair(bisyncArgs, pair[0], pair[1]) {
			t.Errorf("expected %s %s in bisync args: %v", pair[0], pair[1], bisyncArgs)
		}
	}
	for _, forbidden := range []string{"--backup-dir", "--max-delete", "--update", "--ignore-existing"} {
		if slices.Contains(bisyncArgs, forbidden) {
			t.Errorf("bisync must omit %q: %v", forbidden, bisyncArgs)
		}
	}

	syncArgs, _, _, err := BuildRcloneArgs(domain.Job{
		Operation:      "sync",
		Source:         "/a",
		Destination:    "/b",
		DeleteTiming:   "after",
		SymlinkMode:    "copy_links",
		IgnoreExisting: true,
	}, defaults, "/cfg/rclone.conf", false, now)
	if err != nil {
		t.Fatalf("BuildRcloneArgs sync timing failed: %v", err)
	}
	for _, want := range []string{"--delete-after", "--copy-links", "--ignore-existing"} {
		if !slices.Contains(syncArgs, want) {
			t.Errorf("expected %q in sync args: %v", want, syncArgs)
		}
	}

	moveArgs, _, _, err := BuildRcloneArgs(domain.Job{
		Operation:          "move",
		Source:             "/a",
		Destination:        "/b",
		DeleteEmptySrcDirs: true,
	}, defaults, "/cfg/rclone.conf", false, now)
	if err != nil {
		t.Fatalf("BuildRcloneArgs move empty dirs failed: %v", err)
	}
	if !slices.Contains(moveArgs, "--delete-empty-src-dirs") {
		t.Errorf("expected --delete-empty-src-dirs in move args: %v", moveArgs)
	}
}
