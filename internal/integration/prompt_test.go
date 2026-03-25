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

	if len(result.Prompts) != 3 {
		t.Fatalf("expected 3 prompts, got %d", len(result.Prompts))
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
	if !names["generate_report"] {
		t.Error("expected prompt 'generate_report' to be present")
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

func TestGetPromptGenerateReportFullArgs(t *testing.T) {
	session := setupTestServer(t)
	ctx := context.Background()

	result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "generate_report",
		Arguments: map[string]string{
			"log_path":        "/var/log/app.log",
			"comparison_path": "/var/log/app.log.1",
			"incident_id":     "INC-2025-042",
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
		"/var/log/app.log.1",
		"INC-2025-042",
		"summarize_logs",
		"extract_errors",
		"detect_anomalies",
		"diff_logs",
		"timeline",
		"search_logs",
		"Incident Report",
		"Executive Summary",
		"Root Cause Analysis",
		"Comparison with Baseline",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("expected text to contain %q", want)
		}
	}
}

func TestGetPromptGenerateReportMinimalArgs(t *testing.T) {
	session := setupTestServer(t)
	ctx := context.Background()

	result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name: "generate_report",
		Arguments: map[string]string{
			"log_path": "/var/log/nginx/error.log",
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

	if !strings.Contains(text, "/var/log/nginx/error.log") {
		t.Error("expected text to contain '/var/log/nginx/error.log'")
	}

	// Without comparison_path, diff step should say to skip.
	if strings.Contains(text, "diff_logs") {
		t.Error("expected text NOT to contain 'diff_logs' when no comparison_path given")
	}
	if !strings.Contains(text, "skip the comparison step") {
		t.Error("expected text to contain 'skip the comparison step' when no comparison_path given")
	}

	// Without incident_id, report header should not have a specific ID.
	if strings.Contains(text, "INC-") {
		t.Error("expected text NOT to contain an incident ID when none given")
	}

	// Comparison with Baseline section should not appear.
	if strings.Contains(text, "Comparison with Baseline") {
		t.Error("expected text NOT to contain 'Comparison with Baseline' when no comparison_path given")
	}

	// Core tools should still be referenced.
	for _, want := range []string{
		"summarize_logs",
		"extract_errors",
		"detect_anomalies",
		"timeline",
		"search_logs",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("expected text to contain %q", want)
		}
	}
}

func TestGetPromptGenerateReportMissingLogPath(t *testing.T) {
	session := setupTestServer(t)
	ctx := context.Background()

	_, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      "generate_report",
		Arguments: map[string]string{},
	})
	if err == nil {
		t.Fatal("expected error for missing log_path, got nil")
	}
	if !strings.Contains(err.Error(), "log_path") {
		t.Errorf("expected error to mention 'log_path', got: %v", err)
	}
}
