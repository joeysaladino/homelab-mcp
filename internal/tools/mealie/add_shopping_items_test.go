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

func TestAddShoppingItemsToolOverMCP(t *testing.T) {
	writer := &fakeRecipeSearcher{
		shoppingItemsResult: mealieapi.ShoppingListItemsCollection{
			CreatedItems: []mealieapi.ShoppingListItem{
				{ID: "item-one", Display: "Ground cumin — check pantry", Quantity: 0, Position: 0},
				{ID: "item-two", Display: "2 cups rice", Note: "Rice for dinner", Quantity: 0, Position: 1},
			},
		},
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(mealietools.NewModule(writer))

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
		Name: "mealie.add_shopping_items",
		Arguments: map[string]any{
			"shopping_list_id": " list-id ",
			"items": []any{
				map[string]any{"display": " Ground cumin — check pantry "},
				map[string]any{"display": " 2 cups rice ", "note": " Rice for dinner "},
			},
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", result.Content)
	}

	var output mealietools.AddShoppingItemsOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	if !output.Added || output.ShoppingListID != "list-id" || len(output.Items) != 2 {
		t.Fatalf("output = %+v, want two added items", output)
	}
	if output.Items[0].Display != "Ground cumin — check pantry" || output.Items[0].Quantity != 0 || output.Items[1].Display != "2 cups rice" {
		t.Errorf("items = %+v, want normalized display and zero quantities", output.Items)
	}
	if !writer.shoppingItemsCreatedCalled || len(writer.shoppingItemsParams) != 2 {
		t.Fatalf("write params = %+v, want two item params", writer.shoppingItemsParams)
	}
	if writer.shoppingItemsParams[0].ShoppingListID != "list-id" || writer.shoppingItemsParams[0].Display != "Ground cumin — check pantry" || writer.shoppingItemsParams[0].Note != "" || writer.shoppingItemsParams[0].Position != 0 {
		t.Errorf("first write param = %+v, want normalized list/display/position", writer.shoppingItemsParams[0])
	}
	if writer.shoppingItemsParams[1].Note != "Rice for dinner" || writer.shoppingItemsParams[1].Position != 1 {
		t.Errorf("second write param = %+v, want normalized note/position", writer.shoppingItemsParams[1])
	}
}

func TestAddShoppingItemsToolValidation(t *testing.T) {
	writer := &fakeRecipeSearcher{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(mealietools.NewModule(writer))

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

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing list ID", args: map[string]any{"shopping_list_id": "", "items": []any{map[string]any{"display": "Rice"}}}, want: "shopping_list_id is required"},
		{name: "empty items", args: map[string]any{"shopping_list_id": "list-id", "items": []any{}}, want: "at least one item is required"},
		{name: "missing display", args: map[string]any{"shopping_list_id": "list-id", "items": []any{map[string]any{"display": "   "}}}, want: "item 0 display is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "mealie.add_shopping_items",
				Arguments: tt.args,
			})
			if err != nil {
				t.Fatalf("CallTool() error = %v", err)
			}
			if !result.IsError {
				t.Fatal("CallTool() IsError = false, want true")
			}
			if len(result.Content) == 0 || !strings.Contains(contentText(t, result.Content[0]), tt.want) {
				t.Fatalf("error content = %+v, want substring %q", result.Content, tt.want)
			}
		})
	}
	if writer.shoppingItemsCreatedCalled {
		t.Fatal("writer should not be called for invalid input")
	}
}
