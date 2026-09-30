package mealie

import (
	"context"
	"fmt"
	"strings"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultShoppingListLimit = 50
	maxShoppingListLimit     = 50
)

// ShoppingListLister is the small capability this tool needs from the Mealie
// integration.
type ShoppingListLister interface {
	GetShoppingLists(context.Context, mealieapi.ShoppingListQuery) (mealieapi.ShoppingListPage, error)
}

// GetShoppingListsInput controls pagination when discovering household lists.
type GetShoppingListsInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"maximum number of shopping lists to return; defaults to 50 and cannot exceed 50"`
	Page  int `json:"page,omitempty" jsonschema:"page of shopping lists to return, starting at 1; use next_page when has_more is true"`
}

// GetShoppingListsOutput is the stable MCP-facing result shape for list
// discovery. Items are retrieved separately with mealie.get_shopping_list.
type GetShoppingListsOutput struct {
	Page       int                      `json:"page"`
	PageSize   int                      `json:"page_size"`
	Total      int                      `json:"total"`
	TotalPages int                      `json:"total_pages"`
	HasMore    bool                     `json:"has_more"`
	NextPage   *int                     `json:"next_page,omitempty"`
	Lists      []GetShoppingListSummary `json:"lists"`
}

// GetShoppingListSummary is the compact list projection returned by the
// discovery tool.
type GetShoppingListSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// RegisterGetShoppingLists adds the read-only mealie.get_shopping_lists tool.
func RegisterGetShoppingLists(server *mcp.Server, lister ShoppingListLister) {
	if server == nil {
		panic("register mealie get shopping lists tool: nil server")
	}
	if lister == nil {
		panic("register mealie get shopping lists tool: nil lister")
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mealie.get_shopping_lists",
		Title:       "List Mealie shopping lists",
		Description: "List the household's existing Mealie shopping lists so a later call can inspect a specific list by ID. Results are paginated. This is read-only and does not create, modify, or delete lists or items.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
			Title:        "List Mealie shopping lists",
		},
	}, getShoppingListsHandler(lister))
}

func getShoppingListsHandler(lister ShoppingListLister) mcp.ToolHandlerFor[GetShoppingListsInput, GetShoppingListsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input GetShoppingListsInput) (*mcp.CallToolResult, GetShoppingListsOutput, error) {
		limit := input.Limit
		if limit == 0 {
			limit = defaultShoppingListLimit
		}
		if limit < 0 || limit > maxShoppingListLimit {
			return nil, GetShoppingListsOutput{}, fmt.Errorf("get shopping lists: limit must be between 1 and %d", maxShoppingListLimit)
		}

		pageNumber := input.Page
		if pageNumber == 0 {
			pageNumber = 1
		}
		if pageNumber < 1 {
			return nil, GetShoppingListsOutput{}, fmt.Errorf("get shopping lists: page must be at least 1")
		}

		page, err := lister.GetShoppingLists(ctx, mealieapi.ShoppingListQuery{
			Page:    pageNumber,
			PerPage: limit,
		})
		if err != nil {
			return nil, GetShoppingListsOutput{}, fmt.Errorf("get shopping lists: %w", err)
		}

		pageSize := page.PerPage
		if pageSize == 0 {
			pageSize = limit
		}
		hasMore := page.TotalPages > page.Page
		var nextPage *int
		if hasMore {
			next := page.Page + 1
			nextPage = &next
		}

		output := GetShoppingListsOutput{
			Page:       page.Page,
			PageSize:   pageSize,
			Total:      page.Total,
			TotalPages: page.TotalPages,
			HasMore:    hasMore,
			NextPage:   nextPage,
			Lists:      make([]GetShoppingListSummary, 0, len(page.Items)),
		}
		for _, list := range page.Items {
			output.Lists = append(output.Lists, GetShoppingListSummary{
				ID:        list.ID,
				Name:      strings.TrimSpace(list.Name),
				CreatedAt: strings.TrimSpace(list.CreatedAt),
				UpdatedAt: strings.TrimSpace(list.UpdatedAt),
			})
		}

		return nil, output, nil
	}
}
