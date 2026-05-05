package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
	"github.com/trickyearlobe/log-analysis-mcp/internal/parsers"
)

// FileInfoInput defines the parameters for the log_file_info tool.
type FileInfoInput struct {
	Path string `json:"path" jsonschema:"Path to the log file"`
}

// FileInfoOutput is the structured result of the log_file_info tool.
type FileInfoOutput struct {
	Path            string `json:"path"`
	SizeBytes       int64  `json:"size_bytes"`
	LineCount       int    `json:"line_count"`
	FirstTimestamp  string `json:"first_timestamp"`
	LastTimestamp   string `json:"last_timestamp"`
	CompressionType string `json:"compression_type"`
	IsBinary        bool   `json:"is_binary"`
}

// RunFileInfo returns lightweight metadata about a log file.
func RunFileInfo(input FileInfoInput) (FileInfoOutput, error) {
	info, err := os.Stat(input.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileInfoOutput{}, fmt.Errorf("log_file_info: FILE_NOT_FOUND: file not found: %s", input.Path)
		}
		if os.IsPermission(err) {
			return FileInfoOutput{}, fmt.Errorf("log_file_info: PERMISSION_DENIED: %s", input.Path)
		}
		return FileInfoOutput{}, fmt.Errorf("log_file_info: stat: %w", err)
	}
	if info.IsDir() {
		return FileInfoOutput{}, fmt.Errorf("log_file_info: FILE_NOT_FOUND: path is a directory: %s", input.Path)
	}

	absPath, err := filepath.Abs(input.Path)
	if err != nil {
		absPath = input.Path
	}

	out := FileInfoOutput{
		Path:            absPath,
		SizeBytes:       info.Size(),
		CompressionType: fileutil.CompressionName(input.Path),
	}

	// Binary check
	if err := fileutil.CheckBinary(input.Path); err != nil {
		out.IsBinary = true
		return out, nil
	}

	// Stream through file for line count and timestamps
	rc, _, err := fileutil.OpenReader(input.Path)
	if err != nil {
		return FileInfoOutput{}, fmt.Errorf("log_file_info: open: %w", err)
	}
	defer rc.Close()

	// Detect parser from first sample
	sample, _ := SampleLines(input.Path, 10)
	_, parser := parsers.AutoDetectWithHint(sample, "")

	scanner := bufio.NewScanner(rc)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var lineCount int
	var firstTS, lastTS string

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		// Try to extract timestamp
		ts := extractTimestamp(parser, line)
		if ts == "" {
			continue
		}
		if firstTS == "" {
			firstTS = ts
		}
		lastTS = ts
	}

	out.LineCount = lineCount
	out.FirstTimestamp = firstTS
	out.LastTimestamp = lastTS
	return out, nil
}

// extractTimestamp tries to get a timestamp from a line using the detected parser,
// falling back to trying all parsers.
func extractTimestamp(parser parsers.Parser, line string) string {
	if parser != nil {
		entry := parser.Parse(line)
		if entry != nil && entry.Timestamp != nil && *entry.Timestamp != "" {
			return *entry.Timestamp
		}
	}
	return ""
}
