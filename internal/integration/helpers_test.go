package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trickyearlobe/log-analysis-mcp/internal/prompts"
	"github.com/trickyearlobe/log-analysis-mcp/internal/resources"
	"github.com/trickyearlobe/log-analysis-mcp/internal/tools"
)

// setupTestServer creates a wired MCP server with all tools, resources, and
// prompts registered, connects it to an in-memory client, and returns the
// client session. The server and client are torn down when the test finishes.
func setupTestServer(t *testing.T) *mcp.ClientSession {
	t.Helper()

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "log-analysis-mcp",
		Version: "test",
	}, nil)

	tools.Register(srv)
	resources.Register(srv)
	prompts.Register(srv)

	ctx := context.Background()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	// Connect server side first so it's ready for initialization.
	_, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "test",
	}, nil)

	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	t.Cleanup(func() {
		session.Close()
	})

	return session
}

// writeLogFile writes lines to a temp file and returns its path. The file is
// cleaned up when the test finishes via t.TempDir().
func writeLogFile(t *testing.T, dir, name string, lines []string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write log file %s: %v", path, err)
	}
	return path
}

// writeBinaryFile writes a file containing a null byte for binary detection tests.
func writeBinaryFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := []byte("some text\x00more text\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write binary file %s: %v", path, err)
	}
	return path
}

// callTool calls a tool via the MCP client session, asserts no protocol-level
// error, and unmarshals the JSON text content into the target type T. If the
// tool returned IsError:true, the test fails with the error content.
func callTool[T any](t *testing.T, session *mcp.ClientSession, toolName string, args any) T {
	t.Helper()
	ctx := context.Background()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", toolName, err)
	}
	if result.IsError {
		var texts []string
		for _, c := range result.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				texts = append(texts, tc.Text)
			}
		}
		t.Fatalf("CallTool(%s) IsError=true: %s", toolName, strings.Join(texts, "; "))
	}

	// The SDK packs structured output as JSON in the first TextContent element.
	if len(result.Content) == 0 {
		t.Fatalf("CallTool(%s): empty content", toolName)
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s): first content is not TextContent, got %T", toolName, result.Content[0])
	}

	var out T
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("CallTool(%s): unmarshal result: %v\nraw: %s", toolName, err, tc.Text)
	}
	return out
}

// callToolExpectError calls a tool and asserts that it returns IsError:true.
// Returns the error text content for further assertions.
func callToolExpectError(t *testing.T, session *mcp.ClientSession, toolName string, args any) string {
	t.Helper()
	ctx := context.Background()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): protocol error: %v", toolName, err)
	}
	if !result.IsError {
		t.Fatalf("CallTool(%s): expected IsError=true, got false", toolName)
	}
	var texts []string
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			texts = append(texts, tc.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// jsonLogLines returns a slice of JSON-formatted log lines for testing.
func jsonLogLines() []string {
	return []string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","msg":"server started","source":"app"}`,
		`{"timestamp":"2025-01-15T10:00:01Z","level":"DEBUG","msg":"loading config","source":"config"}`,
		`{"timestamp":"2025-01-15T10:00:02Z","level":"INFO","msg":"listening on :8080","source":"app"}`,
		`{"timestamp":"2025-01-15T10:00:03Z","level":"WARN","msg":"deprecated API called","source":"api"}`,
		`{"timestamp":"2025-01-15T10:00:04Z","level":"ERROR","msg":"connection refused to database","source":"db"}`,
		`{"timestamp":"2025-01-15T10:00:05Z","level":"INFO","msg":"retrying connection","source":"db"}`,
		`{"timestamp":"2025-01-15T10:00:06Z","level":"INFO","msg":"connected to database","source":"db"}`,
		`{"timestamp":"2025-01-15T10:00:07Z","level":"ERROR","msg":"connection refused to cache","source":"cache"}`,
		`{"timestamp":"2025-01-15T10:00:08Z","level":"INFO","msg":"request completed","source":"api"}`,
		`{"timestamp":"2025-01-15T10:00:09Z","level":"INFO","msg":"shutting down","source":"app"}`,
	}
}

