// Package fileutil provides streaming file reading utilities for log analysis.
package fileutil

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
)

// maxScannerBuf is the maximum buffer size for the line scanner (1 MB).
const maxScannerBuf = 1048576

// LineRecord holds a single line's number and text content.
type LineRecord struct {
	LineNumber int    `json:"line_number"`
	Text       string `json:"text"`
}

// ReadLinesResult is the output of a streaming line read operation.
type ReadLinesResult struct {
	Lines     []LineRecord `json:"lines"`
	HasMore   bool         `json:"has_more"`
	TotalRead int          `json:"total_read"`
}

// ReadLines reads lines from a file in a streaming fashion with pagination.
// startLine is 1-based. Both startLine and numLines must be >= 1.
// The reader never loads the entire file into memory.
func ReadLines(path string, startLine, numLines int) (ReadLinesResult, error) {
	if startLine < 1 {
		return ReadLinesResult{}, fmt.Errorf("invalid start_line %d: must be >= 1", startLine)
	}
	if numLines < 1 {
		return ReadLinesResult{}, fmt.Errorf("invalid num_lines %d: must be >= 1", numLines)
	}

	rc, _, err := OpenReader(path)
	if err != nil {
		return ReadLinesResult{}, err
	}
	defer rc.Close()

	result := ReadLinesResult{
		Lines: []LineRecord{},
	}

	// Try the scanner-based approach first.
	scanner := bufio.NewScanner(rc)
	scanner.Buffer(make([]byte, maxScannerBuf), maxScannerBuf)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum < startLine {
			continue
		}
		if len(result.Lines) >= numLines {
			// We already have enough lines; this successful scan proves more exist.
			result.HasMore = true
			result.TotalRead = len(result.Lines)
			return result, nil
		}
		result.Lines = append(result.Lines, LineRecord{
			LineNumber: lineNum,
			Text:       scanner.Text(),
		})
	}

	if err := scanner.Err(); err != nil {
		// Check for token-too-long error by examining the error message.
		if isTokenTooLong(err) {
			return readLinesWithFallback(path, startLine, numLines)
		}
		return ReadLinesResult{}, fmt.Errorf("scan %s: %w", path, err)
	}

	// Scanner finished cleanly — no more lines.
	result.TotalRead = len(result.Lines)
	return result, nil
}

// isTokenTooLong checks whether a scanner error is due to a line exceeding the buffer.
func isTokenTooLong(err error) bool {
	return err == bufio.ErrTooLong
}

// readLinesWithFallback re-reads the file using an unbounded line reader
// that handles lines of any length. It is invoked when the scanner fails
// on a line exceeding maxScannerBuf.
func readLinesWithFallback(path string, startLine, numLines int) (ReadLinesResult, error) {
	rc, _, err := OpenReader(path)
	if err != nil {
		return ReadLinesResult{}, err
	}
	defer rc.Close()

	result := ReadLinesResult{
		Lines: []LineRecord{},
	}

	reader := bufio.NewReader(rc)
	lineNum := 0

	for {
		line, err := readFullLine(reader)
		if len(line) > 0 || err == nil {
			lineNum++
			if lineNum >= startLine && len(result.Lines) < numLines {
				result.Lines = append(result.Lines, LineRecord{
					LineNumber: lineNum,
					Text:       string(line),
				})
			} else if len(result.Lines) >= numLines {
				result.HasMore = true
				result.TotalRead = len(result.Lines)
				return result, nil
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return ReadLinesResult{}, fmt.Errorf("read (fallback) %s: %w", path, err)
		}
	}

	result.TotalRead = len(result.Lines)
	return result, nil
}

// readFullLine reads bytes until '\n' or EOF, stripping trailing '\r' and '\n'.
// It returns the line content (without delimiters) and any error.
// Unlike scanner, it imposes no maximum line length.
func readFullLine(r *bufio.Reader) ([]byte, error) {
	var buf bytes.Buffer
	for {
		chunk, isPrefix, err := r.ReadLine()
		// bufio.ReadLine already strips \r\n / \n, but may return isPrefix
		// if the line exceeds the internal buffer. We keep reading until
		// isPrefix is false or we hit an error.
		buf.Write(chunk)
		if !isPrefix || err != nil {
			return buf.Bytes(), err
		}
	}
}
