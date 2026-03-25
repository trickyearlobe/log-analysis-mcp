package tools

import (
	"fmt"
	"regexp"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// searchPageSize is the number of lines to read per streaming page.
const searchPageSize = 1000

// SearchLogsInput defines the parameters for the search_logs tool.
type SearchLogsInput struct {
	Path          string `json:"path"           jsonschema:"required,description=Path to the log file to search"`
	Pattern       string `json:"pattern"        jsonschema:"required,description=Search pattern (plain text or regex)"`
	IsRegex       bool   `json:"is_regex"       jsonschema:"description=Treat pattern as a regular expression"`
	CaseSensitive bool   `json:"case_sensitive" jsonschema:"description=Case-sensitive search"`
	ContextLines  int    `json:"context_lines"  jsonschema:"description=Lines of context before and after each match,minimum=0,maximum=10"`
	MaxResults    int    `json:"max_results"    jsonschema:"description=Maximum number of matches to return (max 500),minimum=1,maximum=500"`
}

// SearchLogsOutput is the structured result of the search_logs tool.
type SearchLogsOutput struct {
	Matches       []types.SearchMatch `json:"matches"`
	TotalMatches  int                 `json:"total_matches"`
	SearchedLines int                 `json:"searched_lines"`
	PatternUsed   string              `json:"pattern_used"`
	Truncated     bool                `json:"truncated"`
}

// RunSearchLogs searches a log file for lines matching a pattern and returns
// matches with optional surrounding context lines.
func RunSearchLogs(input SearchLogsInput) (SearchLogsOutput, error) {
	// Apply defaults and clamp.
	input.MaxResults = DefaultInt(input.MaxResults, 50)
	input.MaxResults = ClampInt(input.MaxResults, 1, 500)
	input.ContextLines = ClampInt(input.ContextLines, 0, 10)

	// Validate file access (exists, readable, not binary).
	if err := CheckFileAccess(input.Path); err != nil {
		return SearchLogsOutput{}, fmt.Errorf("search_logs: %w", err)
	}

	// Compile the search pattern into a regex.
	re, patternUsed, err := CompilePattern(input.Pattern, input.IsRegex, input.CaseSensitive)
	if err != nil {
		return SearchLogsOutput{}, fmt.Errorf("search_logs: %w", err)
	}

	matches, totalMatches, searchedLines, err := streamSearch(input.Path, re, input.ContextLines, input.MaxResults)
	if err != nil {
		return SearchLogsOutput{}, fmt.Errorf("search_logs: %w", err)
	}

	if matches == nil {
		matches = []types.SearchMatch{}
	}

	return SearchLogsOutput{
		Matches:       matches,
		TotalMatches:  totalMatches,
		SearchedLines: searchedLines,
		PatternUsed:   patternUsed,
		Truncated:     totalMatches > len(matches),
	}, nil
}

// pendingAfterCtx tracks a collected match that still needs after-context lines.
type pendingAfterCtx struct {
	idx       int // index into the matches slice
	remaining int // lines of after-context still needed
}

// streamSearch reads the file page-by-page and collects matches with context.
func streamSearch(path string, re *regexp.Regexp, contextLines, maxResults int) ([]types.SearchMatch, int, int, error) {
	var (
		matches       []types.SearchMatch
		totalMatches  int
		searchedLines int
		ringBuf       []string          // last N lines for before-context
		pending       []pendingAfterCtx // matches still collecting after-context
	)

	startLine := 1
	for {
		result, err := fileutil.ReadLines(path, startLine, searchPageSize)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("read %s at line %d: %w", path, startLine, err)
		}

		for _, lr := range result.Lines {
			searchedLines++

			// Feed this line as after-context to any pending matches.
			pending = feedPending(pending, matches, lr.Text)

			if re.MatchString(lr.Text) {
				totalMatches++

				if len(matches) < maxResults {
					before := copyTail(ringBuf, contextLines)
					m := types.SearchMatch{
						LineNumber:    lr.LineNumber,
						Line:          lr.Text,
						BeforeContext: before,
						AfterContext:  []string{},
					}
					matches = append(matches, m)

					if contextLines > 0 {
						pending = append(pending, pendingAfterCtx{
							idx:       len(matches) - 1,
							remaining: contextLines,
						})
					}
				}
			}

			// Update ring buffer with current line for future before-context.
			if contextLines > 0 {
				ringBuf = pushRingStr(ringBuf, lr.Text, contextLines)
			}
		}

		if !result.HasMore || len(result.Lines) == 0 {
			break
		}
		startLine += len(result.Lines)
	}

	return matches, totalMatches, searchedLines, nil
}

// feedPending appends line as after-context to every pending match and removes completed entries.
func feedPending(pending []pendingAfterCtx, matches []types.SearchMatch, line string) []pendingAfterCtx {
	if len(pending) == 0 {
		return pending
	}
	alive := pending[:0]
	for _, p := range pending {
		matches[p.idx].AfterContext = append(matches[p.idx].AfterContext, line)
		p.remaining--
		if p.remaining > 0 {
			alive = append(alive, p)
		}
	}
	return alive
}

// pushRingStr appends s to the ring buffer and evicts the oldest entry if capacity is exceeded.
func pushRingStr(ring []string, s string, capacity int) []string {
	ring = append(ring, s)
	if len(ring) > capacity {
		ring = ring[len(ring)-capacity:]
	}
	return ring
}

// copyTail returns a copy of the last n elements from buf, or all elements if len < n.
func copyTail(buf []string, n int) []string {
	if n == 0 || len(buf) == 0 {
		return []string{}
	}
	start := 0
	if len(buf) > n {
		start = len(buf) - n
	}
	out := make([]string, len(buf)-start)
	copy(out, buf[start:])
	return out
}
