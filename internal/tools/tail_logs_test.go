package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTailTempFile creates a temporary file with the given content and returns its path.
func writeTailTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// writeBinaryTempFile creates a temporary file containing a null byte.
func writeBinaryTempFile(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	data := []byte("hello\x00world\n")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write binary temp file: %v", err)
	}
	return path
}

func TestRunTailLogs(t *testing.T) {
	// Build a 10-line file: "line 1\nline 2\n...\nline 10\n"
	var tenLines strings.Builder
	for i := 1; i <= 10; i++ {
		if i > 1 {
			tenLines.WriteString("\n")
		}
		tenLines.WriteString("line " + itoa(i))
	}
	tenLines.WriteString("\n")
	tenLineContent := tenLines.String()
	tenLinePath := writeTailTempFile(t, "ten.log", tenLineContent)

	emptyPath := writeTailTempFile(t, "empty.log", "")
	singlePath := writeTailTempFile(t, "single.log", "only line\n")
	binaryPath := writeBinaryTempFile(t, "binary.bin")

	tests := []struct {
		name             string
		input            TailLogsInput
		wantErr          bool
		wantLineCount    int
		wantTotalLines   int
		wantFromLine     int
		wantFirstLine    string
		wantLastLine     string
		wantSizePositive bool
	}{
		{
			name: "basic tail last 5 lines of 10-line file",
			input: TailLogsInput{
				Path:     tenLinePath,
				NumLines: 5,
			},
			wantLineCount:    5,
			wantTotalLines:   10,
			wantFromLine:     6,
			wantFirstLine:    "line 6",
			wantLastLine:     "line 10",
			wantSizePositive: true,
		},
		{
			name: "tail more lines than file contains returns all",
			input: TailLogsInput{
				Path:     tenLinePath,
				NumLines: 100,
			},
			wantLineCount:    10,
			wantTotalLines:   10,
			wantFromLine:     1,
			wantFirstLine:    "line 1",
			wantLastLine:     "line 10",
			wantSizePositive: true,
		},
		{
			name: "empty file",
			input: TailLogsInput{
				Path:     emptyPath,
				NumLines: 10,
			},
			wantLineCount:  0,
			wantTotalLines: 0,
			wantFromLine:   0,
		},
		{
			name: "file not found",
			input: TailLogsInput{
				Path:     "/nonexistent/path/to/file.log",
				NumLines: 10,
			},
			wantErr: true,
		},
		{
			name: "binary file",
			input: TailLogsInput{
				Path:     binaryPath,
				NumLines: 10,
			},
			wantErr: true,
		},
		{
			name: "default NumLines applied when zero",
			input: TailLogsInput{
				Path:     tenLinePath,
				NumLines: 0,
			},
			wantLineCount:    10,
			wantTotalLines:   10,
			wantFromLine:     1,
			wantFirstLine:    "line 1",
			wantLastLine:     "line 10",
			wantSizePositive: true,
		},
		{
			name: "NumLines clamped to max 1000",
			input: TailLogsInput{
				Path:     tenLinePath,
				NumLines: 5000,
			},
			wantLineCount:    10,
			wantTotalLines:   10,
			wantFromLine:     1,
			wantFirstLine:    "line 1",
			wantLastLine:     "line 10",
			wantSizePositive: true,
		},
		{
			name: "single line file",
			input: TailLogsInput{
				Path:     singlePath,
				NumLines: 5,
			},
			wantLineCount:    1,
			wantTotalLines:   1,
			wantFromLine:     1,
			wantFirstLine:    "only line",
			wantLastLine:     "only line",
			wantSizePositive: true,
		},
		{
			name: "ShowingFromLine is correct for last 3",
			input: TailLogsInput{
				Path:     tenLinePath,
				NumLines: 3,
			},
			wantLineCount:    3,
			wantTotalLines:   10,
			wantFromLine:     8,
			wantFirstLine:    "line 8",
			wantLastLine:     "line 10",
			wantSizePositive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RunTailLogs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RunTailLogs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if len(got.Lines) != tt.wantLineCount {
				t.Fatalf("got %d lines, want %d", len(got.Lines), tt.wantLineCount)
			}
			if got.TotalLines != tt.wantTotalLines {
				t.Errorf("TotalLines = %d, want %d", got.TotalLines, tt.wantTotalLines)
			}
			if got.ShowingFromLine != tt.wantFromLine {
				t.Errorf("ShowingFromLine = %d, want %d", got.ShowingFromLine, tt.wantFromLine)
			}
			if tt.wantSizePositive && got.FileSizeBytes <= 0 {
				t.Errorf("FileSizeBytes = %d, want > 0", got.FileSizeBytes)
			}

			if tt.wantLineCount > 0 {
				if got.Lines[0].Content != tt.wantFirstLine {
					t.Errorf("first line content = %q, want %q", got.Lines[0].Content, tt.wantFirstLine)
				}
				if got.Lines[len(got.Lines)-1].Content != tt.wantLastLine {
					t.Errorf("last line content = %q, want %q", got.Lines[len(got.Lines)-1].Content, tt.wantLastLine)
				}
				// Verify line numbers are sequential.
				for i := 1; i < len(got.Lines); i++ {
					if got.Lines[i].LineNumber != got.Lines[i-1].LineNumber+1 {
						t.Errorf("line numbers not sequential: line[%d]=%d, line[%d]=%d",
							i-1, got.Lines[i-1].LineNumber, i, got.Lines[i].LineNumber)
					}
				}
			}
		})
	}
}

// itoa converts a small int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
