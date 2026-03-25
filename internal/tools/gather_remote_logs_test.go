package tools

import (
	"strings"
	"testing"
)

func TestRunGatherRemoteLogs_EmptyHosts(t *testing.T) {
	input := GatherRemoteLogsInput{
		Hosts: []string{},
		Paths: []string{"/var/log/syslog"},
	}
	_, err := RunGatherRemoteLogs(input)
	if err == nil {
		t.Fatal("expected error for empty hosts, got nil")
	}
	if !strings.Contains(err.Error(), "hosts") {
		t.Errorf("error should mention hosts, got: %s", err.Error())
	}
}

func TestRunGatherRemoteLogs_NoPathsOrUnits(t *testing.T) {
	input := GatherRemoteLogsInput{
		Hosts: []string{"web1.example.com"},
	}
	_, err := RunGatherRemoteLogs(input)
	if err == nil {
		t.Fatal("expected error when neither paths nor journal_units provided, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "paths") && !strings.Contains(errMsg, "journal_units") {
		t.Errorf("error should mention paths or journal_units, got: %s", errMsg)
	}
}

func TestFlattenPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "absolute path",
			in:   "/var/log/app.log",
			want: "var-log-app.log",
		},
		{
			name: "relative path",
			in:   "var/log/app.log",
			want: "var-log-app.log",
		},
		{
			name: "double slashes",
			in:   "//double//slash",
			want: "double-slash",
		},
		{
			name: "trailing slash",
			in:   "/var/log/",
			want: "var-log",
		},
		{
			name: "single component",
			in:   "messages",
			want: "messages",
		},
		{
			name: "leading slashes only",
			in:   "///",
			want: "",
		},
		{
			name: "deeply nested",
			in:   "/opt/app/logs/2024/01/error.log",
			want: "opt-app-logs-2024-01-error.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flattenPath(tt.in)
			if got != tt.want {
				t.Errorf("flattenPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
