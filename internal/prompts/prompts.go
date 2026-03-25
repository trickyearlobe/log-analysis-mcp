// Package prompts registers MCP prompt definitions for guided log analysis workflows.
package prompts

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register adds all prompt definitions to the MCP server.
func Register(srv *mcp.Server) {
	srv.AddPrompt(&mcp.Prompt{
		Name:        "investigate_error",
		Description: "Guided error investigation workflow for a log file",
		Arguments: []*mcp.PromptArgument{
			{Name: "log_path", Description: "Path to the log file to investigate", Required: true},
			{Name: "error_pattern", Description: "Specific error pattern to look for", Required: false},
		},
	}, handleInvestigateError)

	srv.AddPrompt(&mcp.Prompt{
		Name:        "log_health_check",
		Description: "System health assessment from log analysis",
		Arguments: []*mcp.PromptArgument{
			{Name: "log_path", Description: "Path to the log file to assess", Required: true},
		},
	}, handleLogHealthCheck)

	srv.AddPrompt(&mcp.Prompt{
		Name:        "generate_report",
		Description: "Generate a structured incident report from log analysis",
		Arguments: []*mcp.PromptArgument{
			{Name: "log_path", Description: "Path to the primary log file to investigate", Required: true},
			{Name: "comparison_path", Description: "Path to a baseline log file for before/after comparison", Required: false},
			{Name: "incident_id", Description: "Incident or ticket ID to include in the report header", Required: false},
		},
	}, handleGenerateReport)
}

// handleInvestigateError returns a structured error investigation prompt.
func handleInvestigateError(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	logPath := req.Params.Arguments["log_path"]
	if logPath == "" {
		return nil, fmt.Errorf("missing required argument: log_path")
	}

	errorPattern := req.Params.Arguments["error_pattern"]

	var errorPatternSentence string
	var matchingClause string
	if errorPattern != "" {
		errorPatternSentence = fmt.Sprintf(` Specifically, look for errors matching: "%s".`, errorPattern)
		matchingClause = fmt.Sprintf(` matching "%s"`, errorPattern)
	}

	text := fmt.Sprintf(`I need you to investigate errors in the log file at "%s".%s

Please follow this structured investigation process:

1. **Overview**: First, use the summarize_logs tool to get an overview of the log file. Report the total error count, error rate, and time range.

2. **Error Extraction**: Use the extract_errors tool to identify and cluster all unique error types. List the top 10 error clusters by frequency.

3. **Timeline Analysis**: Use the timeline tool to build a timeline of error events%s. Identify when errors started, whether they correlate with other events (deployments, restarts), and if there are patterns in timing.

4. **Context Gathering**: For the most significant error(s), use search_logs with context_lines=5 to see what happened immediately before and after each occurrence.

5. **Anomaly Detection**: Use detect_anomalies to check for error spikes or unusual patterns that might indicate a trigger event.

6. **Root Cause Analysis**: Based on all the evidence gathered, provide:
   - A summary of findings
   - The most likely root cause
   - The sequence of events that led to the errors
   - Recommended next steps for remediation

Format your analysis as a structured report with clear headings.`, logPath, errorPatternSentence, matchingClause)

	return &mcp.GetPromptResult{
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: text}},
		},
	}, nil
}

// handleLogHealthCheck returns a structured health check prompt.
func handleLogHealthCheck(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	logPath := req.Params.Arguments["log_path"]
	if logPath == "" {
		return nil, fmt.Errorf("missing required argument: log_path")
	}

	text := fmt.Sprintf(`Please perform a health check on the system by analyzing the log file at "%s".

Use the following tools to conduct a comprehensive assessment:

1. **File Overview**: Use summarize_logs to get a high-level picture:
   - How large is the file and what time period does it cover?
   - What is the overall log volume and throughput?

2. **Error Assessment**:
   - What percentage of log entries are errors or warnings?
   - Use extract_errors to identify the most common error types.
   - Is the error rate acceptable (< 1%% is healthy, 1-5%% is concerning, > 5%% is critical)?

3. **Anomaly Scan**: Use detect_anomalies to check for:
   - Error spikes that might indicate instability
   - Gaps in logging that might indicate outages
   - Rate changes that might indicate load issues

4. **Recent Activity**: Use tail_logs to check the most recent entries:
   - Is the system currently logging normally?
   - Are there any active errors or warnings?

5. **Health Report**: Provide a structured health report with:
   - **Overall Status**: 🟢 Healthy / 🟡 Warning / 🔴 Critical
   - **Error Rate**: Current error rate with assessment
   - **Top Issues**: List of the most important issues found
   - **Anomalies**: Any detected anomalies
   - **Recommendations**: Prioritized list of recommended actions
   - **Summary**: A one-paragraph executive summary

Be specific with numbers and evidence from the logs.`, logPath)

	return &mcp.GetPromptResult{
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: text}},
		},
	}, nil
}

