package fileutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// helper to create a temp file with given content and return its path.
func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestReadLines(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		startLine int
		numLines  int
		wantLines []LineRecord
		wantMore  bool
		wantRead  int
		wantErr   bool
	}{
		{
			name:      "basic read from start",
			content:   "line1\nline2\nline3\n",
			startLine: 1,
			numLines:  10,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "line1"},
				{LineNumber: 2, Text: "line2"},
				{LineNumber: 3, Text: "line3"},
			},
			wantMore: false,
			wantRead: 3,
		},
		{
			name:      "pagination with has_more",
			content:   "a\nb\nc\nd\ne\n",
			startLine: 1,
			numLines:  3,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "a"},
				{LineNumber: 2, Text: "b"},
				{LineNumber: 3, Text: "c"},
			},
			wantMore: true,
			wantRead: 3,
		},
		{
			name:      "start from middle",
			content:   "a\nb\nc\nd\ne\n",
			startLine: 3,
			numLines:  10,
			wantLines: []LineRecord{
				{LineNumber: 3, Text: "c"},
				{LineNumber: 4, Text: "d"},
				{LineNumber: 5, Text: "e"},
			},
			wantMore: false,
			wantRead: 3,
		},
		{
			name:      "start from middle with pagination",
			content:   "a\nb\nc\nd\ne\n",
			startLine: 2,
			numLines:  2,
			wantLines: []LineRecord{
				{LineNumber: 2, Text: "b"},
				{LineNumber: 3, Text: "c"},
			},
			wantMore: true,
			wantRead: 2,
		},
		{
			name:      "start_line beyond end of file",
			content:   "a\nb\nc\n",
			startLine: 100,
			numLines:  10,
			wantLines: []LineRecord{},
			wantMore:  false,
			wantRead:  0,
		},
		{
			name:      "empty file",
			content:   "",
			startLine: 1,
			numLines:  10,
			wantLines: []LineRecord{},
			wantMore:  false,
			wantRead:  0,
		},
		{
			name:      "single line no trailing newline",
			content:   "only line",
			startLine: 1,
			numLines:  10,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "only line"},
			},
			wantMore: false,
			wantRead: 1,
		},
		{
			name:      "single line with trailing newline",
			content:   "only line\n",
			startLine: 1,
			numLines:  10,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "only line"},
			},
			wantMore: false,
			wantRead: 1,
		},
		{
			name:      "request exactly available lines",
			content:   "a\nb\nc\n",
			startLine: 1,
			numLines:  3,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "a"},
				{LineNumber: 2, Text: "b"},
				{LineNumber: 3, Text: "c"},
			},
			wantMore: false,
			wantRead: 3,
		},
		{
			name:      "num_lines is 1",
			content:   "a\nb\nc\n",
			startLine: 1,
			numLines:  1,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "a"},
			},
			wantMore: true,
			wantRead: 1,
		},
		{
			name:      "lines with carriage return",
			content:   "line1\r\nline2\r\nline3\r\n",
			startLine: 1,
			numLines:  10,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "line1"},
				{LineNumber: 2, Text: "line2"},
				{LineNumber: 3, Text: "line3"},
			},
			wantMore: false,
			wantRead: 3,
		},
		{
			name:      "file with blank lines",
			content:   "a\n\nb\n\nc\n",
			startLine: 1,
			numLines:  10,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "a"},
				{LineNumber: 2, Text: ""},
				{LineNumber: 3, Text: "b"},
				{LineNumber: 4, Text: ""},
				{LineNumber: 5, Text: "c"},
			},
			wantMore: false,
			wantRead: 5,
		},
		{
			name:      "start at last line",
			content:   "a\nb\nc\n",
			startLine: 3,
			numLines:  10,
			wantLines: []LineRecord{
				{LineNumber: 3, Text: "c"},
			},
			wantMore: false,
			wantRead: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTempFile(t, "test.log", tt.content)

			got, err := ReadLines(path, tt.startLine, tt.numLines)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadLines() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if got.TotalRead != tt.wantRead {
				t.Errorf("TotalRead = %d, want %d", got.TotalRead, tt.wantRead)
			}
			if got.HasMore != tt.wantMore {
				t.Errorf("HasMore = %v, want %v", got.HasMore, tt.wantMore)
			}
			if len(got.Lines) != len(tt.wantLines) {
				t.Fatalf("got %d lines, want %d", len(got.Lines), len(tt.wantLines))
			}
			for i, wl := range tt.wantLines {
				if got.Lines[i].LineNumber != wl.LineNumber {
					t.Errorf("line[%d].LineNumber = %d, want %d", i, got.Lines[i].LineNumber, wl.LineNumber)
				}
				if got.Lines[i].Text != wl.Text {
					t.Errorf("line[%d].Text = %q, want %q", i, got.Lines[i].Text, wl.Text)
				}
			}
		})
	}
}

