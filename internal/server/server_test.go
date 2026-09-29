package server

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNewRegistersModulesInOrder(t *testing.T) {
	var got []string
	first := moduleFunc(func(*mcp.Server) { got = append(got, "first") })
	second := moduleFunc(func(*mcp.Server) { got = append(got, "second") })

	if server := New(first, second); server == nil {
		t.Fatal("New() returned nil server")
	}

	if len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("registered modules = %v, want [first second]", got)
	}
}

type moduleFunc func(*mcp.Server)

func (f moduleFunc) RegisterTools(server *mcp.Server) {
	f(server)
}
