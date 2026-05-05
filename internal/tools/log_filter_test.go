package tools

import (
	"strings"
	"testing"
)

func TestRunFilterLogs(t *testing.T) {
	// JSON log lines with timestamps, levels, sources, and messages.
	jsonLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","source":"auth","message":"User login successful"}`,
		`{"timestamp":"2025-01-15T10:05:00Z","level":"WARN","source":"auth","message":"Failed login attempt for admin"}`,
		`{"timestamp":"2025-01-15T10:10:00Z","level":"ERROR","source":"db","message":"Connection refused to primary"}`,
		`{"timestamp":"2025-01-15T10:15:00Z","level":"ERROR","source":"auth","message":"Token validation failed"}`,
		`{"timestamp":"2025-01-15T10:20:00Z","level":"INFO","source":"api","message":"Health check passed"}`,
		`{"timestamp":"2025-01-15T10:25:00Z","level":"WARN","source":"db","message":"Connection pool exhausted"}`,
		`{"timestamp":"2025-01-15T10:30:00Z","level":"DEBUG","source":"api","message":"Request received for /users"}`,
		`{"timestamp":"2025-01-15T10:35:00Z","level":"ERROR","source":"api","message":"Internal server error on /users"}`,
		`{"timestamp":"2025-01-15T10:40:00Z","level":"INFO","source":"scheduler","message":"Cron job completed"}`,
		`{"timestamp":"2025-01-15T10:45:00Z","level":"WARN","source":"auth","message":"Rate limit approaching for IP 10.0.0.1"}`,
	}, "\n") + "\n"

	jsonPath := writeTempLog(t, "filter.log", jsonLog)
	emptyPath := writeTempLog(t, "empty.log", "")

	// Unparseable content — plain text, not matching any known format.
	unparseablePath := writeTempLog(t, "unparseable.log", "just some random text\nanother random line\n")

	// Large file for truncation tests — 20 ERROR lines.
	var truncLines []string
	for i := 0; i < 20; i++ {
		truncLines = append(truncLines, `{"timestamp":"2025-01-15T10:00:00Z","level":"ERROR","source":"app","message":"failure"}`)
	}
	truncPath := writeTempLog(t, "trunc.log", strings.Join(truncLines, "\n")+"\n")

	tests := []struct {
		name        string
		input       FilterLogsInput
		wantErr     bool
		errContains string
		checkOutput func(t *testing.T, out FilterLogsOutput)
	}{
		{
			name: "filter by single level",
			input: FilterLogsInput{
				Path:  jsonPath,
				Level: []string{"ERROR"},
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				if out.TotalMatched != 3 {
					t.Errorf("TotalMatched = %d, want 3", out.TotalMatched)
				}
				for i, e := range out.Entries {
					if e.Level == nil || string(*e.Level) != "ERROR" {
						t.Errorf("Entries[%d].Level = %v, want ERROR", i, e.Level)
					}
				}
			},
		},
		{
			name: "filter by multiple levels",
			input: FilterLogsInput{
				Path:  jsonPath,
				Level: []string{"ERROR", "WARN"},
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				if out.TotalMatched != 6 {
					t.Errorf("TotalMatched = %d, want 6", out.TotalMatched)
				}
				for i, e := range out.Entries {
					if e.Level == nil {
						t.Fatalf("Entries[%d].Level is nil", i)
					}
					lvl := string(*e.Level)
					if lvl != "ERROR" && lvl != "WARN" {
						t.Errorf("Entries[%d].Level = %q, want ERROR or WARN", i, lvl)
					}
				}
			},
		},
		{
			name: "level filter is case insensitive",
			input: FilterLogsInput{
				Path:  jsonPath,
				Level: []string{"error", "warn"},
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				if out.TotalMatched != 6 {
					t.Errorf("TotalMatched = %d, want 6", out.TotalMatched)
				}
			},
		},
		{
			name: "filter by after timestamp",
			input: FilterLogsInput{
				Path:  jsonPath,
				After: "2025-01-15T10:30:00Z",
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				// Lines at 10:30, 10:35, 10:40, 10:45 (>= 10:30)
				if out.TotalMatched != 4 {
					t.Errorf("TotalMatched = %d, want 4", out.TotalMatched)
				}
				for i, e := range out.Entries {
					if e.Timestamp == nil {
						t.Fatalf("Entries[%d].Timestamp is nil", i)
					}
					if *e.Timestamp < "2025-01-15T10:30:00Z" {
						t.Errorf("Entries[%d].Timestamp = %q, want >= 2025-01-15T10:30:00Z", i, *e.Timestamp)
					}
				}
			},
		},
		{
			name: "filter by before timestamp",
			input: FilterLogsInput{
				Path:   jsonPath,
				Before: "2025-01-15T10:10:00Z",
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				// Lines at 10:00, 10:05 (< 10:10)
				if out.TotalMatched != 2 {
					t.Errorf("TotalMatched = %d, want 2", out.TotalMatched)
				}
				for i, e := range out.Entries {
					if e.Timestamp == nil {
						t.Fatalf("Entries[%d].Timestamp is nil", i)
					}
					if *e.Timestamp >= "2025-01-15T10:10:00Z" {
						t.Errorf("Entries[%d].Timestamp = %q, want < 2025-01-15T10:10:00Z", i, *e.Timestamp)
					}
				}
			},
		},
		{
			name: "filter by time range (after and before)",
			input: FilterLogsInput{
				Path:   jsonPath,
				After:  "2025-01-15T10:10:00Z",
				Before: "2025-01-15T10:30:00Z",
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				// Lines at 10:10, 10:15, 10:20, 10:25 (>= 10:10 AND < 10:30)
				if out.TotalMatched != 4 {
					t.Errorf("TotalMatched = %d, want 4", out.TotalMatched)
				}
			},
		},
		{
			name: "filter by source regex",
			input: FilterLogsInput{
				Path:   jsonPath,
				Source: "^auth$",
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				// auth lines: 1, 2, 4, 10
				if out.TotalMatched != 4 {
					t.Errorf("TotalMatched = %d, want 4", out.TotalMatched)
				}
				for i, e := range out.Entries {
					if e.Source == nil || *e.Source != "auth" {
						t.Errorf("Entries[%d].Source = %v, want auth", i, e.Source)
					}
				}
			},
		},
		{
			name: "filter by source regex partial match",
			input: FilterLogsInput{
				Path:   jsonPath,
				Source: "a",
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				// "auth" (4 entries) and "api" (3 entries) contain "a"; "db" and "scheduler" do not
				if out.TotalMatched != 7 {
					t.Errorf("TotalMatched = %d, want 7", out.TotalMatched)
				}
			},
		},
		{
			name: "filter by message pattern regex",
			input: FilterLogsInput{
				Path:           jsonPath,
				MessagePattern: "failed|failure",
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				// "Failed login attempt for admin" (line 2) and "Token validation failed" (line 4)
				// case insensitive so both match
				if out.TotalMatched != 2 {
					t.Errorf("TotalMatched = %d, want 2", out.TotalMatched)
				}
			},
		},
		{
			name: "combined filters AND logic",
			input: FilterLogsInput{
				Path:   jsonPath,
				Level:  []string{"ERROR"},
				Source: "auth",
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				// ERROR + source=auth: only "Token validation failed" (line 4)
				if out.TotalMatched != 1 {
					t.Errorf("TotalMatched = %d, want 1", out.TotalMatched)
				}
				if len(out.Entries) != 1 {
					t.Fatalf("len(Entries) = %d, want 1", len(out.Entries))
				}
				if !strings.Contains(out.Entries[0].Message, "Token validation failed") {
					t.Errorf("Entries[0].Message = %q, want to contain 'Token validation failed'", out.Entries[0].Message)
				}
			},
		},
		{
			name: "combined level time and message filters",
			input: FilterLogsInput{
				Path:           jsonPath,
				Level:          []string{"WARN"},
				After:          "2025-01-15T10:20:00Z",
				MessagePattern: "pool|rate",
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				// WARN + after 10:20 + message contains pool or rate
				// Line 6 WARN db "Connection pool exhausted" at 10:25 -> matches
				// Line 10 WARN auth "Rate limit approaching" at 10:45 -> matches
				if out.TotalMatched != 2 {
					t.Errorf("TotalMatched = %d, want 2", out.TotalMatched)
				}
			},
		},
		{
			name: "no filters returns all parsed entries",
			input: FilterLogsInput{
				Path: jsonPath,
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				if out.TotalMatched != 10 {
					t.Errorf("TotalMatched = %d, want 10", out.TotalMatched)
				}
				if len(out.Entries) != 10 {
					t.Errorf("len(Entries) = %d, want 10", len(out.Entries))
				}
				if out.TotalScanned != 10 {
					t.Errorf("TotalScanned = %d, want 10", out.TotalScanned)
				}
				if out.Truncated {
					t.Error("Truncated should be false")
				}
			},
		},
		{
			name: "no matches returns empty entries",
			input: FilterLogsInput{
				Path:  jsonPath,
				Level: []string{"FATAL"},
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				if out.TotalMatched != 0 {
					t.Errorf("TotalMatched = %d, want 0", out.TotalMatched)
				}
				if len(out.Entries) != 0 {
					t.Errorf("len(Entries) = %d, want 0", len(out.Entries))
				}
				if out.Entries == nil {
					t.Error("Entries should be non-nil empty slice, got nil")
				}
				if out.TotalScanned != 10 {
					t.Errorf("TotalScanned = %d, want 10", out.TotalScanned)
				}
				if out.Truncated {
					t.Error("Truncated should be false")
				}
			},
		},
		{
			name: "max results truncation",
			input: FilterLogsInput{
				Path:       truncPath,
				MaxResults: 5,
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				if len(out.Entries) != 5 {
					t.Errorf("len(Entries) = %d, want 5", len(out.Entries))
				}
				if out.TotalMatched != 20 {
					t.Errorf("TotalMatched = %d, want 20", out.TotalMatched)
				}
				if !out.Truncated {
					t.Error("Truncated should be true")
				}
			},
		},
		{
			name: "max results exact count not truncated",
			input: FilterLogsInput{
				Path:       truncPath,
				MaxResults: 20,
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				if len(out.Entries) != 20 {
					t.Errorf("len(Entries) = %d, want 20", len(out.Entries))
				}
				if out.TotalMatched != 20 {
					t.Errorf("TotalMatched = %d, want 20", out.TotalMatched)
				}
				if out.Truncated {
					t.Error("Truncated should be false when total == collected")
				}
			},
		},
		{
			name: "file not found error",
			input: FilterLogsInput{
				Path: "/nonexistent/path/to/file.log",
			},
			wantErr:     true,
			errContains: "FILE_NOT_FOUND",
		},
		{
			name: "invalid after timestamp error",
			input: FilterLogsInput{
				Path:  jsonPath,
				After: "not-a-timestamp",
			},
			wantErr:     true,
			errContains: "INVALID_TIMESTAMP",
		},
		{
			name: "invalid before timestamp error",
			input: FilterLogsInput{
				Path:   jsonPath,
				Before: "2025/01/15",
			},
			wantErr:     true,
			errContains: "INVALID_TIMESTAMP",
		},
		{
			name: "empty file",
			input: FilterLogsInput{
				Path: emptyPath,
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				if out.TotalMatched != 0 {
					t.Errorf("TotalMatched = %d, want 0", out.TotalMatched)
				}
				if out.TotalScanned != 0 {
					t.Errorf("TotalScanned = %d, want 0", out.TotalScanned)
				}
				if len(out.Entries) != 0 {
					t.Errorf("len(Entries) = %d, want 0", len(out.Entries))
				}
				if out.Entries == nil {
					t.Error("Entries should be non-nil empty slice, got nil")
				}
				if out.Truncated {
					t.Error("Truncated should be false")
				}
			},
		},
		{
			name: "unparseable format returns empty results",
			input: FilterLogsInput{
				Path: unparseablePath,
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				// Parser is nil — no results possible.
				if out.TotalMatched != 0 {
					t.Errorf("TotalMatched = %d, want 0", out.TotalMatched)
				}
				if len(out.Entries) != 0 {
					t.Errorf("len(Entries) = %d, want 0", len(out.Entries))
				}
			},
		},
		{
			name: "defaults applied for max results",
			input: FilterLogsInput{
				Path: jsonPath,
				// MaxResults left as 0 → should default to 100.
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				// File only has 10 lines, all match, no truncation at default 100.
				if out.Truncated {
					t.Error("Truncated should be false with default MaxResults")
				}
				if out.TotalMatched != 10 {
					t.Errorf("TotalMatched = %d, want 10", out.TotalMatched)
				}
			},
		},
		{
			name: "applied filters populated in output",
			input: FilterLogsInput{
				Path:   jsonPath,
				Level:  []string{"ERROR", "WARN"},
				After:  "2025-01-15T10:00:00Z",
				Before: "2025-01-15T11:00:00Z",
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				if len(out.AppliedFilters.Level) != 2 {
					t.Errorf("AppliedFilters.Level len = %d, want 2", len(out.AppliedFilters.Level))
				}
				if out.AppliedFilters.After != "2025-01-15T10:00:00Z" {
					t.Errorf("AppliedFilters.After = %q, want 2025-01-15T10:00:00Z", out.AppliedFilters.After)
				}
				if out.AppliedFilters.Before != "2025-01-15T11:00:00Z" {
					t.Errorf("AppliedFilters.Before = %q, want 2025-01-15T11:00:00Z", out.AppliedFilters.Before)
				}
			},
		},
		{
			name: "entries have correct line numbers",
			input: FilterLogsInput{
				Path:  jsonPath,
				Level: []string{"ERROR"},
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				if len(out.Entries) != 3 {
					t.Fatalf("len(Entries) = %d, want 3", len(out.Entries))
				}
				// ERROR lines are at positions 3, 4, 8 (1-based).
				wantLines := []int{3, 4, 8}
				for i, want := range wantLines {
					if out.Entries[i].LineNumber != want {
						t.Errorf("Entries[%d].LineNumber = %d, want %d", i, out.Entries[i].LineNumber, want)
					}
				}
			},
		},
		{
			name: "entries preserve raw content",
			input: FilterLogsInput{
				Path:       jsonPath,
				Level:      []string{"DEBUG"},
				MaxResults: 1,
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				if len(out.Entries) != 1 {
					t.Fatalf("len(Entries) = %d, want 1", len(out.Entries))
				}
				if !strings.Contains(out.Entries[0].Raw, `"level":"DEBUG"`) {
					t.Errorf("Raw = %q, should contain original JSON", out.Entries[0].Raw)
				}
			},
		},
		{
			name: "invalid source regex error",
			input: FilterLogsInput{
				Path:   jsonPath,
				Source: "[invalid",
			},
			wantErr:     true,
			errContains: "INVALID_REGEX",
		},
		{
			name: "invalid message pattern regex error",
			input: FilterLogsInput{
				Path:           jsonPath,
				MessagePattern: "(unclosed",
			},
			wantErr:     true,
			errContains: "INVALID_REGEX",
		},
		{
			name: "message pattern with no source skips entries without source",
			input: FilterLogsInput{
				Path:   jsonPath,
				Source: "nonexistent_source",
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				if out.TotalMatched != 0 {
					t.Errorf("TotalMatched = %d, want 0", out.TotalMatched)
				}
			},
		},
		{
			name: "time range excluding all entries",
			input: FilterLogsInput{
				Path:  jsonPath,
				After: "2099-01-01T00:00:00Z",
			},
			checkOutput: func(t *testing.T, out FilterLogsOutput) {
				if out.TotalMatched != 0 {
					t.Errorf("TotalMatched = %d, want 0", out.TotalMatched)
				}
				if out.TotalScanned != 10 {
					t.Errorf("TotalScanned = %d, want 10", out.TotalScanned)
				}
			},
		},
		{
			name: "binary file error",
			input: FilterLogsInput{
				Path: writeTempBinary(t),
			},
			wantErr:     true,
			errContains: "BINARY_FILE",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := RunFilterLogs(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.checkOutput != nil {
				tc.checkOutput(t, out)
			}
		})
	}
}

