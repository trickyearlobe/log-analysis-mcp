package tools

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRegisterDoesNotPanic(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "log-analysis-mcp",
		Version: "test",
	}, nil)
	Register(srv)
}

func TestAllToolsOmitOutputSchema(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "log-analysis-mcp",
		Version: "test",
	}, nil)
	Register(srv)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "1.0",
	}, nil)

	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	result, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(result.Tools) != 15 {
		t.Fatalf("expected 15 tools, got %d", len(result.Tools))
	}

	for _, tool := range result.Tools {
		t.Run(tool.Name, func(t *testing.T) {
			if tool.OutputSchema != nil {
				t.Errorf("tool %q has non-nil OutputSchema (breaks Copilot external agent)", tool.Name)
			}
			if tool.InputSchema == nil {
				t.Errorf("tool %q has nil InputSchema", tool.Name)
			}
		})
	}
}
