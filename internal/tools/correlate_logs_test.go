package tools

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func TestRunCorrelateLogs(t *testing.T) {
	// Two files with shared request_id in extra_fields (JSON structured logs).
	file1Log := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00.100Z","level":"INFO","source":"gateway","message":"Incoming request","request_id":"req-abc-123"}`,
		`{"timestamp":"2025-01-15T10:00:00.200Z","level":"INFO","source":"gateway","message":"Routing to auth service","request_id":"req-abc-123"}`,
		`{"timestamp":"2025-01-15T10:00:01.000Z","level":"INFO","source":"gateway","message":"Incoming request","request_id":"req-def-456"}`,
		`{"timestamp":"2025-01-15T10:00:02.000Z","level":"INFO","source":"gateway","message":"Health check","request_id":"req-only-file1"}`,
	}, "\n") + "\n"

	file2Log := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00.300Z","level":"INFO","source":"auth","message":"Token validated","request_id":"req-abc-123"}`,
		`{"timestamp":"2025-01-15T10:00:01.100Z","level":"ERROR","source":"auth","message":"Auth failed","request_id":"req-def-456"}`,
		`{"timestamp":"2025-01-15T10:00:03.000Z","level":"INFO","source":"auth","message":"Health ok","request_id":"req-only-file2"}`,
	}, "\n") + "\n"

	file1Path := writeTempLog(t, "gateway.log", file1Log)
	file2Path := writeTempLog(t, "auth.log", file2Log)

	// Files with no shared correlation values.
	noShareFile1 := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","message":"hello","request_id":"aaa"}`,
	}, "\n") + "\n"
	noShareFile2 := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","message":"world","request_id":"bbb"}`,
	}, "\n") + "\n"
	noSharePath1 := writeTempLog(t, "noshare1.log", noShareFile1)
	noSharePath2 := writeTempLog(t, "noshare2.log", noShareFile2)

	// Files with correlation value in message text (not in extra_fields).
	msgFile1 := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","source":"svc1","message":"Processing request_id=msg-corr-001 started"}`,
		`{"timestamp":"2025-01-15T10:00:01Z","level":"INFO","source":"svc1","message":"Processing request_id=msg-corr-002 started"}`,
	}, "\n") + "\n"
	msgFile2 := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00.500Z","level":"INFO","source":"svc2","message":"Handled request_id=msg-corr-001 done"}`,
		`{"timestamp":"2025-01-15T10:00:01.500Z","level":"INFO","source":"svc2","message":"Handled request_id=msg-corr-002 done"}`,
	}, "\n") + "\n"
	msgPath1 := writeTempLog(t, "msg1.log", msgFile1)
	msgPath2 := writeTempLog(t, "msg2.log", msgFile2)

	// Files where time window filtering should exclude a group.
	wideFile1 := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","message":"start","request_id":"wide-001"}`,
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","message":"start","request_id":"narrow-001"}`,
	}, "\n") + "\n"
	wideFile2 := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:10:00Z","level":"INFO","message":"end","request_id":"wide-001"}`,
		`{"timestamp":"2025-01-15T10:00:05Z","level":"INFO","message":"end","request_id":"narrow-001"}`,
	}, "\n") + "\n"
	widePath1 := writeTempLog(t, "wide1.log", wideFile1)
	widePath2 := writeTempLog(t, "wide2.log", wideFile2)

	// Empty files.
	emptyPath1 := writeTempLog(t, "empty1.log", "")
	emptyPath2 := writeTempLog(t, "empty2.log", "")

	// Single-file group test: all entries in only one file share an ID;
	// a second file exists but has different IDs.
	singleFileFile1 := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","message":"a","request_id":"only-in-f1"}`,
		`{"timestamp":"2025-01-15T10:00:01Z","level":"INFO","message":"b","request_id":"only-in-f1"}`,
	}, "\n") + "\n"
	singleFileFile2 := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","message":"c","request_id":"only-in-f2"}`,
	}, "\n") + "\n"
	singleFilePath1 := writeTempLog(t, "single1.log", singleFileFile1)
	singleFilePath2 := writeTempLog(t, "single2.log", singleFileFile2)

	// Custom correlation field in extra_fields.
	traceFile1 := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","message":"span start","trace_id":"trace-aaa"}`,
	}, "\n") + "\n"
	traceFile2 := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00.500Z","level":"INFO","message":"span end","trace_id":"trace-aaa"}`,
	}, "\n") + "\n"
	tracePath1 := writeTempLog(t, "trace1.log", traceFile1)
	tracePath2 := writeTempLog(t, "trace2.log", traceFile2)

	// Custom correlation field in message text.
	traceMsgFile1 := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","message":"begin trace_id=trace-bbb work"}`,
	}, "\n") + "\n"
	traceMsgFile2 := strings.Join([]string{
		`{"timestamp":"2025-01-15T10:00:01Z","level":"INFO","message":"end trace_id=trace-bbb work"}`,
	}, "\n") + "\n"
	traceMsgPath1 := writeTempLog(t, "tracemsg1.log", traceMsgFile1)
	traceMsgPath2 := writeTempLog(t, "tracemsg2.log", traceMsgFile2)

	tests := []struct {
		name        string
		input       CorrelateLogsInput
		wantErr     bool
		errContains string
		checkOutput func(t *testing.T, out CorrelateLogsOutput)
	}{
		{
			name: "two files with shared request_id — correlated groups found",
			input: CorrelateLogsInput{
				Paths: []string{file1Path, file2Path},
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				// req-abc-123: 2 events in file1 + 1 in file2 = 3 events, 2 files
				// req-def-456: 1 event in file1 + 1 in file2 = 2 events, 2 files
				// req-only-file1 and req-only-file2: single file each → excluded
				if out.TotalGroups != 2 {
					t.Errorf("TotalGroups = %d, want 2", out.TotalGroups)
				}
				if out.GroupsReturned != 2 {
					t.Errorf("GroupsReturned = %d, want 2", out.GroupsReturned)
				}
				if len(out.CorrelatedGroups) != 2 {
					t.Fatalf("len(CorrelatedGroups) = %d, want 2", len(out.CorrelatedGroups))
				}

				// Sorted by event count desc: req-abc-123 (3) before req-def-456 (2).
				if out.CorrelatedGroups[0].CorrelationID != "req-abc-123" {
					t.Errorf("first group ID = %q, want req-abc-123", out.CorrelatedGroups[0].CorrelationID)
				}
				if len(out.CorrelatedGroups[0].Events) != 3 {
					t.Errorf("first group events = %d, want 3", len(out.CorrelatedGroups[0].Events))
				}
				if out.CorrelatedGroups[1].CorrelationID != "req-def-456" {
					t.Errorf("second group ID = %q, want req-def-456", out.CorrelatedGroups[1].CorrelationID)
				}
				if len(out.CorrelatedGroups[1].Events) != 2 {
					t.Errorf("second group events = %d, want 2", len(out.CorrelatedGroups[1].Events))
				}

				// Events within each group should be sorted by timestamp.
				for gi, g := range out.CorrelatedGroups {
					for i := 1; i < len(g.Events); i++ {
						if g.Events[i].Timestamp < g.Events[i-1].Timestamp {
							t.Errorf("group %d events not sorted: [%d]=%q before [%d]=%q",
								gi, i-1, g.Events[i-1].Timestamp, i, g.Events[i].Timestamp)
						}
					}
				}

				// FilesInvolved should list both files.
				if len(out.CorrelatedGroups[0].FilesInvolved) != 2 {
					t.Errorf("first group FilesInvolved = %d, want 2", len(out.CorrelatedGroups[0].FilesInvolved))
				}

				// CorrelationField should be the default.
				if out.CorrelatedGroups[0].CorrelationField != "request_id" {
					t.Errorf("CorrelationField = %q, want request_id", out.CorrelatedGroups[0].CorrelationField)
				}
			},
		},
		{
			name: "no shared correlation values — empty groups",
			input: CorrelateLogsInput{
				Paths: []string{noSharePath1, noSharePath2},
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				if out.TotalGroups != 0 {
					t.Errorf("TotalGroups = %d, want 0", out.TotalGroups)
				}
				if len(out.CorrelatedGroups) != 0 {
					t.Errorf("len(CorrelatedGroups) = %d, want 0", len(out.CorrelatedGroups))
				}
				if out.CorrelatedGroups == nil {
					t.Error("CorrelatedGroups should be non-nil empty slice, got nil")
				}
			},
		},
		{
			name: "correlation value in extra_fields",
			input: CorrelateLogsInput{
				Paths: []string{file1Path, file2Path},
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				// Verify that events carry the correct data from extra_fields-based correlation.
				found := false
				for _, g := range out.CorrelatedGroups {
					if g.CorrelationID == "req-abc-123" {
						found = true
						if len(g.Events) != 3 {
							t.Errorf("req-abc-123 events = %d, want 3", len(g.Events))
						}
						// Check that events come from different files.
						fileSet := make(map[string]bool)
						for _, ev := range g.Events {
							fileSet[ev.File] = true
						}
						if len(fileSet) != 2 {
							t.Errorf("req-abc-123 unique files = %d, want 2", len(fileSet))
						}
					}
				}
				if !found {
					t.Error("expected group with correlation_id req-abc-123")
				}
			},
		},
		{
			name: "correlation value in message text",
			input: CorrelateLogsInput{
				Paths: []string{msgPath1, msgPath2},
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				if out.TotalGroups != 2 {
					t.Errorf("TotalGroups = %d, want 2", out.TotalGroups)
				}
				ids := make(map[string]bool)
				for _, g := range out.CorrelatedGroups {
					ids[g.CorrelationID] = true
				}
				if !ids["msg-corr-001"] {
					t.Error("expected group with correlation_id msg-corr-001")
				}
				if !ids["msg-corr-002"] {
					t.Error("expected group with correlation_id msg-corr-002")
				}
			},
		},
		{
			name: "time window filtering — exclude groups exceeding window",
			input: CorrelateLogsInput{
				Paths:             []string{widePath1, widePath2},
				TimeWindowSeconds: 30,
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				// wide-001 spans 10 minutes (600s) → excluded by 30s window
				// narrow-001 spans 5 seconds → included
				if out.TotalGroups != 1 {
					t.Errorf("TotalGroups = %d, want 1", out.TotalGroups)
				}
				if len(out.CorrelatedGroups) != 1 {
					t.Fatalf("len(CorrelatedGroups) = %d, want 1", len(out.CorrelatedGroups))
				}
				if out.CorrelatedGroups[0].CorrelationID != "narrow-001" {
					t.Errorf("group ID = %q, want narrow-001", out.CorrelatedGroups[0].CorrelationID)
				}
			},
		},
		{
			name: "time window large enough includes all groups",
			input: CorrelateLogsInput{
				Paths:             []string{widePath1, widePath2},
				TimeWindowSeconds: 3600,
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				if out.TotalGroups != 2 {
					t.Errorf("TotalGroups = %d, want 2", out.TotalGroups)
				}
			},
		},
		{
			name: "single-file groups excluded — must span >= 2 files",
			input: CorrelateLogsInput{
				Paths: []string{singleFilePath1, singleFilePath2},
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				// only-in-f1 appears in file1 only, only-in-f2 in file2 only → both excluded
				if out.TotalGroups != 0 {
					t.Errorf("TotalGroups = %d, want 0", out.TotalGroups)
				}
				if len(out.CorrelatedGroups) != 0 {
					t.Errorf("len(CorrelatedGroups) = %d, want 0", len(out.CorrelatedGroups))
				}
			},
		},
		{
			name: "file not found for one path — error",
			input: CorrelateLogsInput{
				Paths: []string{file1Path, "/nonexistent/path/missing.log"},
			},
			wantErr:     true,
			errContains: "FILE_NOT_FOUND",
		},
		{
			name: "less than 2 paths — validation error",
			input: CorrelateLogsInput{
				Paths: []string{file1Path},
			},
			wantErr:     true,
			errContains: "VALIDATION_ERROR",
		},
		{
			name: "zero paths — validation error",
			input: CorrelateLogsInput{
				Paths: []string{},
			},
			wantErr:     true,
			errContains: "VALIDATION_ERROR",
		},
		{
			name: "more than 10 paths — validation error",
			input: CorrelateLogsInput{
				Paths: func() []string {
					paths := make([]string, 11)
					for i := range paths {
						paths[i] = file1Path
					}
					return paths
				}(),
			},
			wantErr:     true,
			errContains: "VALIDATION_ERROR",
		},
		{
			name: "empty files — empty results",
			input: CorrelateLogsInput{
				Paths: []string{emptyPath1, emptyPath2},
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				if out.TotalGroups != 0 {
					t.Errorf("TotalGroups = %d, want 0", out.TotalGroups)
				}
				if len(out.CorrelatedGroups) != 0 {
					t.Errorf("len(CorrelatedGroups) = %d, want 0", len(out.CorrelatedGroups))
				}
				if out.CorrelatedGroups == nil {
					t.Error("CorrelatedGroups should be non-nil empty slice, got nil")
				}
				if len(out.FilesAnalyzed) != 2 {
					t.Errorf("len(FilesAnalyzed) = %d, want 2", len(out.FilesAnalyzed))
				}
				for _, fa := range out.FilesAnalyzed {
					if fa.EntriesParsed != 0 {
						t.Errorf("EntriesParsed for %q = %d, want 0", fa.Path, fa.EntriesParsed)
					}
				}
			},
		},
		{
			name: "default values applied — correlation_field and time_window_seconds",
			input: CorrelateLogsInput{
				Paths: []string{file1Path, file2Path},
				// CorrelationField and TimeWindowSeconds left as zero values.
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				// Default CorrelationField is "request_id" — should find groups.
				if out.TotalGroups == 0 {
					t.Error("expected groups with default correlation_field=request_id")
				}
				for _, g := range out.CorrelatedGroups {
					if g.CorrelationField != "request_id" {
						t.Errorf("CorrelationField = %q, want request_id", g.CorrelationField)
					}
				}
			},
		},
		{
			name: "files_analyzed populated correctly",
			input: CorrelateLogsInput{
				Paths: []string{file1Path, file2Path},
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				if len(out.FilesAnalyzed) != 2 {
					t.Fatalf("len(FilesAnalyzed) = %d, want 2", len(out.FilesAnalyzed))
				}
				// file1 has 4 parseable lines, file2 has 3.
				fa := make(map[string]int)
				for _, f := range out.FilesAnalyzed {
					fa[f.Path] = f.EntriesParsed
				}
				if fa[file1Path] != 4 {
					t.Errorf("file1 EntriesParsed = %d, want 4", fa[file1Path])
				}
				if fa[file2Path] != 3 {
					t.Errorf("file2 EntriesParsed = %d, want 3", fa[file2Path])
				}
			},
		},
		{
			name: "time_span_ms computed correctly",
			input: CorrelateLogsInput{
				Paths: []string{file1Path, file2Path},
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				for _, g := range out.CorrelatedGroups {
					if g.CorrelationID == "req-abc-123" {
						// Earliest: 10:00:00.100Z, Latest: 10:00:00.300Z → 200ms
						if g.TimeSpanMs != 200 {
							t.Errorf("req-abc-123 TimeSpanMs = %d, want 200", g.TimeSpanMs)
						}
					}
					if g.CorrelationID == "req-def-456" {
						// Earliest: 10:00:01.000Z, Latest: 10:00:01.100Z → 100ms
						if g.TimeSpanMs != 100 {
							t.Errorf("req-def-456 TimeSpanMs = %d, want 100", g.TimeSpanMs)
						}
					}
				}
			},
		},
		{
			name: "custom correlation_field in extra_fields",
			input: CorrelateLogsInput{
				Paths:            []string{tracePath1, tracePath2},
				CorrelationField: "trace_id",
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				if out.TotalGroups != 1 {
					t.Errorf("TotalGroups = %d, want 1", out.TotalGroups)
				}
				if len(out.CorrelatedGroups) != 1 {
					t.Fatalf("len(CorrelatedGroups) = %d, want 1", len(out.CorrelatedGroups))
				}
				if out.CorrelatedGroups[0].CorrelationID != "trace-aaa" {
					t.Errorf("correlation_id = %q, want trace-aaa", out.CorrelatedGroups[0].CorrelationID)
				}
				if out.CorrelatedGroups[0].CorrelationField != "trace_id" {
					t.Errorf("CorrelationField = %q, want trace_id", out.CorrelatedGroups[0].CorrelationField)
				}
			},
		},
		{
			name: "custom correlation_field in message text",
			input: CorrelateLogsInput{
				Paths:            []string{traceMsgPath1, traceMsgPath2},
				CorrelationField: "trace_id",
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				if out.TotalGroups != 1 {
					t.Errorf("TotalGroups = %d, want 1", out.TotalGroups)
				}
				if len(out.CorrelatedGroups) != 1 {
					t.Fatalf("len(CorrelatedGroups) = %d, want 1", len(out.CorrelatedGroups))
				}
				if out.CorrelatedGroups[0].CorrelationID != "trace-bbb" {
					t.Errorf("correlation_id = %q, want trace-bbb", out.CorrelatedGroups[0].CorrelationID)
				}
			},
		},
		{
			name: "events preserve source and level fields",
			input: CorrelateLogsInput{
				Paths: []string{file1Path, file2Path},
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				for _, g := range out.CorrelatedGroups {
					if g.CorrelationID == "req-def-456" {
						for _, ev := range g.Events {
							if ev.Source != nil && *ev.Source == "auth" {
								if ev.Level == nil || string(*ev.Level) != "ERROR" {
									t.Errorf("auth event level = %v, want ERROR", ev.Level)
								}
							}
						}
					}
				}
			},
		},
		{
			name: "binary file error",
			input: CorrelateLogsInput{
				Paths: []string{file1Path, writeTempBinary(t)},
			},
			wantErr:     true,
			errContains: "BINARY_FILE",
		},
		{
			name: "wrong correlation_field yields no groups",
			input: CorrelateLogsInput{
				Paths:            []string{file1Path, file2Path},
				CorrelationField: "nonexistent_field",
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				if out.TotalGroups != 0 {
					t.Errorf("TotalGroups = %d, want 0", out.TotalGroups)
				}
			},
		},
		{
			name: "groups limited to 50 max",
			input: CorrelateLogsInput{
				Paths: func() []string {
					// Create two files with >50 shared correlation IDs.
					var lines1, lines2 []string
					for i := 0; i < 60; i++ {
						rid := fmt.Sprintf("rid-%03d", i)
						lines1 = append(lines1,
							fmt.Sprintf(`{"timestamp":"2025-01-15T10:00:%02d.000Z","level":"INFO","message":"req start","request_id":"%s"}`, i%60, rid))
						lines2 = append(lines2,
							fmt.Sprintf(`{"timestamp":"2025-01-15T10:00:%02d.100Z","level":"INFO","message":"req end","request_id":"%s"}`, i%60, rid))
					}
					p1 := writeTempLog(t, "many1.log", strings.Join(lines1, "\n")+"\n")
					p2 := writeTempLog(t, "many2.log", strings.Join(lines2, "\n")+"\n")
					return []string{p1, p2}
				}(),
			},
			checkOutput: func(t *testing.T, out CorrelateLogsOutput) {
				if out.TotalGroups != 60 {
					t.Errorf("TotalGroups = %d, want 60", out.TotalGroups)
				}
				if out.GroupsReturned != 50 {
					t.Errorf("GroupsReturned = %d, want 50", out.GroupsReturned)
				}
				if len(out.CorrelatedGroups) != 50 {
					t.Errorf("len(CorrelatedGroups) = %d, want 50", len(out.CorrelatedGroups))
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := RunCorrelateLogs(tc.input)
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

func TestExtractCorrelationValue(t *testing.T) {
	// Build a regex for "request_id" field.
	msgRe := compileCorrelationRegex("request_id")

	tests := []struct {
		name        string
		extraFields map[string]interface{}
		message     string
		field       string
		want        string
	}{
		{
			name:        "from extra_fields",
			extraFields: map[string]interface{}{"request_id": "abc-123"},
			message:     "no match here",
			field:       "request_id",
			want:        "abc-123",
		},
		{
			name:    "from message with equals",
			message: "Processing request_id=xyz-789 now",
			field:   "request_id",
			want:    "xyz-789",
		},
		{
			name:    "from message with colon",
			message: "Processing request_id:xyz-789 now",
			field:   "request_id",
			want:    "xyz-789",
		},
		{
			name:    "from message with space",
			message: "Processing request_id xyz-789 now",
			field:   "request_id",
			want:    "xyz-789",
		},
		{
			name:    "from message with quoted value",
			message: `Processing request_id="xyz-789" now`,
			field:   "request_id",
			want:    "xyz-789",
		},
		{
			name:    "from message with single-quoted value",
			message: `Processing request_id='xyz-789' now`,
			field:   "request_id",
			want:    "xyz-789",
		},
		{
			name:        "extra_fields takes precedence over message",
			extraFields: map[string]interface{}{"request_id": "from-fields"},
			message:     "request_id=from-message",
			field:       "request_id",
			want:        "from-fields",
		},
		{
			name:    "no match returns empty",
			message: "no correlation here",
			field:   "request_id",
			want:    "",
		},
		{
			name:        "empty extra_fields value falls through to message",
			extraFields: map[string]interface{}{"request_id": ""},
			message:     "request_id=from-message",
			field:       "request_id",
			want:        "from-message",
		},
		{
			name:        "nil extra_fields",
			extraFields: nil,
			message:     "request_id=fallback-val",
			field:       "request_id",
			want:        "fallback-val",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			re := msgRe
			if tc.field != "request_id" {
				re = compileCorrelationRegex(tc.field)
			}
			entry := &types.ParsedLogEntry{
				Message:     tc.message,
				ExtraFields: tc.extraFields,
			}
			got := extractCorrelationValue(entry, tc.field, re)
			if got != tc.want {
				t.Errorf("extractCorrelationValue() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestComputeTimeSpanMs(t *testing.T) {
	tests := []struct {
		name   string
		events []types.CorrelatedEvent
		want   int64
	}{
		{
			name:   "empty events",
			events: []types.CorrelatedEvent{},
			want:   0,
		},
		{
			name: "single event",
			events: []types.CorrelatedEvent{
				{Timestamp: "2025-01-15T10:00:00Z"},
			},
			want: 0,
		},
		{
			name: "two events 500ms apart",
			events: []types.CorrelatedEvent{
				{Timestamp: "2025-01-15T10:00:00.000Z"},
				{Timestamp: "2025-01-15T10:00:00.500Z"},
			},
			want: 500,
		},
		{
			name: "two events 60 seconds apart",
			events: []types.CorrelatedEvent{
				{Timestamp: "2025-01-15T10:00:00Z"},
				{Timestamp: "2025-01-15T10:01:00Z"},
			},
			want: 60000,
		},
		{
			name: "empty timestamps",
			events: []types.CorrelatedEvent{
				{Timestamp: ""},
				{Timestamp: ""},
			},
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeTimeSpanMs(tc.events)
			if got != tc.want {
				t.Errorf("computeTimeSpanMs() = %d, want %d", got, tc.want)
			}
		})
	}
}

// compileCorrelationRegex is a test helper that compiles the correlation regex
// for a given field name, matching the logic in RunCorrelateLogs.
func compileCorrelationRegex(field string) *regexp.Regexp {
	escaped := regexp.QuoteMeta(field)
	return regexp.MustCompile(escaped + `[=: ]["']?(\S+)`)
}
