package fileutil

import (
	"fmt"
	"io"
	"os"
)

// binaryCheckSize is the number of bytes to read when checking for binary content.
const binaryCheckSize = 8192

// CheckBinary reads the first 8192 bytes of a file and returns an error
// if any null byte (0x00) is found, indicating the file is likely binary.
// Returns nil if the file appears to be text.
func CheckBinary(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	buf := make([]byte, binaryCheckSize)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return fmt.Errorf("read %s: %w", path, err)
	}

	for i := 0; i < n; i++ {
		if buf[i] == 0x00 {
			return fmt.Errorf("binary file detected: %s — this tool only supports text log files", path)
		}
	}

	return nil
}
