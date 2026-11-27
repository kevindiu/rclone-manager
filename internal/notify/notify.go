package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kevindiu/rclone-manager/internal/domain"
)

// ValidateWebhookURL verifies that raw is either empty or an HTTP(S) URL with a valid host.
func ValidateWebhookURL(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("invalid webhook URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook URL must use http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("webhook URL must include a host")
	}
	return nil
}

// NormalizeWebhookOn validates and normalizes the notification mode.
func NormalizeWebhookOn(v string) (string, error) {
	clean := strings.ToLower(strings.TrimSpace(v))
	switch clean {
	case "", "failure":
		return "failure", nil
	case "always", "off":
		return clean, nil
	default:
		return "", fmt.Errorf("webhook_on must be failure, always, or off")
	}
}

// NormalizeWebhookType validates and normalizes the notification provider type.
func NormalizeWebhookType(v string) (string, error) {
	clean := strings.ToLower(strings.TrimSpace(v))
	switch clean {
	case "", "generic":
		return "generic", nil
	case "ntfy", "telegram":
		return clean, nil
	default:
		return "", fmt.Errorf("webhook_type must be generic, ntfy, or telegram")
	}
}

// EffectiveWebhookOn resolves a per-job notification override against global settings.
func EffectiveWebhookOn(globalOn, jobOverride string) string {
	clean := strings.ToLower(strings.TrimSpace(jobOverride))
	if clean != "" {
		return clean
	}
	return globalOn
}

// ResolveTelegramEndpoint returns the Telegram Bot sendMessage endpoint URL using either
// TelegramBotToken or a backwards-compatible explicit WebhookURL.
func ResolveTelegramEndpoint(settings domain.GlobalSettings) (string, error) {
	if token := strings.TrimSpace(settings.TelegramBotToken); token != "" {
		if strings.ContainsAny(token, " \t\r\n/?#") {
			return "", fmt.Errorf("telegram_bot_token contains invalid characters")
		}
		return fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token), nil
	}
	if u := strings.TrimSpace(settings.WebhookURL); u != "" {
		return u, nil
	}
	return "", fmt.Errorf("telegram_bot_token is required for Telegram notifications")
}

// HasDeliveryTarget reports whether notification delivery credentials/URL are configured.
func HasDeliveryTarget(settings domain.GlobalSettings) bool {
	wType, err := NormalizeWebhookType(settings.WebhookType)
	if err != nil {
		return false
	}
	if wType == "telegram" {
		hasEndpoint := strings.TrimSpace(settings.TelegramBotToken) != "" || strings.TrimSpace(settings.WebhookURL) != ""
		return hasEndpoint && strings.TrimSpace(settings.TelegramChatID) != ""
	}
	return strings.TrimSpace(settings.WebhookURL) != ""
}

// ShouldNotify returns true if a run with status should emit a webhook under mode.
func ShouldNotify(webhookOn, status string) bool {
	mode := strings.ToLower(strings.TrimSpace(webhookOn))
	switch mode {
	case "off":
		return false
	case "always":
		return status == "success" || status == "failed" || status == "cancelled" || status == "skipped"
	case "", "failure":
		return status == "failed"
	default:
		return false
	}
}

// Notifier delivers webhook payloads to external endpoints (Discord, Slack, Ntfy, Telegram, generic HTTP).
type Notifier struct {
	client *http.Client
	logger *slog.Logger
}

