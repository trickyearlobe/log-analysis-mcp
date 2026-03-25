// Package resources registers MCP resource definitions for log file access.
package resources

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trickyearlobe/log-analysis-mcp/internal/fileutil"
)

// maxPreviewLines is the number of lines returned when reading a log resource.
const maxPreviewLines = 100

// Register adds all resource templates to the MCP server.
func Register(srv *mcp.Server) {
	srv.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "log:///{+path}",
		Name:        "log-file",
		Description: "Log file metadata and preview",
		MIMEType:    "text/plain",
	}, handleLogFileResource)
}

// handleLogFileResource returns the first 100 lines of a log file as text content.
func handleLogFileResource(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	path := strings.TrimPrefix(uri, "log:///")
	if path == "" {
		return nil, fmt.Errorf("empty path in URI: %s", uri)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory: %s", path)
	}

	result, err := fileutil.ReadLines(path, 1, maxPreviewLines)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var sb strings.Builder
	// Metadata header.
	sb.WriteString(fmt.Sprintf("# File: %s\n", filepath.Base(path)))
	sb.WriteString(fmt.Sprintf("# Path: %s\n", path))
	sb.WriteString(fmt.Sprintf("# Size: %d bytes\n", info.Size()))
	sb.WriteString(fmt.Sprintf("# Modified: %s\n", info.ModTime().UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("# Showing: first %d lines\n", len(result.Lines)))
	if result.HasMore {
		sb.WriteString("# Note: file has more lines beyond this preview\n")
	}
	sb.WriteString("#\n")

	for _, lr := range result.Lines {
		sb.WriteString(lr.Text)
		sb.WriteString("\n")
	}

	text := sb.String()
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			Text:     text,
			MIMEType: "text/plain",
		}},
	}, nil
}
