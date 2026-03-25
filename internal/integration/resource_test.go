package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestListResourceTemplates(t *testing.T) {
	session := setupTestServer(t)
	ctx := context.Background()

	result, err := session.ListResourceTemplates(ctx, nil)
	if err != nil {
		t.Fatalf("ListResourceTemplates: %v", err)
	}

	if len(result.ResourceTemplates) != 1 {
		t.Fatalf("expected 1 resource template, got %d", len(result.ResourceTemplates))
	}

	tmpl := result.ResourceTemplates[0]
	if tmpl.URITemplate != "log:///{+path}" {
		t.Errorf("expected URITemplate %q, got %q", "log:///{+path}", tmpl.URITemplate)
	}
	if tmpl.Name != "log-file" {
		t.Errorf("expected Name %q, got %q", "log-file", tmpl.Name)
	}
}

func TestReadResourceValidFile(t *testing.T) {
	session := setupTestServer(t)
	ctx := context.Background()

	dir := t.TempDir()
	lines := jsonLogLines()
	path := writeLogFile(t, dir, "app.log", lines)

	// Absolute paths start with /, so "log:///" + "/abs/path" = "log:////abs/path".
	// TrimPrefix("log:///") in the handler recovers the absolute path "/abs/path".
	uri := "log:///" + path
	result, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}

	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 contents entry, got %d", len(result.Contents))
	}

	contents := result.Contents[0]
	if contents.Text == "" {
		t.Fatal("expected non-empty Text in contents")
	}
	if !strings.Contains(contents.Text, lines[0]) {
		t.Errorf("expected contents to contain first log line %q", lines[0])
	}
	if contents.MIMEType != "text/plain" {
		t.Errorf("expected MIMEType %q, got %q", "text/plain", contents.MIMEType)
	}
}

func TestReadResourceMissingFile(t *testing.T) {
	session := setupTestServer(t)
	ctx := context.Background()

	_, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "log:///nonexistent/path/to/file.log"})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
