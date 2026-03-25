package fileutil

import (
	"strings"
	"testing"
)

func TestTailLines(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		numLines   int
		wantLines  []LineRecord
		wantTotal  int
		wantSize   int64
		wantErr    bool
	}{
		{
			name:     "basic tail last 2 of 5",
			content:  "a\nb\nc\nd\ne\n",
			numLines: 2,
			wantLines: []LineRecord{
				{LineNumber: 4, Text: "d"},
				{LineNumber: 5, Text: "e"},
			},
			wantTotal: 5,
			wantSize:  10,
		},
		{
			name:     "tail all lines",
			content:  "a\nb\nc\n",
			numLines: 10,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "a"},
				{LineNumber: 2, Text: "b"},
				{LineNumber: 3, Text: "c"},
			},
			wantTotal: 3,
			wantSize:  6,
		},
		{
			name:     "tail exact count",
			content:  "a\nb\nc\n",
			numLines: 3,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "a"},
				{LineNumber: 2, Text: "b"},
				{LineNumber: 3, Text: "c"},
			},
			wantTotal: 3,
			wantSize:  6,
		},
		{
			name:      "empty file",
			content:   "",
			numLines:  10,
			wantLines: []LineRecord{},
			wantTotal: 0,
			wantSize:  0,
		},
		{
			name:     "single line no trailing newline",
			content:  "only line",
			numLines: 10,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "only line"},
			},
			wantTotal: 1,
			wantSize:  9,
		},
		{
			name:     "single line with trailing newline",
			content:  "only line\n",
			numLines: 10,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "only line"},
			},
			wantTotal: 1,
			wantSize:  10,
		},
		{
			name:     "no trailing newline multi line",
			content:  "first\nsecond\nthird",
			numLines: 2,
			wantLines: []LineRecord{
				{LineNumber: 2, Text: "second"},
				{LineNumber: 3, Text: "third"},
			},
			wantTotal: 3,
			wantSize:  18,
		},
		{
			name:     "carriage return line endings",
			content:  "alpha\r\nbeta\r\ngamma\r\n",
			numLines: 2,
			wantLines: []LineRecord{
				{LineNumber: 2, Text: "beta"},
				{LineNumber: 3, Text: "gamma"},
			},
			wantTotal: 3,
			wantSize:  20,
		},
		{
			name:     "file with blank lines",
			content:  "a\n\nb\n\nc\n",
			numLines: 3,
			wantLines: []LineRecord{
				{LineNumber: 3, Text: "b"},
				{LineNumber: 4, Text: ""},
				{LineNumber: 5, Text: "c"},
			},
			wantTotal: 5,
			wantSize:  8,
		},
		{
			name:     "tail 1 line",
			content:  "a\nb\nc\n",
			numLines: 1,
			wantLines: []LineRecord{
				{LineNumber: 3, Text: "c"},
			},
			wantTotal: 3,
			wantSize:  6,
		},
		{
			name:     "only newline characters",
			content:  "\n\n\n",
			numLines: 2,
			wantLines: []LineRecord{
				{LineNumber: 2, Text: ""},
				{LineNumber: 3, Text: ""},
			},
			wantTotal: 3,
			wantSize:  3,
		},
		{
			name:     "default numLines when zero",
			content:  "a\nb\nc\n",
			numLines: 0,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "a"},
				{LineNumber: 2, Text: "b"},
				{LineNumber: 3, Text: "c"},
			},
			wantTotal: 3,
			wantSize:  6,
		},
		{
			name:     "default numLines when negative",
			content:  "a\nb\n",
			numLines: -5,
			wantLines: []LineRecord{
				{LineNumber: 1, Text: "a"},
				{LineNumber: 2, Text: "b"},
			},
			wantTotal: 2,
			wantSize:  4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTempFile(t, "tail_test.log", tt.content)

			got, err := TailLines(path, tt.numLines)
			if (err != nil) != tt.wantErr {
				t.Fatalf("TailLines() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if got.TotalLines != tt.wantTotal {
				t.Errorf("TotalLines = %d, want %d", got.TotalLines, tt.wantTotal)
			}
			if got.FileSize != tt.wantSize {
				t.Errorf("FileSize = %d, want %d", got.FileSize, tt.wantSize)
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

func TestTailLinesFileNotFound(t *testing.T) {
	_, err := TailLines("/nonexistent/path/to/file.log", 10)
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestTailLinesLargerThanChunk(t *testing.T) {
	// Create a file larger than 8 KB (one chunk) to exercise multi-chunk backward reading.
	var b strings.Builder
	lineCount := 0
	for b.Len() < tailChunkSize*3 {
		lineCount++
		b.WriteString("log entry for line number that adds some length to each line\n")
	}
	path := writeTempFile(t, "large_tail.log", b.String())

	got, err := TailLines(path, 5)
	if err != nil {
		t.Fatalf("TailLines() error = %v", err)
	}

	if len(got.Lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(got.Lines))
	}

	// Lines should be in chronological order (earliest first).
	for i := 1; i < len(got.Lines); i++ {
		if got.Lines[i].LineNumber <= got.Lines[i-1].LineNumber {
			t.Errorf("lines not in order: line[%d].LineNumber=%d <= line[%d].LineNumber=%d",
				i, got.Lines[i].LineNumber, i-1, got.Lines[i-1].LineNumber)
		}
	}

	// Last line should be the last line of the file.
	if got.Lines[4].LineNumber != got.TotalLines {
		t.Errorf("last line number = %d, want %d (total)", got.Lines[4].LineNumber, got.TotalLines)
	}

	// When the tail reader doesn't reach the start of the file, TotalLines is
	// a best-effort estimate. Just verify it's at least as large as numLines.
	if got.TotalLines < 5 {
		t.Errorf("TotalLines = %d, want >= 5", got.TotalLines)
	}
}

func TestTailLinesSmallFile(t *testing.T) {
	// File smaller than a single chunk (< 8 KB).
	content := "alpha\nbeta\ngamma\n"
	path := writeTempFile(t, "small.log", content)

	got, err := TailLines(path, 2)
	if err != nil {
		t.Fatalf("TailLines() error = %v", err)
	}

	if len(got.Lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(got.Lines))
	}
	if got.Lines[0].Text != "beta" {
		t.Errorf("line[0].Text = %q, want %q", got.Lines[0].Text, "beta")
	}
	if got.Lines[1].Text != "gamma" {
		t.Errorf("line[1].Text = %q, want %q", got.Lines[1].Text, "gamma")
	}
	if got.Lines[0].LineNumber != 2 {
		t.Errorf("line[0].LineNumber = %d, want 2", got.Lines[0].LineNumber)
	}
	if got.Lines[1].LineNumber != 3 {
		t.Errorf("line[1].LineNumber = %d, want 3", got.Lines[1].LineNumber)
	}
}

func TestTailLinesChronologicalOrder(t *testing.T) {
	// Verify returned lines are in chronological order, not reversed.
	content := "first\nsecond\nthird\nfourth\nfifth\n"
	path := writeTempFile(t, "order.log", content)

	got, err := TailLines(path, 3)
	if err != nil {
		t.Fatalf("TailLines() error = %v", err)
	}

	if len(got.Lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(got.Lines))
	}
	if got.Lines[0].Text != "third" {
		t.Errorf("line[0].Text = %q, want %q", got.Lines[0].Text, "third")
	}
	if got.Lines[1].Text != "fourth" {
		t.Errorf("line[1].Text = %q, want %q", got.Lines[1].Text, "fourth")
	}
	if got.Lines[2].Text != "fifth" {
		t.Errorf("line[2].Text = %q, want %q", got.Lines[2].Text, "fifth")
	}
}

func TestTailLinesSingleNewline(t *testing.T) {
	// File is a single newline character.
	path := writeTempFile(t, "newline.log", "\n")

	got, err := TailLines(path, 10)
	if err != nil {
		t.Fatalf("TailLines() error = %v", err)
	}

	if len(got.Lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(got.Lines))
	}
	if got.Lines[0].Text != "" {
		t.Errorf("line[0].Text = %q, want empty string", got.Lines[0].Text)
	}
	if got.Lines[0].LineNumber != 1 {
		t.Errorf("line[0].LineNumber = %d, want 1", got.Lines[0].LineNumber)
	}
	if got.TotalLines != 1 {
		t.Errorf("TotalLines = %d, want 1", got.TotalLines)
	}
}

