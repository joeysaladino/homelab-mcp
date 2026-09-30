package mealie

import (
	"context"
	"fmt"
	"strings"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ShoppingListGetter is the small capability this tool needs from the Mealie
// integration.
type ShoppingListGetter interface {
	GetShoppingList(context.Context, string) (mealieapi.ShoppingList, error)
}

// GetShoppingListInput identifies one Mealie shopping list by UUID. The ID
// normally comes from mealie.get_shopping_lists.
type GetShoppingListInput struct {
	ShoppingListID string `json:"shopping_list_id" jsonschema:"Mealie shopping-list UUID from mealie.get_shopping_lists"`
}

// GetShoppingListOutput is the stable MCP-facing representation of one list.
// Display is the authoritative human-readable item text; Quantity is retained
// because Mealie uses zero when a complete quantity is already in Display.
type GetShoppingListOutput struct {
	ID        string                `json:"id"`
	Name      string                `json:"name,omitempty"`
	CreatedAt string                `json:"created_at,omitempty"`
	UpdatedAt string                `json:"updated_at,omitempty"`
	Items     []GetShoppingListItem `json:"items"`
}

// GetShoppingListItem is a concise grocery-item projection. Structured Food
// and Unit values are included when Mealie has parsed them, but callers should
// use Display for imported or synthesized items.
type GetShoppingListItem struct {
	ID       string  `json:"id"`
	Display  string  `json:"display"`
	Quantity float64 `json:"quantity"`
	Food     string  `json:"food,omitempty"`
	Unit     string  `json:"unit,omitempty"`
	Checked  bool    `json:"checked"`
	Position int     `json:"position"`
}

// RegisterGetShoppingList adds the read-only mealie.get_shopping_list tool.
func RegisterGetShoppingList(server *mcp.Server, getter ShoppingListGetter) {
	if server == nil {
		panic("register mealie get shopping list tool: nil server")
	}
	if getter == nil {
		panic("register mealie get shopping list tool: nil getter")
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mealie.get_shopping_list",
		Title:       "Get a Mealie shopping list",
		Description: "Retrieve one existing Mealie shopping list and its current items by UUID. Item display text is preserved for human grocery semantics, including synthesized quantities and pantry notes. This is read-only and does not modify the list.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
			Title:        "Get a Mealie shopping list",
		},
	}, getShoppingListHandler(getter))
}

func getShoppingListHandler(getter ShoppingListGetter) mcp.ToolHandlerFor[GetShoppingListInput, GetShoppingListOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input GetShoppingListInput) (*mcp.CallToolResult, GetShoppingListOutput, error) {
		listID := strings.TrimSpace(input.ShoppingListID)
		if listID == "" {
			return nil, GetShoppingListOutput{}, fmt.Errorf("get shopping list: shopping_list_id is required")
		}

		list, err := getter.GetShoppingList(ctx, listID)
		if err != nil {
			return nil, GetShoppingListOutput{}, fmt.Errorf("get shopping list: %w", err)
		}

		output := GetShoppingListOutput{
			ID:        list.ID,
			Name:      strings.TrimSpace(list.Name),
			CreatedAt: strings.TrimSpace(list.CreatedAt),
			UpdatedAt: strings.TrimSpace(list.UpdatedAt),
			Items:     make([]GetShoppingListItem, 0, len(list.Items)),
		}
		for _, item := range list.Items {
			output.Items = append(output.Items, shoppingListItemOutput(item))
		}

		return nil, output, nil
	}
}

func shoppingListItemOutput(item mealieapi.ShoppingListItem) GetShoppingListItem {
	display := strings.TrimSpace(item.Display)
	if display == "" {
		display = strings.TrimSpace(item.Note)
	}

	result := GetShoppingListItem{
		ID:       item.ID,
		Display:  display,
		Quantity: item.Quantity,
		Checked:  item.Checked,
		Position: item.Position,
	}
	if item.Food != nil {
		result.Food = strings.TrimSpace(item.Food.Name)
	}
	if item.Unit != nil {
		result.Unit = strings.TrimSpace(item.Unit.Name)
		if result.Unit == "" {
			result.Unit = strings.TrimSpace(item.Unit.Abbreviation)
		}
	}
	return result
}