// NewNotifier creates a Notifier with a bounded HTTP timeout.
func NewNotifier(logger *slog.Logger) *Notifier {
	if logger == nil {
		logger = slog.Default()
	}
	return &Notifier{
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
}

func formatRunTitleAndMessage(run domain.JobRun) (string, string) {
	statusLabel := strings.ToUpper(run.Status)
	if run.DryRun {
		statusLabel += " (DRY RUN)"
	}
	title := fmt.Sprintf("[rclone-manager] %s: %s", statusLabel, run.JobName)
	var b strings.Builder
	b.WriteString(title)
	fmt.Fprintf(&b, "\nTrigger: %s | Duration: %dms | Files: %d | Bytes: %d", run.Trigger, run.DurationMs, run.FilesTransferred, run.BytesTransferred)
	if run.ErrorsCount > 0 {
		fmt.Fprintf(&b, " | Errors: %d", run.ErrorsCount)
	}
	if run.ErrorMessage != "" {
		fmt.Fprintf(&b, "\nError: %s", run.ErrorMessage)
	}
	return title, b.String()
}

// BuildRunPayload builds a compatible JSON payload containing Slack ("text"), Discord ("content"),
// and structured run fields.
func BuildRunPayload(run domain.JobRun) map[string]any {
	title, msg := formatRunTitleAndMessage(run)
	return map[string]any{
		"event":             "run_completed",
		"title":             title,
		"message":           msg,
		"text":              msg,
		"content":           msg,
		"run_id":            run.ID,
		"job_id":            run.JobID,
		"job_name":          run.JobName,
		"trigger":           run.Trigger,
		"dry_run":           run.DryRun,
		"status":            run.Status,
		"duration_ms":       run.DurationMs,
		"bytes_transferred": run.BytesTransferred,
		"files_transferred": run.FilesTransferred,
		"errors_count":      run.ErrorsCount,
		"error_message":     run.ErrorMessage,
		"timestamp":         time.Now().UTC().Format(time.RFC3339),
	}
}

func (n *Notifier) doRequest(ctx context.Context, webhookURL, contentType string, body []byte, extraHeaders map[string]string) error {
	cleanURL := strings.TrimSpace(webhookURL)
	if cleanURL == "" {
		return fmt.Errorf("webhook URL is empty")
	}
	if err := ValidateWebhookURL(cleanURL); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cleanURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "rclone-manager")
	for k, v := range extraHeaders {
		if v != "" {
			req.Header.Set(k, v)
		}
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		msg := strings.TrimSpace(string(snippet))
		if msg != "" {
			return fmt.Errorf("webhook returned HTTP %d: %s", resp.StatusCode, msg)
		}
		return fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// Send posts payload as JSON to webhookURL.
func (n *Notifier) Send(ctx context.Context, webhookURL string, payload map[string]any) error {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return n.doRequest(ctx, webhookURL, "application/json", bodyBytes, nil)
}

func (n *Notifier) sendFormatted(ctx context.Context, settings domain.GlobalSettings, title, message, status string, genericPayload map[string]any) error {
	wType, err := NormalizeWebhookType(settings.WebhookType)
	if err != nil {
		return err
	}
	switch wType {
	case "ntfy":
		priority := "default"
		tags := "bell"
		if status == "failed" {
			priority = "high"
			tags = "rotating_light"
		} else if status == "success" {
			tags = "white_check_mark"
		}
		return n.doRequest(ctx, settings.WebhookURL, "text/plain; charset=utf-8", []byte(message), map[string]string{
			"Title":    title,
			"Priority": priority,
			"Tags":     tags,
		})
	case "telegram":
		chatID := strings.TrimSpace(settings.TelegramChatID)
		if chatID == "" {
			return fmt.Errorf("telegram_chat_id is required for Telegram notifications")
		}
		endpoint, err := ResolveTelegramEndpoint(settings)
		if err != nil {
			return err
		}
		body, err := json.Marshal(map[string]string{
			"chat_id": chatID,
			"text":    message,
		})
		if err != nil {
			return err
		}
		return n.doRequest(ctx, endpoint, "application/json", body, nil)
	default:
		return n.Send(ctx, settings.WebhookURL, genericPayload)
	}
}

// SendTest sends a generic verification message to webhookURL.
func (n *Notifier) SendTest(ctx context.Context, webhookURL string) error {
	return n.SendTestSettings(ctx, domain.GlobalSettings{
		WebhookURL:  webhookURL,
		WebhookType: "generic",
	})
}

// SendTestSettings sends a verification message formatted according to settings.WebhookType.
func (n *Notifier) SendTestSettings(ctx context.Context, settings domain.GlobalSettings) error {
	title := "[rclone-manager] Test Notification"
	msg := "rclone-manager test notification — webhook delivery is working."
	payload := map[string]any{
		"event":     "test",
		"title":     title,
		"message":   msg,
		"text":      msg,
		"content":   msg,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	return n.sendFormatted(ctx, settings, title, msg, "test", payload)
}

// NotifyRun delivers a run notification when configured in settings and warns (without logging URL secrets) on failure.
func (n *Notifier) NotifyRun(ctx context.Context, settings domain.GlobalSettings, run domain.JobRun) {
	if !HasDeliveryTarget(settings) || !ShouldNotify(settings.WebhookOn, run.Status) {
		return
	}
	title, msg := formatRunTitleAndMessage(run)
	if err := n.sendFormatted(ctx, settings, title, msg, run.Status, BuildRunPayload(run)); err != nil {
		host := "unknown"
		if strings.EqualFold(strings.TrimSpace(settings.WebhookType), "telegram") && strings.TrimSpace(settings.TelegramBotToken) != "" {
			host = "api.telegram.org"
		} else if u, parseErr := url.Parse(strings.TrimSpace(settings.WebhookURL)); parseErr == nil && u.Host != "" {
			host = u.Host
		}
		n.logger.Warn("webhook notification delivery failed",
			"run_id", run.ID,
			"job_id", run.JobID,
			"status", run.Status,
			"host", host,
			"error", err,
		)
	}
}
