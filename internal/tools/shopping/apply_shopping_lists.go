package shopping

import (
	"context"
	"fmt"

	shoppingdomain "github.com/joeysaladino/homelab-mcp/internal/shopping"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ApplyShoppingListsInput contains the final human-readable grocery lists
// produced after the LLM has interpreted the read-only shopping draft.
type ApplyShoppingListsInput struct {
	Confirm bool                     `json:"confirm" jsonschema:"set true only after the user explicitly approves this preview"`
	Trips   []ApplyShoppingTripInput `json:"trips" jsonschema:"new shopping lists and their final grocery items"`
}

// ApplyShoppingTripInput describes one new Mealie shopping list.
type ApplyShoppingTripInput struct {
	Name  string                   `json:"name" jsonschema:"name for the new Mealie shopping list"`
	Items []ApplyShoppingItemInput `json:"items" jsonschema:"final human-readable grocery items"`
}

// ApplyShoppingItemInput keeps the grocery representation human-readable.
// Quantity belongs in Display so Mealie renders the complete grocery text
// without adding an unwanted leading quantity.
type ApplyShoppingItemInput struct {
	Display string `json:"display" jsonschema:"complete human-readable grocery text, including quantity or pantry guidance"`
	Note    string `json:"note,omitempty" jsonschema:"optional note; defaults to display"`
}

// ApplyShoppingListsOutput reports either a no-write preview or newly created
// lists. Items are echoed in both cases so a caller can inspect exactly what
// would be written before confirming.
type ApplyShoppingListsOutput struct {
	Applied              bool                      `json:"applied"`
	ConfirmationRequired bool                      `json:"confirmation_required"`
	Lists                []ApplyShoppingListResult `json:"lists"`
	TotalItems           int                       `json:"total_items"`
}

// ApplyShoppingListResult is the stable MCP projection of one list.
type ApplyShoppingListResult struct {
	Name      string                     `json:"name"`
	ID        string                     `json:"id,omitempty"`
	ItemCount int                        `json:"item_count"`
	Items     []ApplyShoppingItemPreview `json:"items"`
}

// ApplyShoppingItemPreview is one final grocery item shown in a preview or
// returned after a successful apply.
type ApplyShoppingItemPreview struct {
	Display string `json:"display"`
	Note    string `json:"note"`
}

// RegisterApplyShoppingLists adds the additive, confirmation-gated
// mealie.apply_shopping_lists tool.
func RegisterApplyShoppingLists(server *mcp.Server, applier shoppingdomain.ShoppingListApplier) {
	if server == nil {
		panic("register apply shopping lists tool: nil server")
	}
	if applier == nil {
		panic("register apply shopping lists tool: nil applier")
	}

	destructive := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mealie.apply_shopping_lists",
		Title:       "Preview or create Mealie shopping lists",
		Description: "Validate final human-readable grocery lists and return a no-write preview by default. Set confirm=true only after the user explicitly approves the preview; then create new Mealie lists and items. This tool never replaces, merges, or deletes existing lists or items.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: &destructive,
			ReadOnlyHint:    false,
			Title:           "Preview or create Mealie shopping lists",
		},
	}, applyShoppingListsHandler(applier))
}

func applyShoppingListsHandler(applier shoppingdomain.ShoppingListApplier) mcp.ToolHandlerFor[ApplyShoppingListsInput, ApplyShoppingListsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input ApplyShoppingListsInput) (*mcp.CallToolResult, ApplyShoppingListsOutput, error) {
		draft, err := shoppingdomain.NormalizeShoppingListDraft(toShoppingListDraft(input))
		if err != nil {
			return nil, ApplyShoppingListsOutput{}, fmt.Errorf("apply shopping lists: %w", err)
		}

		if !input.Confirm {
			return nil, shoppingListPreviewOutput(draft), nil
		}

		result, err := applier.ApplyShoppingLists(ctx, draft)
		if err != nil {
			return nil, ApplyShoppingListsOutput{}, fmt.Errorf("apply shopping lists: %w", err)
		}
		return nil, shoppingListAppliedOutput(draft, result), nil
	}
}

func toShoppingListDraft(input ApplyShoppingListsInput) shoppingdomain.ShoppingListDraft {
	draft := shoppingdomain.ShoppingListDraft{
		Trips: make([]shoppingdomain.ShoppingTripInput, len(input.Trips)),
	}
	for i, trip := range input.Trips {
		draft.Trips[i] = shoppingdomain.ShoppingTripInput{
			Name:  trip.Name,
			Items: make([]shoppingdomain.ShoppingItemInput, len(trip.Items)),
		}
		for j, item := range trip.Items {
			draft.Trips[i].Items[j] = shoppingdomain.ShoppingItemInput{
				Display: item.Display,
				Note:    item.Note,
			}
		}
	}
	return draft
}

func shoppingListPreviewOutput(draft shoppingdomain.ShoppingListDraft) ApplyShoppingListsOutput {
	return shoppingListsOutput(draft, nil, false)
}

func shoppingListAppliedOutput(draft shoppingdomain.ShoppingListDraft, result shoppingdomain.ApplyShoppingListsResult) ApplyShoppingListsOutput {
	return shoppingListsOutput(draft, &result, true)
}

func shoppingListsOutput(draft shoppingdomain.ShoppingListDraft, applied *shoppingdomain.ApplyShoppingListsResult, didApply bool) ApplyShoppingListsOutput {
	output := ApplyShoppingListsOutput{
		Applied:              didApply,
		ConfirmationRequired: !didApply,
		Lists:                make([]ApplyShoppingListResult, 0, len(draft.Trips)),
	}
	for i, trip := range draft.Trips {
		list := ApplyShoppingListResult{
			Name:      trip.Name,
			ItemCount: len(trip.Items),
			Items:     make([]ApplyShoppingItemPreview, 0, len(trip.Items)),
		}
		for _, item := range trip.Items {
			list.Items = append(list.Items, ApplyShoppingItemPreview{
				Display: item.Display,
				Note:    item.Note,
			})
		}
		if applied != nil && i < len(applied.Lists) {
			list.ID = applied.Lists[i].ID
			list.ItemCount = applied.Lists[i].ItemCount
		}
		output.TotalItems += list.ItemCount
		output.Lists = append(output.Lists, list)
	}
	return output
}