// syslogLines returns a slice of RFC 3164 syslog lines for testing.
func syslogLines() []string {
	return []string{
		`<134>Jan 15 10:00:00 myhost myapp[1234]: server started successfully`,
		`<135>Jan 15 10:00:01 myhost myapp[1234]: loading configuration from /etc/myapp.conf`,
		`<134>Jan 15 10:00:02 myhost myapp[1234]: listening on port 8080`,
		`<132>Jan 15 10:00:03 myhost myapp[1234]: deprecated endpoint /api/v1/old invoked`,
		`<131>Jan 15 10:00:04 myhost myapp[1234]: connection refused to 10.0.0.5:5432`,
		`<134>Jan 15 10:00:05 myhost myapp[1234]: retrying database connection`,
		`<134>Jan 15 10:00:06 myhost myapp[1234]: connected to 10.0.0.5:5432`,
		`<131>Jan 15 10:00:07 myhost myapp[1234]: connection refused to 10.0.0.6:6379`,
		`<134>Jan 15 10:00:08 myhost myapp[1234]: request GET /api/health completed`,
		`<134>Jan 15 10:00:09 myhost myapp[1234]: shutting down gracefully`,
	}
}

// apacheCombinedLines returns a slice of Apache Combined Log Format lines.
func apacheCombinedLines() []string {
	return []string{
		`192.168.1.1 - frank [15/Jan/2025:10:00:00 -0700] "GET /index.html HTTP/1.1" 200 2326 "https://example.com" "Mozilla/5.0"`,
		`192.168.1.2 - - [15/Jan/2025:10:00:01 -0700] "GET /api/users HTTP/1.1" 200 1024 "-" "curl/7.68"`,
		`192.168.1.3 - admin [15/Jan/2025:10:00:02 -0700] "POST /api/data HTTP/1.1" 201 512 "https://example.com" "Mozilla/5.0"`,
		`192.168.1.1 - frank [15/Jan/2025:10:00:03 -0700] "GET /missing HTTP/1.1" 404 128 "-" "Mozilla/5.0"`,
		`192.168.1.4 - - [15/Jan/2025:10:00:04 -0700] "GET /error HTTP/1.1" 500 64 "-" "curl/7.68"`,
	}
}

// errorSpikeLines returns JSON log lines with a cluster of errors in the
// middle, useful for anomaly detection and error extraction tests.
func errorSpikeLines() []string {
	var lines []string
	// Normal traffic
	for i := 0; i < 20; i++ {
		lines = append(lines, `{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","msg":"request completed","source":"api"}`)
	}
	// Error spike
	for i := 0; i < 15; i++ {
		lines = append(lines, `{"timestamp":"2025-01-15T10:05:00Z","level":"ERROR","msg":"connection timeout to 10.0.0.5:5432","source":"db"}`)
	}
	for i := 0; i < 5; i++ {
		lines = append(lines, `{"timestamp":"2025-01-15T10:05:01Z","level":"ERROR","msg":"connection refused to 10.0.0.6:6379","source":"cache"}`)
	}
	// Recovery
	for i := 0; i < 20; i++ {
		lines = append(lines, `{"timestamp":"2025-01-15T10:10:00Z","level":"INFO","msg":"request completed","source":"api"}`)
	}
	return lines
}

// correlationFileA returns JSON log lines with request_id fields for correlation tests.
func correlationFileA() []string {
	return []string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","msg":"request received","source":"gateway","request_id":"req-001"}`,
		`{"timestamp":"2025-01-15T10:00:01Z","level":"INFO","msg":"request received","source":"gateway","request_id":"req-002"}`,
		`{"timestamp":"2025-01-15T10:00:02Z","level":"INFO","msg":"request received","source":"gateway","request_id":"req-003"}`,
		`{"timestamp":"2025-01-15T10:00:10Z","level":"INFO","msg":"response sent","source":"gateway","request_id":"req-001"}`,
	}
}

// correlationFileB returns JSON log lines from a different service sharing request_ids.
func correlationFileB() []string {
	return []string{
		`{"timestamp":"2025-01-15T10:00:00Z","level":"INFO","msg":"processing started","source":"worker","request_id":"req-001"}`,
		`{"timestamp":"2025-01-15T10:00:05Z","level":"ERROR","msg":"processing failed","source":"worker","request_id":"req-001"}`,
		`{"timestamp":"2025-01-15T10:00:01Z","level":"INFO","msg":"processing started","source":"worker","request_id":"req-002"}`,
		`{"timestamp":"2025-01-15T10:00:03Z","level":"INFO","msg":"processing complete","source":"worker","request_id":"req-002"}`,
	}
}
