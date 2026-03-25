package tools

import (
	"fmt"
	"sort"
	"time"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
	"github.com/trickyearlobe/log-analysis-mcp/internal/parsers"
	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// detectAnomaliesPageSize is the number of lines to read per streaming page.
const detectAnomaliesPageSize = 1000

// maxEvidenceLines is the maximum number of evidence lines kept per anomaly.
const maxEvidenceLines = 5

// DetectAnomaliesInput defines the parameters for the detect_anomalies tool.
type DetectAnomaliesInput struct {
	Path          string `json:"path"           jsonschema:"required,description=Path to the log file"`
	WindowMinutes int    `json:"window_minutes" jsonschema:"description=Time window in minutes for rate analysis,minimum=1,maximum=60"`
	Sensitivity   string `json:"sensitivity"    jsonschema:"description=Detection sensitivity level,enum=low,enum=medium,enum=high"`
}

// AnalysisMetadata contains summary statistics about the anomaly detection run.
type AnalysisMetadata struct {
	TotalLinesAnalyzed int     `json:"total_lines_analyzed"`
	TimeSpanHours      float64 `json:"time_span_hours"`
	WindowMinutes      int     `json:"window_minutes"`
	Sensitivity        string  `json:"sensitivity"`
	WindowsAnalyzed    int     `json:"windows_analyzed"`
}

// DetectAnomaliesOutput is the structured result of the detect_anomalies tool.
type DetectAnomaliesOutput struct {
	Anomalies        []types.Anomaly  `json:"anomalies"`
	AnalysisMetadata AnalysisMetadata `json:"analysis_metadata"`
}

// sensitivityThresholds holds multiplier thresholds for a given sensitivity level.
type sensitivityThresholds struct {
	errorSpike float64
	rateChange float64
	gap        float64
}

// thresholdMap maps sensitivity names to their multiplier thresholds.
var thresholdMap = map[string]sensitivityThresholds{
	"low":    {errorSpike: 5.0, rateChange: 5.0, gap: 10.0},
	"medium": {errorSpike: 3.0, rateChange: 3.0, gap: 5.0},
	"high":   {errorSpike: 2.0, rateChange: 2.0, gap: 3.0},
}

// windowData tracks per-window statistics during streaming.
type windowData struct {
	windowStart time.Time
	totalCount  int
	errorCount  int
	evidence    []types.EvidenceLine
}

// gapRecord records a detected gap between consecutive log entries.
type gapRecord struct {
	beforeLine int
	afterLine  int
	beforeTime time.Time
	afterTime  time.Time
	gapSeconds float64
}

// errorOccurrence tracks where a normalized error pattern was first seen.
type errorOccurrence struct {
	count         int
	firstSeenLine int
	firstSeenTime string
	evidence      types.EvidenceLine
}

// RunDetectAnomalies analyzes a log file for anomalous patterns including error
// spikes, rate changes, gaps in logging, and new error types.
func RunDetectAnomalies(input DetectAnomaliesInput) (DetectAnomaliesOutput, error) {
	// Apply defaults and clamp.
	input.WindowMinutes = DefaultInt(input.WindowMinutes, 5)
	input.WindowMinutes = ClampInt(input.WindowMinutes, 1, 60)
	input.Sensitivity = DefaultString(input.Sensitivity, "medium")

	thresholds, ok := thresholdMap[input.Sensitivity]
	if !ok {
		thresholds = thresholdMap["medium"]
		input.Sensitivity = "medium"
	}

	// Validate file access.
	if err := CheckFileAccess(input.Path); err != nil {
		return DetectAnomaliesOutput{}, fmt.Errorf("detect_anomalies: %w", err)
	}

	// Sample lines and auto-detect format.
	sampleLines, err := SampleLines(input.Path, sampleLineCount)
	if err != nil {
		return DetectAnomaliesOutput{}, fmt.Errorf("detect_anomalies: %w", err)
	}

	_, parser := parsers.AutoDetectWithHint(sampleLines, "")

	// Streaming state.
	windows := make(map[string]*windowData)
	windowDuration := time.Duration(input.WindowMinutes) * time.Minute

	var gaps []gapRecord
	var prevTime *time.Time
	var prevLineNum int
	var intervals []float64

	// For new error type detection: track which line index each entry is.
	totalParsedEntries := 0
	type entryRecord struct {
		index      int
		lineNumber int
		raw        string
		message    string
		isError    bool
		pattern    string
		timestamp  string
	}
	var allEntries []entryRecord

	var firstTime, lastTime *time.Time
	totalLines := 0

	startLine := 1
	for {
		result, err := fileutil.ReadLines(input.Path, startLine, detectAnomaliesPageSize)
		if err != nil {
			return DetectAnomaliesOutput{}, fmt.Errorf("detect_anomalies: read at line %d: %w", startLine, err)
		}

		totalLines += len(result.Lines)

		if parser != nil {
			for _, lr := range result.Lines {
				entry := parser.Parse(lr.Text)
				if entry == nil {
					continue
				}
				entry.LineNumber = lr.LineNumber

				totalParsedEntries++
				isErr := isErrorLevel(entry.Level)

				var pattern string
				if isErr {
					pattern = normalizeMessage(entry.Message)
				}

				var tsStr string
				if entry.Timestamp != nil {
					tsStr = *entry.Timestamp
				}

				allEntries = append(allEntries, entryRecord{
					index:      totalParsedEntries,
					lineNumber: lr.LineNumber,
					raw:        lr.Text,
					message:    entry.Message,
					isError:    isErr,
					pattern:    pattern,
					timestamp:  tsStr,
				})

				// Parse timestamp for time-based analysis.
				if entry.Timestamp == nil {
					continue
				}

				ts, err := parseAnomalyTimestamp(*entry.Timestamp)
				if err != nil {
					continue
				}

				// Track global time range.
				if firstTime == nil {
					ft := ts
					firstTime = &ft
				}
				lt := ts
				lastTime = &lt

				// Bucket into windows.
				windowStart := ts.Truncate(windowDuration)
				key := windowStart.Format(time.RFC3339)
				wd, exists := windows[key]
				if !exists {
					wd = &windowData{
						windowStart: windowStart,
						evidence:    []types.EvidenceLine{},
					}
					windows[key] = wd
				}
				wd.totalCount++
				if isErr {
					wd.errorCount++
					if len(wd.evidence) < maxEvidenceLines {
						wd.evidence = append(wd.evidence, types.EvidenceLine{
							LineNumber: lr.LineNumber,
							Content:    lr.Text,
						})
					}
				}

				// Track inter-entry intervals for gap detection.
				if prevTime != nil {
					gap := ts.Sub(*prevTime).Seconds()
					if gap >= 0 {
						intervals = append(intervals, gap)
						gaps = append(gaps, gapRecord{
							beforeLine: prevLineNum,
							afterLine:  lr.LineNumber,
							beforeTime: *prevTime,
							afterTime:  ts,
							gapSeconds: gap,
						})
					}
				}
				pt := ts
				prevTime = &pt
				prevLineNum = lr.LineNumber
			}
		}

		if !result.HasMore || len(result.Lines) == 0 {
			break
		}
		startLine += len(result.Lines)
	}

	// Compute baselines.
	numWindows := len(windows)
	var anomalies []types.Anomaly

	if numWindows > 0 {
		totalErrors := 0
		totalVolume := 0
		for _, wd := range windows {
			totalErrors += wd.errorCount
			totalVolume += wd.totalCount
		}
		avgErrorRate := float64(totalErrors) / float64(numWindows)
		avgVolume := float64(totalVolume) / float64(numWindows)

		var avgInterval float64
		if len(intervals) > 0 {
			sum := 0.0
			for _, iv := range intervals {
				sum += iv
			}
			avgInterval = sum / float64(len(intervals))
		}

		// Detect error spikes.
		if avgErrorRate > 0 {
			for key, wd := range windows {
				if float64(wd.errorCount) > avgErrorRate*thresholds.errorSpike {
					multiplier := float64(wd.errorCount) / avgErrorRate
					windowEnd := wd.windowStart.Add(windowDuration)
					anomalies = append(anomalies, types.Anomaly{
						Type:     "error_spike",
						Severity: "high",
						Description: fmt.Sprintf(
							"Error rate increased %.1fx compared to baseline in %d-minute window",
							multiplier, input.WindowMinutes,
						),
						TimeRange: types.TimeRange{
							Start: key,
							End:   windowEnd.Format(time.RFC3339),
						},
						Details: map[string]interface{}{
							"baseline_error_rate": avgErrorRate,
							"spike_error_rate":    float64(wd.errorCount),
							"multiplier":          multiplier,
						},
						EvidenceLines: wd.evidence,
					})
				}
			}
		}

		// Detect rate changes.
		if avgVolume > 0 {
			for key, wd := range windows {
				if float64(wd.totalCount) > avgVolume*thresholds.rateChange {
					multiplier := float64(wd.totalCount) / avgVolume
					windowEnd := wd.windowStart.Add(windowDuration)
					anomalies = append(anomalies, types.Anomaly{
						Type:     "rate_change",
						Severity: "medium",
						Description: fmt.Sprintf(
							"Log volume increased %.1fx compared to average in %d-minute window",
							multiplier, input.WindowMinutes,
						),
						TimeRange: types.TimeRange{
							Start: key,
							End:   windowEnd.Format(time.RFC3339),
						},
						Details: map[string]interface{}{
							"avg_volume":     avgVolume,
							"window_volume":  float64(wd.totalCount),
							"multiplier":     multiplier,
						},
						EvidenceLines: []types.EvidenceLine{},
					})
				}
			}
		}

		// Detect gaps.
		if avgInterval > 0 {
			gapThreshold := avgInterval * thresholds.gap
			for _, g := range gaps {
				if g.gapSeconds > gapThreshold {
					anomalies = append(anomalies, types.Anomaly{
						Type:     "gap",
						Severity: "high",
						Description: fmt.Sprintf(
							"No log entries for %.0f seconds (expected interval: ~%.1f seconds)",
							g.gapSeconds, avgInterval,
						),
						TimeRange: types.TimeRange{
							Start: g.beforeTime.Format(time.RFC3339),
							End:   g.afterTime.Format(time.RFC3339),
						},
						Details: map[string]interface{}{
							"gap_duration_seconds": g.gapSeconds,
							"avg_interval_seconds": avgInterval,
						},
						EvidenceLines: []types.EvidenceLine{},
					})
				}
			}
		}
	}

	// Detect new error types: collect patterns from first 80%, flag new ones in last 20%.
	if len(allEntries) > 0 {
		threshold80 := len(allEntries) * 80 / 100
		if threshold80 == 0 {
			threshold80 = 1
		}

		knownPatterns := make(map[string]bool)
		for i := 0; i < threshold80 && i < len(allEntries); i++ {
			e := allEntries[i]
			if e.isError {
				knownPatterns[e.pattern] = true
			}
		}

		// Scan last 20% for new patterns.
		newPatterns := make(map[string]*errorOccurrence)
		for i := threshold80; i < len(allEntries); i++ {
			e := allEntries[i]
			if !e.isError {
				continue
			}
			if knownPatterns[e.pattern] {
				continue
			}
			occ, exists := newPatterns[e.pattern]
			if !exists {
				occ = &errorOccurrence{
					firstSeenLine: e.lineNumber,
					firstSeenTime: e.timestamp,
					evidence: types.EvidenceLine{
						LineNumber: e.lineNumber,
						Content:    e.raw,
					},
				}
				newPatterns[e.pattern] = occ
			}
			occ.count++
		}

		for pattern, occ := range newPatterns {
			ts := occ.firstSeenTime
			anomalies = append(anomalies, types.Anomaly{
				Type:        "new_error_type",
				Severity:    "low",
				Description: "New error pattern appeared that was not seen in the first 80% of the file",
				TimeRange: types.TimeRange{
					Start: ts,
					End:   ts,
				},
				Details: map[string]interface{}{
					"pattern":         pattern,
					"occurrences":     occ.count,
					"first_seen_line": occ.firstSeenLine,
				},
				EvidenceLines: []types.EvidenceLine{occ.evidence},
			})
		}
	}

	// Sort anomalies: high first, then medium, then low. Within same severity, by time.
	severityRank := map[string]int{"high": 0, "medium": 1, "low": 2}
	sort.SliceStable(anomalies, func(i, j int) bool {
		ri := severityRank[anomalies[i].Severity]
		rj := severityRank[anomalies[j].Severity]
		if ri != rj {
			return ri < rj
		}
		return anomalies[i].TimeRange.Start < anomalies[j].TimeRange.Start
	})

	// Ensure non-nil slice for JSON marshaling.
	if anomalies == nil {
		anomalies = []types.Anomaly{}
	}

	// Compute time span.
	var timeSpanHours float64
	if firstTime != nil && lastTime != nil {
		timeSpanHours = lastTime.Sub(*firstTime).Hours()
	}

	return DetectAnomaliesOutput{
		Anomalies: anomalies,
		AnalysisMetadata: AnalysisMetadata{
			TotalLinesAnalyzed: totalLines,
			TimeSpanHours:      timeSpanHours,
			WindowMinutes:      input.WindowMinutes,
			Sensitivity:        input.Sensitivity,
			WindowsAnalyzed:    numWindows,
		},
	}, nil
}

// parseAnomalyTimestamp tries RFC3339 then bare ISO8601 without timezone.
func parseAnomalyTimestamp(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognized timestamp: %q", s)
}
