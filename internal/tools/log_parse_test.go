package tools

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRunParseLogs(t *testing.T) {
	dir := t.TempDir()

	// --- Prepare test files ---

	jsonLines := []string{
		`{"timestamp":"2025-01-15T10:30:00Z","level":"info","msg":"server started","service":"api"}`,
		`{"timestamp":"2025-01-15T10:30:01Z","level":"error","msg":"connection failed","service":"db"}`,
		`{"timestamp":"2025-01-15T10:30:02Z","level":"warn","message":"disk usage high","component":"monitor"}`,
	}
	jsonPath := writeTestFile(t, dir, "app.json.log", jsonLines)

	syslogLines := []string{
		`<134>Jan 15 10:30:00 webserver01 nginx[1234]: GET /index.html 200`,
		`<131>Jan 15 10:30:01 webserver01 nginx[1234]: POST /api/data 500`,
		`<134>Jan 15 10:30:02 webserver01 sshd[5678]: Accepted publickey for user1`,
	}
	syslogPath := writeTestFile(t, dir, "syslog.log", syslogLines)

	mixedLines := []string{
		`{"timestamp":"2025-01-15T10:30:00Z","level":"info","msg":"valid json"}`,
		`this is not json or syslog`,
		`{"timestamp":"2025-01-15T10:30:01Z","level":"error","msg":"another valid"}`,
		`another garbage line here`,
	}
	mixedPath := writeTestFile(t, dir, "mixed.log", mixedLines)

	emptyPath := writeTestFile(t, dir, "empty.log", []string{""})

	// File with only unparseable lines (not matching any format).
	garbageLines := []string{
		`just some random text`,
		`more random stuff`,
		`nothing parseable here`,
	}
	garbagePath := writeTestFile(t, dir, "garbage.log", garbageLines)

	// A larger file for pagination testing.
	bigJSON := make([]string, 100)
	for i := range bigJSON {
		bigJSON[i] = `{"timestamp":"2025-01-15T10:30:00Z","level":"info","msg":"line"}`
	}
	bigJSONPath := writeTestFile(t, dir, "big.json.log", bigJSON)

	tests := []struct {
		name        string
		input       ParseLogsInput
		wantErr     bool
		errContains string
		check       func(t *testing.T, out ParseLogsOutput)
	}{
		{
			name:  "parse JSON log lines",
			input: ParseLogsInput{Path: jsonPath},
			check: func(t *testing.T, out ParseLogsOutput) {
				if out.DetectedFormat != "json" {
					t.Errorf("expected detected_format=json, got %q", out.DetectedFormat)
				}
				if out.Confidence < 0.5 {
					t.Errorf("expected confidence >= 0.5, got %f", out.Confidence)
				}
				if out.TotalParsed != 3 {
					t.Errorf("expected 3 parsed records, got %d", out.TotalParsed)
				}
				if out.TotalErrors != 0 {
					t.Errorf("expected 0 errors, got %d", out.TotalErrors)
				}

				// Verify first record fields.
				r := out.Records[0]
				if r.LineNumber != 1 {
					t.Errorf("expected line_number=1, got %d", r.LineNumber)
				}
				if r.Timestamp == nil || *r.Timestamp != "2025-01-15T10:30:00Z" {
					t.Errorf("unexpected timestamp: %v", r.Timestamp)
				}
				if r.Level == nil || string(*r.Level) != "INFO" {
					t.Errorf("unexpected level: %v", r.Level)
				}
				if r.Source == nil || *r.Source != "api" {
					t.Errorf("expected source=api, got %v", r.Source)
				}
				if r.Message != "server started" {
					t.Errorf("expected message='server started', got %q", r.Message)
				}

				// Verify second record has ERROR level.
				r2 := out.Records[1]
				if r2.Level == nil || string(*r2.Level) != "ERROR" {
					t.Errorf("expected second record level=ERROR, got %v", r2.Level)
				}
			},
		},
		{
			name:  "parse syslog log lines",
			input: ParseLogsInput{Path: syslogPath},
			check: func(t *testing.T, out ParseLogsOutput) {
				if out.DetectedFormat != "syslog-rfc3164" {
					t.Errorf("expected detected_format=syslog-rfc3164, got %q", out.DetectedFormat)
				}
				if out.Confidence < 0.5 {
					t.Errorf("expected confidence >= 0.5, got %f", out.Confidence)
				}
				if out.TotalParsed != 3 {
					t.Errorf("expected 3 parsed records, got %d", out.TotalParsed)
				}
				if out.TotalErrors != 0 {
					t.Errorf("expected 0 errors, got %d", out.TotalErrors)
				}

				r := out.Records[0]
				if r.Timestamp == nil {
					t.Fatal("expected non-nil timestamp")
				}
				if r.Source == nil || *r.Source != "nginx" {
					t.Errorf("expected source=nginx, got %v", r.Source)
				}
				if r.Message != "GET /index.html 200" {
					t.Errorf("unexpected message: %q", r.Message)
				}
				if r.ExtraFields == nil {
					t.Fatal("expected extra_fields to be set")
				}
				if r.ExtraFields["hostname"] != "webserver01" {
					t.Errorf("expected hostname=webserver01, got %v", r.ExtraFields["hostname"])
				}
			},
		},
		{
			name:  "auto-detect format (JSON)",
			input: ParseLogsInput{Path: jsonPath, FormatHint: "auto"},
			check: func(t *testing.T, out ParseLogsOutput) {
				if out.DetectedFormat != "json" {
					t.Errorf("expected auto-detected format=json, got %q", out.DetectedFormat)
				}
				if out.TotalParsed != 3 {
					t.Errorf("expected 3 records, got %d", out.TotalParsed)
				}
			},
		},
		{
			name:  "format hint override to json on syslog file",
			input: ParseLogsInput{Path: syslogPath, FormatHint: "json"},
			check: func(t *testing.T, out ParseLogsOutput) {
				// Hint forces json parser, which cannot parse syslog lines.
				if out.DetectedFormat != "json" {
					t.Errorf("expected detected_format=json (forced by hint), got %q", out.DetectedFormat)
				}
				if out.Confidence != 1.0 {
					t.Errorf("expected confidence=1.0 for hint override, got %f", out.Confidence)
				}
				// All lines should fail JSON parsing.
				if out.TotalErrors != 3 {
					t.Errorf("expected 3 parse errors, got %d", out.TotalErrors)
				}
				if out.TotalParsed != 0 {
					t.Errorf("expected 0 parsed records, got %d", out.TotalParsed)
				}
			},
		},
		{
			name:  "format hint override to syslog-rfc3164 on syslog file",
			input: ParseLogsInput{Path: syslogPath, FormatHint: "syslog-rfc3164"},
			check: func(t *testing.T, out ParseLogsOutput) {
				if out.DetectedFormat != "syslog-rfc3164" {
					t.Errorf("expected detected_format=syslog-rfc3164, got %q", out.DetectedFormat)
				}
				if out.Confidence != 1.0 {
					t.Errorf("expected confidence=1.0 for hint override, got %f", out.Confidence)
				}
				if out.TotalParsed != 3 {
					t.Errorf("expected 3 parsed records, got %d", out.TotalParsed)
				}
			},
		},
		{
			name:  "lines that fail parsing produce ParseErrors",
			input: ParseLogsInput{Path: mixedPath, FormatHint: "json"},
			check: func(t *testing.T, out ParseLogsOutput) {
				if out.TotalParsed != 2 {
					t.Errorf("expected 2 parsed records, got %d", out.TotalParsed)
				}
				if out.TotalErrors != 2 {
					t.Errorf("expected 2 parse errors, got %d", out.TotalErrors)
				}
				if len(out.ParseErrors) != 2 {
					t.Fatalf("expected 2 ParseError entries, got %d", len(out.ParseErrors))
				}
				// First error should be line 2.
				if out.ParseErrors[0].LineNumber != 2 {
					t.Errorf("expected first error at line 2, got %d", out.ParseErrors[0].LineNumber)
				}
				if out.ParseErrors[0].Raw != "this is not json or syslog" {
					t.Errorf("unexpected raw content: %q", out.ParseErrors[0].Raw)
				}
				if out.ParseErrors[0].Error == "" {
					t.Error("expected non-empty error message")
				}
				// Second error should be line 4.
				if out.ParseErrors[1].LineNumber != 4 {
					t.Errorf("expected second error at line 4, got %d", out.ParseErrors[1].LineNumber)
				}
			},
		},
		{
			name:  "empty file",
			input: ParseLogsInput{Path: emptyPath},
			check: func(t *testing.T, out ParseLogsOutput) {
				if out.DetectedFormat != "unknown" {
					t.Errorf("expected detected_format=unknown for empty file, got %q", out.DetectedFormat)
				}
				if out.TotalParsed != 0 {
					t.Errorf("expected 0 parsed records, got %d", out.TotalParsed)
				}
				// An empty file may yield 0 or 1 empty lines depending on trailing newline;
				// either way, all should be errors since parser is nil.
				if out.TotalParsed != 0 {
					t.Errorf("expected 0 parsed, got %d", out.TotalParsed)
				}
			},
		},
		{
			name:        "file not found",
			input:       ParseLogsInput{Path: filepath.Join(dir, "nonexistent.log")},
			wantErr:     true,
			errContains: "FILE_NOT_FOUND",
		},
		{
			name:  "default values applied",
			input: ParseLogsInput{Path: jsonPath, StartLine: 0, NumLines: 0, FormatHint: ""},
			check: func(t *testing.T, out ParseLogsOutput) {
				// Defaults: StartLine=1, NumLines=50, FormatHint="auto".
				// With 3 lines in the file, should parse all 3.
				if out.TotalParsed != 3 {
					t.Errorf("expected 3 parsed records with defaults, got %d", out.TotalParsed)
				}
				if out.Records[0].LineNumber != 1 {
					t.Errorf("expected first record at line 1 (default StartLine), got %d", out.Records[0].LineNumber)
				}
				// Format should be auto-detected.
				if out.DetectedFormat != "json" {
					t.Errorf("expected auto-detected format=json, got %q", out.DetectedFormat)
				}
			},
		},
		{
			name:  "start_line pagination",
			input: ParseLogsInput{Path: jsonPath, StartLine: 2, NumLines: 10},
			check: func(t *testing.T, out ParseLogsOutput) {
				if out.TotalParsed != 2 {
					t.Errorf("expected 2 records starting from line 2, got %d", out.TotalParsed)
				}
				if len(out.Records) > 0 && out.Records[0].LineNumber != 2 {
					t.Errorf("expected first record at line 2, got %d", out.Records[0].LineNumber)
				}
			},
		},
		{
			name:  "num_lines clamped to max 500",
			input: ParseLogsInput{Path: bigJSONPath, NumLines: 9999},
			check: func(t *testing.T, out ParseLogsOutput) {
				// 100 lines in file, clamped request to 500 but file only has 100.
				if out.TotalParsed != 100 {
					t.Errorf("expected 100 parsed records, got %d", out.TotalParsed)
				}
			},
		},
		{
			name:  "num_lines clamped to min 1",
			input: ParseLogsInput{Path: jsonPath, NumLines: -5},
			check: func(t *testing.T, out ParseLogsOutput) {
				// Clamped to 1 — should parse exactly 1 line.
				if out.TotalParsed != 1 {
					t.Errorf("expected 1 parsed record (clamped min), got %d", out.TotalParsed)
				}
			},
		},
		{
			name:  "garbage file with auto-detect returns unknown format and all errors",
			input: ParseLogsInput{Path: garbagePath},
			check: func(t *testing.T, out ParseLogsOutput) {
				if out.DetectedFormat != "unknown" {
					t.Errorf("expected detected_format=unknown, got %q", out.DetectedFormat)
				}
				// All lines should be parse errors since no parser matches.
				if out.TotalErrors != 3 {
					t.Errorf("expected 3 parse errors, got %d", out.TotalErrors)
				}
				if out.TotalParsed != 0 {
					t.Errorf("expected 0 parsed records, got %d", out.TotalParsed)
				}
				for _, pe := range out.ParseErrors {
					if !strings.Contains(pe.Error, "no parser available") {
						t.Errorf("expected 'no parser available' in error, got %q", pe.Error)
					}
				}
			},
		},
		{
			name:  "start_line beyond end of file returns empty results",
			input: ParseLogsInput{Path: jsonPath, StartLine: 999},
			check: func(t *testing.T, out ParseLogsOutput) {
				if out.TotalParsed != 0 {
					t.Errorf("expected 0 parsed, got %d", out.TotalParsed)
				}
				if out.TotalErrors != 0 {
					t.Errorf("expected 0 errors, got %d", out.TotalErrors)
				}
			},
		},
		{
			name:  "records preserve raw field",
			input: ParseLogsInput{Path: jsonPath, NumLines: 1},
			check: func(t *testing.T, out ParseLogsOutput) {
				if len(out.Records) != 1 {
					t.Fatalf("expected 1 record, got %d", len(out.Records))
				}
				if out.Records[0].Raw != jsonLines[0] {
					t.Errorf("expected raw to match original line, got %q", out.Records[0].Raw)
				}
			},
		},
		{
			name:  "parse errors preserve raw field",
			input: ParseLogsInput{Path: mixedPath, FormatHint: "json"},
			check: func(t *testing.T, out ParseLogsOutput) {
				if len(out.ParseErrors) < 1 {
					t.Fatal("expected at least 1 parse error")
				}
				if out.ParseErrors[0].Raw != "this is not json or syslog" {
					t.Errorf("expected raw to match original line, got %q", out.ParseErrors[0].Raw)
				}
			},
		},
		{
			name:        "binary file rejected",
			input:       ParseLogsInput{Path: writeBinaryFile(t, dir, "binary.bin")},
			wantErr:     true,
			errContains: "BINARY_FILE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := RunParseLogs(tt.input)
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
