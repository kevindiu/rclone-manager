package rclone

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaskRemoteConfig(t *testing.T) {
	opts := map[string]ProviderOption{
		"password": {Name: "password", IsPassword: true},
		"endpoint": {Name: "endpoint", Sensitive: false},
		"token":    {Name: "token", Sensitive: true},
	}
	raw := map[string]string{
		"type":              "s3",
		"provider":          "Cloudflare",
		"endpoint":          "https://r2.example.com",
		"access_key_id":     "AKIA12345",
		"secret_access_key": "supersecret",
		"password":          "obscuredpw",
		"token":             `{"access_token":"abc"}`,
	}

	masked, keys := MaskRemoteConfig(raw, opts)
	if masked["type"] != "s3" || masked["endpoint"] != "https://r2.example.com" || masked["provider"] != "Cloudflare" {
		t.Fatalf("non-secret fields altered: %+v", masked)
	}
	for _, secretKey := range []string{"access_key_id", "secret_access_key", "password", "token"} {
		if masked[secretKey] != MaskedSecret {
			t.Fatalf("expected %s to be masked, got %q", secretKey, masked[secretKey])
		}
	}
	if len(keys) != 4 {
		t.Fatalf("expected 4 masked keys, got %v", keys)
	}
}

func TestValidateParamKeyRejectsFlagInjection(t *testing.T) {
	client := NewClient("rclone", filepath.Join(t.TempDir(), "rclone.conf"))
	if err := client.CreateRemote(context.Background(), "badremote", "alias", map[string]string{
		"--config": "/etc/passwd",
		"remote":   "/tmp",
	}); err == nil {
		t.Fatal("expected CreateRemote to reject flag-like param key")
	}
	for _, bad := range []string{"-v", "--log-file", "foo=bar", "has space", "1starts_digit"} {
		if err := ValidateParamKey(bad); err == nil {
			t.Fatalf("expected ValidateParamKey(%q) to fail", bad)
		}
	}
	for _, good := range []string{"remote", "access_key_id", "password2", "env_auth"} {
		if err := ValidateParamKey(good); err != nil {
			t.Fatalf("expected ValidateParamKey(%q) to pass: %v", good, err)
		}
	}
}

func TestParseAndMergeINI(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "rclone.conf")

	initial := `[local1]
type = local

[crypt1]
type = crypt
remote = /tmp/crypt
password = secret
`
	if err := os.WriteFile(confPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	incoming := `[crypt1]
type = crypt
remote = /tmp/crypt-v2
password = secret2

[alias1]
type = alias
remote = /tmp/alias
`
	imported, err := MergeOrValidateINI(context.Background(), "rclone", confPath, incoming, "merge")
	if err != nil {
		t.Fatalf("MergeOrValidateINI merge failed: %v", err)
	}
	if len(imported) != 2 {
		t.Fatalf("expected 2 imported sections, got %v", imported)
	}

	content, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, "[local1]") || !strings.Contains(text, "[crypt1]") || !strings.Contains(text, "[alias1]") {
		t.Fatalf("merged config missing expected sections:\n%s", text)
	}
	if !strings.Contains(text, "/tmp/crypt-v2") {
		t.Fatalf("merged config did not update crypt1:\n%s", text)
	}

	// Replace mode should drop local1
	if _, err := MergeOrValidateINI(context.Background(), "rclone", confPath, incoming, "replace"); err != nil {
		t.Fatalf("MergeOrValidateINI replace failed: %v", err)
	}
	replaced, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(replaced), "[local1]") {
		t.Fatalf("replace mode kept old section:\n%s", string(replaced))
	}
}
