package tools

import (
	"strings"
	"testing"
)

func TestRunRunRemoteCommand(t *testing.T) {
	tests := []struct {
		name        string
		input       RunRemoteCommandInput
		wantErr     bool
		errContains string
	}{
		{
			name: "empty hosts returns error",
			input: RunRemoteCommandInput{
				Hosts:   []string{},
				Command: "uptime",
			},
			wantErr:     true,
			errContains: "hosts",
		},
		{
			name: "nil hosts returns error",
			input: RunRemoteCommandInput{
				Hosts:   nil,
				Command: "uptime",
			},
			wantErr:     true,
			errContains: "hosts",
		},
		{
			name: "empty command returns error",
			input: RunRemoteCommandInput{
				Hosts:   []string{"localhost"},
				Command: "",
			},
			wantErr:     true,
			errContains: "command",
		},
		{
			name: "both empty returns error mentioning hosts",
			input: RunRemoteCommandInput{
				Hosts:   []string{},
				Command: "",
			},
			wantErr:     true,
			errContains: "hosts",
		},
		{
			name: "zero defaults do not panic",
			input: RunRemoteCommandInput{
				Hosts:          []string{"localhost"},
				Command:        "echo hello",
				TimeoutSeconds: 0,
				MaxOutputBytes: 0,
			},
			// This will fail at SSH dial (no real server), but must not panic.
			// We just verify it returns a result with an error per host, not a tool-level error.
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := RunRunRemoteCommand(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected tool-level error: %v", err)
			}
			// For valid inputs that fail at SSH layer, each host should have an error recorded.
			if out.Results == nil {
				t.Fatal("expected non-nil Results slice")
			}
			if len(out.Results) != len(tt.input.Hosts) {
				t.Errorf("expected %d results, got %d", len(tt.input.Hosts), len(out.Results))
			}
			for i, r := range out.Results {
				if r.Host != tt.input.Hosts[i] {
					t.Errorf("result[%d]: expected host %q, got %q", i, tt.input.Hosts[i], r.Host)
				}
				// Without a real SSH server, every host should have an error.
				if r.Error == "" {
					t.Errorf("result[%d]: expected per-host error (no SSH server), got empty", i)
				}
			}
		})
	}
}
