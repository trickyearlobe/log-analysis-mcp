package fileutil

import (
	"fmt"
	"io"
	"os"
)

// tailChunkSize is the size of each backward-read chunk (8 KB).
const tailChunkSize = 8192

// TailResult is the output of a tail read operation.
type TailResult struct {
	Lines      []LineRecord `json:"lines"`
	TotalLines int          `json:"total_lines"`
	FileSize   int64        `json:"file_size"`
}

// TailLines reads the last numLines lines from a file by seeking backward
// from the end in fixed-size chunks. Performance is O(N) in the number of
// requested lines, not in total file size. numLines defaults to 100 if <= 0.
func TailLines(path string, numLines int) (TailResult, error) {
	if numLines < 1 {
		numLines = 100
	}

	f, err := os.Open(path)
	if err != nil {
		return TailResult{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return TailResult{}, fmt.Errorf("stat %s: %w", path, err)
	}

	fileSize := info.Size()
	if fileSize == 0 {
		return TailResult{
			Lines:      []LineRecord{},
			TotalLines: 0,
			FileSize:   0,
		}, nil
	}

	// Read backward in chunks, accumulating data until we have enough newlines.
	var accumulated []byte
	newlineCount := 0
	offset := fileSize
	reachedStart := false

	for offset > 0 && newlineCount <= numLines {
		chunkSize := int64(tailChunkSize)
		if chunkSize > offset {
			chunkSize = offset
		}
		offset -= chunkSize

		buf := make([]byte, chunkSize)
		n, err := f.ReadAt(buf, offset)
		if err != nil && err != io.EOF {
			return TailResult{}, fmt.Errorf("read %s at offset %d: %w", path, offset, err)
		}
		buf = buf[:n]

		// Count newlines in this chunk.
		for _, b := range buf {
			if b == '\n' {
				newlineCount++
			}
		}

		// Prepend chunk to accumulated data.
		accumulated = append(buf, accumulated...)
	}

	if offset == 0 {
		reachedStart = true
	}

	// Split accumulated data into lines.
	lines := splitLines(accumulated)

	// Determine total line count. If we read from the start of the file
	// (offset reached 0), we know the exact total. Otherwise, estimate
	// based on the average line length in the data we did read.
	var totalLines int
	if reachedStart {
		totalLines = len(lines)
	} else {
		// Best-effort estimate: extrapolate from bytes read vs file size.
		bytesRead := int64(len(accumulated))
		if bytesRead > 0 && len(lines) > 0 {
			avgLineLen := float64(bytesRead) / float64(len(lines))
			totalLines = int(float64(fileSize) / avgLineLen)
		} else {
			totalLines = len(lines)
		}
	}

	// Take the last numLines entries.
	if len(lines) > numLines {
		lines = lines[len(lines)-numLines:]
	}

	// Build result with correct 1-based line numbers.
	records := make([]LineRecord, len(lines))
	startLineNum := totalLines - len(lines) + 1
	for i, text := range lines {
		records[i] = LineRecord{
			LineNumber: startLineNum + i,
			Text:       text,
		}
	}

	return TailResult{
		Lines:      records,
		TotalLines: totalLines,
		FileSize:   fileSize,
	}, nil
}

// splitLines splits raw bytes by '\n', stripping trailing '\r' from each line.
// A trailing newline does not produce an extra empty line at the end.
// An empty input produces an empty slice.
func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}

	// Remove a single trailing newline to avoid a phantom empty line.
	if data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	// After stripping, if data is empty the file was just a single newline.
	if len(data) == 0 {
		return []string{""}
	}

	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			line := string(data[start:i])
			line = stripCR(line)
			lines = append(lines, line)
			start = i + 1
		}
	}
	// Last segment (no trailing newline after it since we stripped it).
	line := string(data[start:])
	line = stripCR(line)
	lines = append(lines, line)

	return lines
}

// stripCR removes a trailing '\r' from a string.
func stripCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}
