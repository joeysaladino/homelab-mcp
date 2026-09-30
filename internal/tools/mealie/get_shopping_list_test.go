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

func TestGetShoppingListToolOverMCP(t *testing.T) {
	two := 2.0
	getter := &fakeRecipeSearcher{
		shoppingList: mealieapi.ShoppingList{
			ID:   "list-id",
			Name: "Mon-Tue",
			Items: []mealieapi.ShoppingListItem{
				{ID: "item-one", Display: "Ground cumin — check pantry", Note: "Ground cumin — check pantry", Position: 0},
				{
					ID:       "item-two",
					Display:  "2 cups rice",
					Quantity: two,
					Food:     &mealieapi.IngredientFood{Name: "rice"},
					Unit:     &mealieapi.IngredientUnit{Name: "cup", Abbreviation: "c"},
					Checked:  true,
					Position: 1,
				},
			},
		},
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(mealietools.NewModule(getter))

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
		Name: "mealie.get_shopping_list",
		Arguments: map[string]any{
			"shopping_list_id": " list-id ",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", result.Content)
	}

	var output mealietools.GetShoppingListOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	if output.ID != "list-id" || output.Name != "Mon-Tue" || len(output.Items) != 2 {
		t.Fatalf("output = %+v, want list with two items", output)
	}
	if output.Items[0].Display != "Ground cumin — check pantry" || output.Items[0].Quantity != 0 || output.Items[0].Checked {
		t.Errorf("unstructured item = %+v, want display and zero quantity", output.Items[0])
	}
	if output.Items[1].Display != "2 cups rice" || output.Items[1].Food != "rice" || output.Items[1].Unit != "cup" || !output.Items[1].Checked {
		t.Errorf("structured item = %+v, want display/food/unit/checked", output.Items[1])
	}
	if !getter.shoppingListCalled || getter.shoppingListID != "list-id" {
		t.Errorf("shopping-list ID = %q, want normalized list-id", getter.shoppingListID)
	}
}

func TestGetShoppingListToolValidation(t *testing.T) {
	getter := &fakeRecipeSearcher{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(mealietools.NewModule(getter))

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
		Name:      "mealie.get_shopping_list",
		Arguments: map[string]any{"shopping_list_id": "   "},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want true")
	}
	if len(result.Content) == 0 || !strings.Contains(contentText(t, result.Content[0]), "shopping_list_id is required") {
		t.Fatalf("error content = %+v, want required list ID message", result.Content)
	}
	if getter.shoppingListCalled {
		t.Fatal("getter should not be called for invalid input")
	}
}
