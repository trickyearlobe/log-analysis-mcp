package tools

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
	"github.com/trickyearlobe/log-analysis-mcp/internal/parsers"
	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// summarizePageSize is the number of lines to read per streaming page.
const summarizePageSize = 1000

// summarizeTopN is the maximum number of top sources/errors to return.
const summarizeTopN = 10

// SummarizeLogsInput defines the parameters for the summarize_logs tool.
type SummarizeLogsInput struct {
	Path       string `json:"path"                 jsonschema:"Path to the log file"`
	SampleSize int    `json:"sample_size,omitempty" jsonschema:"Number of lines to sample; 0 means analyze all lines"`
}

// TimeRangeInfo contains the earliest and latest timestamps and their duration.
type TimeRangeInfo struct {
	Earliest      string  `json:"earliest"`
	Latest        string  `json:"latest"`
	DurationHours float64 `json:"duration_hours"`
}

// FileInfoSummary contains metadata about the analyzed log file.
type FileInfoSummary struct {
	Name       string         `json:"name"`
	Path       string         `json:"path"`
	SizeBytes  int64          `json:"size_bytes"`
	SizeHuman  string         `json:"size_human"`
	TotalLines int            `json:"total_lines"`
	TimeRange  *TimeRangeInfo `json:"time_range"`
}

// LevelStats holds the count and percentage for a single log level.
type LevelStats struct {
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// SourceCount pairs a source name with its occurrence count.
type SourceCount struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
}

// ErrorCount pairs an error message with its occurrence count.
type ErrorCount struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// MinuteStats holds a per-minute timestamp bucket and its line count.
type MinuteStats struct {
	Timestamp string `json:"timestamp"`
	Count     int    `json:"count"`
}

// ThroughputInfo contains lines-per-minute and peak/quietest minute stats.
type ThroughputInfo struct {
	LinesPerMinute float64     `json:"lines_per_minute"`
	PeakMinute     MinuteStats `json:"peak_minute"`
	QuietestMinute MinuteStats `json:"quietest_minute"`
}

// SummarizeLogsOutput is the structured result of the summarize_logs tool.
type SummarizeLogsOutput struct {
	FileInfo          FileInfoSummary       `json:"file_info"`
	DetectedFormat    string                `json:"detected_format"`
	LevelDistribution map[string]LevelStats `json:"level_distribution"`
	TopSources        []SourceCount         `json:"top_sources"`
	TopErrors         []ErrorCount          `json:"top_errors"`
	Throughput        ThroughputInfo        `json:"throughput"`
	Sampled           bool                  `json:"sampled"`
	LinesAnalyzed     int                   `json:"lines_analyzed"`
}

// formatSizeHuman formats a byte count into a human-readable string.
func formatSizeHuman(b int64) string {
	const (
		kb = 1024
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
	)
	switch {
	case b < kb:
		return fmt.Sprintf("%d B", b)
	case b < mb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	case b < gb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	default:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	}
}

// truncateToMinute returns the first 16 characters of a timestamp string,
// producing a "YYYY-MM-DDTHH:MM" minute bucket key. Returns "" if the
// timestamp is shorter than 16 characters.
func truncateToMinute(ts string) string {
	if len(ts) < 16 {
		return ""
	}
	return ts[:16]
}

// isSummarizeErrorLevel returns true if the level is ERROR, FATAL, or CRITICAL.
func isSummarizeErrorLevel(level *types.LogLevel) bool {
	if level == nil {
		return false
	}
	switch *level {
	case types.LogLevelError, types.LogLevelFatal, types.LogLevelCritical:
		return true
	}
	return false
}

