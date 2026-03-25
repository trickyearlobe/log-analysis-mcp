package parsers

import (
	"regexp"
	"strings"

	"github.com/trickyearlobe/log-analysis-mcp/internal/types"
)

// Compiled regex patterns for multiline detection. These are RE2-compatible
// and compiled once at package init time.
var (
	javaStackRe    = regexp.MustCompile(`^\tat `)
	causedByRe     = regexp.MustCompile(`^Caused by:`)
	pythonTBRe     = regexp.MustCompile(`^Traceback \(most recent call last\):`)
	dotnetStackRe  = regexp.MustCompile(`^   at `)
	continuationRe = regexp.MustCompile(`^\s+`)
)

// isContinuation returns true if the line looks like a stack trace or
// continuation of a previous log entry.
func isContinuation(line string) bool {
	return javaStackRe.MatchString(line) ||
		causedByRe.MatchString(line) ||
		pythonTBRe.MatchString(line) ||
		dotnetStackRe.MatchString(line) ||
		continuationRe.MatchString(line)
}

// isPythonTracebackHeader returns true if the line is the start of a Python traceback.
func isPythonTracebackHeader(line string) bool {
	return pythonTBRe.MatchString(line)
}

// MultilineCombiner aggregates continuation lines (such as stack traces) with
// their originating log entry. It wraps an underlying Parser to detect entry
// boundaries.
type MultilineCombiner struct {
	parser Parser
}

// NewMultilineCombiner creates a combiner that uses the given parser to detect
// the start of new log entries.
func NewMultilineCombiner(parser Parser) *MultilineCombiner {
	return &MultilineCombiner{parser: parser}
}

// CombinedEntry holds a parsed log entry that may span multiple raw lines.
type CombinedEntry struct {
	Entry     *types.ParsedLogEntry
	RawLines  []string
	StartLine int
}

// Combine processes a slice of lines and returns parsed entries with multiline
// continuation lines (stack traces, wrapped lines) merged into the entry that
// precedes them. Each returned entry has LineCount set to the number of raw
// lines it spans and StackTrace populated with any continuation content.
func (mc *MultilineCombiner) Combine(lines []string, startLineNum int) []*types.ParsedLogEntry {
	if len(lines) == 0 {
		return nil
	}

	var results []*types.ParsedLogEntry
	var current *types.ParsedLogEntry
	var stackLines []string
	currentLineCount := 0

	finalize := func() {
		if current == nil {
			return
		}
		current.LineCount = currentLineCount
		if len(stackLines) > 0 {
			current.StackTrace = strings.Join(stackLines, "\n")
		}
		results = append(results, current)
		current = nil
		stackLines = nil
		currentLineCount = 0
	}

	for i, line := range lines {
		lineNum := startLineNum + i

		// Try to parse this line as a new log entry.
		parsed := mc.parser.Parse(line)

		if parsed != nil {
			// This line starts a new log entry — finalize the previous one.
			finalize()
			parsed.LineNumber = lineNum
			current = parsed
			currentLineCount = 1
			continue
		}

		// Line did not parse as a new entry. Check if it's a continuation.
		if current != nil {
			if isContinuation(line) || isPythonTracebackHeader(line) {
				// Append to the current entry's stack trace.
				stackLines = append(stackLines, line)
				currentLineCount++
			} else {
				// Non-parseable, non-continuation line following a parsed entry.
				// Treat it as a wrapped line belonging to the current entry.
				stackLines = append(stackLines, line)
				currentLineCount++
			}
			continue
		}

		// No current entry and line doesn't parse — create a raw entry.
		entry := &types.ParsedLogEntry{
			LineNumber: lineNum,
			LineCount:  1,
			Message:    line,
			Raw:        line,
		}
		results = append(results, entry)
	}

	// Finalize any remaining entry.
	finalize()

	return results
}
