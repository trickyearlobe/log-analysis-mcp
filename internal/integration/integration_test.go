package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/tools"
)

// Local anonymous-style structs for unmarshaling tool output over the wire.
// These intentionally do NOT import from internal/tools — the whole point is
// to validate the JSON contract between client and server.

type logLine struct {
	LineNumber int    `json:"line_number"`
	Content    string `json:"content"`
}

type lineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type readLogsResult struct {
	Lines         []logLine `json:"lines"`
	TotalLines    int       `json:"total_lines"`
	HasMore       bool      `json:"has_more"`
	FileSizeBytes int64     `json:"file_size_bytes"`
	CurrentRange  lineRange `json:"current_range"`
}

type tailLogsResult struct {
	Lines           []logLine `json:"lines"`
	TotalLines      int       `json:"total_lines"`
	FileSizeBytes   int64     `json:"file_size_bytes"`
	ShowingFromLine int       `json:"showing_from_line"`
}

type searchMatch struct {
	LineNumber    int      `json:"line_number"`
	Line          string   `json:"line"`
	BeforeContext []string `json:"before_context"`
	AfterContext  []string `json:"after_context"`
}

type searchLogsResult struct {
	Matches       []searchMatch `json:"matches"`
	TotalMatches  int           `json:"total_matches"`
	SearchedLines int           `json:"searched_lines"`
	PatternUsed   string        `json:"pattern_used"`
	Truncated     bool          `json:"truncated"`
}

type parsedRecord struct {
	LineNumber  int                    `json:"line_number"`
	Timestamp   *string                `json:"timestamp"`
	Level       *string                `json:"level"`
	Source      *string                `json:"source"`
	Message     string                 `json:"message"`
	Raw         string                 `json:"raw"`
	ExtraFields map[string]interface{} `json:"extra_fields,omitempty"`
}

type parseError struct {
	LineNumber int    `json:"line_number"`
	Raw        string `json:"raw"`
	Error      string `json:"error"`
}

type parseLogsResult struct {
	DetectedFormat string         `json:"detected_format"`
	Confidence     float64        `json:"confidence"`
	Records        []parsedRecord `json:"records"`
	ParseErrors    []parseError   `json:"parse_errors"`
	TotalParsed    int            `json:"total_parsed"`
	TotalErrors    int            `json:"total_errors"`
}

type filteredEntry struct {
	LineNumber int     `json:"line_number"`
	Timestamp  *string `json:"timestamp"`
	Level      *string `json:"level"`
	Source     *string `json:"source"`
	Message    string  `json:"message"`
	Raw        string  `json:"raw"`
}

type filterLogsResult struct {
	Entries      []filteredEntry `json:"entries"`
	TotalMatched int             `json:"total_matched"`
	TotalScanned int             `json:"total_scanned"`
	Truncated    bool            `json:"truncated"`
}

type errorCluster struct {
	Pattern    string  `json:"pattern"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

type extractErrorsResult struct {
	Clusters       []errorCluster `json:"clusters"`
	TotalErrors    int            `json:"total_errors"`
	LevelsIncluded []string       `json:"levels_included"`
}

type levelStats struct {
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

type fileInfoSummary struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"size_bytes"`
}

type summarizeLogsResult struct {
	FileInfo          fileInfoSummary       `json:"file_info"`
	DetectedFormat    string                `json:"detected_format"`
	LevelDistribution map[string]levelStats `json:"level_distribution"`
	LinesAnalyzed     int                   `json:"lines_analyzed"`
}

type anomaly struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
}

type analysisMetadata struct {
	TotalLinesAnalyzed int `json:"total_lines_analyzed"`
}

type detectAnomaliesResult struct {
	Anomalies        []anomaly        `json:"anomalies"`
	AnalysisMetadata analysisMetadata `json:"analysis_metadata"`
}

type timelineEvent struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type timelineResult struct {
	Events     []timelineEvent `json:"events"`
	EventCount int             `json:"event_count"`
	Truncated  bool            `json:"truncated"`
}

