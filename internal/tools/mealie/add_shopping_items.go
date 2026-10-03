package mealie

import (
	"context"
	"fmt"
	"strings"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxShoppingItemsPerWrite = 100

// ShoppingListItemWriter is the small capability this tool needs from the
// Mealie integration.
type ShoppingListItemWriter interface {
	CreateShoppingListItems(context.Context, []mealieapi.CreateShoppingListItemParams) (mealieapi.ShoppingListItemsCollection, error)
}

// AddShoppingItemsInput describes human-readable grocery items to append to an
// existing list. The server supplies quantity 0 to Mealie for every item.
type AddShoppingItemsInput struct {
	ShoppingListID string                 `json:"shopping_list_id" jsonschema:"Mealie shopping-list UUID from mealie.get_shopping_lists"`
	Items          []AddShoppingItemInput `json:"items" jsonschema:"one or more human-readable grocery items to add"`
}

// AddShoppingItemInput is intentionally centered on grocery-store text rather
// than Mealie's optional food/unit entities.
type AddShoppingItemInput struct {
	Display string `json:"display" jsonschema:"complete human-readable grocery text, including quantity or pantry guidance"`
	Note    string `json:"note,omitempty" jsonschema:"optional note; defaults to display"`
}

// AddShoppingItemsOutput reports the items Mealie created.
type AddShoppingItemsOutput struct {
	Added          bool                  `json:"added"`
	ShoppingListID string                `json:"shopping_list_id"`
	Items          []GetShoppingListItem `json:"items"`
}

// RegisterAddShoppingItems adds the additive mealie.add_shopping_items tool.
func RegisterAddShoppingItems(server *mcp.Server, writer ShoppingListItemWriter) {
	if server == nil {
		panic("register mealie add shopping items tool: nil server")
	}
	if writer == nil {
		panic("register mealie add shopping items tool: nil writer")
	}

	destructive := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mealie.add_shopping_items",
		Title:       "Add items to a Mealie shopping list",
		Description: "Append clean human-readable grocery items to an existing Mealie shopping list. This is an additive write: it does not replace, merge, check, or delete existing items. Inspect the list first when duplicate avoidance matters.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: &destructive,
			ReadOnlyHint:    false,
			Title:           "Add items to a Mealie shopping list",
		},
	}, addShoppingItemsHandler(writer))
}

func addShoppingItemsHandler(writer ShoppingListItemWriter) mcp.ToolHandlerFor[AddShoppingItemsInput, AddShoppingItemsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input AddShoppingItemsInput) (*mcp.CallToolResult, AddShoppingItemsOutput, error) {
		listID := strings.TrimSpace(input.ShoppingListID)
		if listID == "" {
			return nil, AddShoppingItemsOutput{}, fmt.Errorf("add shopping items: shopping_list_id is required")
		}
		if len(input.Items) == 0 {
			return nil, AddShoppingItemsOutput{}, fmt.Errorf("add shopping items: at least one item is required")
		}
		if len(input.Items) > maxShoppingItemsPerWrite {
			return nil, AddShoppingItemsOutput{}, fmt.Errorf("add shopping items: cannot add more than %d items at once", maxShoppingItemsPerWrite)
		}

		params := make([]mealieapi.CreateShoppingListItemParams, len(input.Items))
		for i, item := range input.Items {
			display := strings.TrimSpace(item.Display)
			if display == "" {
				return nil, AddShoppingItemsOutput{}, fmt.Errorf("add shopping items: item %d display is required", i)
			}
			params[i] = mealieapi.CreateShoppingListItemParams{
				ShoppingListID: listID,
				Display:        display,
				Note:           strings.TrimSpace(item.Note),
				Position:       i,
			}
		}

		result, err := writer.CreateShoppingListItems(ctx, params)
		if err != nil {
			return nil, AddShoppingItemsOutput{}, fmt.Errorf("add shopping items: %w", err)
		}

		output := AddShoppingItemsOutput{
			Added:          true,
			ShoppingListID: listID,
			Items:          make([]GetShoppingListItem, 0, len(result.CreatedItems)),
		}
		for _, item := range result.CreatedItems {
			output.Items = append(output.Items, shoppingListItemOutput(item))
		}
		return nil, output, nil
	}
}
