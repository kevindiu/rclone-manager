package rclone

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var paramKeyRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

// ValidateParamKey rejects option keys that could be interpreted as CLI flags or contain invalid characters.
func ValidateParamKey(key string) error {
	if !paramKeyRe.MatchString(key) {
		return fmt.Errorf("invalid remote option key %q", key)
	}
	return nil
}

func (c *Client) dumpRaw(ctx context.Context) (map[string]map[string]string, error) {
	if _, err := os.Stat(c.confPath); errors.Is(err, os.ErrNotExist) {
		return map[string]map[string]string{}, nil
	}
	cmd := exec.CommandContext(ctx, c.bin, "--config", c.confPath, "config", "dump")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("rclone config dump failed: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	if len(bytes.TrimSpace(stdout.Bytes())) == 0 {
		return map[string]map[string]string{}, nil
	}
	var dump map[string]map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &dump); err != nil {
		return nil, fmt.Errorf("parse config dump: %w", err)
	}
	if dump == nil {
		dump = map[string]map[string]string{}
	}
	return dump, nil
}

// ListRemotes lists all configured remotes from rclone.conf with secret fields masked.
func (c *Client) ListRemotes(ctx context.Context) ([]RemoteInfo, error) {
	c.confMu.Lock()
	dump, err := c.dumpRaw(ctx)
	c.confMu.Unlock()
	if err != nil {
		return nil, err
	}

	provs := c.providerMap(ctx)
	remotes := make([]RemoteInfo, 0, len(dump))
	for name, cfg := range dump {
		rType := cfg["type"]
		var opts map[string]ProviderOption
		if p, ok := provs[rType]; ok {
			opts = providerOptsMap(p)
		}
		masked, maskedKeys := MaskRemoteConfig(cfg, opts)
		remotes = append(remotes, RemoteInfo{
			Name:        name,
			Type:        rType,
			Description: cfg["description"],
			Config:      masked,
			MaskedKeys:  maskedKeys,
		})
	}
	sort.Slice(remotes, func(i, j int) bool {
		return strings.ToLower(remotes[i].Name) < strings.ToLower(remotes[j].Name)
	})
	return remotes, nil
}