func TestReadLinesValidation(t *testing.T) {
	path := writeTempFile(t, "test.log", "content\n")

	tests := []struct {
		name      string
		startLine int
		numLines  int
		errSubstr string
	}{
		{
			name:      "start_line zero",
			startLine: 0,
			numLines:  10,
			errSubstr: "invalid start_line",
		},
		{
			name:      "start_line negative",
			startLine: -5,
			numLines:  10,
			errSubstr: "invalid start_line",
		},
		{
			name:      "num_lines zero",
			startLine: 1,
			numLines:  0,
			errSubstr: "invalid num_lines",
		},
		{
			name:      "num_lines negative",
			startLine: 1,
			numLines:  -1,
			errSubstr: "invalid num_lines",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadLines(path, tt.startLine, tt.numLines)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
			}
		})
	}
}

func TestReadLinesFileNotFound(t *testing.T) {
	_, err := ReadLines("/nonexistent/path/to/file.log", 1, 10)
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestReadLinesLargeLineFallback(t *testing.T) {
	// Create a file with a line that exceeds the 1 MB scanner buffer.
	// The fallback reader should handle it without error.
	hugeLine := strings.Repeat("X", maxScannerBuf+1000)
	content := "normal line 1\n" + hugeLine + "\nnormal line 3\n"
	path := writeTempFile(t, "huge.log", content)

	tests := []struct {
		name      string
		startLine int
		numLines  int
		wantLines []LineRecord
		wantMore  bool
		wantRead  int
	}{
		{
			name:      "read all including huge line",
			startLine: 1,
			numLines:  10,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "normal line 1"},
				{LineNumber: 2, Text: hugeLine},
				{LineNumber: 3, Text: "normal line 3"},
			},
			wantMore: false,
			wantRead: 3,
		},
		{
			name:      "read only the huge line",
			startLine: 2,
			numLines:  1,
			wantLines: []LineRecord{
				{LineNumber: 2, Text: hugeLine},
			},
			wantMore: true,
			wantRead: 1,
		},
		{
			name:      "skip past huge line",
			startLine: 3,
			numLines:  10,
			wantLines: []LineRecord{
				{LineNumber: 3, Text: "normal line 3"},
			},
			wantMore: false,
			wantRead: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadLines(path, tt.startLine, tt.numLines)
			if err != nil {
				t.Fatalf("ReadLines() error = %v", err)
			}

			if got.TotalRead != tt.wantRead {
				t.Errorf("TotalRead = %d, want %d", got.TotalRead, tt.wantRead)
			}
			if got.HasMore != tt.wantMore {
				t.Errorf("HasMore = %v, want %v", got.HasMore, tt.wantMore)
			}
			if len(got.Lines) != len(tt.wantLines) {
				t.Fatalf("got %d lines, want %d", len(got.Lines), len(tt.wantLines))
			}
			for i, wl := range tt.wantLines {
				if got.Lines[i].LineNumber != wl.LineNumber {
					t.Errorf("line[%d].LineNumber = %d, want %d", i, got.Lines[i].LineNumber, wl.LineNumber)
				}
				if got.Lines[i].Text != wl.Text {
					t.Errorf("line[%d].Text length = %d, want %d", i, len(got.Lines[i].Text), len(wl.Text))
				}
			}
		})
	}
}

