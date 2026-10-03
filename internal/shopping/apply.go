package shopping

import (
	"context"
	"fmt"
	"strings"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
)

const (
	maxShoppingListsPerApply = 10
	maxShoppingItemsPerList  = 100
	maxShoppingItemsPerApply = 500
)

// ShoppingListCreator is the Mealie capability needed to create one list.
type ShoppingListCreator interface {
	CreateShoppingList(context.Context, mealieapi.CreateShoppingListParams) (mealieapi.ShoppingList, error)
}

// ShoppingListItemWriter is the Mealie capability needed to populate one
// list. Keeping it separate makes partial-write behavior explicit and easy to
// test.
type ShoppingListItemWriter interface {
	CreateShoppingListItems(context.Context, []mealieapi.CreateShoppingListItemParams) (mealieapi.ShoppingListItemsCollection, error)
}

// ShoppingListApplier is the application boundary used by the MCP adapter.
type ShoppingListApplier interface {
	ApplyShoppingLists(context.Context, ShoppingListDraft) (ApplyShoppingListsResult, error)
}

// ShoppingListDraft is the normalized, write-ready grocery representation
// submitted after the LLM has interpreted raw recipe ingredients.
type ShoppingListDraft struct {
	Trips []ShoppingTripInput
}

// ShoppingTripInput is one new Mealie list and its final grocery items.
type ShoppingTripInput struct {
	Name  string
	Items []ShoppingItemInput
}

// ShoppingItemInput is intentionally human-readable. Quantity is embedded in
// Display and the Mealie client writes quantity 0 for clean UI rendering.
type ShoppingItemInput struct {
	Display string
	Note    string
}

// ApplyShoppingListsResult describes lists created by a successful apply.
type ApplyShoppingListsResult struct {
	Lists      []AppliedShoppingList
	TotalItems int
}

// AppliedShoppingList identifies one newly created Mealie list.
type AppliedShoppingList struct {
	Name      string
	ID        string
	ItemCount int
}

// Applier creates new lists and bulk-adds their items. It never updates or
// deletes existing Mealie data.
type Applier struct {
	lists ShoppingListCreator
	items ShoppingListItemWriter
}

// NewApplier constructs the additive shopping-list use case.
func NewApplier(lists ShoppingListCreator, items ShoppingListItemWriter) *Applier {
	if lists == nil {
		panic("create shopping applier: nil list creator")
	}
	if items == nil {
		panic("create shopping applier: nil item writer")
	}
	return &Applier{lists: lists, items: items}
}

// NormalizeShoppingListDraft trims and validates an LLM-produced draft before
// any network write occurs. It also defaults an omitted note to the display
// text, matching the server-side grocery semantics.
func NormalizeShoppingListDraft(input ShoppingListDraft) (ShoppingListDraft, error) {
	if len(input.Trips) == 0 {
		return ShoppingListDraft{}, fmt.Errorf("at least one shopping trip is required")
	}
	if len(input.Trips) > maxShoppingListsPerApply {
		return ShoppingListDraft{}, fmt.Errorf("cannot apply more than %d shopping trips at once", maxShoppingListsPerApply)
	}

	result := ShoppingListDraft{Trips: make([]ShoppingTripInput, len(input.Trips))}
	seenNames := make(map[string]int, len(input.Trips))
	totalItems := 0
	for tripIndex, trip := range input.Trips {
		name := strings.TrimSpace(trip.Name)
		if name == "" {
			return ShoppingListDraft{}, fmt.Errorf("trip %d: name is required", tripIndex)
		}
		nameKey := strings.ToLower(name)
		if previous, ok := seenNames[nameKey]; ok {
			return ShoppingListDraft{}, fmt.Errorf("trip %d: duplicate name also used by trip %d", tripIndex, previous)
		}
		seenNames[nameKey] = tripIndex
		if len(trip.Items) == 0 {
			return ShoppingListDraft{}, fmt.Errorf("trip %d %q: at least one item is required", tripIndex, name)
		}
		if len(trip.Items) > maxShoppingItemsPerList {
			return ShoppingListDraft{}, fmt.Errorf("trip %d %q: cannot contain more than %d items", tripIndex, name, maxShoppingItemsPerList)
		}
		totalItems += len(trip.Items)
		if totalItems > maxShoppingItemsPerApply {
			return ShoppingListDraft{}, fmt.Errorf("cannot apply more than %d shopping items at once", maxShoppingItemsPerApply)
		}

		result.Trips[tripIndex] = ShoppingTripInput{
			Name:  name,
			Items: make([]ShoppingItemInput, len(trip.Items)),
		}
		for itemIndex, item := range trip.Items {
			display := strings.TrimSpace(item.Display)
			if display == "" {
				return ShoppingListDraft{}, fmt.Errorf("trip %d %q item %d: display is required", tripIndex, name, itemIndex)
			}
			note := strings.TrimSpace(item.Note)
			if note == "" {
				note = display
			}
			result.Trips[tripIndex].Items[itemIndex] = ShoppingItemInput{
				Display: display,
				Note:    note,
			}
		}
	}
	return result, nil
}

// ApplyShoppingLists creates each normalized trip as a new Mealie list and
// bulk-adds its items. If a later trip fails, earlier lists remain because
// Mealie does not expose a transaction spanning both endpoints.
func (a *Applier) ApplyShoppingLists(ctx context.Context, input ShoppingListDraft) (ApplyShoppingListsResult, error) {
	normalized, err := NormalizeShoppingListDraft(input)
	if err != nil {
		return ApplyShoppingListsResult{}, fmt.Errorf("apply shopping lists: %w", err)
	}

	result := ApplyShoppingListsResult{
		Lists: make([]AppliedShoppingList, 0, len(normalized.Trips)),
	}
	for _, trip := range normalized.Trips {
		list, err := a.lists.CreateShoppingList(ctx, mealieapi.CreateShoppingListParams{Name: trip.Name})
		if err != nil {
			return result, fmt.Errorf("apply shopping lists: create list %q: %w", trip.Name, err)
		}

		items := make([]mealieapi.CreateShoppingListItemParams, len(trip.Items))
		for i, item := range trip.Items {
			items[i] = mealieapi.CreateShoppingListItemParams{
				ShoppingListID: list.ID,
				Display:        item.Display,
				Note:           item.Note,
				Position:       i,
			}
		}
		created, err := a.items.CreateShoppingListItems(ctx, items)
		if err != nil {
			return result, fmt.Errorf("apply shopping lists: add items to list %q (%s): %w", trip.Name, list.ID, err)
		}

		itemCount := len(created.CreatedItems)
		result.Lists = append(result.Lists, AppliedShoppingList{
			Name:      trip.Name,
			ID:        list.ID,
			ItemCount: itemCount,
		})
		result.TotalItems += itemCount
	}
	return result, nil
}
