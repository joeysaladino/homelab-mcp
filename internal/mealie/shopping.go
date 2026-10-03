package mealie

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ShoppingListQuery controls a request to Mealie's shopping-list endpoint.
// Zero values for Page and PerPage leave those choices to Mealie's defaults.
type ShoppingListQuery struct {
	Page    int
	PerPage int
}

// CreateShoppingListParams describes an additive shopping-list creation.
type CreateShoppingListParams struct {
	Name string
}

// CreateShoppingListItemParams describes one human-readable shopping item.
// Quantity is deliberately not exposed here: synthesized grocery text goes in
// Display and the API payload always sends quantity 0 so Mealie does not add a
// misleading leading "1" in its UI.
type CreateShoppingListItemParams struct {
	ShoppingListID string
	Display        string
	Note           string
	Position       int
}

// ShoppingListItemsCollection is the response returned by Mealie's bulk item
// creation endpoint. The update/delete fields are retained because Mealie uses
// one collection shape for bulk changes, even though this client method only
// creates items.
type ShoppingListItemsCollection struct {
	CreatedItems []ShoppingListItem `json:"createdItems"`
	UpdatedItems []ShoppingListItem `json:"updatedItems"`
	DeletedItems []ShoppingListItem `json:"deletedItems"`
}

// ShoppingListPage is the paginated response returned by GET
// /api/households/shopping/lists.
type ShoppingListPage struct {
	Page       int                   `json:"page"`
	PerPage    int                   `json:"per_page"`
	Total      int                   `json:"total"`
	TotalPages int                   `json:"total_pages"`
	Items      []ShoppingListSummary `json:"items"`
	Next       *string               `json:"next"`
	Previous   *string               `json:"previous"`
}

// ShoppingListSummary is the compact projection returned when listing lists.
type ShoppingListSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	HouseholdID string `json:"householdId"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// ShoppingList is the useful projection returned by the shopping-list detail
// endpoint. Mealie embeds all list items in the list response.
type ShoppingList struct {
	ID               string                        `json:"id"`
	Name             string                        `json:"name"`
	HouseholdID      string                        `json:"householdId"`
	CreatedAt        string                        `json:"createdAt"`
	UpdatedAt        string                        `json:"updatedAt"`
	Items            []ShoppingListItem            `json:"listItems"`
	RecipeReferences []ShoppingListRecipeReference `json:"recipeReferences"`
}

// ShoppingListItem preserves both Mealie's structured fields and its
// human-readable display text. Imported or synthesized items commonly rely on
// Display and leave Food/Unit unset.
type ShoppingListItem struct {
	ID             string          `json:"id"`
	ShoppingListID string          `json:"shoppingListId"`
	Quantity       float64         `json:"quantity"`
	Unit           *IngredientUnit `json:"unit"`
	Food           *IngredientFood `json:"food"`
	Note           string          `json:"note"`
	Display        string          `json:"display"`
	Checked        bool            `json:"checked"`
	Position       int             `json:"position"`
	FoodID         *string         `json:"foodId"`
	LabelID        *string         `json:"labelId"`
	UnitID         *string         `json:"unitId"`
	CreatedAt      string          `json:"createdAt"`
	UpdatedAt      string          `json:"updatedAt"`
}

// ShoppingListRecipeReference identifies a recipe associated with a list.
// It is retained by the typed client for future shopping-list workflows.
type ShoppingListRecipeReference struct {
	ID             string         `json:"id"`
	ShoppingListID string         `json:"shoppingListId"`
	RecipeID       string         `json:"recipeId"`
	RecipeQuantity float64        `json:"recipeQuantity"`
	Recipe         *RecipeSummary `json:"recipe"`
}

// GetShoppingLists retrieves the household's shopping lists.
func (c *Client) GetShoppingLists(ctx context.Context, params ShoppingListQuery) (ShoppingListPage, error) {
	if params.Page < 0 {
		return ShoppingListPage{}, fmt.Errorf("get mealie shopping lists: page must not be negative")
	}
	if params.PerPage < 0 {
		return ShoppingListPage{}, fmt.Errorf("get mealie shopping lists: per-page must not be negative")
	}

	query := url.Values{}
	if params.Page > 0 {
		query.Set("page", strconv.Itoa(params.Page))
	}
	if params.PerPage > 0 {
		query.Set("perPage", strconv.Itoa(params.PerPage))
	}

	var page ShoppingListPage
	if err := c.getJSON(ctx, "/api/households/shopping/lists", query, &page); err != nil {
		return ShoppingListPage{}, fmt.Errorf("get mealie shopping lists: %w", err)
	}
	return page, nil
}

// GetShoppingList retrieves one list and its embedded shopping items by UUID.
func (c *Client) GetShoppingList(ctx context.Context, listID string) (ShoppingList, error) {
	identifier := strings.TrimSpace(listID)
	if identifier == "" {
		return ShoppingList{}, fmt.Errorf("get mealie shopping list: list ID is required")
	}
	if strings.ContainsAny(identifier, `/\\?#`) {
		return ShoppingList{}, fmt.Errorf("get mealie shopping list: list ID contains an invalid path character")
	}

	var list ShoppingList
	resource := "/api/households/shopping/lists/" + identifier
	if err := c.getJSON(ctx, resource, nil, &list); err != nil {
		return ShoppingList{}, fmt.Errorf("get mealie shopping list %q: %w", identifier, err)
	}
	return list, nil
}

