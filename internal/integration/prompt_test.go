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

    if len(result.Prompts) != 4 {
        t.Fatalf("expected 4 prompts, got %d", len(result.Prompts))
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
    if !names["investigate_remote"] {
        t.Error("expected prompt 'investigate_remote' to be present")
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
        "log_summarize",
        "log_extract_errors",
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
    if !strings.Contains(text, "log_summarize") {
        t.Error("expected text to contain 'log_summarize'")
    }
    if !strings.Contains(text, "log_extract_errors") {
        t.Error("expected text to contain 'log_extract_errors'")
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
        "log_summarize",
        "log_detect_anomalies",
        "log_tail",
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
        "log_summarize",
        "log_extract_errors",
        "log_detect_anomalies",
        "log_diff",
        "log_timeline",
        "log_search",
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
    if strings.Contains(text, "log_diff") {
        t.Error("expected text NOT to contain 'log_diff' when no comparison_path given")
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
        "log_summarize",
        "log_extract_errors",
        "log_detect_anomalies",
        "log_timeline",
        "log_search",
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

func TestGetPromptInvestigateRemoteFullArgs(t *testing.T) {
    session := setupTestServer(t)
    ctx := context.Background()

    result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
        Name: "investigate_remote",
        Arguments: map[string]string{
            "hosts":       "root@web1.example.com,root@web2.example.com",
            "incident_id": "INC-2025-100",
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

    // All remote tools should be referenced.
    for _, want := range []string{
        "root@web1.example.com",
        "root@web2.example.com",
        "INC-2025-100",
        "log_discover_remote",
        "log_gather_remote",
        "log_run_remote_command",
        "log_summarize",
        "log_extract_errors",
        "log_detect_anomalies",
        "log_correlate",
        "log_diff",
        "log_search",
        "Incident Report",
        "Executive Summary",
        "Cross-Host Correlation",
        "Cross-Host Comparison",
    } {
        if !strings.Contains(text, want) {
            t.Errorf("expected text to contain %q", want)
        }
    }

    // Multi-host: should NOT contain "skip cross-host"
    if strings.Contains(text, "skip cross-host correlation") {
        t.Error("multi-host prompt should NOT say 'skip cross-host correlation'")
    }
    if strings.Contains(text, "skip cross-host comparison") {
        t.Error("multi-host prompt should NOT say 'skip cross-host comparison'")
    }
}

func TestGetPromptInvestigateRemoteMinimalArgs(t *testing.T) {
    session := setupTestServer(t)
    ctx := context.Background()

    result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
        Name: "investigate_remote",
        Arguments: map[string]string{
            "hosts": "root@db.example.com",
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

    // Single host: should contain discovery step (no log_paths given).
    if !strings.Contains(text, "log_discover_remote") {
        t.Error("expected text to contain 'log_discover_remote' when no log_paths given")
    }

    // Single host: cross-host steps should say "skip".
    if !strings.Contains(text, "skip cross-host correlation") {
        t.Error("single-host prompt should say 'skip cross-host correlation'")
    }
    if !strings.Contains(text, "skip cross-host comparison") {
        t.Error("single-host prompt should say 'skip cross-host comparison'")
    }

    // No incident ID: should not contain a specific ID.
    if strings.Contains(text, "INC-") {
        t.Error("expected text NOT to contain an incident ID when none given")
    }

    // Core tools should still be referenced.
    for _, want := range []string{
        "root@db.example.com",
        "log_gather_remote",
        "log_run_remote_command",
        "log_summarize",
        "log_extract_errors",
        "log_detect_anomalies",
        "log_search",
    } {
        if !strings.Contains(text, want) {
            t.Errorf("expected text to contain %q", want)
        }
    }
}

func TestGetPromptInvestigateRemoteWithPaths(t *testing.T) {
    session := setupTestServer(t)
    ctx := context.Background()

    result, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
        Name: "investigate_remote",
        Arguments: map[string]string{
            "hosts":     "root@app.example.com",
            "log_paths": "/var/log/app.log,/var/log/nginx/error.log",
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

    // With log_paths: should skip discovery.
    if !strings.Contains(text, "/var/log/app.log") {
        t.Error("expected text to contain '/var/log/app.log'")
    }
    if !strings.Contains(text, "/var/log/nginx/error.log") {
        t.Error("expected text to contain '/var/log/nginx/error.log'")
    }
    if !strings.Contains(text, "Skip discovery") {
        t.Error("expected text to contain 'Skip discovery' when log_paths given")
    }

    // log_discover_remote should NOT be referenced when paths are provided.
    if strings.Contains(text, "log_discover_remote") {
        t.Error("expected text NOT to contain 'log_discover_remote' when log_paths given")
    }

    // log_gather_remote should still be referenced.
    if !strings.Contains(text, "log_gather_remote") {
        t.Error("expected text to contain 'log_gather_remote'")
    }
}

func TestGetPromptInvestigateRemoteMissingHosts(t *testing.T) {
    session := setupTestServer(t)
    ctx := context.Background()

    _, err := session.GetPrompt(ctx, &mcp.GetPromptParams{
        Name:      "investigate_remote",
        Arguments: map[string]string{},
    })
    if err == nil {
        t.Fatal("expected error for missing hosts, got nil")
    }
    if !strings.Contains(err.Error(), "hosts") {
        t.Errorf("expected error to mention 'hosts', got: %v", err)
    }
}
