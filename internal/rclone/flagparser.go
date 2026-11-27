package rclone

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var managedFlags = []string{
	"--config",
	"--use-json-log",
	"--rc",
	"--rc-addr",
	"--log-file",
	"--stats-log-level",
	"--dry-run",
	"-n",
	"--password-command",
	"--dump",
	"--dump-bodies",
	"--backup-dir",
	"--ask-password",
}

// ParseFlags tokenizes a CLI flag string into argument tokens using shell-like
// quoting and escaping rules without invoking a shell.
func ParseFlags(input string) ([]string, error) {
	if strings.ContainsRune(input, '\x00') {
		return nil, errors.New("custom flags contain invalid null byte")
	}

	var tokens []string
	var current strings.Builder
	inToken := false
	inSingle := false
	inDouble := false
	escaped := false

	flush := func() {
		if inToken {
			tokens = append(tokens, current.String())
			current.Reset()
			inToken = false
		}
	}

	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			inToken = true
			continue
		}

		if inSingle {
			if r == '\'' {
				inSingle = false
			} else {
				current.WriteRune(r)
			}
			inToken = true
			continue
		}

		if inDouble {
			switch r {
			case '"':
				inDouble = false
			case '\\':
				escaped = true
			default:
				current.WriteRune(r)
			}
			inToken = true
			continue
		}

		switch {
		case unicode.IsSpace(r):
			flush()
		case r == '\'':
			inSingle = true
			inToken = true
		case r == '"':
			inDouble = true
			inToken = true
		case r == '\\':
			escaped = true
			inToken = true
		default:
			current.WriteRune(r)
			inToken = true
		}
	}

	if escaped {
		return nil, errors.New("trailing escape character in custom flags")
	}
	if inSingle || inDouble {
		return nil, errors.New("unclosed quote in custom flags")
	}
	flush()
	return tokens, nil
}

// ValidateCustomFlags ensures user-supplied flags do not override managed internal flags.
func ValidateCustomFlags(tokens []string) error {
	for _, tok := range tokens {
		if strings.ContainsRune(tok, '\x00') {
			return errors.New("custom flag contains null byte")
		}
		lower := strings.ToLower(tok)
		for _, blocked := range managedFlags {
			if lower == blocked || strings.HasPrefix(lower, blocked+"=") {
				return fmt.Errorf("flag %q is managed by rclone-manager and cannot be overridden", blocked)
			}
		}
		// Reject bundled short flags containing -n (e.g. -nv, -vn) while allowing filter rules like "- *.tmp".
		if len(lower) >= 2 && lower[0] == '-' && lower[1] != '-' {
			rest := lower[1:]
			onlyLetters := true
			hasN := false
			for i := 0; i < len(rest); i++ {
				c := rest[i]
				if c < 'a' || c > 'z' {
					onlyLetters = false
					break
				}
				if c == 'n' {
					hasN = true
				}
			}
			if onlyLetters && hasN {
				return fmt.Errorf("flag %q is managed by rclone-manager and cannot be overridden", "-n")
			}
		}
	}
	return nil
}

// ParseAndValidateFlags tokenizes and validates custom rclone flags.
func ParseAndValidateFlags(input string) ([]string, error) {
	tokens, err := ParseFlags(input)
	if err != nil {
		return nil, err
	}
	if err := ValidateCustomFlags(tokens); err != nil {
		return nil, err
	}
	return tokens, nil
}
