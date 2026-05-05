// Package tools implements the 16 MCP tool handlers for log analysis.
package tools

import (
	"fmt"
	"os"
	"regexp"

	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
)

// sampleLineCount is the number of non-empty lines to read for format detection.
const sampleLineCount = 10

// ClampInt clamps val to the range [min, max].
func ClampInt(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// DefaultInt returns val if val != 0, otherwise returns def.
func DefaultInt(val, def int) int {
	if val == 0 {
		return def
	}
	return val
}

// DefaultString returns val if val != "", otherwise returns def.
func DefaultString(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

// CheckFileAccess verifies the file exists, is readable, and is not binary.
// Returns a descriptive error with an appropriate error code context.
func CheckFileAccess(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("FILE_NOT_FOUND: file not found: %s — verify the path is correct and accessible", path)
		}
		if os.IsPermission(err) {
			return fmt.Errorf("PERMISSION_DENIED: permission denied: %s — check file permissions", path)
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("FILE_NOT_FOUND: path is a directory, not a file: %s", path)
	}
	if err := fileutil.CheckBinary(path); err != nil {
		return fmt.Errorf("BINARY_FILE: %w", err)
	}
	return nil
}

// FileSize returns the size of the file in bytes.
func FileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", path, err)
	}
	return info.Size(), nil
}

// SampleLines reads the first n non-empty lines from a file for format detection.
func SampleLines(path string, n int) ([]string, error) {
	result, err := fileutil.ReadLines(path, 1, n*2) // read extra to skip blanks
	if err != nil {
		return nil, fmt.Errorf("sample lines from %s: %w", path, err)
	}
	lines := make([]string, 0, n)
	for _, lr := range result.Lines {
		if lr.Text != "" {
			lines = append(lines, lr.Text)
			if len(lines) >= n {
				break
			}
		}
	}
	return lines, nil
}

// CompilePattern compiles a search pattern into a regexp. If isRegex is false,
// the pattern is escaped for literal matching. If caseSensitive is false, the
// pattern is wrapped with (?i). Returns the compiled regex and the pattern
// string that was used (for reporting in output).
func CompilePattern(pattern string, isRegex, caseSensitive bool) (*regexp.Regexp, string, error) {
	p := pattern
	if !isRegex {
		p = regexp.QuoteMeta(pattern)
	}
	if !caseSensitive {
		p = "(?i)" + p
	}
	re, err := regexp.Compile(p)
	if err != nil {
		return nil, "", fmt.Errorf("INVALID_REGEX: invalid regular expression: %q — %v", pattern, err)
	}
	return re, pattern, nil
}