type correlatedGroup struct {
	CorrelationID string   `json:"correlation_id"`
	FilesInvolved []string `json:"files_involved"`
}

type correlateLogsResult struct {
	CorrelatedGroups []correlatedGroup `json:"correlated_groups"`
	TotalGroups      int               `json:"total_groups"`
	GroupsReturned   int               `json:"groups_returned"`
}

func TestListToolsReturnsAll11(t *testing.T) {
	session := setupTestServer(t)

	resp, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	expected := map[string]bool{
		"read_logs":        false,
		"tail_logs":        false,
		"search_logs":      false,
		"parse_logs":       false,
		"filter_logs":      false,
		"extract_errors":   false,
		"summarize_logs":   false,
		"detect_anomalies": false,
		"timeline":         false,
		"correlate_logs":   false,
		"decompress_file":  false,
	}

	if len(resp.Tools) != 11 {
		t.Errorf("expected 11 tools, got %d", len(resp.Tools))
	}

	for _, tool := range resp.Tools {
		if _, ok := expected[tool.Name]; ok {
			expected[tool.Name] = true
		} else {
			t.Errorf("unexpected tool: %s", tool.Name)
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestReadLogsRoundTrip(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	path := writeLogFile(t, dir, "app.log", jsonLogLines())

	out := callTool[readLogsResult](t, session, "read_logs", map[string]any{
		"path": path,
	})

	if len(out.Lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(out.Lines))
	}
	if out.TotalLines != 10 {
		t.Errorf("expected total_lines=10, got %d", out.TotalLines)
	}
	if out.HasMore {
		t.Error("expected has_more=false")
	}
	if out.FileSizeBytes <= 0 {
		t.Error("expected file_size_bytes > 0")
	}
	if out.CurrentRange.Start != 1 {
		t.Errorf("expected current_range.start=1, got %d", out.CurrentRange.Start)
	}
	if out.CurrentRange.End != 10 {
		t.Errorf("expected current_range.end=10, got %d", out.CurrentRange.End)
	}
	// Verify first line content contains expected JSON.
	if !strings.Contains(out.Lines[0].Content, "server started") {
		t.Errorf("first line should contain 'server started', got: %s", out.Lines[0].Content)
	}
}

func TestTailLogsRoundTrip(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	path := writeLogFile(t, dir, "app.log", jsonLogLines())

	out := callTool[tailLogsResult](t, session, "tail_logs", map[string]any{
		"path":      path,
		"num_lines": 3,
	})

	if len(out.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(out.Lines))
	}
	if out.ShowingFromLine != 8 {
		t.Errorf("expected showing_from_line=8, got %d", out.ShowingFromLine)
	}
	if out.TotalLines != 10 {
		t.Errorf("expected total_lines=10, got %d", out.TotalLines)
	}
	// Last line should be the shutdown message.
	lastLine := out.Lines[len(out.Lines)-1]
	if !strings.Contains(lastLine.Content, "shutting down") {
		t.Errorf("last line should contain 'shutting down', got: %s", lastLine.Content)
	}
}

func TestSearchLogsRoundTrip(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	path := writeLogFile(t, dir, "app.log", jsonLogLines())

	out := callTool[searchLogsResult](t, session, "search_logs", map[string]any{
		"path":    path,
		"pattern": "ERROR",
	})

	if out.TotalMatches != 2 {
		t.Fatalf("expected 2 matches, got %d", out.TotalMatches)
	}
	if len(out.Matches) != 2 {
		t.Fatalf("expected 2 match entries, got %d", len(out.Matches))
	}

	// The ERROR lines are at lines 5 and 8 in jsonLogLines().
	if out.Matches[0].LineNumber != 5 {
		t.Errorf("expected first match at line 5, got %d", out.Matches[0].LineNumber)
	}
	if out.Matches[1].LineNumber != 8 {
		t.Errorf("expected second match at line 8, got %d", out.Matches[1].LineNumber)
	}
	if out.SearchedLines != 10 {
		t.Errorf("expected searched_lines=10, got %d", out.SearchedLines)
	}
	if out.Truncated {
		t.Error("expected truncated=false")
	}
}

func TestSearchLogsWithContext(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	path := writeLogFile(t, dir, "app.log", jsonLogLines())

	out := callTool[searchLogsResult](t, session, "search_logs", map[string]any{
		"path":          path,
		"pattern":       "ERROR",
		"context_lines": 1,
	})

	if out.TotalMatches != 2 {
		t.Fatalf("expected 2 matches, got %d", out.TotalMatches)
	}

	// First match (line 5): should have 1 before-context line (line 4) and 1 after-context line (line 6).
	m0 := out.Matches[0]
	if len(m0.BeforeContext) != 1 {
		t.Errorf("match[0]: expected 1 before_context line, got %d", len(m0.BeforeContext))
	} else if !strings.Contains(m0.BeforeContext[0], "deprecated") {
		t.Errorf("match[0]: before_context should contain 'deprecated', got: %s", m0.BeforeContext[0])
	}
	if len(m0.AfterContext) != 1 {
		t.Errorf("match[0]: expected 1 after_context line, got %d", len(m0.AfterContext))
	} else if !strings.Contains(m0.AfterContext[0], "retrying") {
		t.Errorf("match[0]: after_context should contain 'retrying', got: %s", m0.AfterContext[0])
	}

	// Second match (line 8): should have 1 before-context line (line 7) and 1 after-context line (line 9).
	m1 := out.Matches[1]
	if len(m1.BeforeContext) != 1 {
		t.Errorf("match[1]: expected 1 before_context line, got %d", len(m1.BeforeContext))
	}
	if len(m1.AfterContext) != 1 {
		t.Errorf("match[1]: expected 1 after_context line, got %d", len(m1.AfterContext))
	}
}

func TestParseLogsAutoDetectJSON(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	path := writeLogFile(t, dir, "app.log", jsonLogLines())

	out := callTool[parseLogsResult](t, session, "parse_logs", map[string]any{
		"path":        path,
		"format_hint": "auto",
	})

	if out.DetectedFormat != "json" {
		t.Errorf("expected detected_format='json', got %q", out.DetectedFormat)
	}
	if out.Confidence <= 0 {
		t.Errorf("expected confidence > 0, got %f", out.Confidence)
	}
	if out.TotalParsed != 10 {
		t.Errorf("expected total_parsed=10, got %d", out.TotalParsed)
	}
	if out.TotalErrors != 0 {
		t.Errorf("expected total_errors=0, got %d", out.TotalErrors)
	}

	// Check that records have timestamps and levels.
	for i, rec := range out.Records {
		if rec.Timestamp == nil {
			t.Errorf("record[%d]: expected non-nil timestamp", i)
		}
		if rec.Level == nil {
			t.Errorf("record[%d]: expected non-nil level", i)
		}
	}
}

func TestParseLogsAutoDetectSyslog(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	path := writeLogFile(t, dir, "syslog.log", syslogLines())

	out := callTool[parseLogsResult](t, session, "parse_logs", map[string]any{
		"path":        path,
		"format_hint": "auto",
	})

	if !strings.Contains(out.DetectedFormat, "syslog") {
		t.Errorf("expected detected_format to contain 'syslog', got %q", out.DetectedFormat)
	}
	if out.TotalParsed == 0 {
		t.Error("expected at least some records parsed from syslog")
	}
}

func TestFilterLogsByLevel(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	path := writeLogFile(t, dir, "app.log", jsonLogLines())

	out := callTool[filterLogsResult](t, session, "filter_logs", map[string]any{
		"path":  path,
		"level": []string{"ERROR"},
	})

	if out.TotalMatched != 2 {
		t.Fatalf("expected total_matched=2, got %d", out.TotalMatched)
	}
	if out.TotalScanned != 10 {
		t.Errorf("expected total_scanned=10, got %d", out.TotalScanned)
	}

	for i, entry := range out.Entries {
		if entry.Level == nil || *entry.Level != "ERROR" {
			lvl := "<nil>"
			if entry.Level != nil {
				lvl = *entry.Level
			}
			t.Errorf("entry[%d]: expected level=ERROR, got %s", i, lvl)
		}
	}
}

func TestExtractErrorsClustering(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	path := writeLogFile(t, dir, "spike.log", errorSpikeLines())

	out := callTool[extractErrorsResult](t, session, "extract_errors", map[string]any{
		"path": path,
	})

	if out.TotalErrors != 20 {
		t.Errorf("expected total_errors=20, got %d", out.TotalErrors)
	}
	if len(out.Clusters) == 0 {
		t.Fatal("expected at least 1 cluster")
	}

	// The error spike data has two distinct error messages, so we expect 2 clusters.
	if len(out.Clusters) != 2 {
		t.Errorf("expected 2 clusters, got %d", len(out.Clusters))
	}

	// Verify total counts across clusters sum to 20.
	totalCounted := 0
	for _, c := range out.Clusters {
		totalCounted += c.Count
	}
	if totalCounted != 20 {
		t.Errorf("expected cluster counts to sum to 20, got %d", totalCounted)
	}

	// First cluster (sorted by count desc) should be the timeout one with 15 occurrences.
	if out.Clusters[0].Count != 15 {
		t.Errorf("expected first cluster count=15, got %d", out.Clusters[0].Count)
	}
}

func TestSummarizeLogsStatistics(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	path := writeLogFile(t, dir, "app.log", jsonLogLines())

	out := callTool[summarizeLogsResult](t, session, "summarize_logs", map[string]any{
		"path": path,
	})

	if out.DetectedFormat != "json" {
		t.Errorf("expected detected_format='json', got %q", out.DetectedFormat)
	}
	if out.LinesAnalyzed != 10 {
		t.Errorf("expected lines_analyzed=10, got %d", out.LinesAnalyzed)
	}
	if out.FileInfo.SizeBytes <= 0 {
		t.Error("expected file_info.size_bytes > 0")
	}
	if out.FileInfo.Name != "app.log" {
		t.Errorf("expected file_info.name='app.log', got %q", out.FileInfo.Name)
	}

	// jsonLogLines has: 1 DEBUG, 1 WARN, 2 ERROR, 6 INFO.
	if out.LevelDistribution == nil {
		t.Fatal("expected non-nil level_distribution")
	}

	requiredLevels := []string{"INFO", "ERROR", "DEBUG", "WARN"}
	for _, lvl := range requiredLevels {
		stats, ok := out.LevelDistribution[lvl]
		if !ok {
			t.Errorf("expected level_distribution to contain %q", lvl)
			continue
		}
		if stats.Count <= 0 {
			t.Errorf("expected level_distribution[%q].count > 0", lvl)
		}
	}

	infoStats := out.LevelDistribution["INFO"]
	if infoStats.Count != 6 {
		t.Errorf("expected INFO count=6, got %d", infoStats.Count)
	}
	errorStats := out.LevelDistribution["ERROR"]
	if errorStats.Count != 2 {
		t.Errorf("expected ERROR count=2, got %d", errorStats.Count)
	}
}

func TestDetectAnomaliesErrorSpike(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	path := writeLogFile(t, dir, "spike.log", errorSpikeLines())

	out := callTool[detectAnomaliesResult](t, session, "detect_anomalies", map[string]any{
		"path":        path,
		"sensitivity": "high",
	})

	if out.AnalysisMetadata.TotalLinesAnalyzed != 60 {
		t.Errorf("expected total_lines_analyzed=60, got %d", out.AnalysisMetadata.TotalLinesAnalyzed)
	}

	if len(out.Anomalies) == 0 {
		t.Fatal("expected at least 1 anomaly detected")
	}

	// Verify at least one anomaly has a recognized type.
	validTypes := map[string]bool{
		"error_spike":    true,
		"rate_change":    true,
		"gap":            true,
		"new_error_type": true,
	}
	foundValid := false
	for _, a := range out.Anomalies {
		if validTypes[a.Type] {
			foundValid = true
			break
		}
	}
	if !foundValid {
		t.Error("expected at least one anomaly with a recognized type")
	}
}

func TestTimelineEventClassification(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()

	// jsonLogLines() contains "server started" (startup) and "shutting down" (shutdown)
	// as well as ERROR entries — all are significant timeline events.
	path := writeLogFile(t, dir, "app.log", jsonLogLines())

	out := callTool[timelineResult](t, session, "timeline", map[string]any{
		"path": path,
	})

	if out.EventCount == 0 {
		t.Fatal("expected at least 1 timeline event")
	}

	// Look for startup and shutdown events.
	typeSet := make(map[string]bool)
	for _, ev := range out.Events {
		typeSet[ev.Type] = true
	}

	if !typeSet["startup"] {
		t.Error("expected a 'startup' event for 'server started' line")
	}
	if !typeSet["shutdown"] {
		t.Error("expected a 'shutdown' event for 'shutting down' line")
	}
	if !typeSet["ERROR"] {
		t.Error("expected 'ERROR' events in the timeline")
	}
	if out.Truncated {
		t.Error("expected truncated=false for 10-line file")
	}
}

func TestCorrelateLogsCrossFile(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	pathA := writeLogFile(t, dir, "gateway.log", correlationFileA())
	pathB := writeLogFile(t, dir, "worker.log", correlationFileB())

	out := callTool[correlateLogsResult](t, session, "correlate_logs", map[string]any{
		"paths":             []string{pathA, pathB},
		"correlation_field": "request_id",
	})

	if out.TotalGroups == 0 {
		t.Fatal("expected at least 1 correlated group")
	}
	if out.GroupsReturned == 0 {
		t.Fatal("expected groups_returned > 0")
	}

	// Both req-001 and req-002 appear in both files.
	expectedIDs := map[string]bool{
		"req-001": false,
		"req-002": false,
	}

	for _, g := range out.CorrelatedGroups {
		if _, ok := expectedIDs[g.CorrelationID]; ok {
			expectedIDs[g.CorrelationID] = true
		}

		// Each group spanning both files must have exactly 2 files involved.
		if len(g.FilesInvolved) < 2 {
			t.Errorf("group %q: expected >= 2 files_involved, got %d", g.CorrelationID, len(g.FilesInvolved))
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected correlated group for %q spanning both files", id)
		}
	}
}

func TestToolErrorPropagation(t *testing.T) {
	session := setupTestServer(t)

	t.Run("nonexistent file", func(t *testing.T) {
		errText := callToolExpectError(t, session, "read_logs", map[string]any{
			"path": "/no/such/file.log",
		})
		if !strings.Contains(errText, "FILE_NOT_FOUND") {
			t.Errorf("expected error to contain 'FILE_NOT_FOUND', got: %s", errText)
		}
	})

	t.Run("nonexistent file via tail", func(t *testing.T) {
		errText := callToolExpectError(t, session, "tail_logs", map[string]any{
			"path": "/no/such/file.log",
		})
		if !strings.Contains(errText, "FILE_NOT_FOUND") {
			t.Errorf("expected error to contain 'FILE_NOT_FOUND', got: %s", errText)
		}
	})

	t.Run("nonexistent file via search", func(t *testing.T) {
		errText := callToolExpectError(t, session, "search_logs", map[string]any{
			"path":    "/no/such/file.log",
			"pattern": "test",
		})
		if !strings.Contains(errText, "FILE_NOT_FOUND") {
			t.Errorf("expected error to contain 'FILE_NOT_FOUND', got: %s", errText)
		}
	})
}

func TestBinaryFileRejection(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()

	tools := []string{
		"read_logs",
		"tail_logs",
		"summarize_logs",
		"parse_logs",
		"filter_logs",
		"extract_errors",
		"detect_anomalies",
		"timeline",
	}

	for _, toolName := range tools {
		t.Run(toolName, func(t *testing.T) {
			binPath := writeBinaryFile(t, dir, toolName+".bin")

			args := map[string]any{
				"path": binPath,
			}
			// search_logs and parse_logs require extra args, but they aren't
			// in this list or would fail on binary check before needing them.
			if toolName == "search_logs" {
				args["pattern"] = "test"
			}

			errText := callToolExpectError(t, session, toolName, args)

			if !strings.Contains(strings.ToLower(errText), "binary") {
				t.Errorf("%s: expected error to mention 'binary', got: %s", toolName, errText)
			}
		})
	}
}

// --- Compressed file integration tests ---

type decompressFileResult struct {
	TempPath         string `json:"temp_path"`
	OriginalPath     string `json:"original_path"`
	CompressedSize   int64  `json:"compressed_size"`
	DecompressedSize int64  `json:"decompressed_size"`
	Note             string `json:"note"`
}

func TestReadLogsGzipRoundTrip(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	lines := jsonLogLines()
	path := writeGzipLogFile(t, dir, "app.log.gz", lines)

	out := callTool[readLogsResult](t, session, "read_logs", map[string]any{
		"path": path,
	})

	if len(out.Lines) != len(lines) {
		t.Fatalf("expected %d lines, got %d", len(lines), len(out.Lines))
	}
	if out.HasMore {
		t.Error("expected has_more=false")
	}
	if !strings.Contains(out.Lines[0].Content, "server started") {
		t.Errorf("first line mismatch: %s", out.Lines[0].Content)
	}
	if out.FileSizeBytes <= 0 {
		t.Error("expected file_size_bytes > 0")
	}
}

func TestReadLogsZipRoundTrip(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	lines := jsonLogLines()
	path := writeZipLogFile(t, dir, "app.log.zip", "app.log", lines)

	out := callTool[readLogsResult](t, session, "read_logs", map[string]any{
		"path": path,
	})

	if len(out.Lines) != len(lines) {
		t.Fatalf("expected %d lines, got %d", len(lines), len(out.Lines))
	}
	if !strings.Contains(out.Lines[0].Content, "server started") {
		t.Errorf("first line mismatch: %s", out.Lines[0].Content)
	}
}

func TestTailLogsGzipRoundTrip(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	lines := jsonLogLines()
	path := writeGzipLogFile(t, dir, "app.log.gz", lines)

	out := callTool[tailLogsResult](t, session, "tail_logs", map[string]any{
		"path":      path,
		"num_lines": 3,
	})

	if len(out.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(out.Lines))
	}
	if out.TotalLines != len(lines) {
		t.Errorf("expected total_lines=%d, got %d", len(lines), out.TotalLines)
	}
	if !strings.Contains(out.Lines[2].Content, "shutting down") {
		t.Errorf("last line mismatch: %s", out.Lines[2].Content)
	}
}