func TestTailLinesExactChunkBoundary(t *testing.T) {
	// Create content that is exactly one chunk size to test boundary conditions.
	line := "abcdefghij" // 10 bytes + newline = 11 bytes per line
	linesNeeded := tailChunkSize / 11
	var b strings.Builder
	for i := 0; i < linesNeeded; i++ {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	// Pad to exactly tailChunkSize bytes.
	for b.Len() < tailChunkSize {
		b.WriteByte('X')
	}
	// Trim to exactly tailChunkSize.
	content := b.String()[:tailChunkSize]

	path := writeTempFile(t, "boundary.log", content)

	got, err := TailLines(path, 3)
	if err != nil {
		t.Fatalf("TailLines() error = %v", err)
	}

	if len(got.Lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(got.Lines))
	}

	if got.FileSize != int64(tailChunkSize) {
		t.Errorf("FileSize = %d, want %d", got.FileSize, tailChunkSize)
	}
}

func TestTailLinesUnreadableFile(t *testing.T) {
	path := writeTempFile(t, "noperm_tail.log", "content\n")
	if err := setUnreadable(path); err != nil {
		t.Skip("cannot remove read permissions on this OS")
	}
	t.Cleanup(func() { restoreReadable(path) })

	_, err := TailLines(path, 10)
	if err == nil {
		t.Fatal("expected error for unreadable file, got nil")
	}
}

// setUnreadable removes read permission from a file. Returns an error if it fails.
func setUnreadable(path string) error {
	return chmod0(path)
}

// restoreReadable restores read permission on a file.
func restoreReadable(path string) {
	chmod644(path)
}
