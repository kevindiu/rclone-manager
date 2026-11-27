package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestValidateWebhookURLAndNormalizeMode(t *testing.T) {
	if err := ValidateWebhookURL(""); err != nil {
		t.Fatalf("empty URL should be allowed: %v", err)
	}
	if err := ValidateWebhookURL("https://discord.com/api/webhooks/123/abc"); err != nil {
		t.Fatalf("valid https URL rejected: %v", err)
	}
	if err := ValidateWebhookURL("ftp://example.com/hook"); err == nil {
		t.Fatal("expected error for non-http(s) scheme")
	}
	if err := ValidateWebhookURL("https://"); err == nil {
		t.Fatal("expected error for missing host")
	}

	if mode, err := NormalizeWebhookOn(""); err != nil || mode != "failure" {
		t.Fatalf("expected default failure, got %q err=%v", mode, err)
	}
	if mode, err := NormalizeWebhookOn("ALWAYS"); err != nil || mode != "always" {
		t.Fatalf("expected always, got %q err=%v", mode, err)
	}
	if _, err := NormalizeWebhookOn("invalid"); err == nil {
		t.Fatal("expected error for invalid webhook_on")
	}
}

func TestShouldNotify(t *testing.T) {
	if !ShouldNotify("failure", "failed") || ShouldNotify("failure", "success") {
		t.Fatal("unexpected failure mode behavior")
	}
	if ShouldNotify("off", "failed") || ShouldNotify("off", "success") {
		t.Fatal("off mode should never notify")
	}
	for _, st := range []string{"success", "failed", "cancelled", "skipped"} {
		if !ShouldNotify("always", st) {
			t.Fatalf("always mode should notify on %s", st)
		}
	}
	if ShouldNotify("always", "running") {
		t.Fatal("always mode should not notify while running")
	}
}

func TestBuildRunPayloadAndSend(t *testing.T) {
	run := domain.JobRun{
		ID:               "run-1",
		JobID:            "job-1",
		JobName:          "Photos",
		Trigger:          "schedule",
		Status:           "failed",
		DurationMs:       1234,
		BytesTransferred: 2048,
		FilesTransferred: 3,
		ErrorsCount:      1,
		ErrorMessage:     "permission denied",
	}
	payload := BuildRunPayload(run)
	if payload["event"] != "run_completed" || payload["status"] != "failed" {
		t.Fatalf("unexpected payload metadata: %+v", payload)
	}
	if payload["text"] == "" || payload["content"] == "" || payload["message"] == "" {
		t.Fatal("expected Slack text, Discord content, and generic message fields")
	}

	var capturedBody map[string]any
	var capturedHeader string
	n := NewNotifier(slog.New(slog.NewTextHandler(io.Discard, nil)))
	n.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedHeader = req.Header.Get("Content-Type")
			raw, _ := io.ReadAll(req.Body)
			_ = json.Unmarshal(raw, &capturedBody)
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	if err := n.SendTest(context.Background(), "https://hooks.example.com/secret"); err != nil {
		t.Fatalf("SendTest failed: %v", err)
	}
	if capturedHeader != "application/json" || capturedBody["event"] != "test" {
		t.Fatalf("unexpected request: header=%q body=%+v", capturedHeader, capturedBody)
	}

	// Non-2xx returns error cleanly.
	n.client = &http.Client{
		Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(bytes.NewBufferString("bad gateway")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	if err := n.Send(context.Background(), "https://hooks.example.com/secret", payload); err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("expected HTTP 502 error, got %v", err)
	}
}

func TestNotifyProvidersAndOverrides(t *testing.T) {
	if got := EffectiveWebhookOn("failure", ""); got != "failure" {
		t.Fatalf("expected global fallback 'failure', got %q", got)
	}
	if got := EffectiveWebhookOn("failure", "always"); got != "always" {
		t.Fatalf("expected override 'always', got %q", got)
	}
	if got := EffectiveWebhookOn("always", "off"); got != "off" {
		t.Fatalf("expected override 'off', got %q", got)
	}

	n := NewNotifier(slog.New(slog.NewTextHandler(io.Discard, nil)))
	var gotURL, gotCT, gotTitle, gotPriority, gotTags, gotBody string
	n.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotURL = req.URL.String()
			gotCT = req.Header.Get("Content-Type")
			gotTitle = req.Header.Get("Title")
			gotPriority = req.Header.Get("Priority")
			gotTags = req.Header.Get("Tags")
			raw, _ := io.ReadAll(req.Body)
			gotBody = string(raw)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	// Ntfy failed run uses high priority + rotating_light tag + plain text.
	n.NotifyRun(context.Background(), domain.GlobalSettings{
		WebhookURL:  "https://ntfy.sh/my-topic",
		WebhookOn:   "failure",
		WebhookType: "ntfy",
	}, domain.JobRun{ID: "r1", JobName: "Photos", Status: "failed", Trigger: "schedule"})
	if !strings.HasPrefix(gotCT, "text/plain") || gotPriority != "high" || gotTags != "rotating_light" || gotTitle == "" {
		t.Fatalf("unexpected ntfy headers: ct=%q title=%q prio=%q tags=%q", gotCT, gotTitle, gotPriority, gotTags)
	}

	// Telegram posts {"chat_id","text"} and validates chat ID & bot token.
	if HasDeliveryTarget(domain.GlobalSettings{WebhookType: "telegram", TelegramBotToken: "123:ABC"}) {
		t.Fatal("expected HasDeliveryTarget=false when telegram_chat_id is missing")
	}
	if !HasDeliveryTarget(domain.GlobalSettings{WebhookType: "telegram", TelegramBotToken: "123:ABC", TelegramChatID: "-100123456"}) {
		t.Fatal("expected HasDeliveryTarget=true with bot token and chat ID")
	}
	if err := n.SendTestSettings(context.Background(), domain.GlobalSettings{
		WebhookURL:  "https://api.telegram.org/botTOKEN/sendMessage",
		WebhookType: "telegram",
	}); err == nil {
		t.Fatal("expected error when telegram_chat_id is empty")
	}
	if err := n.SendTestSettings(context.Background(), domain.GlobalSettings{
		WebhookType:      "telegram",
		TelegramBotToken: "123:bad token/slash",
		TelegramChatID:   "-100123456",
	}); err == nil {
		t.Fatal("expected error for malformed telegram bot token")
	}
	if err := n.SendTestSettings(context.Background(), domain.GlobalSettings{
		WebhookType:      "telegram",
		TelegramBotToken: "123456:ABC-DEF",
		TelegramChatID:   "-100123456",
	}); err != nil {
		t.Fatalf("telegram SendTestSettings failed: %v", err)
	}
	if gotURL != "https://api.telegram.org/bot123456:ABC-DEF/sendMessage" {
		t.Fatalf("unexpected resolved telegram URL: %q", gotURL)
	}
	var tg map[string]string
	if err := json.Unmarshal([]byte(gotBody), &tg); err != nil || tg["chat_id"] != "-100123456" || tg["text"] == "" {
		t.Fatalf("unexpected telegram payload: %q err=%v", gotBody, err)
	}
}