func TestDecompressFileRoundTrip(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	lines := jsonLogLines()
	path := writeGzipLogFile(t, dir, "app.log.gz", lines)

	t.Cleanup(tools.CleanupTempFiles)

	out := callTool[decompressFileResult](t, session, "decompress_file", map[string]any{
		"path": path,
	})

	if out.OriginalPath != path {
		t.Errorf("original_path mismatch: got %s, want %s", out.OriginalPath, path)
	}
	if out.CompressedSize <= 0 {
		t.Error("expected compressed_size > 0")
	}
	if out.DecompressedSize <= 0 {
		t.Error("expected decompressed_size > 0")
	}
	if out.DecompressedSize <= out.CompressedSize {
		t.Errorf("decompressed (%d) should be larger than compressed (%d)", out.DecompressedSize, out.CompressedSize)
	}
	if out.TempPath == "" {
		t.Fatal("expected non-empty temp_path")
	}
	if out.Note == "" {
		t.Error("expected non-empty note")
	}

	// Verify the temp file is a plain file that other tools can read
	readOut := callTool[readLogsResult](t, session, "read_logs", map[string]any{
		"path": out.TempPath,
	})
	if len(readOut.Lines) != len(lines) {
		t.Fatalf("read_logs on temp file: expected %d lines, got %d", len(lines), len(readOut.Lines))
	}

	// Verify tail_logs also works on the temp file with O(N) seek
	tailOut := callTool[tailLogsResult](t, session, "tail_logs", map[string]any{
		"path":      out.TempPath,
		"num_lines": 2,
	})
	if len(tailOut.Lines) != 2 {
		t.Fatalf("tail_logs on temp file: expected 2 lines, got %d", len(tailOut.Lines))
	}
}

