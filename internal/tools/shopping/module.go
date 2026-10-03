// Package shopping contains MCP tools for household grocery workflows.
package shopping

import (
	shoppingdomain "github.com/joeysaladino/homelab-mcp/internal/shopping"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Module registers shopping-workflow tools independently from service-specific
// Mealie API tools.
type Module struct {
	builder shoppingdomain.DraftBuilder
}

// NewModule constructs the shopping-workflow module.
func NewModule(builder shoppingdomain.DraftBuilder) *Module {
	if builder == nil {
		panic("create shopping module: nil draft builder")
	}
	return &Module{builder: builder}
}

// RegisterTools implements server.Module.
func (m *Module) RegisterTools(server *mcp.Server) {
	if m == nil {
		panic("register shopping module: nil module")
	}
	if server == nil {
		panic("register shopping module: nil server")
	}
	RegisterPrepareShoppingDraft(server, m.builder)
}
