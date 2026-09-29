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

// Recipe is the detail projection returned by GET /api/recipes/{slug}. It is
// intentionally limited to fields the MCP layer can use for recipe selection,
// meal planning, and future ingredient processing.
type Recipe struct {
	ID                  string              `json:"id"`
	Name                string              `json:"name"`
	Slug                string              `json:"slug"`
	Description         string              `json:"description"`
	OrgURL              string              `json:"orgURL"`
	PrepTime            string              `json:"prepTime"`
	CookTime            string              `json:"cookTime"`
	PerformTime         string              `json:"performTime"`
	TotalTime           string              `json:"totalTime"`
	RecipeServings      float64             `json:"recipeServings"`
	RecipeYieldQuantity float64             `json:"recipeYieldQuantity"`
	RecipeYield         string              `json:"recipeYield"`
	Ingredients         []RecipeIngredient  `json:"recipeIngredient"`
	Instructions        []RecipeInstruction `json:"recipeInstructions"`
}

// RecipeIngredient represents both Mealie's parsed fields and its fallback
// human-readable fields. Imported recipes commonly populate Display or Note
// while leaving Food, Unit, and Quantity unstructured.
type RecipeIngredient struct {
	Quantity     *float64        `json:"quantity"`
	Unit         *IngredientUnit `json:"unit"`
	Food         *IngredientFood `json:"food"`
	Note         *string         `json:"note"`
	Display      string          `json:"display"`
	OriginalText *string         `json:"originalText"`
}

// IngredientUnit is the small unit projection needed by recipe consumers.
type IngredientUnit struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
}

// IngredientFood is the small food projection needed by recipe consumers.
type IngredientFood struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RecipeInstruction is one ordered instruction step from a Mealie recipe.
type RecipeInstruction struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Text    string `json:"text"`
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

// GetRecipe retrieves one recipe by its Mealie slug or UUID.
func (c *Client) GetRecipe(ctx context.Context, slugOrID string) (Recipe, error) {
	identifier := strings.TrimSpace(slugOrID)
	if identifier == "" {
		return Recipe{}, fmt.Errorf("get mealie recipe: slug or id is required")
	}
	if strings.ContainsAny(identifier, `/\\?#`) {
		return Recipe{}, fmt.Errorf("get mealie recipe: slug or id contains an invalid path character")
	}

	var recipe Recipe
	resource := "/api/recipes/" + identifier
	if err := c.getJSON(ctx, resource, nil, &recipe); err != nil {
		return Recipe{}, fmt.Errorf("get mealie recipe %q: %w", identifier, err)
	}
	return recipe, nil
}
