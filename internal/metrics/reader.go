package metrics

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// QueryInput defines the parameters for querying metrics.
type QueryInput struct {
	Since   string `json:"since"`
	GroupBy string `json:"group_by"`
	Tool    string `json:"tool"`
	TopK    int    `json:"top_k"`
}

// QueryOutput is the aggregated metrics result.
type QueryOutput struct {
	TotalEvents int          `json:"total_events"`
	Window      WindowInfo   `json:"window"`
	Groups      []GroupStats `json:"groups"`
}

// WindowInfo describes the time range covered.
type WindowInfo struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// GroupStats holds aggregated stats for one group key.
type GroupStats struct {
	Key              string         `json:"key"`
	Calls            int64          `json:"calls"`
	Errors           int64          `json:"errors"`
	ErrorRate        float64        `json:"error_rate"`
	DurationP50Ms    int64          `json:"duration_p50_ms"`
	DurationP95Ms    int64          `json:"duration_p95_ms"`
	AvgResponseBytes int64          `json:"avg_response_bytes"`
	Warnings         map[string]int `json:"warnings,omitempty"`
}

// Query reads event log files from dir and returns aggregated stats.
func Query(dir string, input QueryInput) (QueryOutput, error) {
	if input.TopK <= 0 {
		input.TopK = 10
	}
	if input.GroupBy == "" {
		input.GroupBy = "tool"
	}

	since, err := parseDuration(input.Since)
	if err != nil {
		return QueryOutput{}, fmt.Errorf("metrics: invalid since %q: %w", input.Since, err)
	}

	cutoff := time.Now().Add(-since)
	now := time.Now()

	events, err := readEvents(dir, cutoff)
	if err != nil {
		return QueryOutput{}, err
	}

	// Filter by tool if specified
	if input.Tool != "" {
		filtered := events[:0]
		for _, e := range events {
			if e.Tool == input.Tool {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	groups := aggregate(events, input.GroupBy)

	// Sort by call count descending, take top_k
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Calls > groups[j].Calls
	})
	if len(groups) > input.TopK {
		groups = groups[:input.TopK]
	}

	return QueryOutput{
		TotalEvents: len(events),
		Window: WindowInfo{
			From: cutoff.UTC().Format(time.RFC3339),
			To:   now.UTC().Format(time.RFC3339),
		},
		Groups: groups,
	}, nil
}

func readEvents(dir string, cutoff time.Time) ([]Event, error) {
	files, err := filepath.Glob(filepath.Join(dir, "events-*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("metrics: glob: %w", err)
	}

	var events []Event
	for _, path := range files {
		fileEvents, err := readFile(path, cutoff)
		if err != nil {
			continue // skip corrupt files
		}
		events = append(events, fileEvents...)
	}
	return events, nil
}

func readFile(path string, cutoff time.Time) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	for scanner.Scan() {
		e, err := UnmarshalEvent(scanner.Bytes())
		if err != nil {
			continue
		}
		if e.Timestamp.Before(cutoff) {
			continue
		}
		events = append(events, e)
	}
	return events, nil
}

func aggregate(events []Event, groupBy string) []GroupStats {
	type accumulator struct {
		calls     int64
		errors    int64
		durations []int64
		respTotal int64
		warnings  map[string]int
	}

	groups := make(map[string]*accumulator)

	for _, e := range events {
		key := groupKey(e, groupBy)
		acc, ok := groups[key]
		if !ok {
			acc = &accumulator{warnings: make(map[string]int)}
			groups[key] = acc
		}
		acc.calls++
		if e.Status == StatusError {
			acc.errors++
		}
		acc.durations = append(acc.durations, e.DurationMs)
		acc.respTotal += int64(e.ResponseBytes)
		if e.Warning != "" {
			acc.warnings[e.Warning]++
		}
	}

	result := make([]GroupStats, 0, len(groups))
	for key, acc := range groups {
		var errRate float64
		if acc.calls > 0 {
			errRate = float64(acc.errors) / float64(acc.calls)
		}
		var avgResp int64
		if acc.calls > 0 {
			avgResp = acc.respTotal / acc.calls
		}

		var warnings map[string]int
		if len(acc.warnings) > 0 {
			warnings = acc.warnings
		}

		result = append(result, GroupStats{
			Key:              key,
			Calls:            acc.calls,
			Errors:           acc.errors,
			ErrorRate:        math.Round(errRate*1000) / 1000,
			DurationP50Ms:    percentile(acc.durations, 50),
			DurationP95Ms:    percentile(acc.durations, 95),
			AvgResponseBytes: avgResp,
			Warnings:         warnings,
		})
	}
	return result
}

func groupKey(e Event, groupBy string) string {
	switch groupBy {
	case "status":
		return e.Status
	case "warning":
		if e.Warning == "" {
			return "<none>"
		}
		return e.Warning
	default:
		return e.Tool
	}
}

func percentile(values []int64, pct int) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]int64, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	idx := int(math.Ceil(float64(pct)/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		s = "24h"
	}
	s = strings.TrimSpace(s)

	// Support "Nd" for days
	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		var days int
		if _, err := fmt.Sscanf(numStr, "%d", &days); err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	return time.ParseDuration(s)
}
