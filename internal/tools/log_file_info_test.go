package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFileInfo(t *testing.T) {
	// JSON log format is reliably detected by the parser
	structuredLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:01Z","level":"info","msg":"Starting up"}`,
		`{"timestamp":"2025-01-15T10:00:02Z","level":"info","msg":"Connected"}`,
		`{"timestamp":"2025-01-15T10:00:03Z","level":"warn","msg":"Slow query"}`,
		`{"timestamp":"2025-01-15T10:00:04Z","level":"error","msg":"Connection lost"}`,
		`{"timestamp":"2025-01-15T10:00:05Z","level":"info","msg":"Done"}`,
	}, "\n") + "\n"
	structuredPath := writeTempLog(t, "structured.log", structuredLog)

	emptyPath := writeTempLog(t, "empty.log", "")

	// Create a binary file
	binaryPath := filepath.Join(t.TempDir(), "binary.log")
	if err := os.WriteFile(binaryPath, []byte("hello\x00world\nline2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Freeform log (no parseable timestamps)
	freeformLog := "just some text\nanother line\nthird line\n"
	freeformPath := writeTempLog(t, "freeform.log", freeformLog)

	tests := []struct {
		name        string
		input       FileInfoInput
		wantErr     bool
		errContains string
		checkOutput func(t *testing.T, out FileInfoOutput)
	}{
		{
			name:  "structured log with timestamps",
			input: FileInfoInput{Path: structuredPath},
			checkOutput: func(t *testing.T, out FileInfoOutput) {
				if out.LineCount != 5 {
					t.Errorf("LineCount = %d, want 5", out.LineCount)
				}
				if out.SizeBytes == 0 {
					t.Error("SizeBytes should be > 0")
				}
				if out.FirstTimestamp == "" {
					t.Error("FirstTimestamp should not be empty")
				}
				if out.LastTimestamp == "" {
					t.Error("LastTimestamp should not be empty")
				}
				if out.CompressionType != "none" {
					t.Errorf("CompressionType = %q, want \"none\"", out.CompressionType)
				}
				if out.IsBinary {
					t.Error("IsBinary should be false")
				}
				if !filepath.IsAbs(out.Path) {
					t.Errorf("Path should be absolute, got %q", out.Path)
				}
			},
		},
		{
			name:  "empty file",
			input: FileInfoInput{Path: emptyPath},
			checkOutput: func(t *testing.T, out FileInfoOutput) {
				if out.LineCount != 0 {
					t.Errorf("LineCount = %d, want 0", out.LineCount)
				}
				if out.SizeBytes != 0 {
					t.Errorf("SizeBytes = %d, want 0", out.SizeBytes)
				}
				if out.FirstTimestamp != "" {
					t.Errorf("FirstTimestamp = %q, want empty", out.FirstTimestamp)
				}
				if out.LastTimestamp != "" {
					t.Errorf("LastTimestamp = %q, want empty", out.LastTimestamp)
				}
			},
		},
		{
			name:  "binary file",
			input: FileInfoInput{Path: binaryPath},
			checkOutput: func(t *testing.T, out FileInfoOutput) {
				if !out.IsBinary {
					t.Error("IsBinary should be true")
				}
				if out.LineCount != 0 {
					t.Error("LineCount should be 0 for binary file (not scanned)")
				}
			},
		},
		{
			name:  "freeform log without timestamps",
			input: FileInfoInput{Path: freeformPath},
			checkOutput: func(t *testing.T, out FileInfoOutput) {
				if out.LineCount != 3 {
					t.Errorf("LineCount = %d, want 3", out.LineCount)
				}
				if out.FirstTimestamp != "" {
					t.Errorf("FirstTimestamp = %q, want empty", out.FirstTimestamp)
				}
				if out.LastTimestamp != "" {
					t.Errorf("LastTimestamp = %q, want empty", out.LastTimestamp)
				}
			},
		},
		{
			name:        "file not found",
			input:       FileInfoInput{Path: "/nonexistent/path.log"},
			wantErr:     true,
			errContains: "FILE_NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := RunFileInfo(tt.input)
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
