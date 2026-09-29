package mealie

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// RecipeSearchParams controls a request to Mealie's recipe search endpoint.
// Zero values for Page and PerPage leave those choices to Mealie's defaults.
type RecipeSearchParams struct {
	Query   string
	Page    int
	PerPage int
}

// RecipePage is the paginated response returned by GET /api/recipes.
type RecipePage struct {
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
	Total      int             `json:"total"`
	TotalPages int             `json:"total_pages"`
	Items      []RecipeSummary `json:"items"`
	Next       *string         `json:"next"`
	Previous   *string         `json:"previous"`
}

// RecipeSummary is the intentionally small recipe projection needed for
// search. Full recipe details belong to a separate client method.
type RecipeSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	OrgURL      string `json:"orgURL"`
}

// SearchRecipes searches the existing Mealie recipe library.
func (c *Client) SearchRecipes(ctx context.Context, params RecipeSearchParams) (RecipePage, error) {
	search := strings.TrimSpace(params.Query)
	if search == "" {
		return RecipePage{}, fmt.Errorf("search mealie recipes: query is required")
	}
	if params.Page < 0 {
		return RecipePage{}, fmt.Errorf("search mealie recipes: page must not be negative")
	}
	if params.PerPage < 0 {
		return RecipePage{}, fmt.Errorf("search mealie recipes: per-page must not be negative")
	}

	query := url.Values{}
	query.Set("search", search)
	if params.Page > 0 {
		query.Set("page", strconv.Itoa(params.Page))
	}
	if params.PerPage > 0 {
		query.Set("perPage", strconv.Itoa(params.PerPage))
	}

	var page RecipePage
	if err := c.getJSON(ctx, "/api/recipes", query, &page); err != nil {
		return RecipePage{}, fmt.Errorf("search mealie recipes: %w", err)
	}
	return page, nil
}
