package tools

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSummaryTempLog creates a temporary log file and returns its path.
func writeSummaryTempLog(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestRunSummarizeLogs(t *testing.T) {
	// JSON log with mixed levels across several minutes.
	mixedLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","source":"auth","message":"User login"}`,
		`{"timestamp":"2025-01-15T10:00:30Z","level":"INFO","source":"auth","message":"Token issued"}`,
		`{"timestamp":"2025-01-15T10:01:00Z","level":"DEBUG","source":"db","message":"Query executed"}`,
		`{"timestamp":"2025-01-15T10:01:30Z","level":"WARN","source":"cache","message":"Cache miss"}`,
		`{"timestamp":"2025-01-15T10:02:00Z","level":"ERROR","source":"api","message":"Connection timeout"}`,
		`{"timestamp":"2025-01-15T10:02:30Z","level":"ERROR","source":"api","message":"Connection timeout"}`,
		`{"timestamp":"2025-01-15T10:03:00Z","level":"INFO","source":"scheduler","message":"Job completed"}`,
		`{"timestamp":"2025-01-15T10:03:30Z","level":"ERROR","source":"db","message":"Deadlock detected"}`,
		`{"timestamp":"2025-01-15T10:04:00Z","level":"FATAL","source":"core","message":"Out of memory"}`,
		`{"timestamp":"2025-01-15T10:04:30Z","level":"INFO","source":"auth","message":"User logout"}`,
	}, "\n") + "\n"
	mixedPath := writeSummaryTempLog(t, "mixed.log", mixedLog)

	emptyPath := writeSummaryTempLog(t, "empty.log", "")

	// Log for sampling test — 20 lines.
	var sampleLines []string
	for i := 0; i < 20; i++ {
		sampleLines = append(sampleLines,
			`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","source":"app","message":"line"}`)
	}
	sampleLog := strings.Join(sampleLines, "\n") + "\n"
	samplePath := writeSummaryTempLog(t, "sample.log", sampleLog)

	// Log with many sources to test top-10 capping.
	sourceNames := []string{
		"alpha", "bravo", "charlie", "delta", "echo",
		"foxtrot", "golf", "hotel", "india", "juliet",
		"kilo", "lima",
	}
	var manySourceLines []string
	for i, name := range sourceNames {
		// Give different counts: alpha appears 12 times, bravo 11, etc.
		for j := 0; j < 12-i; j++ {
			manySourceLines = append(manySourceLines,
				`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","source":"`+name+`","message":"msg"}`)
		}
	}
	manySourceLog := strings.Join(manySourceLines, "\n") + "\n"
	manySourcePath := writeSummaryTempLog(t, "manysource.log", manySourceLog)

	// Log with multiple error messages to verify top errors.
	var errorLines []string
	for i := 0; i < 5; i++ {
		errorLines = append(errorLines,
			`{"timestamp":"2025-01-15T10:00:00Z","level":"ERROR","source":"svc","message":"Timeout error"}`)
	}
	for i := 0; i < 3; i++ {
		errorLines = append(errorLines,
			`{"timestamp":"2025-01-15T10:00:00Z","level":"ERROR","source":"svc","message":"Auth failed"}`)
	}
	errorLines = append(errorLines,
		`{"timestamp":"2025-01-15T10:00:00Z","level":"FATAL","source":"svc","message":"Crash"}`)
	errorLog := strings.Join(errorLines, "\n") + "\n"
	errorPath := writeSummaryTempLog(t, "errors.log", errorLog)

	// Log spanning exactly 2 hours for throughput testing.
	throughputLog := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","source":"app","message":"start"}`,
		`{"timestamp":"2025-01-15T10:00:30Z","level":"INFO","source":"app","message":"mid1"}`,
		`{"timestamp":"2025-01-15T10:01:00Z","level":"INFO","source":"app","message":"mid2"}`,
		`{"timestamp":"2025-01-15T10:01:30Z","level":"INFO","source":"app","message":"mid3"}`,
		`{"timestamp":"2025-01-15T12:00:00Z","level":"INFO","source":"app","message":"end"}`,
	}, "\n") + "\n"
	throughputPath := writeSummaryTempLog(t, "throughput.log", throughputLog)

	tests := []struct {
		name        string
		input       SummarizeLogsInput
		wantErr     bool
		errContains string
		checkOutput func(t *testing.T, out SummarizeLogsOutput)
	}{
		{
			name:  "mixed JSON levels produces correct distribution",
			input: SummarizeLogsInput{Path: mixedPath},
			checkOutput: func(t *testing.T, out SummarizeLogsOutput) {
				if out.DetectedFormat != "json" {
					t.Errorf("DetectedFormat = %q, want %q", out.DetectedFormat, "json")
				}
				if out.LinesAnalyzed != 10 {
					t.Errorf("LinesAnalyzed = %d, want 10", out.LinesAnalyzed)
				}
				if out.Sampled {
					t.Error("Sampled should be false when SampleSize=0")
				}

				// Check level counts.
				wantLevels := map[string]int{
					"INFO":  4,
					"DEBUG": 1,
					"WARN":  1,
					"ERROR": 3,
					"FATAL": 1,
				}
				for level, wantCount := range wantLevels {
					got, ok := out.LevelDistribution[level]
					if !ok {
						t.Errorf("missing level %q in distribution", level)
						continue
					}
					if got.Count != wantCount {
						t.Errorf("level %q count = %d, want %d", level, got.Count, wantCount)
					}
					if got.Percentage <= 0 {
						t.Errorf("level %q percentage should be > 0, got %f", level, got.Percentage)
					}
				}

				// Percentages should sum to approximately 100.
				totalPct := 0.0
				for _, ls := range out.LevelDistribution {
					totalPct += ls.Percentage
				}
				if totalPct < 99.0 || totalPct > 101.0 {
					t.Errorf("level percentages sum to %.1f, want ~100", totalPct)
				}
			},
		},
		{
			name:  "top sources and top errors populated",
			input: SummarizeLogsInput{Path: mixedPath},
			checkOutput: func(t *testing.T, out SummarizeLogsOutput) {
				if len(out.TopSources) == 0 {
					t.Fatal("TopSources should not be empty")
				}
				// auth appears 3 times, should be first or second.
				found := false
				for _, sc := range out.TopSources {
					if sc.Source == "auth" && sc.Count == 3 {
						found = true
						break
					}
				}
				if !found {
					t.Error("expected source 'auth' with count 3 in TopSources")
				}

				if len(out.TopErrors) == 0 {
					t.Fatal("TopErrors should not be empty")
				}
				// "Connection timeout" appears twice.
				if out.TopErrors[0].Message != "Connection timeout" || out.TopErrors[0].Count != 2 {
					t.Errorf("top error = %+v, want {Connection timeout, 2}", out.TopErrors[0])
				}
			},
		},
		{
			name:  "empty file returns zero counts",
			input: SummarizeLogsInput{Path: emptyPath},
			checkOutput: func(t *testing.T, out SummarizeLogsOutput) {
				if out.LinesAnalyzed != 0 {
					t.Errorf("LinesAnalyzed = %d, want 0", out.LinesAnalyzed)
				}
				if out.FileInfo.TotalLines != 0 {
					t.Errorf("TotalLines = %d, want 0", out.FileInfo.TotalLines)
				}
				if len(out.LevelDistribution) != 0 {
					t.Errorf("LevelDistribution should be empty, got %v", out.LevelDistribution)
				}
				if len(out.TopSources) != 0 {
					t.Errorf("TopSources should be empty, got %v", out.TopSources)
				}
				if len(out.TopErrors) != 0 {
					t.Errorf("TopErrors should be empty, got %v", out.TopErrors)
				}
				if out.FileInfo.TimeRange != nil {
					t.Error("TimeRange should be nil for empty file")
				}
			},
		},
		{
			name:        "file not found returns error",
			input:       SummarizeLogsInput{Path: "/nonexistent/path/to/file.log"},
			wantErr:     true,
			errContains: "FILE_NOT_FOUND",
		},
		{
			name:  "SampleSize limits analysis",
			input: SummarizeLogsInput{Path: samplePath, SampleSize: 5},
			checkOutput: func(t *testing.T, out SummarizeLogsOutput) {
				if out.LinesAnalyzed != 5 {
					t.Errorf("LinesAnalyzed = %d, want 5", out.LinesAnalyzed)
				}
				if !out.Sampled {
					t.Error("Sampled should be true when SampleSize is set")
				}
			},
		},
		{
			name:  "SampleSize larger than file reads all lines",
			input: SummarizeLogsInput{Path: samplePath, SampleSize: 1000},
			checkOutput: func(t *testing.T, out SummarizeLogsOutput) {
				if out.LinesAnalyzed != 20 {
					t.Errorf("LinesAnalyzed = %d, want 20", out.LinesAnalyzed)
				}
				// File has fewer lines than sample size, so not marked sampled.
				if out.Sampled {
					t.Error("Sampled should be false when file is smaller than SampleSize")
				}
			},
		},
		{
			name:  "TimeRange computed correctly",
			input: SummarizeLogsInput{Path: mixedPath},
			checkOutput: func(t *testing.T, out SummarizeLogsOutput) {
				if out.FileInfo.TimeRange == nil {
					t.Fatal("TimeRange should not be nil")
				}
				tr := out.FileInfo.TimeRange
				if tr.Earliest != "2025-01-15T10:00:00Z" {
					t.Errorf("Earliest = %q, want %q", tr.Earliest, "2025-01-15T10:00:00Z")
				}
				if tr.Latest != "2025-01-15T10:04:30Z" {
					t.Errorf("Latest = %q, want %q", tr.Latest, "2025-01-15T10:04:30Z")
				}
				// 4.5 minutes = 0.075 hours.
				wantHours := 0.08 // rounded to 2 decimal places
				if math.Abs(tr.DurationHours-wantHours) > 0.01 {
					t.Errorf("DurationHours = %f, want ~%f", tr.DurationHours, wantHours)
				}
			},
		},
		{
			name:  "throughput metrics computed",
			input: SummarizeLogsInput{Path: throughputPath},
			checkOutput: func(t *testing.T, out SummarizeLogsOutput) {
				if out.FileInfo.TimeRange == nil {
					t.Fatal("TimeRange should not be nil")
				}
				if out.FileInfo.TimeRange.DurationHours != 2.0 {
					t.Errorf("DurationHours = %f, want 2.0", out.FileInfo.TimeRange.DurationHours)
				}
				// 5 lines / 120 minutes = 0.0 rounded... let's check it's positive.
				if out.Throughput.LinesPerMinute <= 0 {
					t.Errorf("LinesPerMinute should be > 0, got %f", out.Throughput.LinesPerMinute)
				}
				// Peak minute: 10:00 has 2 lines, 10:01 has 2, 12:00 has 1.
				// Whichever has the max count should be peak.
				if out.Throughput.PeakMinute.Count < 1 {
					t.Errorf("PeakMinute count should be >= 1, got %d", out.Throughput.PeakMinute.Count)
				}
				if out.Throughput.PeakMinute.Timestamp == "" {
					t.Error("PeakMinute timestamp should not be empty")
				}
				if out.Throughput.QuietestMinute.Count < 1 {
					t.Errorf("QuietestMinute count should be >= 1, got %d", out.Throughput.QuietestMinute.Count)
				}
				if out.Throughput.QuietestMinute.Timestamp == "" {
					t.Error("QuietestMinute timestamp should not be empty")
				}
			},
		},
		{
			name:  "top sources capped at 10",
			input: SummarizeLogsInput{Path: manySourcePath},
			checkOutput: func(t *testing.T, out SummarizeLogsOutput) {
				if len(out.TopSources) != 10 {
					t.Errorf("TopSources length = %d, want 10", len(out.TopSources))
				}
				// First source should be the one with most entries.
				if out.TopSources[0].Source != "alpha" {
					t.Errorf("first source = %q, want %q", out.TopSources[0].Source, "alpha")
				}
				// Verify descending order.
				for i := 1; i < len(out.TopSources); i++ {
					if out.TopSources[i].Count > out.TopSources[i-1].Count {
						t.Errorf("TopSources not sorted descending at index %d: %d > %d",
							i, out.TopSources[i].Count, out.TopSources[i-1].Count)
					}
				}
			},
		},
		{
			name:  "top errors includes FATAL and CRITICAL messages",
			input: SummarizeLogsInput{Path: errorPath},
			checkOutput: func(t *testing.T, out SummarizeLogsOutput) {
				if len(out.TopErrors) != 3 {
					t.Errorf("TopErrors length = %d, want 3", len(out.TopErrors))
				}
				// Sorted by count desc: Timeout(5), Auth(3), Crash(1).
				if out.TopErrors[0].Message != "Timeout error" || out.TopErrors[0].Count != 5 {
					t.Errorf("first error = %+v, want {Timeout error, 5}", out.TopErrors[0])
				}
				if out.TopErrors[1].Message != "Auth failed" || out.TopErrors[1].Count != 3 {
					t.Errorf("second error = %+v, want {Auth failed, 3}", out.TopErrors[1])
				}
				// FATAL message should be included.
				if out.TopErrors[2].Message != "Crash" || out.TopErrors[2].Count != 1 {
					t.Errorf("third error = %+v, want {Crash, 1}", out.TopErrors[2])
				}
			},
		},
		{
			name:  "file info populated correctly",
			input: SummarizeLogsInput{Path: mixedPath},
			checkOutput: func(t *testing.T, out SummarizeLogsOutput) {
				if out.FileInfo.Name != "mixed.log" {
					t.Errorf("Name = %q, want %q", out.FileInfo.Name, "mixed.log")
				}
				if out.FileInfo.Path != mixedPath {
					t.Errorf("Path = %q, want %q", out.FileInfo.Path, mixedPath)
				}
				if out.FileInfo.SizeBytes <= 0 {
					t.Errorf("SizeBytes should be > 0, got %d", out.FileInfo.SizeBytes)
				}
				if out.FileInfo.SizeHuman == "" {
					t.Error("SizeHuman should not be empty")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := RunSummarizeLogs(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
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

func TestFormatSizeHuman(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{name: "zero bytes", bytes: 0, want: "0 B"},
		{name: "small bytes", bytes: 512, want: "512 B"},
		{name: "one byte", bytes: 1, want: "1 B"},
		{name: "just under 1KB", bytes: 1023, want: "1023 B"},
		{name: "exactly 1KB", bytes: 1024, want: "1.0 KB"},
		{name: "kilobytes", bytes: 2560, want: "2.5 KB"},
		{name: "just under 1MB", bytes: 1048575, want: "1024.0 KB"},
		{name: "exactly 1MB", bytes: 1048576, want: "1.0 MB"},
		{name: "megabytes", bytes: 4718592, want: "4.5 MB"},
		{name: "just under 1GB", bytes: 1073741823, want: "1024.0 MB"},
		{name: "exactly 1GB", bytes: 1073741824, want: "1.0 GB"},
		{name: "gigabytes", bytes: 2684354560, want: "2.5 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSizeHuman(tt.bytes)
			if got != tt.want {
				t.Errorf("formatSizeHuman(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestTruncateToMinute(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "full RFC3339", input: "2025-01-15T10:30:45Z", want: "2025-01-15T10:30"},
		{name: "with millis", input: "2025-01-15T10:30:45.123Z", want: "2025-01-15T10:30"},
		{name: "short string", input: "2025-01-15", want: ""},
		{name: "exactly 16 chars", input: "2025-01-15T10:30", want: "2025-01-15T10:30"},
		{name: "empty string", input: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateToMinute(tt.input)
			if got != tt.want {
				t.Errorf("truncateToMinute(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
