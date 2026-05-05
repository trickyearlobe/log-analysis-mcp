package fileutil

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

// MaxRecordLines is the maximum number of raw lines in a single record.
const MaxRecordLines = 500

// MaxRecordBytes is the maximum byte size of a single record's text.
const MaxRecordBytes = 65536

// Record holds one logical log record that may span multiple raw lines.
type Record struct {
	StartLine int    `json:"start_line"`
	LineCount int    `json:"line_count"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated,omitempty"`
}

// RecordScanner streams records from a file using a separator regex.
// It implements the standard Scan/Record/Err pattern.
type RecordScanner struct {
	reader    *bufio.Reader
	closer    io.Closer
	separator *regexp.Regexp

	// State
	current    Record
	err        error
	closed     bool
	seenFirst  bool // true once we've seen the first separator match
	lineNum    int  // current 1-based line number
	pending    *string // buffered line that triggered boundary (already read, not yet consumed)
	pendingNum int     // line number of the pending line
	skipping   bool    // true when discarding lines after truncation
}

// NewRecordScanner creates a RecordScanner that reads records from path,
// splitting on lines matching separator. Returns an error if the file
// cannot be opened.
func NewRecordScanner(path string, separator *regexp.Regexp) (*RecordScanner, error) {
	rc, _, err := OpenReader(path)
	if err != nil {
		return nil, err
	}

	return &RecordScanner{
		reader:    bufio.NewReaderSize(rc, maxScannerBuf),
		closer:    rc,
		separator: separator,
	}, nil
}

// Scan advances to the next record. Returns false when no more records
// are available or an error occurred.
func (rs *RecordScanner) Scan() bool {
	if rs.closed || rs.err != nil {
		return false
	}

	for {
		rec, ok := rs.nextRecord()
		if !ok {
			return false
		}
		if rec.LineCount > 0 {
			rs.current = rec
			return true
		}
	}
}

// Record returns the most recent record produced by Scan.
func (rs *RecordScanner) Record() Record {
	return rs.current
}

// Err returns the first non-EOF error encountered during scanning.
func (rs *RecordScanner) Err() error {
	return rs.err
}

// Close releases the underlying file handle. Safe to call multiple times.
func (rs *RecordScanner) Close() error {
	if rs.closed {
		return nil
	}
	rs.closed = true
	return rs.closer.Close()
}

// readLine reads one line, stripping the trailing \n (and \r\n).
// Returns the line text, whether a line was read, and any error.
func (rs *RecordScanner) readLine() (string, bool, error) {
	line, err := rs.reader.ReadString('\n')
	if len(line) > 0 {
		// Strip trailing newline characters.
		line = strings.TrimRight(line, "\r\n")
		rs.lineNum++
		return line, true, err
	}
	if err != nil {
		return "", false, err
	}
	return "", false, nil
}

// nextRecord produces the next record. Returns (record, true) on success,
// or (zero, false) when done.
func (rs *RecordScanner) nextRecord() (Record, bool) {
	// If we have a pending line from a previous boundary detection, consume it.
	var startLine int
	var lines []string
	var byteCount int

	if rs.pending != nil {
		startLine = rs.pendingNum
		lines = append(lines, *rs.pending)
		byteCount = len(*rs.pending)
		rs.pending = nil
		rs.skipping = false
	}

	for {
		line, ok, err := rs.readLine()
		if !ok {
			if err != nil && err != io.EOF {
				rs.err = err
				// Still emit what we have if anything.
				if len(lines) > 0 {
					return makeRecord(startLine, lines, false), true
				}
				return Record{}, false
			}
			// EOF — emit accumulated record if any.
			if len(lines) > 0 {
				return makeRecord(startLine, lines, false), true
			}
			return Record{}, false
		}

		isMatch := rs.separator.MatchString(line)

		// If we're in skip mode (after truncation), discard until next separator.
		if rs.skipping {
			if isMatch {
				// Found next boundary — start fresh.
				rs.skipping = false
				startLine = rs.lineNum
				lines = []string{line}
				byteCount = len(line)
				rs.seenFirst = true
				continue
			}
			// Still skipping. But if we haven't seen first match yet,
			// emit individual lines.
			if !rs.seenFirst {
				return Record{StartLine: rs.lineNum, LineCount: 1, Text: line}, true
			}
			continue
		}

		// Before first separator match: emit each line individually.
		if !rs.seenFirst {
			if isMatch {
				rs.seenFirst = true
				startLine = rs.lineNum
				lines = []string{line}
				byteCount = len(line)
				continue
			}
			// Not a match and haven't seen first — individual record.
			return Record{StartLine: rs.lineNum, LineCount: 1, Text: line}, true
		}

		// After first match: accumulate or split on boundary.
		if isMatch && len(lines) > 0 {
			// This line starts a new record — finalize current, buffer this line.
			l := line
			rs.pending = &l
			rs.pendingNum = rs.lineNum
			return makeRecord(startLine, lines, false), true
		}

		if len(lines) == 0 {
			startLine = rs.lineNum
		}
		lines = append(lines, line)
		byteCount += len(line)

		// Check safety bounds.
		if len(lines) >= MaxRecordLines || byteCount >= MaxRecordBytes {
			rs.skipping = true
			return makeRecord(startLine, lines, true), true
		}
	}
}

func makeRecord(startLine int, lines []string, truncated bool) Record {
	return Record{
		StartLine: startLine,
		LineCount: len(lines),
		Text:      strings.Join(lines, "\n"),
		Truncated: truncated,
	}
}