func TestRunFilterLogsRecordSeparator(t *testing.T) {
	// JSON log with multi-line stack traces following some entries.
	log := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"ERROR","message":"NullPointerException"}`,
		`	at com.example.Handler.process(Handler.java:42)`,
		`	at com.example.Server.handle(Server.java:118)`,
		`{"timestamp":"2025-01-15T10:01:00Z","level":"INFO","message":"Request completed"}`,
		`{"timestamp":"2025-01-15T10:02:00Z","level":"ERROR","message":"Connection timeout"}`,
		`	at com.example.Net.connect(Net.java:5)`,
	}, "\n") + "\n"
	path := writeTempLog(t, "filter_record.log", log)

	t.Run("level filter works with record_separator", func(t *testing.T) {
		out, err := RunFilterLogs(FilterLogsInput{
			Path:            path,
			Level:           []string{"ERROR"},
			RecordSeparator: `^\{`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if out.TotalMatched != 2 {
			t.Errorf("TotalMatched = %d, want 2", out.TotalMatched)
		}
		// The raw field should contain the full record (multi-line).
		if len(out.Entries) > 0 && !strings.Contains(out.Entries[0].Raw, "Handler.java") {
			t.Errorf("first entry Raw should include stack trace, got: %q", out.Entries[0].Raw)
		}
	})

	t.Run("message_pattern matches full record text", func(t *testing.T) {
		out, err := RunFilterLogs(FilterLogsInput{
			Path:            path,
			MessagePattern:  "Handler.java",
			RecordSeparator: `^\{`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if out.TotalMatched != 1 {
			t.Errorf("TotalMatched = %d, want 1 (only NPE record has Handler.java)", out.TotalMatched)
		}
	})

	t.Run("TotalScanned counts raw lines", func(t *testing.T) {
		out, err := RunFilterLogs(FilterLogsInput{
			Path:            path,
			Level:           []string{"ERROR"},
			RecordSeparator: `^\{`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if out.TotalScanned != 6 {
			t.Errorf("TotalScanned = %d, want 6 (raw lines)", out.TotalScanned)
		}
	})
}
