package mealie

import (
	"context"
	"fmt"
	"strings"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RecipeImporter is the small capability this tool needs from the Mealie
// integration.
type RecipeImporter interface {
	ImportRecipeURL(context.Context, mealieapi.ImportRecipeParams) (string, error)
}

// ImportRecipeURLInput controls a URL-based recipe import. The operation is
// intentionally explicit because it creates a new recipe in Mealie.
type ImportRecipeURLInput struct {
	URL               string `json:"url" jsonschema:"public HTTP or HTTPS URL of the recipe to import"`
	IncludeTags       bool   `json:"include_tags,omitempty" jsonschema:"whether Mealie should import source tags"`
	IncludeCategories bool   `json:"include_categories,omitempty" jsonschema:"whether Mealie should import source categories"`
}

// ImportRecipeURLOutput reports the successful additive write without
// pretending Mealie returned a complete recipe object.
type ImportRecipeURLOutput struct {
	URL          string `json:"url"`
	Created      bool   `json:"created"`
	MealieResult string `json:"mealie_result"`
}

// RegisterImportRecipeURL adds the additive mealie.import_recipe_url tool to
// an MCP server.
func RegisterImportRecipeURL(server *mcp.Server, importer RecipeImporter) {
	if server == nil {
		panic("register mealie import recipe tool: nil server")
	}
	if importer == nil {
		panic("register mealie import recipe tool: nil importer")
	}

	destructive := false
	openWorld := true
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mealie.import_recipe_url",
		Title:       "Import a recipe into Mealie",
		Description: "Import a recipe from a public HTTP or HTTPS URL into Mealie. This creates a new recipe; search existing recipes first to avoid duplicates. The source URL is fetched server-side and the backend token is never exposed to the caller.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: &destructive,
			OpenWorldHint:   &openWorld,
			ReadOnlyHint:    false,
			Title:           "Import a recipe into Mealie",
		},
	}, importRecipeURLHandler(importer))
}

func importRecipeURLHandler(importer RecipeImporter) mcp.ToolHandlerFor[ImportRecipeURLInput, ImportRecipeURLOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input ImportRecipeURLInput) (*mcp.CallToolResult, ImportRecipeURLOutput, error) {
		sourceURL := strings.TrimSpace(input.URL)
		if sourceURL == "" {
			return nil, ImportRecipeURLOutput{}, fmt.Errorf("import recipe: url is required")
		}

		result, err := importer.ImportRecipeURL(ctx, mealieapi.ImportRecipeParams{
			URL:               sourceURL,
			IncludeTags:       input.IncludeTags,
			IncludeCategories: input.IncludeCategories,
		})
		if err != nil {
			return nil, ImportRecipeURLOutput{}, fmt.Errorf("import recipe: %w", err)
		}

		return nil, ImportRecipeURLOutput{
			URL:          sourceURL,
			Created:      true,
			MealieResult: result,
		}, nil
	}
}
