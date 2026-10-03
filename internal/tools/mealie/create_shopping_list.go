package mealie

import (
	"context"
	"fmt"
	"strings"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ShoppingListCreator is the small capability this tool needs from the
// Mealie integration.
type ShoppingListCreator interface {
	CreateShoppingList(context.Context, mealieapi.CreateShoppingListParams) (mealieapi.ShoppingList, error)
}

// CreateShoppingListInput describes one additive shopping-list creation.
type CreateShoppingListInput struct {
	Name string `json:"name" jsonschema:"human-readable name for the new shopping list"`
}

// CreateShoppingListOutput reports the new list identity without exposing the
// full Mealie response model.
type CreateShoppingListOutput struct {
	Created bool                   `json:"created"`
	List    GetShoppingListSummary `json:"list"`
}

// RegisterCreateShoppingList adds the additive mealie.create_shopping_list
// tool.
func RegisterCreateShoppingList(server *mcp.Server, creator ShoppingListCreator) {
	if server == nil {
		panic("register mealie create shopping list tool: nil server")
	}
	if creator == nil {
		panic("register mealie create shopping list tool: nil creator")
	}

	destructive := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mealie.create_shopping_list",
		Title:       "Create a Mealie shopping list",
		Description: "Create one new empty Mealie shopping list. This is an additive write and does not modify or delete existing lists. Use mealie.add_shopping_items to populate it.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: &destructive,
			ReadOnlyHint:    false,
			Title:           "Create a Mealie shopping list",
		},
	}, createShoppingListHandler(creator))
}

func createShoppingListHandler(creator ShoppingListCreator) mcp.ToolHandlerFor[CreateShoppingListInput, CreateShoppingListOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input CreateShoppingListInput) (*mcp.CallToolResult, CreateShoppingListOutput, error) {
		name := strings.TrimSpace(input.Name)
		if name == "" {
			return nil, CreateShoppingListOutput{}, fmt.Errorf("create shopping list: name is required")
		}

		list, err := creator.CreateShoppingList(ctx, mealieapi.CreateShoppingListParams{Name: name})
		if err != nil {
			return nil, CreateShoppingListOutput{}, fmt.Errorf("create shopping list: %w", err)
		}

		return nil, CreateShoppingListOutput{
			Created: true,
			List: GetShoppingListSummary{
				ID:        list.ID,
				Name:      strings.TrimSpace(list.Name),
				CreatedAt: strings.TrimSpace(list.CreatedAt),
				UpdatedAt: strings.TrimSpace(list.UpdatedAt),
			},
		}, nil
	}
}
