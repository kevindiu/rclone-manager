package auth

import (
	"strings"
	"testing"
	"time"
)

func TestHashAndVerifyPassword(t *testing.T) {
	h1, err := HashPasswordWithIterations("CorrectHorseBatteryStaple!", 10000)
	if err != nil {
		t.Fatalf("HashPasswordWithIterations failed: %v", err)
	}
	h2, err := HashPasswordWithIterations("CorrectHorseBatteryStaple!", 10000)
	if err != nil {
		t.Fatalf("HashPasswordWithIterations 2 failed: %v", err)
	}
	if h1 == h2 {
		t.Fatal("expected distinct salts to produce distinct hashes")
	}
	if !strings.HasPrefix(h1, "pbkdf2_sha256$10000$") {
		t.Fatalf("unexpected hash format: %q", h1)
	}
	if !VerifyPassword("CorrectHorseBatteryStaple!", h1) {
		t.Fatal("expected VerifyPassword to accept matching password")
	}
	if VerifyPassword("WrongPassword", h1) {
		t.Fatal("expected VerifyPassword to reject wrong password")
	}
	if VerifyPassword("CorrectHorseBatteryStaple!", "malformed$hash") {
		t.Fatal("expected VerifyPassword to reject malformed hash")
	}
}

func TestSessionTokenGenerationAndHash(t *testing.T) {
	raw, hash, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken failed: %v", err)
	}
	if raw == "" || len(hash) != 64 {
		t.Fatalf("unexpected token/hash: raw=%q hash=%q", raw, hash)
	}
	if HashSessionToken(raw) != hash {
		t.Fatal("expected HashSessionToken to be deterministic")
	}
	raw2, hash2, err := GenerateSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if raw == raw2 || hash == hash2 {
		t.Fatal("expected unique session tokens")
	}
}

func TestLoginRateLimiter(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	lim := NewLoginRateLimiter(3, 5*time.Minute)
	lim.now = func() time.Time { return now }

	for i := 0; i < 3; i++ {
		allowed, retry := lim.Allow("1.2.3.4")
		if !allowed || retry != 0 {
			t.Fatalf("attempt %d should be allowed, got allowed=%v retry=%v", i, allowed, retry)
		}
		lim.RecordFailure("1.2.3.4")
	}

	allowed, retry := lim.Allow("1.2.3.4")
	if allowed || retry <= 0 {
		t.Fatalf("expected block after 3 failures, got allowed=%v retry=%v", allowed, retry)
	}

	// Unrelated key remains unaffected.
	if ok, _ := lim.Allow("5.6.7.8"); !ok {
		t.Fatal("expected separate IP to remain allowed")
	}

	// Advancing past window allows attempts again.
	now = now.Add(5*time.Minute + time.Second)
	if ok, _ := lim.Allow("1.2.3.4"); !ok {
		t.Fatal("expected key to recover after sliding window")
	}

	// Recording and resetting clears failures immediately.
	lim.RecordFailure("1.2.3.4")
	lim.RecordFailure("1.2.3.4")
	lim.RecordFailure("1.2.3.4")
	if ok, _ := lim.Allow("1.2.3.4"); ok {
		t.Fatal("expected key blocked before reset")
	}
	lim.Reset("1.2.3.4")
	if ok, _ := lim.Allow("1.2.3.4"); !ok {
		t.Fatal("expected key allowed immediately after Reset")
	}
}
