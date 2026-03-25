package tools

import (
	"fmt"
	"time"

	"github.com/trickyearlobe/log-analysis-mcp/internal/remote"
)

// RunRemoteCommandInput defines the parameters for the run_remote_command tool.
type RunRemoteCommandInput struct {
	Hosts          []string `json:"hosts"                      jsonschema:"SSH targets in [user@]host[:port] format"`
	Command        string   `json:"command"                    jsonschema:"Shell command to execute on each host"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"  jsonschema:"Max seconds per host (default 30)"`
	MaxOutputBytes int      `json:"max_output_bytes,omitempty" jsonschema:"Max bytes of stdout/stderr per host (default 1MB)"`
}

// HostCommandResult holds the output of a command executed on a single host.
type HostCommandResult struct {
	Host     string `json:"host"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// RunRemoteCommandOutput is the structured result of the run_remote_command tool.
type RunRemoteCommandOutput struct {
	Results []HostCommandResult `json:"results"`
}

// RunRunRemoteCommand executes a shell command on one or more remote hosts over SSH.
func RunRunRemoteCommand(input RunRemoteCommandInput) (RunRemoteCommandOutput, error) {
	if len(input.Hosts) == 0 {
		return RunRemoteCommandOutput{}, fmt.Errorf("run_remote_command: hosts must not be empty")
	}
	if input.Command == "" {
		return RunRemoteCommandOutput{}, fmt.Errorf("run_remote_command: command must not be empty")
	}

	input.TimeoutSeconds = DefaultInt(input.TimeoutSeconds, 30)
	input.MaxOutputBytes = DefaultInt(input.MaxOutputBytes, 1048576)

	timeout := time.Duration(input.TimeoutSeconds) * time.Second
	pool := remote.DefaultPool()

	results := make([]HostCommandResult, len(input.Hosts))
	for i, host := range input.Hosts {
		results[i].Host = host

		target, err := remote.ParseTarget(host)
		if err != nil {
			results[i].Error = fmt.Sprintf("parse target: %v", err)
			continue
		}

		client, err := pool.Get(target)
		if err != nil {
			results[i].Error = fmt.Sprintf("connect: %v", err)
			continue
		}

		execResult, err := remote.Exec(client, input.Command, timeout, input.MaxOutputBytes)
		if err != nil {
			results[i].Error = fmt.Sprintf("exec: %v", err)
			continue
		}

		results[i].Stdout = execResult.Stdout
		results[i].Stderr = execResult.Stderr
		results[i].ExitCode = execResult.ExitCode
	}

	return RunRemoteCommandOutput{Results: results}, nil
}
