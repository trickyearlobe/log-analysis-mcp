package tools

import (
	"strings"
	"testing"
)

func TestRunCountByLevel(t *testing.T) {
	structuredLog := strings.Join([]string{
		"2025-01-15T10:00:01Z INFO [app] Starting up",
		"2025-01-15T10:00:02Z INFO [app] Connected",
		"2025-01-15T10:00:03Z WARN [db] Slow query",
		"2025-01-15T10:00:04Z ERROR [db] Connection lost",
		"2025-01-15T10:00:05Z ERROR [db] Retry failed",
		"2025-01-15T10:00:06Z INFO [app] Recovered",
		"2025-01-15T10:00:07Z DEBUG [app] Heartbeat",
	}, "\n") + "\n"
	structuredPath := writeTempLog(t, "structured.log", structuredLog)

	freeformLog := strings.Join([]string{
		"some random line",
		"INFO starting service",
		"ERROR something broke",
		"another random line",
		"WARN low disk space",
		"FATAL out of memory",
	}, "\n") + "\n"
	freeformPath := writeTempLog(t, "freeform.log", freeformLog)

	emptyPath := writeTempLog(t, "empty.log", "")

	tests := []struct {
		name        string
		input       CountByLevelInput
		wantErr     bool
		errContains string
		checkOutput func(t *testing.T, out CountByLevelOutput)
	}{
		{
			name:  "structured log counts",
			input: CountByLevelInput{Path: structuredPath},
			checkOutput: func(t *testing.T, out CountByLevelOutput) {
				if out.TotalLines != 7 {
					t.Errorf("TotalLines = %d, want 7", out.TotalLines)
				}
				if out.ParsedLines != 7 {
					t.Errorf("ParsedLines = %d, want 7", out.ParsedLines)
				}
				if out.Counts["INFO"] != 3 {
					t.Errorf("INFO = %d, want 3", out.Counts["INFO"])
				}
				if out.Counts["WARN"] != 1 {
					t.Errorf("WARN = %d, want 1", out.Counts["WARN"])
				}
				if out.Counts["ERROR"] != 2 {
					t.Errorf("ERROR = %d, want 2", out.Counts["ERROR"])
				}
				if out.Counts["DEBUG"] != 1 {
					t.Errorf("DEBUG = %d, want 1", out.Counts["DEBUG"])
				}
			},
		},
		{
			name:  "freeform log with keyword inference",
			input: CountByLevelInput{Path: freeformPath},
			checkOutput: func(t *testing.T, out CountByLevelOutput) {
				if out.TotalLines != 6 {
					t.Errorf("TotalLines = %d, want 6", out.TotalLines)
				}
				if out.ParsedLines != 4 {
					t.Errorf("ParsedLines = %d, want 4 (INFO, ERROR, WARN, FATAL)", out.ParsedLines)
				}
				if out.Counts["INFO"] != 1 {
					t.Errorf("INFO = %d, want 1", out.Counts["INFO"])
				}
				if out.Counts["ERROR"] != 1 {
					t.Errorf("ERROR = %d, want 1", out.Counts["ERROR"])
				}
				if out.Counts["WARN"] != 1 {
					t.Errorf("WARN = %d, want 1", out.Counts["WARN"])
				}
				if out.Counts["FATAL"] != 1 {
					t.Errorf("FATAL = %d, want 1", out.Counts["FATAL"])
				}
			},
		},
		{
			name:  "empty file",
			input: CountByLevelInput{Path: emptyPath},
			checkOutput: func(t *testing.T, out CountByLevelOutput) {
				if out.TotalLines != 0 {
					t.Errorf("TotalLines = %d, want 0", out.TotalLines)
				}
				if out.ParsedLines != 0 {
					t.Errorf("ParsedLines = %d, want 0", out.ParsedLines)
				}
				if len(out.Counts) != 0 {
					t.Errorf("len(Counts) = %d, want 0", len(out.Counts))
				}
			},
		},
		{
			name:        "file not found",
			input:       CountByLevelInput{Path: "/nonexistent/file.log"},
			wantErr:     true,
			errContains: "FILE_NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := RunCountByLevel(tt.input)
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
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.checkOutput != nil {
				tt.checkOutput(t, out)
			}
		})
	}
}