// CreateRemote creates a new remote via `rclone config create` so passwords/tokens are properly obscured.
func (c *Client) CreateRemote(ctx context.Context, name, remoteType string, params map[string]string) error {
	name = strings.TrimSpace(name)
	remoteType = strings.TrimSpace(remoteType)
	if err := ValidateRemoteName(name); err != nil {
		return err
	}
	if remoteType == "" {
		return errors.New("remote type is required")
	}

	provs := c.providerMap(ctx)
	prov, ok := provs[remoteType]
	if len(provs) > 0 && !ok {
		return fmt.Errorf("unknown remote provider type %q", remoteType)
	}

	keys := sortedNonEmptyKeys(params, false)
	for _, k := range keys {
		if k == "type" {
			continue
		}
		if err := ValidateParamKey(k); err != nil {
			return err
		}
	}

	c.confMu.Lock()
	defer c.confMu.Unlock()

	existing, err := c.dumpRaw(ctx)
	if err != nil {
		return err
	}
	if _, exists := existing[name]; exists {
		return fmt.Errorf("%w: %q", ErrRemoteExists, name)
	}

	args := []string{"--config", c.confPath, "config", "create", name, remoteType}
	for _, k := range keys {
		if k == "type" {
			continue
		}
		args = append(args, k+"="+params[k])
	}
	if prov.IsOAuth {
		args = append(args, "config_refresh_token=false")
	}
	args = append(args, "--obscure", "--non-interactive")

	cmd := exec.CommandContext(ctx, c.bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rclone config create failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	_ = os.Chmod(c.confPath, 0o600)
	return nil
}

// UpdateRemote updates an existing remote via `rclone config update`, preserving masked secrets.
func (c *Client) UpdateRemote(ctx context.Context, name string, params map[string]string) error {
	name = strings.TrimSpace(name)
	if err := ValidateRemoteName(name); err != nil {
		return err
	}

	keys := sortedNonEmptyKeys(params, true)
	for _, k := range keys {
		if k == "type" {
			continue
		}
		if err := ValidateParamKey(k); err != nil {
			return err
		}
	}

	provs := c.providerMap(ctx)

	c.confMu.Lock()
	defer c.confMu.Unlock()

	existing, err := c.dumpRaw(ctx)
	if err != nil {
		return err
	}
	curCfg, exists := existing[name]
	if !exists {
		return fmt.Errorf("%w: %q", ErrRemoteNotFound, name)
	}
	prov := provs[curCfg["type"]]

	args := []string{"--config", c.confPath, "config", "update", name}
	hasUpdates := false
	for _, k := range keys {
		if k == "type" {
			continue
		}
		v := params[k]
		if v == MaskedSecret {
			continue
		}
		hasUpdates = true
		args = append(args, k+"="+v)
	}
	if !hasUpdates {
		return nil
	}
	if prov.IsOAuth {
		args = append(args, "config_refresh_token=false")
	}
	args = append(args, "--obscure", "--non-interactive")

	cmd := exec.CommandContext(ctx, c.bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rclone config update failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	_ = os.Chmod(c.confPath, 0o600)
	return nil
}

// DeleteRemote removes a remote from rclone.conf via `rclone config delete`.
func (c *Client) DeleteRemote(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if err := ValidateRemoteName(name); err != nil {
		return err
	}

	c.confMu.Lock()
	defer c.confMu.Unlock()

	existing, err := c.dumpRaw(ctx)
	if err != nil {
		return err
	}
	if _, exists := existing[name]; !exists {
		return fmt.Errorf("%w: %q", ErrRemoteNotFound, name)
	}

	cmd := exec.CommandContext(ctx, c.bin, "--config", c.confPath, "config", "delete", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rclone config delete failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	_ = os.Chmod(c.confPath, 0o600)
	return nil
}

// ImportConf validates and merges or replaces rclone.conf from raw INI content.
func (c *Client) ImportConf(ctx context.Context, content, mode string) ([]string, error) {
	c.confMu.Lock()
	defer c.confMu.Unlock()
	return MergeOrValidateINI(ctx, c.bin, c.confPath, content, mode)
}

// ClearConf removes rclone.conf if it exists (used when replacing with an empty backup).
func (c *Client) ClearConf(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.confMu.Lock()
	defer c.confMu.Unlock()
	if err := os.Remove(c.confPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// ReadRawConf returns the raw rclone.conf INI contents, or empty string if the file does not exist yet.
func (c *Client) ReadRawConf(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	c.confMu.Lock()
	defer c.confMu.Unlock()

	raw, err := os.ReadFile(c.confPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// TestRemoteConfig tests connectivity for an unsaved or edited remote configuration
// inside a temporary isolated copy of rclone.conf without mutating persisted config.
func (c *Client) TestRemoteConfig(ctx context.Context, name, remoteType string, params map[string]string) (RemoteTestResult, error) {
	remoteType = strings.TrimSpace(remoteType)
	if remoteType == "" {
		return RemoteTestResult{}, errors.New("remote type is required")
	}

	provs := c.providerMap(ctx)
	if len(provs) > 0 {
		if _, ok := provs[remoteType]; !ok {
			return RemoteTestResult{}, fmt.Errorf("unknown remote provider type %q", remoteType)
		}
	}

	for k := range params {
		if k == "type" {
			continue
		}
		if err := ValidateParamKey(k); err != nil {
			return RemoteTestResult{}, err
		}
	}

	tmpDir, err := os.MkdirTemp("", "rclone-test-draft-*")
	if err != nil {
		return RemoteTestResult{}, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tmpConf := filepath.Join(tmpDir, "rclone.conf")
	c.confMu.Lock()
	rawConf, readErr := os.ReadFile(c.confPath)
	c.confMu.Unlock()
	if readErr == nil && len(rawConf) > 0 {
		if writeErr := os.WriteFile(tmpConf, rawConf, 0o600); writeErr != nil {
			return RemoteTestResult{}, writeErr
		}
	}

	cleanName := strings.TrimSpace(name)
	displayName := cleanName
	if displayName == "" {
		displayName = remoteType
	}
	testName := cleanName
	if err := ValidateRemoteName(testName); err != nil {
		testName = "__draft_test__"
	}

	tmpClient := NewClient(c.bin, tmpConf)
	existing, err := tmpClient.dumpRaw(ctx)
	if err != nil {
		return RemoteTestResult{}, err
	}
	if cur, exists := existing[testName]; exists {
		if cur["type"] == remoteType {
			if err := tmpClient.UpdateRemote(ctx, testName, params); err != nil {
				return RemoteTestResult{}, err
			}
		} else {
			if err := tmpClient.DeleteRemote(ctx, testName); err != nil {
				return RemoteTestResult{}, err
			}
			if err := tmpClient.CreateRemote(ctx, testName, remoteType, params); err != nil {
				return RemoteTestResult{}, err
			}
		}
	} else {
		if err := tmpClient.CreateRemote(ctx, testName, remoteType, params); err != nil {
			return RemoteTestResult{}, err
		}
	}

	return testRemoteWithConf(ctx, c.bin, tmpConf, testName, displayName), nil
}

func (c *Client) ensureRemoteExists(ctx context.Context, name string) error {
	c.confMu.Lock()
	dump, err := c.dumpRaw(ctx)
	c.confMu.Unlock()
	if err != nil {
		return err
	}
	if _, ok := dump[name]; !ok {
		return fmt.Errorf("%w: %q", ErrRemoteNotFound, name)
	}
	return nil
}

func sortedNonEmptyKeys(m map[string]string, includeEmpty bool) []string {
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if !includeEmpty && v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
