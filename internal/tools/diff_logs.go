package tools

import (
	"fmt"
	"math"
	"sort"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
	"github.com/trickyearlobe/log-analysis-mcp/internal/parsers"
	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

const diffLogsPageSize = 1000
const maxChangedEntries = 50

// DiffLogsInput defines the parameters for comparing two log files or time periods.
type DiffLogsInput struct {
	BasePath     string `json:"base_path"                jsonschema:"Path to the baseline (before) log file"`
	TargetPath   string `json:"target_path,omitempty"     jsonschema:"Path to the target (after) log file; omit for single-file time range mode"`
	BaseAfter    string `json:"base_after,omitempty"      jsonschema:"ISO 8601 timestamp — start of baseline period"`
	BaseBefore   string `json:"base_before,omitempty"     jsonschema:"ISO 8601 timestamp — end of baseline period"`
	TargetAfter  string `json:"target_after,omitempty"    jsonschema:"ISO 8601 timestamp — start of target period"`
	TargetBefore string `json:"target_before,omitempty"   jsonschema:"ISO 8601 timestamp — end of target period"`
}

// ErrorDiff represents a single error pattern difference between base and target.
type ErrorDiff struct {
	Pattern     string `json:"pattern"`
	BaseCount   int    `json:"base_count"`
	TargetCount int    `json:"target_count"`
	Change      int    `json:"change"`
}

// LevelDiff represents a level distribution difference between base and target.
type LevelDiff struct {
	Level            string  `json:"level"`
	BaseCount        int     `json:"base_count"`
	BasePercentage   float64 `json:"base_percentage"`
	TargetCount      int     `json:"target_count"`
	TargetPercentage float64 `json:"target_percentage"`
}

// SourceDiff represents a source count difference between base and target.
type SourceDiff struct {
	Source      string `json:"source"`
	BaseCount   int    `json:"base_count"`
	TargetCount int    `json:"target_count"`
	Change      int    `json:"change"`
}

// PeriodSummary contains summary statistics for one side of the diff.
type PeriodSummary struct {
	Path           string  `json:"path"`
	TotalLines     int     `json:"total_lines"`
	ParsedLines    int     `json:"parsed_lines"`
	ErrorCount     int     `json:"error_count"`
	Earliest       string  `json:"earliest,omitempty"`
	Latest         string  `json:"latest,omitempty"`
	LinesPerMinute float64 `json:"lines_per_minute"`
}

// DiffLogsOutput contains the structured comparison results.
type DiffLogsOutput struct {
	BaseSummary        PeriodSummary `json:"base_summary"`
	TargetSummary      PeriodSummary `json:"target_summary"`
	NewErrors          []ErrorDiff   `json:"new_errors"`
	ResolvedErrors     []ErrorDiff   `json:"resolved_errors"`
	ChangedErrors      []ErrorDiff   `json:"changed_errors"`
	LevelChanges       []LevelDiff   `json:"level_changes"`
	NewSources         []SourceDiff  `json:"new_sources"`
	DisappearedSources []SourceDiff  `json:"disappeared_sources"`
	ChangedSources     []SourceDiff  `json:"changed_sources"`
}

// diffAccumulator collects statistics for one side of the comparison.
type diffAccumulator struct {
	path         string
	totalLines   int
	parsedLines  int
	errorCount   int
	earliest     string
	latest       string
	levelCounts  map[string]int
	sourceCounts map[string]int
	errorPatterns map[string]int // normalized pattern → count
}

func newDiffAccumulator(path string) *diffAccumulator {
	return &diffAccumulator{
		path:          path,
		levelCounts:   make(map[string]int),
		sourceCounts:  make(map[string]int),
		errorPatterns: make(map[string]int),
	}
}

func (a *diffAccumulator) addEntry(entry *types.ParsedLogEntry) {
	a.parsedLines++

	if entry.Level != nil {
		a.levelCounts[string(*entry.Level)]++
	}

	if entry.Source != nil && *entry.Source != "" {
		a.sourceCounts[*entry.Source]++
	}

	if entry.Timestamp != nil && *entry.Timestamp != "" {
		ts := *entry.Timestamp
		if a.earliest == "" || ts < a.earliest {
			a.earliest = ts
		}
		if a.latest == "" || ts > a.latest {
			a.latest = ts
		}
	}

	if isErrorLevel(entry.Level) {
		a.errorCount++
		normalized := normalizeMessage(entry.Message)
		a.errorPatterns[normalized]++
	}
}

func (a *diffAccumulator) summary() PeriodSummary {
	s := PeriodSummary{
		Path:        a.path,
		TotalLines:  a.totalLines,
		ParsedLines: a.parsedLines,
		ErrorCount:  a.errorCount,
		Earliest:    a.earliest,
		Latest:      a.latest,
	}

	if a.earliest != "" && a.latest != "" {
		t1, err1 := parseTimestamp(a.earliest)
		t2, err2 := parseTimestamp(a.latest)
		if err1 == nil && err2 == nil {
			dur := t2.Sub(t1)
			minutes := dur.Minutes()
			if minutes > 0 {
				s.LinesPerMinute = math.Round(float64(a.totalLines)/minutes*100) / 100
			}
		}
	}

	return s
}

// inTimeWindow checks whether a log entry's timestamp falls within the given
// after/before bounds. Empty bounds mean unbounded on that side.
func inTimeWindow(entry *types.ParsedLogEntry, after, before string) bool {
	if after == "" && before == "" {
		return true
	}
	if entry.Timestamp == nil || *entry.Timestamp == "" {
		return false
	}
	ts := *entry.Timestamp
	if after != "" && ts < after {
		return false
	}
	if before != "" && ts > before {
		return false
	}
	return true
}

// RunDiffLogs compares two log files or two time periods and returns structured differences.
func RunDiffLogs(input DiffLogsInput) (DiffLogsOutput, error) {
	if input.BasePath == "" {
		return DiffLogsOutput{}, fmt.Errorf("diff_logs: INVALID_INPUT: base_path is required")
	}

	singleFileMode := input.TargetPath == ""

	// Validate time range parameters in single-file mode.
	if singleFileMode {
		hasAnyTimeParam := input.BaseAfter != "" || input.BaseBefore != "" ||
			input.TargetAfter != "" || input.TargetBefore != ""
		hasAllTimeParams := input.BaseAfter != "" && input.BaseBefore != "" &&
			input.TargetAfter != "" && input.TargetBefore != ""

		if hasAnyTimeParam && !hasAllTimeParams {
			return DiffLogsOutput{}, fmt.Errorf("diff_logs: INVALID_INPUT: single-file mode requires all four time range parameters (base_after, base_before, target_after, target_before)")
		}
		if !hasAnyTimeParam {
			return DiffLogsOutput{}, fmt.Errorf("diff_logs: INVALID_INPUT: single-file mode requires all four time range parameters (base_after, base_before, target_after, target_before)")
		}
	}

	// Validate timestamp formats.
	for _, param := range []struct {
		name  string
		value string
	}{
		{"base_after", input.BaseAfter},
		{"base_before", input.BaseBefore},
		{"target_after", input.TargetAfter},
		{"target_before", input.TargetBefore},
	} {
		if param.value == "" {
			continue
		}
		if _, err := parseTimestamp(param.value); err != nil {
			return DiffLogsOutput{}, fmt.Errorf("diff_logs: INVALID_INPUT: invalid timestamp for %s: %s", param.name, param.value)
		}
	}

	// Validate non-overlapping time ranges in single-file mode.
	if singleFileMode && input.BaseBefore != "" && input.TargetAfter != "" {
		if input.BaseBefore > input.TargetAfter {
			return DiffLogsOutput{}, fmt.Errorf("diff_logs: INVALID_INPUT: time ranges must not overlap: base_before (%s) must be <= target_after (%s)", input.BaseBefore, input.TargetAfter)
		}
	}

	// Check file access.
	if err := CheckFileAccess(input.BasePath); err != nil {
		return DiffLogsOutput{}, fmt.Errorf("diff_logs: %w", err)
	}
	if !singleFileMode {
		if err := CheckFileAccess(input.TargetPath); err != nil {
			return DiffLogsOutput{}, fmt.Errorf("diff_logs: %w", err)
		}
	}

	// Detect format from base file.
	sampleLines, err := SampleLines(input.BasePath, sampleLineCount)
	if err != nil {
		return DiffLogsOutput{}, fmt.Errorf("diff_logs: %w", err)
	}
	_, parser := parsers.AutoDetectWithHint(sampleLines, "")

	baseAcc := newDiffAccumulator(input.BasePath)
	targetPath := input.TargetPath
	if singleFileMode {
		targetPath = input.BasePath
	}
	targetAcc := newDiffAccumulator(targetPath)

	if singleFileMode {
		// Single pass: assign entries to base or target period based on timestamp.
		if err := scanFile(input.BasePath, parser, func(entry *types.ParsedLogEntry, totalLine bool) {
			if totalLine {
				baseAcc.totalLines++
				targetAcc.totalLines++
				return
			}
			if inTimeWindow(entry, input.BaseAfter, input.BaseBefore) {
				baseAcc.addEntry(entry)
			}
			if inTimeWindow(entry, input.TargetAfter, input.TargetBefore) {
				targetAcc.addEntry(entry)
			}
		}); err != nil {
			return DiffLogsOutput{}, err
		}
	} else {
		// Two-file mode: scan each file independently.
		if err := scanFile(input.BasePath, parser, func(entry *types.ParsedLogEntry, totalLine bool) {
			if totalLine {
				baseAcc.totalLines++
				return
			}
			if inTimeWindow(entry, input.BaseAfter, input.BaseBefore) {
				baseAcc.addEntry(entry)
			}
		}); err != nil {
			return DiffLogsOutput{}, err
		}

		// Detect format for target file separately if needed.
		targetParser := parser
		if input.TargetPath != input.BasePath {
			targetSampleLines, sErr := SampleLines(input.TargetPath, sampleLineCount)
			if sErr != nil {
				return DiffLogsOutput{}, fmt.Errorf("diff_logs: %w", sErr)
			}
			_, targetParser = parsers.AutoDetectWithHint(targetSampleLines, "")
		}

		if err := scanFile(input.TargetPath, targetParser, func(entry *types.ParsedLogEntry, totalLine bool) {
			if totalLine {
				targetAcc.totalLines++
				return
			}
			if inTimeWindow(entry, input.TargetAfter, input.TargetBefore) {
				targetAcc.addEntry(entry)
			}
		}); err != nil {
			return DiffLogsOutput{}, err
		}
	}

	return buildDiffOutput(baseAcc, targetAcc), nil
}

// entryCallback is called for each line/entry during scanning.
// When entry is nil and totalLine is true, it's a line-count increment (even unparseable lines).
// When entry is non-nil, it's a successfully parsed entry.
type entryCallback func(entry *types.ParsedLogEntry, totalLine bool)

// scanFile streams a file and calls cb for each line and parsed entry.
func scanFile(path string, parser parsers.Parser, cb entryCallback) error {
	startLine := 1
	for {
		result, err := fileutil.ReadLines(path, startLine, diffLogsPageSize)
		if err != nil {
			return fmt.Errorf("diff_logs: read %s at line %d: %w", path, startLine, err)
		}

		for _, lr := range result.Lines {
			cb(nil, true)

			if parser == nil {
				continue
			}

			entry := parser.Parse(lr.Text)
			if entry == nil {
				continue
			}
			entry.LineNumber = lr.LineNumber
			entry.LineCount = 1

			cb(entry, false)
		}

		if !result.HasMore || len(result.Lines) == 0 {
			break
		}
		startLine += len(result.Lines)
	}
	return nil
}

// buildDiffOutput computes all diff sections from two accumulators.
func buildDiffOutput(base, target *diffAccumulator) DiffLogsOutput {
	out := DiffLogsOutput{
		BaseSummary:   base.summary(),
		TargetSummary: target.summary(),
	}

	// Error diffs.
	out.NewErrors, out.ResolvedErrors, out.ChangedErrors = computeErrorDiffs(base.errorPatterns, target.errorPatterns)

	// Level diffs.
	out.LevelChanges = computeLevelDiffs(base.levelCounts, target.levelCounts, base.parsedLines, target.parsedLines)

	// Source diffs.
	out.NewSources, out.DisappearedSources, out.ChangedSources = computeSourceDiffs(base.sourceCounts, target.sourceCounts)

	return out
}

func computeErrorDiffs(base, target map[string]int) (newErrors, resolved, changed []ErrorDiff) {
	newErrors = []ErrorDiff{}
	resolved = []ErrorDiff{}
	changed = []ErrorDiff{}

	// All patterns from both sides.
	allPatterns := make(map[string]bool)
	for p := range base {
		allPatterns[p] = true
	}
	for p := range target {
		allPatterns[p] = true
	}

	for p := range allPatterns {
		bc := base[p]
		tc := target[p]
		diff := ErrorDiff{
			Pattern:     p,
			BaseCount:   bc,
			TargetCount: tc,
			Change:      tc - bc,
		}

		switch {
		case bc == 0 && tc > 0:
			newErrors = append(newErrors, diff)
		case bc > 0 && tc == 0:
			resolved = append(resolved, diff)
		default:
			changed = append(changed, diff)
		}
	}

	// Sort new errors by target count descending.
	sort.Slice(newErrors, func(i, j int) bool {
		if newErrors[i].TargetCount != newErrors[j].TargetCount {
			return newErrors[i].TargetCount > newErrors[j].TargetCount
		}
		return newErrors[i].Pattern < newErrors[j].Pattern
	})

	// Sort resolved by base count descending.
	sort.Slice(resolved, func(i, j int) bool {
		if resolved[i].BaseCount != resolved[j].BaseCount {
			return resolved[i].BaseCount > resolved[j].BaseCount
		}
		return resolved[i].Pattern < resolved[j].Pattern
	})

	// Sort changed by |Change| descending.
	sort.Slice(changed, func(i, j int) bool {
		ai := changed[i].Change
		if ai < 0 {
			ai = -ai
		}
		aj := changed[j].Change
		if aj < 0 {
			aj = -aj
		}
		if ai != aj {
			return ai > aj
		}
		return changed[i].Pattern < changed[j].Pattern
	})

	if len(changed) > maxChangedEntries {
		changed = changed[:maxChangedEntries]
	}

	return newErrors, resolved, changed
}

func computeLevelDiffs(base, target map[string]int, baseParsed, targetParsed int) []LevelDiff {
	allLevels := make(map[string]bool)
	for l := range base {
		allLevels[l] = true
	}
	for l := range target {
		allLevels[l] = true
	}

	diffs := make([]LevelDiff, 0, len(allLevels))
	for l := range allLevels {
		bc := base[l]
		tc := target[l]
		var bpct, tpct float64
		if baseParsed > 0 {
			bpct = math.Round(float64(bc)/float64(baseParsed)*1000) / 10
		}
		if targetParsed > 0 {
			tpct = math.Round(float64(tc)/float64(targetParsed)*1000) / 10
		}
		diffs = append(diffs, LevelDiff{
			Level:            l,
			BaseCount:        bc,
			BasePercentage:   bpct,
			TargetCount:      tc,
			TargetPercentage: tpct,
		})
	}

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Level < diffs[j].Level
	})

	if diffs == nil {
		diffs = []LevelDiff{}
	}
	return diffs
}

