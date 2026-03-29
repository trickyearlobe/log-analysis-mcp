// Package tools implements the 10 MCP tool handlers for log analysis.
package tools

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
)

// DecompressFileInput defines the parameters for the log_decompress tool.
type DecompressFileInput struct {
	Path string `json:"path" jsonschema:"Path to the compressed log file (.gz, .bz2, .zip)"`
}

// DecompressFileOutput is the structured result of the log_decompress tool.
type DecompressFileOutput struct {
	TempPath         string `json:"temp_path"`
	OriginalPath     string `json:"original_path"`
	CompressedSize   int64  `json:"compressed_size"`
	DecompressedSize int64  `json:"decompressed_size"`
	Note             string `json:"note"`
}

var (
	tempFilesMu sync.Mutex
	tempFiles   []string
)

// registerTempFile records a temp file path for later cleanup.
func registerTempFile(path string) {
	tempFilesMu.Lock()
	defer tempFilesMu.Unlock()
	tempFiles = append(tempFiles, path)
}

// CleanupTempFiles removes all registered temp files and directories. Called from main during shutdown.
func CleanupTempFiles() {
	tempFilesMu.Lock()
	defer tempFilesMu.Unlock()
	for _, path := range tempFiles {
		if err := os.RemoveAll(path); err != nil {
			slog.Error("failed to remove temp path", "path", path, "error", err)
		}
	}
	tempFiles = nil
}

// RunDecompressFile decompresses a compressed log file to a temporary plain-text file.
func RunDecompressFile(input DecompressFileInput) (DecompressFileOutput, error) {
	// Validate the file exists and is not a directory.
	info, err := os.Stat(input.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return DecompressFileOutput{}, fmt.Errorf("FILE_NOT_FOUND: file not found: %s — verify the path is correct and accessible", input.Path)
		}
		return DecompressFileOutput{}, fmt.Errorf("log_decompress: stat %s: %w", input.Path, err)
	}
	if info.IsDir() {
		return DecompressFileOutput{}, fmt.Errorf("INVALID_INPUT: path is a directory, not a file: %s", input.Path)
	}

	// Check that the file has a recognised compressed extension.
	if !fileutil.IsCompressed(input.Path) {
		return DecompressFileOutput{}, fmt.Errorf("INVALID_INPUT: file does not have a recognised compressed extension (.gz, .bz2, .zip). Pass the path directly to other tools — no decompression needed")
	}

	// Open compressed file and get decompressed reader.
	reader, compressedSize, err := fileutil.OpenReader(input.Path)
	if err != nil {
		return DecompressFileOutput{}, fmt.Errorf("DECOMPRESS_ERROR: decompression failed for %s: %w", input.Path, err)
	}
	defer reader.Close()

	// Create temp file with pattern based on original basename.
	basename := filepath.Base(input.Path)
	pattern := "log-analysis-" + basename + "-*"
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return DecompressFileOutput{}, fmt.Errorf("DECOMPRESS_ERROR: failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Stream decompressed content to temp file.
	_, err = io.Copy(tmpFile, reader)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return DecompressFileOutput{}, fmt.Errorf("DECOMPRESS_ERROR: failed to write decompressed data: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return DecompressFileOutput{}, fmt.Errorf("DECOMPRESS_ERROR: failed to close temp file: %w", err)
	}

	// Stat the temp file for decompressed size.
	tmpInfo, err := os.Stat(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return DecompressFileOutput{}, fmt.Errorf("DECOMPRESS_ERROR: failed to stat temp file: %w", err)
	}

	registerTempFile(tmpPath)

	return DecompressFileOutput{
		TempPath:         tmpPath,
		OriginalPath:     input.Path,
		CompressedSize:   compressedSize,
		DecompressedSize: tmpInfo.Size(),
		Note:             "Temporary file — use this path with other tools. File will be cleaned up when the server exits.",
	}, nil
}