func TestDecompressFileNotCompressedError(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	path := writeLogFile(t, dir, "plain.log", jsonLogLines())

	errText := callToolExpectError(t, session, "decompress_file", map[string]any{
		"path": path,
	})

	if !strings.Contains(errText, "compressed extension") {
		t.Errorf("expected error about compressed extension, got: %s", errText)
	}
}

func TestSearchLogsGzipRoundTrip(t *testing.T) {
	session := setupTestServer(t)
	dir := t.TempDir()
	path := writeGzipLogFile(t, dir, "app.log.gz", jsonLogLines())

	out := callTool[searchLogsResult](t, session, "search_logs", map[string]any{
		"path":    path,
		"pattern": "ERROR",
	})

	if out.TotalMatches != 2 {
		t.Errorf("expected 2 ERROR matches, got %d", out.TotalMatches)
	}
}

func TestDecompressFileThenMultiTool(t *testing.T) {
	// Simulate the recommended workflow: decompress once, then use temp path
	// for multiple tools.
	session := setupTestServer(t)
	dir := t.TempDir()

	// Create a gzip file with enough data to make decompression worthwhile.
	var lines []string
	for i := 1; i <= 50; i++ {
		level := "INFO"
		if i%10 == 0 {
			level = "ERROR"
		}
		lines = append(lines, fmt.Sprintf(
			`{"timestamp":"2025-01-15T10:%02d:00Z","level":"%s","msg":"event %d","source":"app"}`,
			i%60, level, i,
		))
	}
	path := writeGzipLogFile(t, dir, "big.log.gz", lines)
	t.Cleanup(tools.CleanupTempFiles)

	// Step 1: Decompress
	decomp := callTool[decompressFileResult](t, session, "decompress_file", map[string]any{
		"path": path,
	})
	tempPath := decomp.TempPath

	// Step 2: summarize_logs on the temp file
	type summarizeFileInfo struct {
		TotalLines int `json:"total_lines"`
	}
	type summarizeResult struct {
		FileInfo      summarizeFileInfo `json:"file_info"`
		LinesAnalyzed int              `json:"lines_analyzed"`
	}
	summary := callTool[summarizeResult](t, session, "summarize_logs", map[string]any{
		"path": tempPath,
	})
	if summary.LinesAnalyzed != 50 {
		t.Errorf("summarize: expected lines_analyzed=50, got %d", summary.LinesAnalyzed)
	}
	if summary.FileInfo.TotalLines != 50 {
		t.Errorf("summarize: expected file_info.total_lines=50, got %d", summary.FileInfo.TotalLines)
	}

	// Step 3: search on the temp file
	search := callTool[searchLogsResult](t, session, "search_logs", map[string]any{
		"path":    tempPath,
		"pattern": "ERROR",
	})
	if search.TotalMatches != 5 {
		t.Errorf("search: expected 5 ERROR matches, got %d", search.TotalMatches)
	}

	// Step 4: tail on the temp file (uses fast O(N) seek, not streaming)
	tail := callTool[tailLogsResult](t, session, "tail_logs", map[string]any{
		"path":      tempPath,
		"num_lines": 5,
	})
	if len(tail.Lines) != 5 {
		t.Errorf("tail: expected 5 lines, got %d", len(tail.Lines))
	}
	if tail.TotalLines != 50 {
		t.Errorf("tail: expected total_lines=50, got %d", tail.TotalLines)
	}
}
