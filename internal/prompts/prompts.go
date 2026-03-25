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