func TestReadLinesSingleHugeLine(t *testing.T) {
	// File contains only one enormous line with no trailing newline.
	hugeLine := strings.Repeat("A", maxScannerBuf+500)
	path := writeTempFile(t, "single_huge.log", hugeLine)

	got, err := ReadLines(path, 1, 10)
	if err != nil {
		t.Fatalf("ReadLines() error = %v", err)
	}
	if len(got.Lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(got.Lines))
	}
	if got.Lines[0].LineNumber != 1 {
		t.Errorf("LineNumber = %d, want 1", got.Lines[0].LineNumber)
	}
	if got.Lines[0].Text != hugeLine {
		t.Errorf("Text length = %d, want %d", len(got.Lines[0].Text), len(hugeLine))
	}
	if got.HasMore {
		t.Error("HasMore = true, want false")
	}
	if got.TotalRead != 1 {
		t.Errorf("TotalRead = %d, want 1", got.TotalRead)
	}
}

// buildCompressedContent creates 10 lines of "line 1" through "line 10".
func buildCompressedContent() string {
	var b strings.Builder
	for i := 1; i <= 10; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

func TestReadLines_GzipFile(t *testing.T) {
	dir := t.TempDir()
	content := buildCompressedContent()
	path := createGzipFile(t, dir, "test.log.gz", content)

	tests := []struct {
		name      string
		startLine int
		numLines  int
		wantCount int
		wantMore  bool
		wantFirst string
	}{
		{
			name:      "read all lines",
			startLine: 1,
			numLines:  20,
			wantCount: 10,
			wantMore:  false,
			wantFirst: "line 1",
		},
		{
			name:      "read first 3 lines",
			startLine: 1,
			numLines:  3,
			wantCount: 3,
			wantMore:  true,
			wantFirst: "line 1",
		},
		{
			name:      "read from middle",
			startLine: 5,
			numLines:  3,
			wantCount: 3,
			wantMore:  true,
			wantFirst: "line 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadLines(path, tt.startLine, tt.numLines)
			if err != nil {
				t.Fatalf("ReadLines() error = %v", err)
			}
			if len(got.Lines) != tt.wantCount {
				t.Fatalf("got %d lines, want %d", len(got.Lines), tt.wantCount)
			}
			if got.HasMore != tt.wantMore {
				t.Errorf("HasMore = %v, want %v", got.HasMore, tt.wantMore)
			}
			if got.Lines[0].Text != tt.wantFirst {
				t.Errorf("first line = %q, want %q", got.Lines[0].Text, tt.wantFirst)
			}
			if got.Lines[0].LineNumber != tt.startLine {
				t.Errorf("first LineNumber = %d, want %d", got.Lines[0].LineNumber, tt.startLine)
			}
		})
	}
}

// createBzip2File creates a bzip2-compressed file using the bzip2 command.
// Returns the path to the .bz2 file.
func createBzip2File(t *testing.T, dir, name, content string) string {
	t.Helper()
	// Write plain text to a temp file, then compress with bzip2.
	plainName := strings.TrimSuffix(name, ".bz2")
	plainPath := filepath.Join(dir, plainName)
	if err := os.WriteFile(plainPath, []byte(content), 0644); err != nil {
		t.Fatalf("write plain file for bzip2: %v", err)
	}
	cmd := exec.Command("bzip2", plainPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bzip2 command failed: %v: %s", err, out)
	}
	return plainPath + ".bz2"
}

func TestReadLines_Bzip2File(t *testing.T) {
	if _, err := exec.LookPath("bzip2"); err != nil {
		t.Skip("bzip2 command not available")
	}

	dir := t.TempDir()
	content := buildCompressedContent()
	path := createBzip2File(t, dir, "test.log.bz2", content)

	tests := []struct {
		name      string
		startLine int
		numLines  int
		wantCount int
		wantMore  bool
		wantFirst string
	}{
		{
			name:      "read all lines",
			startLine: 1,
			numLines:  20,
			wantCount: 10,
			wantMore:  false,
			wantFirst: "line 1",
		},
		{
			name:      "read first 3 lines",
			startLine: 1,
			numLines:  3,
			wantCount: 3,
			wantMore:  true,
			wantFirst: "line 1",
		},
		{
			name:      "read from middle",
			startLine: 5,
			numLines:  3,
			wantCount: 3,
			wantMore:  true,
			wantFirst: "line 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadLines(path, tt.startLine, tt.numLines)
			if err != nil {
				t.Fatalf("ReadLines() error = %v", err)
			}
			if len(got.Lines) != tt.wantCount {
				t.Fatalf("got %d lines, want %d", len(got.Lines), tt.wantCount)
			}
			if got.HasMore != tt.wantMore {
				t.Errorf("HasMore = %v, want %v", got.HasMore, tt.wantMore)
			}
			if got.Lines[0].Text != tt.wantFirst {
				t.Errorf("first line = %q, want %q", got.Lines[0].Text, tt.wantFirst)
			}
			if got.Lines[0].LineNumber != tt.startLine {
				t.Errorf("first LineNumber = %d, want %d", got.Lines[0].LineNumber, tt.startLine)
			}
		})
	}
}

