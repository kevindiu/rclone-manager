package rclone

import (
	"context"
	"path/filepath"
	"testing"
)

func TestValidateRemoteName(t *testing.T) {
	for _, okName := range []string{"gdrive", "Google Drive", "my-r2_1", "S3_Prod", "a1"} {
		if err := ValidateRemoteName(okName); err != nil {
			t.Errorf("ValidateRemoteName(%q) unexpected error: %v", okName, err)
		}
	}
	for _, badName := range []string{"", "bad..name", "-leadingdash", "_leading", "a:b", "a/b", "a@b"} {
		if err := ValidateRemoteName(badName); err == nil {
			t.Errorf("ValidateRemoteName(%q) expected error, got nil", badName)
		}
	}
}

func TestUpdateRemotePreservesMaskedSecretAndCachesVersion(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	client := NewClient("rclone", filepath.Join(dir, "rclone.conf"))

	ver1, arch1, err := client.Version(ctx)
	if err != nil || ver1 == "" {
		t.Fatalf("first Version() failed: ver=%q err=%v", ver1, err)
	}
	ver2, arch2, err := client.Version(ctx)
	if err != nil || ver2 != ver1 || arch2 != arch1 {
		t.Fatalf("cached Version() mismatch: (%q,%q) vs (%q,%q) err=%v", ver2, arch2, ver1, arch1, err)
	}

	cryptStore := filepath.Join(dir, "crypt-data")
	if err := client.CreateRemote(ctx, "mycrypt", "crypt", map[string]string{
		"remote":              cryptStore,
		"password":            "super-secret-passphrase",
		"filename_encryption": "standard",
	}); err != nil {
		t.Fatalf("CreateRemote failed: %v", err)
	}

	rawBefore, err := client.dumpRaw(ctx)
	if err != nil {
		t.Fatalf("dumpRaw before failed: %v", err)
	}
	origPassword := rawBefore["mycrypt"]["password"]
	if origPassword == "" || origPassword == "super-secret-passphrase" {
		t.Fatalf("expected obscured non-empty password in rclone.conf, got %q", origPassword)
	}

	// Updating with MaskedSecret for password must preserve origPassword while updating filename_encryption.
	if err := client.UpdateRemote(ctx, "mycrypt", map[string]string{
		"remote":              cryptStore,
		"password":            MaskedSecret,
		"filename_encryption": "off",
	}); err != nil {
		t.Fatalf("UpdateRemote failed: %v", err)
	}

	rawAfter, err := client.dumpRaw(ctx)
	if err != nil {
		t.Fatalf("dumpRaw after failed: %v", err)
	}
	if rawAfter["mycrypt"]["password"] != origPassword {
		t.Fatalf("expected obscured password preserved, got %q want %q", rawAfter["mycrypt"]["password"], origPassword)
	}
	if rawAfter["mycrypt"]["filename_encryption"] != "off" {
		t.Fatalf("expected filename_encryption=off, got %q", rawAfter["mycrypt"]["filename_encryption"])
	}
}