// RunSummarizeLogs generates a statistical summary of a log file.
func RunSummarizeLogs(input SummarizeLogsInput) (SummarizeLogsOutput, error) {
	// Validate file access.
	if err := CheckFileAccess(input.Path); err != nil {
		return SummarizeLogsOutput{}, fmt.Errorf("summarize_logs: %w", err)
	}

	// Get file size.
	sizeBytes, err := FileSize(input.Path)
	if err != nil {
		return SummarizeLogsOutput{}, fmt.Errorf("summarize_logs: %w", err)
	}

	// Sample lines for format detection.
	sampleLines, err := SampleLines(input.Path, sampleLineCount)
	if err != nil {
		return SummarizeLogsOutput{}, fmt.Errorf("summarize_logs: %w", err)
	}

	// Detect format and obtain parser.
	detection, parser := parsers.AutoDetectWithHint(sampleLines, "")

	// Accumulators for single-pass stats.
	levelCounts := make(map[string]int)
	sourceCounts := make(map[string]int)
	errorMsgCounts := make(map[string]int)
	minuteBuckets := make(map[string]int)

	var earliest, latest string
	totalLines := 0
	sampled := false

	// Stream entire file in pages.
	startLine := 1
	for {
		// If sampling, only read what we need.
		pageSize := summarizePageSize
		if input.SampleSize > 0 {
			remaining := input.SampleSize - totalLines
			if remaining <= 0 {
				sampled = true
				break
			}
			if remaining < pageSize {
				pageSize = remaining
			}
		}

		result, err := fileutil.ReadLines(input.Path, startLine, pageSize)
		if err != nil {
			return SummarizeLogsOutput{}, fmt.Errorf("summarize_logs: read at line %d: %w", startLine, err)
		}

		if len(result.Lines) == 0 {
			break
		}

		for _, lr := range result.Lines {
			totalLines++

			if parser == nil {
				continue
			}

			entry := parser.Parse(lr.Text)
			if entry == nil {
				continue
			}

			// Level distribution.
			if entry.Level != nil {
				levelCounts[string(*entry.Level)]++
			}

			// Source counts.
			if entry.Source != nil && *entry.Source != "" {
				sourceCounts[*entry.Source]++
			}

			// Error message counts.
			if isSummarizeErrorLevel(entry.Level) {
				errorMsgCounts[entry.Message]++
			}

			// Timestamp tracking.
			if entry.Timestamp != nil && *entry.Timestamp != "" {
				ts := *entry.Timestamp
				if earliest == "" || ts < earliest {
					earliest = ts
				}
				if latest == "" || ts > latest {
					latest = ts
				}

				// Per-minute bucket.
				minuteKey := truncateToMinute(ts)
				if minuteKey != "" {
					minuteBuckets[minuteKey]++
				}
			}
		}

		if input.SampleSize > 0 && totalLines >= input.SampleSize {
			sampled = true
			break
		}

		if !result.HasMore {
			break
		}
		startLine += len(result.Lines)
	}

	// Build level distribution with percentages.
	levelDist := make(map[string]LevelStats, len(levelCounts))
	for level, count := range levelCounts {
		pct := 0.0
		if totalLines > 0 {
			pct = math.Round(float64(count)/float64(totalLines)*1000) / 10
		}
		levelDist[level] = LevelStats{
			Count:      count,
			Percentage: pct,
		}
	}

	// Build top sources (sorted by count desc, then name asc).
	topSources := buildTopSources(sourceCounts)

	// Build top errors (sorted by count desc, then message asc).
	topErrors := buildTopErrors(errorMsgCounts)

	// Compute time range.
	var timeRange *TimeRangeInfo
	if earliest != "" && latest != "" {
		tr := TimeRangeInfo{
			Earliest: earliest,
			Latest:   latest,
		}
		t1, err1 := parseTimestamp(earliest)
		t2, err2 := parseTimestamp(latest)
		if err1 == nil && err2 == nil {
			dur := t2.Sub(t1)
			tr.DurationHours = math.Round(dur.Hours()*100) / 100
		}
		timeRange = &tr
	}

	// Compute throughput.
	throughput := computeThroughput(minuteBuckets, totalLines, timeRange)

	return SummarizeLogsOutput{
		FileInfo: FileInfoSummary{
			Name:       filepath.Base(input.Path),
			Path:       input.Path,
			SizeBytes:  sizeBytes,
			SizeHuman:  formatSizeHuman(sizeBytes),
			TotalLines: totalLines,
			TimeRange:  timeRange,
		},
		DetectedFormat:    string(detection.Format),
		LevelDistribution: levelDist,
		TopSources:        topSources,
		TopErrors:         topErrors,
		Throughput:        throughput,
		Sampled:           sampled,
		LinesAnalyzed:     totalLines,
	}, nil
}

// buildTopSources sorts sources by count descending and returns the top N.
func buildTopSources(counts map[string]int) []SourceCount {
	result := make([]SourceCount, 0, len(counts))
	for src, cnt := range counts {
		result = append(result, SourceCount{Source: src, Count: cnt})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Source < result[j].Source
	})
	if len(result) > summarizeTopN {
		result = result[:summarizeTopN]
	}
	return result
}

// buildTopErrors sorts error messages by count descending and returns the top N.
func buildTopErrors(counts map[string]int) []ErrorCount {
	result := make([]ErrorCount, 0, len(counts))
	for msg, cnt := range counts {
		result = append(result, ErrorCount{Message: msg, Count: cnt})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Message < result[j].Message
	})
	if len(result) > summarizeTopN {
		result = result[:summarizeTopN]
	}
	return result
}

// computeThroughput calculates lines-per-minute and finds peak/quietest minutes.
func computeThroughput(minuteBuckets map[string]int, totalLines int, timeRange *TimeRangeInfo) ThroughputInfo {
	info := ThroughputInfo{}

	if timeRange != nil && timeRange.DurationHours > 0 {
		durationMinutes := timeRange.DurationHours * 60
		info.LinesPerMinute = math.Round(float64(totalLines)/durationMinutes*100) / 100
	}

	if len(minuteBuckets) == 0 {
		return info
	}

	// Find peak and quietest (non-zero) minutes.
	peakCount := 0
	quietCount := math.MaxInt64
	peakTS := ""
	quietTS := ""

	// Sort keys for deterministic tie-breaking (earliest timestamp wins).
	keys := make([]string, 0, len(minuteBuckets))
	for k := range minuteBuckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, ts := range keys {
		cnt := minuteBuckets[ts]
		if cnt > peakCount {
			peakCount = cnt
			peakTS = ts
		}
		if cnt < quietCount {
			quietCount = cnt
			quietTS = ts
		}
	}

	info.PeakMinute = MinuteStats{Timestamp: peakTS, Count: peakCount}
	if quietTS != "" {
		info.QuietestMinute = MinuteStats{Timestamp: quietTS, Count: quietCount}
	}

	return info
}