func computeSourceDiffs(base, target map[string]int) (newSources, disappeared, changed []SourceDiff) {
	newSources = []SourceDiff{}
	disappeared = []SourceDiff{}
	changed = []SourceDiff{}

	allSources := make(map[string]bool)
	for s := range base {
		allSources[s] = true
	}
	for s := range target {
		allSources[s] = true
	}

	for s := range allSources {
		bc := base[s]
		tc := target[s]
		diff := SourceDiff{
			Source:      s,
			BaseCount:   bc,
			TargetCount: tc,
			Change:      tc - bc,
		}

		switch {
		case bc == 0 && tc > 0:
			newSources = append(newSources, diff)
		case bc > 0 && tc == 0:
			disappeared = append(disappeared, diff)
		default:
			changed = append(changed, diff)
		}
	}

	// Sort new sources by target count descending.
	sort.Slice(newSources, func(i, j int) bool {
		if newSources[i].TargetCount != newSources[j].TargetCount {
			return newSources[i].TargetCount > newSources[j].TargetCount
		}
		return newSources[i].Source < newSources[j].Source
	})

	// Sort disappeared by base count descending.
	sort.Slice(disappeared, func(i, j int) bool {
		if disappeared[i].BaseCount != disappeared[j].BaseCount {
			return disappeared[i].BaseCount > disappeared[j].BaseCount
		}
		return disappeared[i].Source < disappeared[j].Source
	})

	// Sort changed by |Change| descending.
	sort.Slice(changed, func(i, j int) bool {
		ai := changed[i].Change
		if ai < 0 {
			ai = -ai
		}
		aj := changed[j].Change
		if aj < 0 {
			aj = -aj
		}
		if ai != aj {
			return ai > aj
		}
		return changed[i].Source < changed[j].Source
	})

	if len(changed) > maxChangedEntries {
		changed = changed[:maxChangedEntries]
	}

	return newSources, disappeared, changed
}
