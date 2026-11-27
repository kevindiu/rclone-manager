package rclone

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// MaskedSecret is the sentinel shown in UI/API responses for sensitive remote options.
const MaskedSecret = "••••••••"

var remoteNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _.-]*$`)

var fallbackSecretKeys = map[string]bool{
	"password":                    true,
	"password2":                   true,
	"pass":                        true,
	"secret":                      true,
	"token":                       true,
	"key":                         true,
	"passphrase":                  true,
	"secret_access_key":           true,
	"access_key_id":               true,
	"session_token":               true,
	"client_secret":               true,
	"service_account_credentials": true,
	"key_pem":                     true,
	"key_file_pass":               true,
	"bearer_token":                true,
	"sas_url":                     true,
	"account_key":                 true,
}

// ValidateRemoteName checks if a remote name is valid for rclone.conf.
func ValidateRemoteName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return errors.New("remote name is required")
	}
	if !remoteNameRe.MatchString(trimmed) || strings.Contains(trimmed, "..") {
		return fmt.Errorf("invalid remote name %q", name)
	}
	return nil
}

// IsSecretKey determines whether an rclone config key should be masked in UI responses.
func IsSecretKey(key string, providerOpts map[string]ProviderOption) bool {
	if opt, ok := providerOpts[key]; ok {
		if opt.IsPassword || opt.Sensitive {
			return true
		}
	}
	lower := strings.ToLower(key)
	if fallbackSecretKeys[lower] {
		return true
	}
	for _, needle := range []string{"password", "secret", "token", "passphrase", "private_key", "credential"} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

// MaskRemoteConfig returns a copy of cfg with secret values replaced by MaskedSecret,
// along with a sorted list of masked keys.
func MaskRemoteConfig(cfg map[string]string, providerOpts map[string]ProviderOption) (map[string]string, []string) {
	masked := make(map[string]string, len(cfg))
	var maskedKeys []string
	for k, v := range cfg {
		if k == "type" {
			masked[k] = v
			continue
		}
		if v != "" && IsSecretKey(k, providerOpts) {
			masked[k] = MaskedSecret
			maskedKeys = append(maskedKeys, k)
		} else {
			masked[k] = v
		}
	}
	sort.Strings(maskedKeys)
	return masked, maskedKeys
}

// INISection represents a single [remote] block in rclone.conf.
type INISection struct {
	Name   string
	Values []INIEntry
}

// INIEntry is a key-value entry in an INI section.
type INIEntry struct {
	Key   string
	Value string
}

// ParseINI parses rclone.conf INI text while validating section headers and key=value lines.
func ParseINI(content string) ([]INISection, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var sections []INISection
	var current *INISection
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(line[1 : len(line)-1])
			if err := ValidateRemoteName(name); err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNo, err)
			}
			sections = append(sections, INISection{Name: name})
			current = &sections[len(sections)-1]
			continue
		}
		if current == nil {
			return nil, fmt.Errorf("line %d: key-value pair outside any [remote] section", lineNo)
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			return nil, fmt.Errorf("line %d: expected key = value", lineNo)
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			return nil, fmt.Errorf("line %d: empty key", lineNo)
		}
		current.Values = append(current.Values, INIEntry{Key: key, Value: val})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(sections) == 0 {
		return nil, errors.New("no valid [remote] sections found")
	}
	for _, s := range sections {
		hasType := false
		for _, kv := range s.Values {
			if kv.Key == "type" && kv.Value != "" {
				hasType = true
				break
			}
		}
		if !hasType {
			return nil, fmt.Errorf("section [%s] is missing required \"type\" field", s.Name)
		}
	}
	return sections, nil
}

// RenderINI renders INI sections back to rclone.conf format.
func RenderINI(sections []INISection) string {
	var buf bytes.Buffer
	for i, s := range sections {
		if i > 0 {
			buf.WriteByte('\n')
		}
		fmt.Fprintf(&buf, "[%s]\n", s.Name)
		for _, kv := range s.Values {
			fmt.Fprintf(&buf, "%s = %s\n", kv.Key, kv.Value)
		}
	}
	return buf.String()
}

// MergeOrValidateINI parses incoming rclone.conf text, merges or replaces existingConfPath,
// validates the resulting file via `rclone config dump`, and atomically writes it with 0600 permissions.
func MergeOrValidateINI(ctx context.Context, rcloneBin, existingConfPath, incomingContent, mode string) ([]string, error) {
	if mode != "merge" && mode != "replace" {
		return nil, fmt.Errorf("invalid import mode %q: expected merge or replace", mode)
	}

	incomingSections, err := ParseINI(incomingContent)
	if err != nil {
		return nil, err
	}

	var finalSections []INISection
	if mode == "merge" {
		if rawExisting, err := os.ReadFile(existingConfPath); err == nil && len(bytes.TrimSpace(rawExisting)) > 0 {
			existingSections, parseErr := ParseINI(string(rawExisting))
			if parseErr == nil {
				incomingByName := make(map[string]INISection, len(incomingSections))
				for _, s := range incomingSections {
					incomingByName[s.Name] = s
				}
				seen := make(map[string]bool, len(incomingSections))
				for _, s := range existingSections {
					if repl, ok := incomingByName[s.Name]; ok {
						finalSections = append(finalSections, repl)
						seen[s.Name] = true
					} else {
						finalSections = append(finalSections, s)
					}
				}
				for _, s := range incomingSections {
					if !seen[s.Name] {
						finalSections = append(finalSections, s)
					}
				}
			} else {
				finalSections = incomingSections
			}
		} else {
			finalSections = incomingSections
		}
	} else {
		finalSections = incomingSections
	}

	rendered := RenderINI(finalSections)
	dir := filepath.Dir(existingConfPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	tmpFile, err := os.CreateTemp(dir, ".rclone.conf.tmp-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmpFile.Chmod(0o600); err != nil {
		_ = tmpFile.Close()
		return nil, err
	}
	if _, err := tmpFile.WriteString(rendered); err != nil {
		_ = tmpFile.Close()
		return nil, err
	}
	if err := tmpFile.Close(); err != nil {
		return nil, err
	}

	// Validate via rclone itself before committing.
	cmd := exec.CommandContext(ctx, rcloneBin, "--config", tmpPath, "config", "dump")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("rclone rejected config: %v (%s)", err, strings.TrimSpace(string(out)))
	}

	if err := os.Rename(tmpPath, existingConfPath); err != nil {
		return nil, err
	}
	_ = os.Chmod(existingConfPath, 0o600)

	names := make([]string, 0, len(incomingSections))
	for _, s := range incomingSections {
		names = append(names, s.Name)
	}
	return names, nil
}
