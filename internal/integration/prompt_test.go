package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestListPrompts(t *testing.T) {
	session := setupTestServer(t)
	ctx := context.Background()

	result, err := session.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}

	if len(result.Prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(result.Prompts))
	}

	names := make(map[string]bool)
	for _, p := range result.Prompts {
		names[p.Name] = true
	}

	if !names["investigate_error"] {
		t.Error("expected prompt 'investigate_error' to be present")
	}
	if !names["log_health_check"] {
		t.Error("expected prompt 'log_health_check' to be present")
	}
}

func TestGetPromptInvestigateErrorWithPattern(t *testing.T) {
	session := setupTestServer(t)
	ctx := context.Background()

	result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "investigate_error",
		Arguments: map[string]string{
			"log_path":      "/var/log/app.log",
			"error_pattern": "NullPointer",
		},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}

	msg := result.Messages[0]
	if msg.Role != "user" {
		t.Errorf("expected role 'user', got %q", msg.Role)
	}

	tc, ok := msg.Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", msg.Content)
	}
	text := tc.Text

	for _, want := range []string{
		"/var/log/app.log",
		"NullPointer",
		"summarize_logs",
		"extract_errors",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("expected text to contain %q", want)
		}
	}
}

func TestGetPromptInvestigateErrorWithoutPattern(t *testing.T) {
	session := setupTestServer(t)
	ctx := context.Background()

	result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "investigate_error",
		Arguments: map[string]string{
			"log_path": "/var/log/app.log",
		},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}

	tc, ok := result.Messages[0].Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", result.Messages[0].Content)
	}
	text := tc.Text

	if !strings.Contains(text, "/var/log/app.log") {
		t.Error("expected text to contain '/var/log/app.log'")
	}
	if strings.Contains(text, "Specifically, look for errors matching") {
		t.Error("expected text NOT to contain 'Specifically, look for errors matching' when no pattern given")
	}
	if !strings.Contains(text, "summarize_logs") {
		t.Error("expected text to contain 'summarize_logs'")
	}
	if !strings.Contains(text, "extract_errors") {
		t.Error("expected text to contain 'extract_errors'")
	}
}

func TestGetPromptLogHealthCheck(t *testing.T) {
	session := setupTestServer(t)
	ctx := context.Background()

	result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "log_health_check",
		Arguments: map[string]string{
			"log_path": "/var/log/system.log",
		},
	})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}

	if len(result.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result.Messages))
	}

	msg := result.Messages[0]
	if msg.Role != "user" {
		t.Errorf("expected role 'user', got %q", msg.Role)
	}

	tc, ok := msg.Content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", msg.Content)
	}
	text := tc.Text

	for _, want := range []string{
		"/var/log/system.log",
		"summarize_logs",
		"detect_anomalies",
		"tail_logs",
		"Health Report",
		"🟢",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("expected text to contain %q", want)
		}
	}
}
