package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

func TestEncryptDecryptBackupPayloadRoundTripAndFailures(t *testing.T) {
	plaintext := []byte(`{"hello":"secret-world"}`)
	if _, err := EncryptBackupPayload(plaintext, "short", minBackupPBKDF2Iterations); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for short passphrase, got %v", err)
	}

	encrypted, err := EncryptBackupPayload(plaintext, "correct-horse-battery", minBackupPBKDF2Iterations)
	if err != nil {
		t.Fatalf("EncryptBackupPayload error = %v", err)
	}
	if strings.Contains(string(encrypted), "secret-world") {
		t.Fatal("encrypted envelope leaked plaintext")
	}

	got, err := DecryptBackupPayload(encrypted, "correct-horse-battery")
	if err != nil {
		t.Fatalf("DecryptBackupPayload error = %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("decrypted = %q, want %q", string(got), string(plaintext))
	}

	if _, err := DecryptBackupPayload(encrypted, "wrong-passphrase"); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for wrong passphrase, got %v", err)
	}

	tampered := strings.Replace(string(encrypted), "aes-256-gcm", "aes-128-cbc", 1)
	if _, err := DecryptBackupPayload([]byte(tampered), "correct-horse-battery"); !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation for tampered cipher field, got %v", err)
	}
}

func TestAppServiceFullEncryptedBackupRoundTrip(t *testing.T) {
	svc, st, _, sc, hb, br := newTestService()
	ctx := context.Background()

	br.rawConf = "[myremote]\ntype = s3\nsecret_access_key = supersecret123\n"
	br.importRemotes = []string{"myremote"}
	st.settings = domain.GlobalSettings{
		Transfers:         6,
		Checkers:          12,
		Retries:           3,
		LowLevelRetries:   10,
		Timeout:           "5m",
		ConTimeout:        "1m",
		LogRetentionDays:  30,
		Timezone:          "Asia/Hong_Kong",
		MaxConcurrentJobs: 2,
		WebhookType:       "telegram",
		TelegramBotToken:  "123456:ABCDEF",
		TelegramChatID:    "9999",
	}
	st.jobs["job-1"] = domain.Job{
		ID:          "job-1",
		Name:        "Nightly Sync",
		Operation:   "sync",
		Source:      "/data",
		Destination: "myremote:bucket",
		Enabled:     true,
	}

	rawBackup, err := svc.ExportEncryptedBackup(ctx, "StrongBackupPass!1")
	if err != nil {
		t.Fatalf("ExportEncryptedBackup error = %v", err)
	}
	if strings.Contains(string(rawBackup), "supersecret123") || strings.Contains(string(rawBackup), "123456:ABCDEF") {
		t.Fatal("encrypted backup leaked secrets")
	}

	// Wipe store and restore via ImportEncryptedBackup
	st.jobs = make(map[string]domain.Job)
	st.settings = domain.GlobalSettings{}

	res, err := svc.ImportEncryptedBackup(ctx, rawBackup, "StrongBackupPass!1", "replace")
	if err != nil {
		t.Fatalf("ImportEncryptedBackup error = %v", err)
	}
	if res.JobsImported != 1 || res.RemotesImported != 1 {
		t.Fatalf("unexpected import result: %+v", res)
	}
	if br.importedMode != "replace" || br.importedConf != br.rawConf {
		t.Fatalf("unexpected imported conf/mode: mode=%q conf=%q", br.importedMode, br.importedConf)
	}
	if st.settings.TelegramBotToken != "123456:ABCDEF" {
		t.Fatalf("expected restored TelegramBotToken, got %q", st.settings.TelegramBotToken)
	}
	if sc.reloads == 0 || len(hb.events) == 0 {
		t.Fatalf("expected scheduler reload and hub broadcast, reloads=%d events=%v", sc.reloads, hb.events)
	}
}

func TestAppServiceImportEncryptedBackupSkipsEmptyRcloneConf(t *testing.T) {
	svc, _, _, _, _, br := newTestService()
	ctx := context.Background()

	br.importErr = errors.New("should not be called when rclone_conf is empty")
	rawBackup, err := svc.ExportEncryptedBackup(ctx, "StrongBackupPass!1")
	if err != nil {
		t.Fatalf("ExportEncryptedBackup error = %v", err)
	}
	res, err := svc.ImportEncryptedBackup(ctx, rawBackup, "StrongBackupPass!1", "merge")
	if err != nil {
		t.Fatalf("ImportEncryptedBackup with empty rclone.conf error = %v", err)
	}
	if res.RemotesImported != 0 || len(res.RemoteNames) != 0 {
		t.Fatalf("expected 0 remotes imported, got %+v", res)
	}
}
