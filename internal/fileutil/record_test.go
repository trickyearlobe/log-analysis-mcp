package fileutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRecordScannerBasic(t *testing.T) {
	sep := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

	tests := []struct {
		name    string
		content string
		want    []Record
	}{
		{
			name:    "empty file",
			content: "",
			want:    nil,
		},
		{
			name:    "single line matching separator",
			content: "2025-01-15 ERROR something broke\n",
			want: []Record{
				{StartLine: 1, LineCount: 1, Text: "2025-01-15 ERROR something broke"},
			},
		},
		{
			name: "multi-line record with stack trace",
			content: strings.Join([]string{
				"2025-01-15 ERROR NullPointerException",
				"\tat com.example.Foo.bar(Foo.java:42)",
				"\tat com.example.Main.main(Main.java:10)",
				"2025-01-15 INFO Recovery complete",
			}, "\n") + "\n",
			want: []Record{
				{StartLine: 1, LineCount: 3, Text: "2025-01-15 ERROR NullPointerException\n\tat com.example.Foo.bar(Foo.java:42)\n\tat com.example.Main.main(Main.java:10)"},
				{StartLine: 4, LineCount: 1, Text: "2025-01-15 INFO Recovery complete"},
			},
		},
		{
			name: "lines before first match are individual records",
			content: strings.Join([]string{
				"preamble line 1",
				"preamble line 2",
				"2025-01-15 ERROR first real entry",
				"\tcontinuation",
				"2025-01-15 INFO second entry",
			}, "\n") + "\n",
			want: []Record{
				{StartLine: 1, LineCount: 1, Text: "preamble line 1"},
				{StartLine: 2, LineCount: 1, Text: "preamble line 2"},
				{StartLine: 3, LineCount: 2, Text: "2025-01-15 ERROR first real entry\n\tcontinuation"},
				{StartLine: 5, LineCount: 1, Text: "2025-01-15 INFO second entry"},
			},
		},
		{
			name: "separator never matches — each line is individual",
			content: strings.Join([]string{
				"no timestamp here",
				"also no timestamp",
				"still none",
			}, "\n") + "\n",
			want: []Record{
				{StartLine: 1, LineCount: 1, Text: "no timestamp here"},
				{StartLine: 2, LineCount: 1, Text: "also no timestamp"},
				{StartLine: 3, LineCount: 1, Text: "still none"},
			},
		},
		{
			name: "separator matches every line",
			content: strings.Join([]string{
				"2025-01-15 first",
				"2025-01-16 second",
				"2025-01-17 third",
			}, "\n") + "\n",
			want: []Record{
				{StartLine: 1, LineCount: 1, Text: "2025-01-15 first"},
				{StartLine: 2, LineCount: 1, Text: "2025-01-16 second"},
				{StartLine: 3, LineCount: 1, Text: "2025-01-17 third"},
			},
		},
		{
			name:    "no trailing newline",
			content: "2025-01-15 only entry",
			want: []Record{
				{StartLine: 1, LineCount: 1, Text: "2025-01-15 only entry"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestFile(t, "test.log", tt.content)
			rs, err := NewRecordScanner(path, sep)
			if err != nil {
				t.Fatal(err)
			}
			defer rs.Close()

			var got []Record
			for rs.Scan() {
				got = append(got, rs.Record())
			}
			if err := rs.Err(); err != nil {
				t.Fatal(err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("got %d records, want %d\ngot: %+v", len(got), len(tt.want), got)
			}
			for i, w := range tt.want {
				g := got[i]
				if g.StartLine != w.StartLine {
					t.Errorf("record[%d].StartLine = %d, want %d", i, g.StartLine, w.StartLine)
				}
				if g.LineCount != w.LineCount {
					t.Errorf("record[%d].LineCount = %d, want %d", i, g.LineCount, w.LineCount)
				}
				if g.Text != w.Text {
					t.Errorf("record[%d].Text = %q, want %q", i, g.Text, w.Text)
				}
				if g.Truncated != w.Truncated {
					t.Errorf("record[%d].Truncated = %v, want %v", i, g.Truncated, w.Truncated)
				}
			}
		})
	}
}

func TestRecordScannerTruncation(t *testing.T) {
	// Build a record that exceeds MaxRecordLines (500).
	sep := regexp.MustCompile(`^START`)
	var lines []string
	lines = append(lines, "START record one")
	for i := 0; i < 600; i++ {
		lines = append(lines, "\tcontinuation line")
	}
	lines = append(lines, "START record two")
	content := strings.Join(lines, "\n") + "\n"

	path := writeTestFile(t, "big.log", content)
	rs, err := NewRecordScanner(path, sep)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()

	var records []Record
	for rs.Scan() {
		records = append(records, rs.Record())
	}
	if err := rs.Err(); err != nil {
		t.Fatal(err)
	}

	// First record should be truncated at MaxRecordLines.
	if len(records) < 2 {
		t.Fatalf("expected at least 2 records, got %d", len(records))
	}

	first := records[0]
	if first.LineCount != MaxRecordLines {
		t.Errorf("truncated record LineCount = %d, want %d", first.LineCount, MaxRecordLines)
	}
	if !first.Truncated {
		t.Error("expected first record to be Truncated")
	}

	// Last record should be "START record two".
	last := records[len(records)-1]
	if last.Text != "START record two" {
		t.Errorf("last record Text = %q, want %q", last.Text, "START record two")
	}
	if last.Truncated {
		t.Error("last record should not be Truncated")
	}
}

func TestRecordScannerByteTruncation(t *testing.T) {
	// Build a record that exceeds MaxRecordBytes (64KB).
	sep := regexp.MustCompile(`^START`)
	bigLine := strings.Repeat("x", 10000) // 10KB per line
	var lines []string
	lines = append(lines, "START big record")
	for i := 0; i < 10; i++ { // 100KB total
		lines = append(lines, bigLine)
	}
	lines = append(lines, "START next record")
	content := strings.Join(lines, "\n") + "\n"

	path := writeTestFile(t, "bigbytes.log", content)
	rs, err := NewRecordScanner(path, sep)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()

	var records []Record
	for rs.Scan() {
		records = append(records, rs.Record())
	}
	if err := rs.Err(); err != nil {
		t.Fatal(err)
	}

	if len(records) < 2 {
		t.Fatalf("expected at least 2 records, got %d", len(records))
	}

	first := records[0]
	if !first.Truncated {
		t.Error("expected first record to be Truncated (byte limit)")
	}
	if len(first.Text) > MaxRecordBytes+10000 {
		t.Errorf("truncated record text too large: %d bytes", len(first.Text))
	}

	last := records[len(records)-1]
	if last.Text != "START next record" {
		t.Errorf("last record Text = %q, want %q", last.Text, "START next record")
	}
}

func TestRecordScannerCloseBeforeEOF(t *testing.T) {
	sep := regexp.MustCompile(`^START`)
	content := "START one\nSTART two\nSTART three\n"
	path := writeTestFile(t, "close.log", content)

	rs, err := NewRecordScanner(path, sep)
	if err != nil {
		t.Fatal(err)
	}

	// Read only one record then close.
	if !rs.Scan() {
		t.Fatal("expected first Scan to succeed")
	}
	r := rs.Record()
	if r.Text != "START one" {
		t.Errorf("first record = %q, want %q", r.Text, "START one")
	}

	// Close early.
	if err := rs.Close(); err != nil {
		t.Fatal(err)
	}

	// Subsequent Scan should return false.
	if rs.Scan() {
		t.Error("Scan after Close should return false")
	}
}

func TestRecordScannerDoubleClose(t *testing.T) {
	sep := regexp.MustCompile(`^START`)
	path := writeTestFile(t, "double.log", "START one\n")

	rs, err := NewRecordScanner(path, sep)
	if err != nil {
		t.Fatal(err)
	}

	if err := rs.Close(); err != nil {
		t.Fatal(err)
	}
	// Second close should not panic.
	if err := rs.Close(); err != nil {
		t.Fatal(err)
	}
}