// CreateShoppingList creates one empty household shopping list.
func (c *Client) CreateShoppingList(ctx context.Context, params CreateShoppingListParams) (ShoppingList, error) {
	name := strings.TrimSpace(params.Name)
	if name == "" {
		return ShoppingList{}, fmt.Errorf("create mealie shopping list: name is required")
	}

	payload := struct {
		Name string `json:"name"`
	}{Name: name}

	var list ShoppingList
	if err := c.postJSON(ctx, "/api/households/shopping/lists", payload, &list); err != nil {
		return ShoppingList{}, fmt.Errorf("create mealie shopping list: %w", err)
	}
	return list, nil
}

// CreateShoppingListItems adds human-readable grocery items to one existing
// list in a single Mealie request.
func (c *Client) CreateShoppingListItems(ctx context.Context, params []CreateShoppingListItemParams) (ShoppingListItemsCollection, error) {
	if len(params) == 0 {
		return ShoppingListItemsCollection{}, fmt.Errorf("create mealie shopping items: at least one item is required")
	}

	payload := make([]struct {
		Quantity       float64 `json:"quantity"`
		Note           string  `json:"note"`
		Display        string  `json:"display"`
		ShoppingListID string  `json:"shoppingListId"`
		Checked        bool    `json:"checked"`
		Position       int     `json:"position"`
	}, len(params))
	for i, item := range params {
		listID, err := validateShoppingListID(item.ShoppingListID)
		if err != nil {
			return ShoppingListItemsCollection{}, fmt.Errorf("create mealie shopping items: item %d: %w", i, err)
		}
		display := strings.TrimSpace(item.Display)
		if display == "" {
			return ShoppingListItemsCollection{}, fmt.Errorf("create mealie shopping items: item %d: display is required", i)
		}
		if item.Position < 0 {
			return ShoppingListItemsCollection{}, fmt.Errorf("create mealie shopping items: item %d: position must not be negative", i)
		}
		note := strings.TrimSpace(item.Note)
		if note == "" {
			note = display
		}

		payload[i] = struct {
			Quantity       float64 `json:"quantity"`
			Note           string  `json:"note"`
			Display        string  `json:"display"`
			ShoppingListID string  `json:"shoppingListId"`
			Checked        bool    `json:"checked"`
			Position       int     `json:"position"`
		}{
			Quantity:       0,
			Note:           note,
			Display:        display,
			ShoppingListID: listID,
			Position:       item.Position,
		}
	}

	var result ShoppingListItemsCollection
	if err := c.postJSON(ctx, "/api/households/shopping/items/create-bulk", payload, &result); err != nil {
		return ShoppingListItemsCollection{}, fmt.Errorf("create mealie shopping items: %w", err)
	}
	return result, nil
}

func validateShoppingListID(value string) (string, error) {
	identifier := strings.TrimSpace(value)
	if identifier == "" {
		return "", fmt.Errorf("list ID is required")
	}
	if strings.ContainsAny(identifier, `/\\?#`) {
		return "", fmt.Errorf("list ID contains an invalid path character")
	}
	return identifier, nil
}
