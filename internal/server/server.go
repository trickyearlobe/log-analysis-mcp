// Package server provides the MCP server configuration and lifecycle management.
package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trickyearlobe/log-analysis-mcp/internal/prompts"
	"github.com/trickyearlobe/log-analysis-mcp/internal/resources"
	"github.com/trickyearlobe/log-analysis-mcp/internal/tools"
)

// Server wraps the MCP SDK server with log-analysis-mcp configuration.
type Server struct {
	srv *mcp.Server
}

// New creates a configured MCP server with all tools, resources, and prompts registered.
func New(version string) *Server {
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "log-analysis-mcp",
		Version: version,
	}, nil)

	tools.Register(srv)
	resources.Register(srv)
	prompts.Register(srv)

	return &Server{srv: srv}
}

// Run starts the MCP server on the stdio transport, blocking until the context
// is cancelled or the transport closes.
func (s *Server) Run(ctx context.Context) error {
	return s.srv.Run(ctx, &mcp.StdioTransport{})
}
