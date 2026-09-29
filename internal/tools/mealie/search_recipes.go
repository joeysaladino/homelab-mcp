// Package mealie contains the MCP tools backed by Mealie.
package mealie

import (
	"context"
	"fmt"
	"strings"

	"github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultSearchLimit = 10
	maxSearchLimit     = 50
)

// RecipeSearcher is the small capability this tool needs from the Mealie
// integration. Keeping the interface here makes the MCP layer easy to test
// without an HTTP server and keeps it independent of the concrete client.
type RecipeSearcher interface {
	SearchRecipes(context.Context, mealie.RecipeSearchParams) (mealie.RecipePage, error)
}

// SearchRecipesInput is the human-oriented input for mealie.search_recipes.
// It intentionally does not expose Mealie's raw pagination model.
type SearchRecipesInput struct {
	Query string `json:"query" jsonschema:"recipe name, ingredient, or phrase to search for"`
	Limit int    `json:"limit,omitempty" jsonschema:"maximum number of matching recipes to return; defaults to 10 and cannot exceed 50"`
	Page  int    `json:"page,omitempty" jsonschema:"page of results to return, starting at 1; use next_page when has_more is true"`
}

// SearchRecipesOutput is the stable MCP-facing result shape for recipe search.
type SearchRecipesOutput struct {
	Query      string               `json:"query"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"page_size"`
	Total      int                  `json:"total"`
	TotalPages int                  `json:"total_pages"`
	HasMore    bool                 `json:"has_more"`
	NextPage   *int                 `json:"next_page,omitempty"`
	Recipes    []SearchRecipeResult `json:"recipes"`
}

// SearchRecipeResult is a concise recipe result suitable for choosing a
// recipe. Full ingredients and instructions belong to a later detail tool.
type SearchRecipeResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
}

// RegisterSearchRecipes adds the read-only mealie.search_recipes tool to an
// MCP server.
func RegisterSearchRecipes(server *mcp.Server, searcher RecipeSearcher) {
	if server == nil {
		panic("register mealie search tool: nil server")
	}
	if searcher == nil {
		panic("register mealie search tool: nil searcher")
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mealie.search_recipes",
		Title:       "Search Mealie recipes",
		Description: "Search the existing Mealie recipe library by name, ingredient, or phrase. Results are paginated; when has_more is true, call again with the same query and next_page. This is read-only and does not import or modify recipes.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
			Title:        "Search Mealie recipes",
		},
	}, searchRecipesHandler(searcher))
}

func searchRecipesHandler(searcher RecipeSearcher) mcp.ToolHandlerFor[SearchRecipesInput, SearchRecipesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input SearchRecipesInput) (*mcp.CallToolResult, SearchRecipesOutput, error) {
		query := strings.TrimSpace(input.Query)
		if query == "" {
			return nil, SearchRecipesOutput{}, fmt.Errorf("search recipes: query is required")
		}

		limit := input.Limit
		if limit == 0 {
			limit = defaultSearchLimit
		}
		if limit < 0 || limit > maxSearchLimit {
			return nil, SearchRecipesOutput{}, fmt.Errorf("search recipes: limit must be between 1 and %d", maxSearchLimit)
		}
		pageNumber := input.Page
		if pageNumber == 0 {
			pageNumber = 1
		}
		if pageNumber < 1 {
			return nil, SearchRecipesOutput{}, fmt.Errorf("search recipes: page must be at least 1")
		}

		page, err := searcher.SearchRecipes(ctx, mealie.RecipeSearchParams{
			Query:   query,
			Page:    pageNumber,
			PerPage: limit,
		})
		if err != nil {
			return nil, SearchRecipesOutput{}, fmt.Errorf("search recipes: %w", err)
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

		output := SearchRecipesOutput{
			Query:      query,
			Page:       page.Page,
			PageSize:   pageSize,
			Total:      page.Total,
			TotalPages: page.TotalPages,
			HasMore:    hasMore,
			NextPage:   nextPage,
			Recipes:    make([]SearchRecipeResult, 0, len(page.Items)),
		}
		for _, recipe := range page.Items {
			output.Recipes = append(output.Recipes, SearchRecipeResult{
				ID:          recipe.ID,
				Name:        recipe.Name,
				Slug:        recipe.Slug,
				Description: recipe.Description,
				SourceURL:   recipe.OrgURL,
			})
		}

		return nil, output, nil
	}
}
