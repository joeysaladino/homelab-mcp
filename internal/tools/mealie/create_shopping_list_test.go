package mealie_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/joeysaladino/homelab-mcp/internal/server"
	mealietools "github.com/joeysaladino/homelab-mcp/internal/tools/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCreateShoppingListToolOverMCP(t *testing.T) {
	creator := &fakeRecipeSearcher{
		shoppingListCreated: mealieapi.ShoppingList{
			ID:        "list-id",
			Name:      "Mon-Tue",
			CreatedAt: "2026-10-03T12:00:00Z",
		},
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(mealietools.NewModule(creator))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = mcpServer.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "mealie.create_shopping_list",
		Arguments: map[string]any{
			"name": " Mon-Tue ",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", result.Content)
	}

	var output mealietools.CreateShoppingListOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	if !output.Created || output.List.ID != "list-id" || output.List.Name != "Mon-Tue" {
		t.Fatalf("output = %+v, want created list-id/Mon-Tue", output)
	}
	if !creator.shoppingListCreatedCalled || creator.shoppingListCreate.Name != "Mon-Tue" {
		t.Errorf("create params = %+v, want trimmed list name", creator.shoppingListCreate)
	}
}

func TestCreateShoppingListToolValidation(t *testing.T) {
	creator := &fakeRecipeSearcher{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(mealietools.NewModule(creator))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = mcpServer.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "mealie.create_shopping_list",
		Arguments: map[string]any{"name": "   "},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want true")
	}
	if len(result.Content) == 0 || !strings.Contains(contentText(t, result.Content[0]), "name is required") {
		t.Fatalf("error content = %+v, want required name message", result.Content)
	}
	if creator.shoppingListCreatedCalled {
		t.Fatal("creator should not be called for invalid input")
	}
}
