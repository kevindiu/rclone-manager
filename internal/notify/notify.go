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

// Notifier delivers JSON webhook payloads to external endpoints (Discord, Slack, Ntfy, generic HTTP).
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

// BuildRunPayload builds a compatible JSON payload containing Slack ("text"), Discord ("content"),
// and structured run fields.
func BuildRunPayload(run domain.JobRun) map[string]any {
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
	msg := b.String()

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

// Send posts payload as JSON to webhookURL.
func (n *Notifier) Send(ctx context.Context, webhookURL string, payload map[string]any) error {
	cleanURL := strings.TrimSpace(webhookURL)
	if cleanURL == "" {
		return fmt.Errorf("webhook URL is empty")
	}
	if err := ValidateWebhookURL(cleanURL); err != nil {
		return err
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cleanURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "rclone-manager")

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

// SendTest sends a verification message to webhookURL.
func (n *Notifier) SendTest(ctx context.Context, webhookURL string) error {
	msg := "rclone-manager test notification — webhook delivery is working."
	return n.Send(ctx, webhookURL, map[string]any{
		"event":     "test",
		"title":     "[rclone-manager] Test Notification",
		"message":   msg,
		"text":      msg,
		"content":   msg,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// NotifyRun delivers a run notification when configured in settings and warns (without logging URL secrets) on failure.
func (n *Notifier) NotifyRun(ctx context.Context, settings domain.GlobalSettings, run domain.JobRun) {
	if strings.TrimSpace(settings.WebhookURL) == "" || !ShouldNotify(settings.WebhookOn, run.Status) {
		return
	}
	if err := n.Send(ctx, settings.WebhookURL, BuildRunPayload(run)); err != nil {
		host := "unknown"
		if u, parseErr := url.Parse(strings.TrimSpace(settings.WebhookURL)); parseErr == nil && u.Host != "" {
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
