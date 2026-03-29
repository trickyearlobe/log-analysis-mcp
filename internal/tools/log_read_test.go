package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestFile creates a text file in dir with the given lines joined by newlines.
func writeTestFile(t *testing.T, dir, name string, lines []string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
	return path
}

// writeBinaryFile creates a file containing a null byte so CheckFileAccess rejects it.
func writeBinaryFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("hello\x00world"), 0644); err != nil {
		t.Fatalf("write binary file %s: %v", path, err)
	}
	return path
}

func TestRunReadLogs(t *testing.T) {
	dir := t.TempDir()

	// Prepare shared test files.
	tenLines := make([]string, 10)
	for i := range tenLines {
		tenLines[i] = "line " + strings.Repeat("x", i+1) // varying lengths
	}
	tenLinePath := writeTestFile(t, dir, "ten.log", tenLines)

	singleLinePath := writeTestFile(t, dir, "single.log", []string{"only line"})
	emptyPath := writeTestFile(t, dir, "empty.log", []string{""})
	binaryPath := writeBinaryFile(t, dir, "binary.bin")

	// A file with 1500 lines to test HasMore and clamping.
	bigLines := make([]string, 1500)
	for i := range bigLines {
		bigLines[i] = "log entry"
	}
	bigPath := writeTestFile(t, dir, "big.log", bigLines)

	tests := []struct {
		name        string
		input       ReadLogsInput
		wantErr     bool
		errContains string
		check       func(t *testing.T, out ReadLogsOutput)
	}{
		{
			name:  "basic read from start with defaults",
			input: ReadLogsInput{Path: tenLinePath},
			check: func(t *testing.T, out ReadLogsOutput) {
				if len(out.Lines) != 10 {
					t.Errorf("expected 10 lines, got %d", len(out.Lines))
				}
				if out.HasMore {
					t.Error("expected HasMore=false for 10-line file with default num_lines=100")
				}
				if out.TotalLines != 10 {
					t.Errorf("expected TotalLines=10, got %d", out.TotalLines)
				}
				if out.CurrentRange.Start != 1 {
					t.Errorf("expected range start=1, got %d", out.CurrentRange.Start)
				}
				if out.CurrentRange.End != 10 {
					t.Errorf("expected range end=10, got %d", out.CurrentRange.End)
				}
				if out.FileSizeBytes <= 0 {
					t.Errorf("expected positive file size, got %d", out.FileSizeBytes)
				}
				// Verify line numbers are sequential.
				for i, l := range out.Lines {
					if l.LineNumber != i+1 {
						t.Errorf("line %d: expected LineNumber=%d, got %d", i, i+1, l.LineNumber)
					}
				}
			},
		},
		{
			name:  "pagination with start_line > 1",
			input: ReadLogsInput{Path: tenLinePath, StartLine: 5, NumLines: 3},
			check: func(t *testing.T, out ReadLogsOutput) {
				if len(out.Lines) != 3 {
					t.Errorf("expected 3 lines, got %d", len(out.Lines))
				}
				if out.Lines[0].LineNumber != 5 {
					t.Errorf("expected first line number=5, got %d", out.Lines[0].LineNumber)
				}
				if out.Lines[2].LineNumber != 7 {
					t.Errorf("expected last line number=7, got %d", out.Lines[2].LineNumber)
				}
				if out.CurrentRange.Start != 5 {
					t.Errorf("expected range start=5, got %d", out.CurrentRange.Start)
				}
				if out.CurrentRange.End != 7 {
					t.Errorf("expected range end=7, got %d", out.CurrentRange.End)
				}
				if !out.HasMore {
					t.Error("expected HasMore=true since lines 8-10 remain")
				}
			},
		},
		{
			name:  "has_more when more lines exist",
			input: ReadLogsInput{Path: tenLinePath, NumLines: 5},
			check: func(t *testing.T, out ReadLogsOutput) {
				if len(out.Lines) != 5 {
					t.Errorf("expected 5 lines, got %d", len(out.Lines))
				}
				if !out.HasMore {
					t.Error("expected HasMore=true")
				}
				// TotalLines should be 0 (unknown) when HasMore is true.
				if out.TotalLines != 0 {
					t.Errorf("expected TotalLines=0 when HasMore=true, got %d", out.TotalLines)
				}
			},
		},
		{
			name:  "empty file",
			input: ReadLogsInput{Path: emptyPath},
			check: func(t *testing.T, out ReadLogsOutput) {
				// An "empty" file with a single trailing newline may yield 1 empty line.
				// The key invariant: no error, HasMore is false.
				if out.HasMore {
					t.Error("expected HasMore=false for empty file")
				}
			},
		},
		{
			name:        "file not found",
			input:       ReadLogsInput{Path: filepath.Join(dir, "nonexistent.log")},
			wantErr:     true,
			errContains: "FILE_NOT_FOUND",
		},
		{
			name:        "binary file rejected",
			input:       ReadLogsInput{Path: binaryPath},
			wantErr:     true,
			errContains: "BINARY_FILE",
		},
		{
			name:  "default values applied",
			input: ReadLogsInput{Path: tenLinePath, StartLine: 0, NumLines: 0, Encoding: ""},
			check: func(t *testing.T, out ReadLogsOutput) {
				// Defaults: StartLine=1, NumLines=100, Encoding="utf-8".
				if out.CurrentRange.Start != 1 {
					t.Errorf("expected default start=1, got %d", out.CurrentRange.Start)
				}
				if len(out.Lines) != 10 {
					t.Errorf("expected 10 lines with default num_lines=100, got %d", len(out.Lines))
				}
			},
		},
		{
			name:  "num_lines clamped to max 1000",
			input: ReadLogsInput{Path: bigPath, NumLines: 5000},
			check: func(t *testing.T, out ReadLogsOutput) {
				if len(out.Lines) != 1000 {
					t.Errorf("expected 1000 lines (clamped), got %d", len(out.Lines))
				}
				if !out.HasMore {
					t.Error("expected HasMore=true since 1500 lines > 1000")
				}
			},
		},
		{
			name:  "single line file",
			input: ReadLogsInput{Path: singleLinePath},
			check: func(t *testing.T, out ReadLogsOutput) {
				if len(out.Lines) != 1 {
					t.Errorf("expected 1 line, got %d", len(out.Lines))
				}
				if out.Lines[0].Content != "only line" {
					t.Errorf("expected content 'only line', got %q", out.Lines[0].Content)
				}
				if out.Lines[0].LineNumber != 1 {
					t.Errorf("expected line number 1, got %d", out.Lines[0].LineNumber)
				}
				if out.HasMore {
					t.Error("expected HasMore=false")
				}
				if out.TotalLines != 1 {
					t.Errorf("expected TotalLines=1, got %d", out.TotalLines)
				}
			},
		},
		{
			name:  "start_line beyond end of file returns empty",
			input: ReadLogsInput{Path: tenLinePath, StartLine: 999},
			check: func(t *testing.T, out ReadLogsOutput) {
				if len(out.Lines) != 0 {
					t.Errorf("expected 0 lines, got %d", len(out.Lines))
				}
				if out.HasMore {
					t.Error("expected HasMore=false")
				}
				// Range should default to start_line when no lines returned.
				if out.CurrentRange.Start != 999 {
					t.Errorf("expected range start=999, got %d", out.CurrentRange.Start)
				}
			},
		},
		{
			name:  "total_lines accurate when reading from middle to end",
			input: ReadLogsInput{Path: tenLinePath, StartLine: 8, NumLines: 100},
			check: func(t *testing.T, out ReadLogsOutput) {
				if len(out.Lines) != 3 {
					t.Errorf("expected 3 lines (8,9,10), got %d", len(out.Lines))
				}
				if out.HasMore {
					t.Error("expected HasMore=false")
				}
				// TotalLines = startLine - 1 + totalRead = 7 + 3 = 10.
				if out.TotalLines != 10 {
					t.Errorf("expected TotalLines=10, got %d", out.TotalLines)
				}
			},
		},
		{
			name:  "content fidelity preserved",
			input: ReadLogsInput{Path: tenLinePath, StartLine: 1, NumLines: 2},
			check: func(t *testing.T, out ReadLogsOutput) {
				if len(out.Lines) < 2 {
					t.Fatalf("expected at least 2 lines, got %d", len(out.Lines))
				}
				if out.Lines[0].Content != "line x" {
					t.Errorf("expected first line content 'line x', got %q", out.Lines[0].Content)
				}
				if out.Lines[1].Content != "line xx" {
					t.Errorf("expected second line content 'line xx', got %q", out.Lines[1].Content)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := RunReadLogs(tt.input)
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
			if tt.check != nil {
				tt.check(t, out)
			}
		})
	}
}
