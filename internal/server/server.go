// Package server assembles the application's MCP server and its integrations.
package server

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	Name    = "homelab-mcp"
	Version = "0.1.0"
)

// Module registers one integration's MCP tools with the application server.
// Vikunja and future integrations can implement this without changing server
// assembly.
type Module interface {
	RegisterTools(*mcp.Server)
}

// New creates the MCP server and registers the supplied integration modules in
// order.
func New(modules ...Module) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    Name,
		Version: Version,
	}, nil)
	for _, module := range modules {
		if module == nil {
			panic("create MCP server: nil module")
		}
		module.RegisterTools(server)
	}
	return server
}
