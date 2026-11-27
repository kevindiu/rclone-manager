package rclone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Client wraps invocations to the bundled rclone CLI.
type Client struct {
	bin      string
	confPath string
	confMu   sync.Mutex

	verMu     sync.Mutex
	verCached bool
	version   string
	osArch    string

	provMu    sync.Mutex
	providers []ProviderInfo
}

// NewClient constructs a new rclone CLI client.
func NewClient(bin, confPath string) *Client {
	if bin == "" {
		bin = "rclone"
	}
	return &Client{
		bin:      bin,
		confPath: confPath,
	}
}

// Bin returns the configured rclone executable path.
func (c *Client) Bin() string {
	return c.bin
}

// ConfPath returns the managed rclone.conf path.
func (c *Client) ConfPath() string {
	return c.confPath
}

// Version returns the installed rclone version metadata (cached after first successful call).
func (c *Client) Version(ctx context.Context) (version string, osArch string, err error) {
	c.verMu.Lock()
	defer c.verMu.Unlock()

	if c.verCached {
		return c.version, c.osArch, nil
	}

	cmd := exec.CommandContext(ctx, c.bin, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("rclone version failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 0 {
		fields := strings.Fields(lines[0])
		if len(fields) >= 2 {
			version = fields[1]
		} else {
			version = strings.TrimSpace(lines[0])
		}
	}
	var osPart, archPart string
	for _, l := range lines[1:] {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "- os/version:") {
			osPart = strings.TrimSpace(strings.TrimPrefix(l, "- os/version:"))
		}
		if strings.HasPrefix(l, "- os/arch:") {
			archPart = strings.TrimSpace(strings.TrimPrefix(l, "- os/arch:"))
		}
	}
	if osPart != "" && archPart != "" {
		osArch = osPart + " (" + archPart + ")"
	} else if archPart != "" {
		osArch = archPart
	}

	c.version = version
	c.osArch = osArch
	c.verCached = true
	return version, osArch, nil
}

// Providers returns all supported rclone providers and their option schemas (cached).
func (c *Client) Providers(ctx context.Context) ([]ProviderInfo, error) {
	c.provMu.Lock()
	defer c.provMu.Unlock()

	if len(c.providers) > 0 {
		cached := make([]ProviderInfo, len(c.providers))
		copy(cached, c.providers)
		return cached, nil
	}

	cmd := exec.CommandContext(ctx, c.bin, "config", "providers")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("rclone config providers failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	var parsed []ProviderInfo
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return nil, fmt.Errorf("parse rclone providers: %w", err)
	}
	for i := range parsed {
		for _, opt := range parsed[i].Options {
			if opt.Name == "token" {
				parsed[i].IsOAuth = true
				break
			}
		}
	}

	c.providers = parsed

	out := make([]ProviderInfo, len(parsed))
	copy(out, parsed)
	return out, nil
}

func (c *Client) providerMap(ctx context.Context) map[string]ProviderInfo {
	provs, err := c.Providers(ctx)
	if err != nil {
		return nil
	}
	m := make(map[string]ProviderInfo, len(provs))
	for _, p := range provs {
		m[p.Name] = p
	}
	return m
}

func providerOptsMap(p ProviderInfo) map[string]ProviderOption {
	m := make(map[string]ProviderOption, len(p.Options))
	for _, o := range p.Options {
		m[o.Name] = o
	}
	return m
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
