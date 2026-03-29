package tools

import (
	"fmt"
	"strings"
	"testing"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

func TestRunDetectAnomalies(t *testing.T) {
	// --- Test data ---

	// Error spike: many errors concentrated in one window, normal elsewhere.
	errorSpikeLinesData := []string{
		`{"timestamp":"2025-01-15T02:00:00Z","level":"info","msg":"Normal operation"}`,
		`{"timestamp":"2025-01-15T02:01:00Z","level":"info","msg":"Normal operation"}`,
		`{"timestamp":"2025-01-15T02:02:00Z","level":"info","msg":"Normal operation"}`,
		`{"timestamp":"2025-01-15T02:03:00Z","level":"info","msg":"Normal operation"}`,
		`{"timestamp":"2025-01-15T02:04:00Z","level":"info","msg":"Normal operation"}`,
		`{"timestamp":"2025-01-15T02:05:00Z","level":"info","msg":"Normal operation"}`,
		`{"timestamp":"2025-01-15T02:06:00Z","level":"info","msg":"Normal operation"}`,
		`{"timestamp":"2025-01-15T02:07:00Z","level":"info","msg":"Normal operation"}`,
		`{"timestamp":"2025-01-15T02:08:00Z","level":"info","msg":"Normal operation"}`,
		`{"timestamp":"2025-01-15T02:09:00Z","level":"info","msg":"Normal operation"}`,
	}
	// Add a burst of errors in the 02:10 window.
	for i := 0; i < 30; i++ {
		errorSpikeLinesData = append(errorSpikeLinesData,
			fmt.Sprintf(`{"timestamp":"2025-01-15T02:10:%02dZ","level":"error","msg":"Database connection refused attempt %d"}`, i, i))
	}
	// More normal lines after.
	for i := 15; i < 25; i++ {
		errorSpikeLinesData = append(errorSpikeLinesData,
			fmt.Sprintf(`{"timestamp":"2025-01-15T02:%02d:00Z","level":"info","msg":"Normal operation resumed"}`, i))
	}
	errorSpikeLog := strings.Join(errorSpikeLinesData, "\n") + "\n"
	errorSpikePath := writeTempLog(t, "error_spike.log", errorSpikeLog)

	// Normal distribution: uniform log lines, no anomalies expected.
	var normalLines []string
	for i := 0; i < 50; i++ {
		min := i % 60
		normalLines = append(normalLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T02:%02d:00Z","level":"info","msg":"Routine heartbeat check"}`, min))
	}
	normalLog := strings.Join(normalLines, "\n") + "\n"
	normalPath := writeTempLog(t, "normal.log", normalLog)

	// Gap: long silence between entries.
	gapLines := []string{
		`{"timestamp":"2025-01-15T02:00:00Z","level":"info","msg":"Entry one"}`,
		`{"timestamp":"2025-01-15T02:00:01Z","level":"info","msg":"Entry two"}`,
		`{"timestamp":"2025-01-15T02:00:02Z","level":"info","msg":"Entry three"}`,
		`{"timestamp":"2025-01-15T02:00:03Z","level":"info","msg":"Entry four"}`,
		`{"timestamp":"2025-01-15T02:00:04Z","level":"info","msg":"Entry five"}`,
		`{"timestamp":"2025-01-15T02:00:05Z","level":"info","msg":"Entry six"}`,
		`{"timestamp":"2025-01-15T02:00:06Z","level":"info","msg":"Entry seven"}`,
		`{"timestamp":"2025-01-15T02:00:07Z","level":"info","msg":"Entry eight"}`,
		`{"timestamp":"2025-01-15T02:00:08Z","level":"info","msg":"Entry nine"}`,
		`{"timestamp":"2025-01-15T02:00:09Z","level":"info","msg":"Entry ten"}`,
		// Big gap here: 2 hours later
		`{"timestamp":"2025-01-15T04:00:09Z","level":"info","msg":"Service recovered"}`,
		`{"timestamp":"2025-01-15T04:00:10Z","level":"info","msg":"Resuming operations"}`,
		`{"timestamp":"2025-01-15T04:00:11Z","level":"info","msg":"All systems go"}`,
	}
	gapLog := strings.Join(gapLines, "\n") + "\n"
	gapPath := writeTempLog(t, "gap.log", gapLog)

	// Rate change: burst of lines in one window.
	var rateChangeLines []string
	// 4 windows with 2 lines each (02:00, 02:05, 02:10, 02:15).
	for w := 0; w < 4; w++ {
		base := w * 5
		for j := 0; j < 2; j++ {
			rateChangeLines = append(rateChangeLines,
				fmt.Sprintf(`{"timestamp":"2025-01-15T02:%02d:%02dZ","level":"info","msg":"Normal rate"}`, base, j))
		}
	}
	// Burst window at 02:20: 30 lines — 15x the average of 2, well above 3x threshold.
	for i := 0; i < 30; i++ {
		rateChangeLines = append(rateChangeLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T02:20:%02dZ","level":"info","msg":"Burst line %d"}`, i%60, i))
	}
	rateChangeLog := strings.Join(rateChangeLines, "\n") + "\n"
	rateChangePath := writeTempLog(t, "rate_change.log", rateChangeLog)

	// Empty file.
	emptyPath := writeTempLog(t, "empty.log", "")

	// New error type: errors in last 20% that were not in first 80%.
	var newErrorLines []string
	// First 80 entries: a mix of info and one recurring error pattern.
	for i := 0; i < 80; i++ {
		sec := i % 60
		min := i / 60
		if i%10 == 0 {
			newErrorLines = append(newErrorLines,
				fmt.Sprintf(`{"timestamp":"2025-01-15T02:%02d:%02dZ","level":"error","msg":"Known database timeout error"}`, min, sec))
		} else {
			newErrorLines = append(newErrorLines,
				fmt.Sprintf(`{"timestamp":"2025-01-15T02:%02d:%02dZ","level":"info","msg":"Normal operation %d"}`, min, sec, i))
		}
	}
	// Last 20 entries: introduce a brand new error.
	for i := 80; i < 100; i++ {
		sec := i % 60
		min := i / 60
		if i%5 == 0 {
			newErrorLines = append(newErrorLines,
				fmt.Sprintf(`{"timestamp":"2025-01-15T02:%02d:%02dZ","level":"error","msg":"SSL handshake failed: certificate expired"}`, min, sec))
		} else {
			newErrorLines = append(newErrorLines,
				fmt.Sprintf(`{"timestamp":"2025-01-15T02:%02d:%02dZ","level":"info","msg":"Normal operation %d"}`, min, sec, i))
		}
	}
	newErrorLog := strings.Join(newErrorLines, "\n") + "\n"
	newErrorPath := writeTempLog(t, "new_error.log", newErrorLog)

	// Sensitivity test: borderline data detectable at high but not low.
	// Create data where errors in one window are ~2.5x the average.
	var sensitivityLines []string
	// 4 windows with 1 error each (window = 5 min).
	for w := 0; w < 4; w++ {
		base := w * 5
		sensitivityLines = append(sensitivityLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T02:%02d:00Z","level":"error","msg":"Baseline error"}`, base))
		for j := 1; j < 5; j++ {
			sensitivityLines = append(sensitivityLines,
				fmt.Sprintf(`{"timestamp":"2025-01-15T02:%02d:00Z","level":"info","msg":"Info line"}`, base+j))
		}
	}
	// 5th window with 5 errors (~5x baseline of 1 avg error per window).
	for j := 0; j < 5; j++ {
		sensitivityLines = append(sensitivityLines,
			fmt.Sprintf(`{"timestamp":"2025-01-15T02:20:%02dZ","level":"error","msg":"Spike error %d"}`, j, j))
	}
	sensitivityLog := strings.Join(sensitivityLines, "\n") + "\n"
	sensitivityPath := writeTempLog(t, "sensitivity.log", sensitivityLog)

	tests := []struct {
		name        string
		input       DetectAnomaliesInput
		wantErr     bool
		errContains string
		checkOutput func(t *testing.T, out DetectAnomaliesOutput)
	}{
		{
			name:  "error spike detected",
			input: DetectAnomaliesInput{Path: errorSpikePath, WindowMinutes: 5, Sensitivity: "medium"},
			checkOutput: func(t *testing.T, out DetectAnomaliesOutput) {
				found := false
				for _, a := range out.Anomalies {
					if a.Type == "error_spike" {
						found = true
						if a.Severity != "high" {
							t.Errorf("error_spike severity = %q, want high", a.Severity)
						}
						if len(a.EvidenceLines) == 0 {
							t.Error("error_spike should have evidence lines")
						}
						if a.Details["multiplier"] == nil {
							t.Error("error_spike should have multiplier in details")
						}
						break
					}
				}
				if !found {
					t.Errorf("expected error_spike anomaly, got types: %v", anomalyTypes(out.Anomalies))
				}
			},
		},
		{
			name:  "normal distribution produces no anomalies",
			input: DetectAnomaliesInput{Path: normalPath, WindowMinutes: 5, Sensitivity: "medium"},
			checkOutput: func(t *testing.T, out DetectAnomaliesOutput) {
				if len(out.Anomalies) != 0 {
					t.Errorf("expected no anomalies for uniform log, got %d: %v", len(out.Anomalies), anomalyTypes(out.Anomalies))
				}
			},
		},
		{
			name:  "time gap detected",
			input: DetectAnomaliesInput{Path: gapPath, WindowMinutes: 5, Sensitivity: "medium"},
			checkOutput: func(t *testing.T, out DetectAnomaliesOutput) {
				found := false
				for _, a := range out.Anomalies {
					if a.Type == "gap" {
						found = true
						if a.Severity != "high" {
							t.Errorf("gap severity = %q, want high", a.Severity)
						}
						gapDur, ok := a.Details["gap_duration_seconds"].(float64)
						if !ok {
							t.Fatal("gap_duration_seconds not a float64")
						}
						// Gap is ~2 hours = 7200 seconds.
						if gapDur < 7000 {
							t.Errorf("gap_duration_seconds = %f, expected >= 7000", gapDur)
						}
						break
					}
				}
				if !found {
					t.Errorf("expected gap anomaly, got types: %v", anomalyTypes(out.Anomalies))
				}
			},
		},
		{
			name:  "rate change detected",
			input: DetectAnomaliesInput{Path: rateChangePath, WindowMinutes: 5, Sensitivity: "medium"},
			checkOutput: func(t *testing.T, out DetectAnomaliesOutput) {
				found := false
				for _, a := range out.Anomalies {
					if a.Type == "rate_change" {
						found = true
						if a.Severity != "medium" {
							t.Errorf("rate_change severity = %q, want medium", a.Severity)
						}
						break
					}
				}
				if !found {
					t.Errorf("expected rate_change anomaly, got types: %v", anomalyTypes(out.Anomalies))
				}
			},
		},
		{
			name:  "empty file returns no anomalies",
			input: DetectAnomaliesInput{Path: emptyPath},
			checkOutput: func(t *testing.T, out DetectAnomaliesOutput) {
				if len(out.Anomalies) != 0 {
					t.Errorf("expected no anomalies for empty file, got %d", len(out.Anomalies))
				}
				if out.AnalysisMetadata.TotalLinesAnalyzed != 0 {
					t.Errorf("TotalLinesAnalyzed = %d, want 0", out.AnalysisMetadata.TotalLinesAnalyzed)
				}
			},
		},
		{
			name:        "file not found",
			input:       DetectAnomaliesInput{Path: "/nonexistent/file.log"},
			wantErr:     true,
			errContains: "FILE_NOT_FOUND",
		},
		{
			name:  "default values applied",
			input: DetectAnomaliesInput{Path: normalPath},
			checkOutput: func(t *testing.T, out DetectAnomaliesOutput) {
				if out.AnalysisMetadata.WindowMinutes != 5 {
					t.Errorf("default WindowMinutes = %d, want 5", out.AnalysisMetadata.WindowMinutes)
				}
				if out.AnalysisMetadata.Sensitivity != "medium" {
					t.Errorf("default Sensitivity = %q, want medium", out.AnalysisMetadata.Sensitivity)
				}
			},
		},
		{
			name:  "metadata fields populated correctly",
			input: DetectAnomaliesInput{Path: errorSpikePath, WindowMinutes: 5, Sensitivity: "medium"},
			checkOutput: func(t *testing.T, out DetectAnomaliesOutput) {
				m := out.AnalysisMetadata
				if m.TotalLinesAnalyzed == 0 {
					t.Error("TotalLinesAnalyzed should be > 0")
				}
				if m.WindowMinutes != 5 {
					t.Errorf("WindowMinutes = %d, want 5", m.WindowMinutes)
				}
				if m.Sensitivity != "medium" {
					t.Errorf("Sensitivity = %q, want medium", m.Sensitivity)
				}
				if m.WindowsAnalyzed == 0 {
					t.Error("WindowsAnalyzed should be > 0")
				}
				if m.TimeSpanHours <= 0 {
					t.Errorf("TimeSpanHours = %f, want > 0", m.TimeSpanHours)
				}
			},
		},
		{
			name:  "high sensitivity catches more anomalies than low",
			input: DetectAnomaliesInput{Path: sensitivityPath, WindowMinutes: 5, Sensitivity: "high"},
			checkOutput: func(t *testing.T, outHigh DetectAnomaliesOutput) {
				// Run again with low sensitivity on the same data.
				outLow, err := RunDetectAnomalies(DetectAnomaliesInput{
					Path:          sensitivityPath,
					WindowMinutes: 5,
					Sensitivity:   "low",
				})
				if err != nil {
					t.Fatalf("low sensitivity run failed: %v", err)
				}
				if len(outHigh.Anomalies) < len(outLow.Anomalies) {
					t.Errorf("high sensitivity found %d anomalies, low found %d — expected high >= low",
						len(outHigh.Anomalies), len(outLow.Anomalies))
				}
			},
		},
		{
			name:  "new error type detected",
			input: DetectAnomaliesInput{Path: newErrorPath, WindowMinutes: 5, Sensitivity: "medium"},
			checkOutput: func(t *testing.T, out DetectAnomaliesOutput) {
				found := false
				for _, a := range out.Anomalies {
					if a.Type == "new_error_type" {
						found = true
						if a.Severity != "low" {
							t.Errorf("new_error_type severity = %q, want low", a.Severity)
						}
						if a.Details["pattern"] == nil {
							t.Error("new_error_type should have pattern in details")
						}
						if len(a.EvidenceLines) == 0 {
							t.Error("new_error_type should have evidence lines")
						}
						break
					}
				}
				if !found {
					t.Errorf("expected new_error_type anomaly, got types: %v", anomalyTypes(out.Anomalies))
				}
			},
		},
		{
			name:  "anomalies sorted by severity then time",
			input: DetectAnomaliesInput{Path: errorSpikePath, WindowMinutes: 5, Sensitivity: "medium"},
			checkOutput: func(t *testing.T, out DetectAnomaliesOutput) {
				if len(out.Anomalies) < 2 {
					return // can't check ordering with < 2 anomalies
				}
				rank := map[string]int{"high": 0, "medium": 1, "low": 2}
				for i := 1; i < len(out.Anomalies); i++ {
					prev := out.Anomalies[i-1]
					curr := out.Anomalies[i]
					rp := rank[prev.Severity]
					rc := rank[curr.Severity]
					if rp > rc {
						t.Errorf("anomaly %d (severity=%s) should come before anomaly %d (severity=%s)",
							i, curr.Severity, i-1, prev.Severity)
					}
					if rp == rc && prev.TimeRange.Start > curr.TimeRange.Start {
						t.Errorf("within severity %s, anomaly at %s should come before %s",
							prev.Severity, curr.TimeRange.Start, prev.TimeRange.Start)
					}
				}
			},
		},
		{
			name:  "window minutes clamped to minimum",
			input: DetectAnomaliesInput{Path: normalPath, WindowMinutes: -5},
			checkOutput: func(t *testing.T, out DetectAnomaliesOutput) {
				if out.AnalysisMetadata.WindowMinutes != 1 {
					t.Errorf("WindowMinutes = %d, want 1 (clamped minimum)", out.AnalysisMetadata.WindowMinutes)
				}
			},
		},
		{
			name:  "window minutes clamped to maximum",
			input: DetectAnomaliesInput{Path: normalPath, WindowMinutes: 999},
			checkOutput: func(t *testing.T, out DetectAnomaliesOutput) {
				if out.AnalysisMetadata.WindowMinutes != 60 {
					t.Errorf("WindowMinutes = %d, want 60 (clamped maximum)", out.AnalysisMetadata.WindowMinutes)
				}
			},
		},
		{
			name:  "anomalies slice is non-nil even with no anomalies",
			input: DetectAnomaliesInput{Path: normalPath},
			checkOutput: func(t *testing.T, out DetectAnomaliesOutput) {
				if out.Anomalies == nil {
					t.Error("Anomalies should be non-nil (empty slice), not nil")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := RunDetectAnomalies(tc.input)
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

func TestParseAnomalyTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "RFC3339", input: "2025-01-15T02:00:00Z", wantErr: false},
		{name: "RFC3339 with offset", input: "2025-01-15T02:00:00+05:00", wantErr: false},
		{name: "bare ISO8601", input: "2025-01-15T02:00:00", wantErr: false},
		{name: "garbage", input: "not-a-timestamp", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseAnomalyTimestamp(tc.input)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// anomalyTypes returns a slice of type strings from a list of anomalies, for error messages.
func anomalyTypes(anomalies []types.Anomaly) []string {
	out := make([]string, len(anomalies))
	for i, a := range anomalies {
		out[i] = a.Type
	}
	return out
}