// handleGenerateReport returns a structured incident report generation prompt.
func handleGenerateReport(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	logPath := req.Params.Arguments["log_path"]
	if logPath == "" {
		return nil, fmt.Errorf("missing required argument: log_path")
	}

	comparisonPath := req.Params.Arguments["comparison_path"]
	incidentID := req.Params.Arguments["incident_id"]

	var incidentSentence string
	if incidentID != "" {
		incidentSentence = fmt.Sprintf("\nInclude the incident ID %q in the report header.", incidentID)
	}

	var comparisonStep string
	if comparisonPath != "" {
		comparisonStep = fmt.Sprintf(`5. **Comparison Analysis**: Use the diff_logs tool to compare %q (target) against %q (baseline). Report new errors, resolved errors, rate changes, source changes, and throughput shifts.`, logPath, comparisonPath)
	} else {
		comparisonStep = "5. **Comparison Analysis**: No baseline file was provided, so skip the comparison step."
	}

	var reportHeader string
	if incidentID != "" {
		reportHeader = fmt.Sprintf("**Incident Report: %s** (header)", incidentID)
	} else {
		reportHeader = "**Incident Report** (header)"
	}

	var comparisonSection string
	if comparisonPath != "" {
		comparisonSection = "\n   - **Comparison with Baseline** (new errors, resolved errors, rate changes)"
	}

	text := fmt.Sprintf(`Generate a comprehensive incident report by analyzing the log file at %q.%s

Follow this structured investigation process, using the tools listed for each step:

1. **Executive Summary** (after completing all steps below):
   Summarize the incident in 2-3 sentences: what happened, when, impact, and current status.

2. **System Overview**: Use summarize_logs to establish baseline metrics:
   - File size, time range, and total line count
   - Log volume and throughput (lines/minute)
   - Detected log format

3. **Error Analysis**: Use extract_errors to identify and cluster all error types:
   - List the top 10 error clusters by frequency
   - Note error rate (errors/hour and percentage of all lines)
   - Identify any error patterns that suggest a root cause

4. **Anomaly Detection**: Use detect_anomalies to find unusual patterns:
   - Error spikes (sudden increases in error rate)
   - Gaps in logging (possible outages or restarts)
   - Rate changes (load shifts)
   - New error types not seen before

%s

6. **Timeline**: Use the timeline tool to build a chronological sequence of significant events:
   - When did the incident start?
   - What were the key events leading up to the incident?
   - When was it resolved (if applicable)?

7. **Deep Dive**: For the top 3 most significant errors, use search_logs with context_lines=5 to examine surrounding context. Look for:
   - What triggered each error
   - Whether errors cascade (one causing another)
   - Any recovery attempts visible in the logs

8. **Report**: Compile all findings into a structured Markdown report with these sections:
   - %s
   - **Executive Summary**
   - **Timeline of Events** (chronological table)
   - **Error Analysis** (clusters, rates, patterns)
   - **Anomalies Detected**%s
   - **Root Cause Analysis** (your assessment based on evidence)
   - **Impact Assessment** (what was affected, duration, severity)
   - **Recommendations** (prioritized remediation steps)
   - **Appendix: Raw Data** (key metrics, tool outputs referenced)

Be specific with numbers, timestamps, and evidence from the logs. Every claim in the report should be backed by data from one of the tools.`,
		logPath, incidentSentence, comparisonStep, reportHeader, comparisonSection)

	return &mcp.GetPromptResult{
		Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: text}},
		},
	}, nil
}