func TestReadLines_ZipFile(t *testing.T) {
	dir := t.TempDir()
	content := buildCompressedContent()
	entries := map[string]string{"test.log": content}
	path := createZipFile(t, dir, "test.zip", entries)

	tests := []struct {
		name      string
		startLine int
		numLines  int
		wantCount int
		wantMore  bool
		wantFirst string
	}{
		{
			name:      "read all lines",
			startLine: 1,
			numLines:  20,
			wantCount: 10,
			wantMore:  false,
			wantFirst: "line 1",
		},
		{
			name:      "read first 3 lines",
			startLine: 1,
			numLines:  3,
			wantCount: 3,
			wantMore:  true,
			wantFirst: "line 1",
		},
		{
			name:      "read from middle",
			startLine: 5,
			numLines:  3,
			wantCount: 3,
			wantMore:  true,
			wantFirst: "line 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadLines(path, tt.startLine, tt.numLines)
			if err != nil {
				t.Fatalf("ReadLines() error = %v", err)
			}
			if len(got.Lines) != tt.wantCount {
				t.Fatalf("got %d lines, want %d", len(got.Lines), tt.wantCount)
			}
			if got.HasMore != tt.wantMore {
				t.Errorf("HasMore = %v, want %v", got.HasMore, tt.wantMore)
			}
			if got.Lines[0].Text != tt.wantFirst {
				t.Errorf("first line = %q, want %q", got.Lines[0].Text, tt.wantFirst)
			}
			if got.Lines[0].LineNumber != tt.startLine {
				t.Errorf("first LineNumber = %d, want %d", got.Lines[0].LineNumber, tt.startLine)
			}
		})
	}
}

func TestReadLinesUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noperm.log")
	if err := os.WriteFile(path, []byte("content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Remove read permission.
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(path, 0644) // restore so cleanup can remove
	})

	_, err := ReadLines(path, 1, 10)
	if err == nil {
		t.Fatal("expected error for unreadable file, got nil")
	}
}

func TestReadLinesMixedLengthLines(t *testing.T) {
	// Mix of normal and huge lines to verify fallback handles all correctly.
	normalA := "short line A"
	huge := strings.Repeat("B", maxScannerBuf+100)
	normalC := "short line C"
	content := normalA + "\n" + huge + "\n" + normalC + "\n"
	path := writeTempFile(t, "mixed.log", content)

	got, err := ReadLines(path, 1, 10)
	if err != nil {
		t.Fatalf("ReadLines() error = %v", err)
	}
	if len(got.Lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(got.Lines))
	}
	if got.Lines[0].Text != normalA {
		t.Errorf("line 1 text = %q, want %q", got.Lines[0].Text, normalA)
	}
	if got.Lines[1].Text != huge {
		t.Errorf("line 2 length = %d, want %d", len(got.Lines[1].Text), len(huge))
	}
	if got.Lines[2].Text != normalC {
		t.Errorf("line 3 text = %q, want %q", got.Lines[2].Text, normalC)
	}
}

func TestReadLinesNoTrailingNewline(t *testing.T) {
	// Multiple lines, last one has no trailing newline.
	content := "first\nsecond\nthird"
	path := writeTempFile(t, "notail.log", content)

	got, err := ReadLines(path, 1, 10)
	if err != nil {
		t.Fatalf("ReadLines() error = %v", err)
	}
	if len(got.Lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(got.Lines))
	}
	if got.Lines[2].Text != "third" {
		t.Errorf("last line = %q, want %q", got.Lines[2].Text, "third")
	}
	if got.HasMore {
		t.Error("HasMore = true, want false")
	}
}
