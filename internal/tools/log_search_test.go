package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempLog creates a temporary log file with the given content and returns its path.
// The caller does not need to clean up; t.TempDir handles it.
func writeTempLog(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// writeTempBinary creates a temporary file containing a null byte so CheckFileAccess rejects it.
func writeTempBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "binary.bin")
	data := []byte("hello\x00world\n")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write binary file: %v", err)
	}
	return path
}

func TestRunSearchLogs(t *testing.T) {
	// Shared log content used by several subtests.
	sampleLog := strings.Join([]string{
		"line1 INFO starting up",
		"line2 DEBUG connecting to database",
		"line3 ERROR connection refused",
		"line4 WARN retrying in 5s",
		"line5 ERROR timeout reached",
		"line6 INFO recovered",
		"line7 DEBUG heartbeat ok",
		"line8 INFO shutting down",
	}, "\n") + "\n"

	samplePath := writeTempLog(t, "sample.log", sampleLog)
	emptyPath := writeTempLog(t, "empty.log", "")
	binaryPath := writeTempBinary(t)

	// Content where the pattern is on the very first and very last line.
	edgeLog := strings.Join([]string{
		"ERROR first line error",
		"INFO middle line",
		"DEBUG another middle",
		"ERROR last line error",
	}, "\n") + "\n"
	edgePath := writeTempLog(t, "edge.log", edgeLog)

	// Content for context overlap testing.
	contextLog := strings.Join([]string{
		"line1 before",
		"line2 MATCH first",
		"line3 between",
		"line4 MATCH second",
		"line5 after",
	}, "\n") + "\n"
	contextPath := writeTempLog(t, "context.log", contextLog)

	// Large log for truncation testing: 10 lines all matching.
	var truncLines []string
	for i := 1; i <= 10; i++ {
		truncLines = append(truncLines, "ERROR something broke")
	}
	truncLog := strings.Join(truncLines, "\n") + "\n"
	truncPath := writeTempLog(t, "trunc.log", truncLog)

	tests := []struct {
		name          string
		input         SearchLogsInput
		wantErr       bool
		errContains   string
		checkOutput   func(t *testing.T, out SearchLogsOutput)
	}{
		{
			name: "plain text match case insensitive",
			input: SearchLogsInput{
				Path:    samplePath,
				Pattern: "error",
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				if out.TotalMatches != 2 {
					t.Errorf("TotalMatches = %d, want 2", out.TotalMatches)
				}
				if len(out.Matches) != 2 {
					t.Fatalf("len(Matches) = %d, want 2", len(out.Matches))
				}
				if out.Matches[0].LineNumber != 3 {
					t.Errorf("first match line = %d, want 3", out.Matches[0].LineNumber)
				}
				if out.Matches[1].LineNumber != 5 {
					t.Errorf("second match line = %d, want 5", out.Matches[1].LineNumber)
				}
				if out.PatternUsed != "error" {
					t.Errorf("PatternUsed = %q, want %q", out.PatternUsed, "error")
				}
				if out.SearchedLines != 8 {
					t.Errorf("SearchedLines = %d, want 8", out.SearchedLines)
				}
				if out.Truncated {
					t.Error("Truncated should be false")
				}
			},
		},
		{
			name: "plain text match case sensitive",
			input: SearchLogsInput{
				Path:          samplePath,
				Pattern:       "ERROR",
				CaseSensitive: true,
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				if out.TotalMatches != 2 {
					t.Errorf("TotalMatches = %d, want 2", out.TotalMatches)
				}
				if len(out.Matches) != 2 {
					t.Fatalf("len(Matches) = %d, want 2", len(out.Matches))
				}
				// "error" lowercase should not match
				for _, m := range out.Matches {
					if !strings.Contains(m.Line, "ERROR") {
						t.Errorf("match line %q does not contain ERROR", m.Line)
					}
				}
			},
		},
		{
			name: "case sensitive no match for wrong case",
			input: SearchLogsInput{
				Path:          samplePath,
				Pattern:       "error",
				CaseSensitive: true,
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				if out.TotalMatches != 0 {
					t.Errorf("TotalMatches = %d, want 0", out.TotalMatches)
				}
				if len(out.Matches) != 0 {
					t.Errorf("len(Matches) = %d, want 0", len(out.Matches))
				}
			},
		},
		{
			name: "regex match",
			input: SearchLogsInput{
				Path:    samplePath,
				Pattern: "line[35]",
				IsRegex: true,
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				if out.TotalMatches != 2 {
					t.Errorf("TotalMatches = %d, want 2", out.TotalMatches)
				}
				if len(out.Matches) != 2 {
					t.Fatalf("len(Matches) = %d, want 2", len(out.Matches))
				}
				if out.Matches[0].LineNumber != 3 {
					t.Errorf("first match line = %d, want 3", out.Matches[0].LineNumber)
				}
				if out.Matches[1].LineNumber != 5 {
					t.Errorf("second match line = %d, want 5", out.Matches[1].LineNumber)
				}
			},
		},
		{
			name: "no matches",
			input: SearchLogsInput{
				Path:    samplePath,
				Pattern: "CRITICAL",
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				if out.TotalMatches != 0 {
					t.Errorf("TotalMatches = %d, want 0", out.TotalMatches)
				}
				if len(out.Matches) != 0 {
					t.Errorf("len(Matches) = %d, want 0", len(out.Matches))
				}
				if out.SearchedLines != 8 {
					t.Errorf("SearchedLines = %d, want 8", out.SearchedLines)
				}
				if out.Truncated {
					t.Error("Truncated should be false")
				}
			},
		},
		{
			name: "context lines before and after",
			input: SearchLogsInput{
				Path:         samplePath,
				Pattern:      "ERROR connection refused",
				ContextLines: 2,
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				if len(out.Matches) != 1 {
					t.Fatalf("len(Matches) = %d, want 1", len(out.Matches))
				}
				m := out.Matches[0]
				if m.LineNumber != 3 {
					t.Errorf("match line = %d, want 3", m.LineNumber)
				}
				// Before context: lines 1 and 2.
				if len(m.BeforeContext) != 2 {
					t.Fatalf("len(BeforeContext) = %d, want 2", len(m.BeforeContext))
				}
				if !strings.Contains(m.BeforeContext[0], "line1") {
					t.Errorf("BeforeContext[0] = %q, want line1", m.BeforeContext[0])
				}
				if !strings.Contains(m.BeforeContext[1], "line2") {
					t.Errorf("BeforeContext[1] = %q, want line2", m.BeforeContext[1])
				}
				// After context: lines 4 and 5.
				if len(m.AfterContext) != 2 {
					t.Fatalf("len(AfterContext) = %d, want 2", len(m.AfterContext))
				}
				if !strings.Contains(m.AfterContext[0], "line4") {
					t.Errorf("AfterContext[0] = %q, want line4", m.AfterContext[0])
				}
				if !strings.Contains(m.AfterContext[1], "line5") {
					t.Errorf("AfterContext[1] = %q, want line5", m.AfterContext[1])
				}
			},
		},
		{
			name: "max results truncation",
			input: SearchLogsInput{
				Path:       truncPath,
				Pattern:    "ERROR",
				MaxResults: 3,
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				if len(out.Matches) != 3 {
					t.Errorf("len(Matches) = %d, want 3", len(out.Matches))
				}
				if out.TotalMatches != 10 {
					t.Errorf("TotalMatches = %d, want 10", out.TotalMatches)
				}
				if !out.Truncated {
					t.Error("Truncated should be true")
				}
			},
		},
		{
			name: "file not found error",
			input: SearchLogsInput{
				Path:    "/nonexistent/path/to/file.log",
				Pattern: "test",
			},
			wantErr:     true,
			errContains: "FILE_NOT_FOUND",
		},
		{
			name: "binary file error",
			input: SearchLogsInput{
				Path:    binaryPath,
				Pattern: "hello",
			},
			wantErr:     true,
			errContains: "BINARY_FILE",
		},
		{
			name: "invalid regex error",
			input: SearchLogsInput{
				Path:    samplePath,
				Pattern: "[invalid",
				IsRegex: true,
			},
			wantErr:     true,
			errContains: "INVALID_REGEX",
		},
		{
			name: "empty file no matches",
			input: SearchLogsInput{
				Path:    emptyPath,
				Pattern: "anything",
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				if out.TotalMatches != 0 {
					t.Errorf("TotalMatches = %d, want 0", out.TotalMatches)
				}
				if len(out.Matches) != 0 {
					t.Errorf("len(Matches) = %d, want 0", len(out.Matches))
				}
				if out.SearchedLines != 0 {
					t.Errorf("SearchedLines = %d, want 0", out.SearchedLines)
				}
			},
		},
		{
			name: "pattern at first line no before context",
			input: SearchLogsInput{
				Path:         edgePath,
				Pattern:      "first line",
				ContextLines: 3,
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				if len(out.Matches) < 1 {
					t.Fatalf("len(Matches) = %d, want >= 1", len(out.Matches))
				}
				m := out.Matches[0]
				if m.LineNumber != 1 {
					t.Errorf("match line = %d, want 1", m.LineNumber)
				}
				if len(m.BeforeContext) != 0 {
					t.Errorf("len(BeforeContext) = %d, want 0", len(m.BeforeContext))
				}
				// After context should have up to 3 lines.
				if len(m.AfterContext) != 3 {
					t.Errorf("len(AfterContext) = %d, want 3", len(m.AfterContext))
				}
			},
		},
		{
			name: "pattern at last line no after context",
			input: SearchLogsInput{
				Path:         edgePath,
				Pattern:      "last line",
				ContextLines: 3,
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				if len(out.Matches) < 1 {
					t.Fatalf("len(Matches) = %d, want >= 1", len(out.Matches))
				}
				m := out.Matches[0]
				if m.LineNumber != 4 {
					t.Errorf("match line = %d, want 4", m.LineNumber)
				}
				// Before context: lines 1, 2, 3.
				if len(m.BeforeContext) != 3 {
					t.Errorf("len(BeforeContext) = %d, want 3", len(m.BeforeContext))
				}
				// No lines after the last line.
				if len(m.AfterContext) != 0 {
					t.Errorf("len(AfterContext) = %d, want 0", len(m.AfterContext))
				}
			},
		},
		{
			name: "multiple matches",
			input: SearchLogsInput{
				Path:    samplePath,
				Pattern: "line",
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				// Every line contains "line".
				if out.TotalMatches != 8 {
					t.Errorf("TotalMatches = %d, want 8", out.TotalMatches)
				}
				if len(out.Matches) != 8 {
					t.Errorf("len(Matches) = %d, want 8", len(out.Matches))
				}
				for i, m := range out.Matches {
					if m.LineNumber != i+1 {
						t.Errorf("Matches[%d].LineNumber = %d, want %d", i, m.LineNumber, i+1)
					}
				}
			},
		},
		{
			name: "truncated flag set correctly when equal",
			input: SearchLogsInput{
				Path:       truncPath,
				Pattern:    "ERROR",
				MaxResults: 10,
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				// Exactly 10 matches, MaxResults = 10 → not truncated.
				if out.Truncated {
					t.Error("Truncated should be false when total == collected")
				}
				if out.TotalMatches != 10 {
					t.Errorf("TotalMatches = %d, want 10", out.TotalMatches)
				}
				if len(out.Matches) != 10 {
					t.Errorf("len(Matches) = %d, want 10", len(out.Matches))
				}
			},
		},
		{
			name: "context lines with overlapping matches",
			input: SearchLogsInput{
				Path:         contextPath,
				Pattern:      "MATCH",
				ContextLines: 2,
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				if len(out.Matches) != 2 {
					t.Fatalf("len(Matches) = %d, want 2", len(out.Matches))
				}
				// First match at line 2.
				m0 := out.Matches[0]
				if m0.LineNumber != 2 {
					t.Errorf("first match line = %d, want 2", m0.LineNumber)
				}
				if len(m0.BeforeContext) != 1 {
					t.Errorf("first match BeforeContext len = %d, want 1", len(m0.BeforeContext))
				}
				if len(m0.AfterContext) != 2 {
					t.Errorf("first match AfterContext len = %d, want 2", len(m0.AfterContext))
				}
				// Second match at line 4.
				m1 := out.Matches[1]
				if m1.LineNumber != 4 {
					t.Errorf("second match line = %d, want 4", m1.LineNumber)
				}
				if len(m1.BeforeContext) != 2 {
					t.Errorf("second match BeforeContext len = %d, want 2", len(m1.BeforeContext))
				}
				if len(m1.AfterContext) != 1 {
					t.Errorf("second match AfterContext len = %d, want 1", len(m1.AfterContext))
				}
			},
		},
		{
			name: "defaults applied for max results",
			input: SearchLogsInput{
				Path:    samplePath,
				Pattern: "line",
				// MaxResults left as 0 → should default to 50.
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				// File only has 8 lines, all match, so no truncation at default 50.
				if out.Truncated {
					t.Error("Truncated should be false with default MaxResults")
				}
				if out.TotalMatches != 8 {
					t.Errorf("TotalMatches = %d, want 8", out.TotalMatches)
				}
			},
		},
		{
			name: "context lines clamped to max 10",
			input: SearchLogsInput{
				Path:         samplePath,
				Pattern:      "shutting down",
				ContextLines: 99, // should be clamped to 10
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				if len(out.Matches) != 1 {
					t.Fatalf("len(Matches) = %d, want 1", len(out.Matches))
				}
				m := out.Matches[0]
				if m.LineNumber != 8 {
					t.Errorf("match line = %d, want 8", m.LineNumber)
				}
				// Only 7 lines before line 8, and clamped context is 10, so we get all 7.
				if len(m.BeforeContext) != 7 {
					t.Errorf("BeforeContext len = %d, want 7", len(m.BeforeContext))
				}
			},
		},
		{
			name: "matches slice non nil when no results",
			input: SearchLogsInput{
				Path:    samplePath,
				Pattern: "ZZZZNOTFOUND",
			},
			checkOutput: func(t *testing.T, out SearchLogsOutput) {
				if out.Matches == nil {
					t.Error("Matches should be non-nil empty slice, got nil")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := RunSearchLogs(tc.input)
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

func TestRunSearchLogsRecordSeparator(t *testing.T) {
	// Multi-line log: searching for "Handler.java" should return the full
	// record (including the first line with the error message), not just
	// the stack trace line.
	log := strings.Join([]string{
		"2025-01-15 ERROR NullPointerException in handler",
		"\tat com.example.Handler.process(Handler.java:42)",
		"\tat com.example.Server.handle(Server.java:118)",
		"2025-01-15 INFO Request completed successfully",
		"2025-01-15 ERROR Timeout connecting to database",
		"\tat com.example.Dao.query(Dao.java:88)",
	}, "\n") + "\n"
	path := writeTempLog(t, "search_record.log", log)

	t.Run("match in continuation line returns full record", func(t *testing.T) {
		out, err := RunSearchLogs(SearchLogsInput{
			Path:            path,
			Pattern:         "Handler.java",
			RecordSeparator: `^\d{4}-\d{2}-\d{2}`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if out.TotalMatches != 1 {
			t.Fatalf("TotalMatches = %d, want 1", out.TotalMatches)
		}
		match := out.Matches[0]
		// Full record returned — includes the ERROR line and both stack frames.
		if !strings.Contains(match.Line, "NullPointerException") {
			t.Errorf("match.Line should contain the record's first line, got: %q", match.Line)
		}
		if !strings.Contains(match.Line, "Handler.java:42") {
			t.Errorf("match.Line should contain the matching stack frame, got: %q", match.Line)
		}
		if match.LineNumber != 1 {
			t.Errorf("LineNumber = %d, want 1 (record start)", match.LineNumber)
		}
	})

	t.Run("match on first line of record works", func(t *testing.T) {
		out, err := RunSearchLogs(SearchLogsInput{
			Path:            path,
			Pattern:         "Timeout",
			RecordSeparator: `^\d{4}-\d{2}-\d{2}`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if out.TotalMatches != 1 {
			t.Fatalf("TotalMatches = %d, want 1", out.TotalMatches)
		}
		if !strings.Contains(out.Matches[0].Line, "Dao.java") {
			t.Errorf("expected full record with stack trace, got: %q", out.Matches[0].Line)
		}
	})

	t.Run("searched_lines counts raw lines", func(t *testing.T) {
		out, err := RunSearchLogs(SearchLogsInput{
			Path:            path,
			Pattern:         "NOMATCH",
			RecordSeparator: `^\d{4}-\d{2}-\d{2}`,
		})
		if err != nil {
			t.Fatal(err)
		}
		if out.SearchedLines != 6 {
			t.Errorf("SearchedLines = %d, want 6", out.SearchedLines)
		}
	})
}
