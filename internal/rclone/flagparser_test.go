package rclone

import (
	"reflect"
	"testing"
)

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "empty",
			input: "   \t\n ",
			want:  nil,
		},
		{
			name:  "simple flags",
			input: "--fast-list --drive-chunk-size 64M --tpslimit=10",
			want:  []string{"--fast-list", "--drive-chunk-size", "64M", "--tpslimit=10"},
		},
		{
			name:  "quoted values with spaces",
			input: `--header "X-Custom: hello world" --exclude 'dir with spaces/**' --comment ""`,
			want:  []string{"--header", "X-Custom: hello world", "--exclude", "dir with spaces/**", "--comment", ""},
		},
		{
			name:  "escaped characters",
			input: `--filter "+\ /my\ docs/**" --label foo\ bar`,
			want:  []string{"--filter", `+ /my docs/**`, "--label", "foo bar"},
		},
		{
			name:    "unclosed double quote",
			input:   `--header "unclosed`,
			wantErr: true,
		},
		{
			name:    "unclosed single quote",
			input:   `--header 'unclosed`,
			wantErr: true,
		},
		{
			name:    "trailing backslash",
			input:   `--fast-list \`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseFlags(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (tokens=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateCustomFlags(t *testing.T) {
	if _, err := ParseAndValidateFlags(`--fast-list --transfers 8 --filter "- *.tmp"`); err != nil {
		t.Fatalf("unexpected error for valid flags: %v", err)
	}
	for _, bad := range []string{
		`--config /etc/passwd`,
		`--config=/tmp/other.conf`,
		`--use-json-log`,
		`--rc-addr=0.0.0.0:5572`,
		`--log-file /tmp/foo`,
		`--dry-run=false`,
		`-n`,
		`-nv`,
		`--password-command "id"`,
		`--dump=headers`,
		`--dump-bodies`,
		`--backup-dir=/tmp/evil`,
		`--ask-password`,
	} {
		if _, err := ParseAndValidateFlags(bad); err == nil {
			t.Fatalf("expected error for blocked flag %q", bad)
		}
	}
}
