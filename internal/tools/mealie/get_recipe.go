package mealie

import (
	"context"
	"fmt"
	"strings"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RecipeGetter is the small capability this tool needs from the Mealie
// integration. The MCP layer depends on the behavior, not the HTTP client.
type RecipeGetter interface {
	GetRecipe(context.Context, string) (mealieapi.Recipe, error)
}

// GetRecipeInput identifies a Mealie recipe by the slug or ID returned from
// mealie.search_recipes.
type GetRecipeInput struct {
	RecipeIDOrSlug string `json:"recipe_id_or_slug" jsonschema:"Mealie recipe slug or UUID"`
}

// GetRecipeOutput is the stable MCP-facing detail shape for one recipe.
type GetRecipeOutput struct {
	ID                  string                 `json:"id"`
	Name                string                 `json:"name"`
	Slug                string                 `json:"slug"`
	Description         string                 `json:"description,omitempty"`
	SourceURL           string                 `json:"source_url,omitempty"`
	PrepTime            string                 `json:"prep_time,omitempty"`
	CookTime            string                 `json:"cook_time,omitempty"`
	PerformTime         string                 `json:"perform_time,omitempty"`
	TotalTime           string                 `json:"total_time,omitempty"`
	RecipeServings      float64                `json:"recipe_servings,omitempty"`
	RecipeYieldQuantity float64                `json:"recipe_yield_quantity,omitempty"`
	RecipeYield         string                 `json:"recipe_yield,omitempty"`
	Ingredients         []GetRecipeIngredient  `json:"ingredients"`
	Instructions        []GetRecipeInstruction `json:"instructions"`
}

// GetRecipeIngredient preserves a human-readable ingredient string while
// retaining structured values when Mealie parsed them successfully.
type GetRecipeIngredient struct {
	Text     string  `json:"text"`
	Note     string  `json:"note,omitempty"`
	Quantity float64 `json:"quantity,omitempty"`
	Unit     string  `json:"unit,omitempty"`
	Food     string  `json:"food,omitempty"`
}

// GetRecipeInstruction is one ordered, human-readable recipe step.
type GetRecipeInstruction struct {
	ID      string `json:"id,omitempty"`
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
	Text    string `json:"text"`
}

// RegisterGetRecipe adds the read-only mealie.get_recipe tool to an MCP
// server.
func RegisterGetRecipe(server *mcp.Server, getter RecipeGetter) {
	if server == nil {
		panic("register mealie get recipe tool: nil server")
	}
	if getter == nil {
		panic("register mealie get recipe tool: nil getter")
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mealie.get_recipe",
		Title:       "Get a Mealie recipe",
		Description: "Retrieve one existing Mealie recipe by slug or UUID, including human-readable ingredients and ordered instructions. This is read-only and does not modify the recipe.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
			Title:        "Get a Mealie recipe",
		},
	}, getRecipeHandler(getter))
}

func getRecipeHandler(getter RecipeGetter) mcp.ToolHandlerFor[GetRecipeInput, GetRecipeOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input GetRecipeInput) (*mcp.CallToolResult, GetRecipeOutput, error) {
		identifier := strings.TrimSpace(input.RecipeIDOrSlug)
		if identifier == "" {
			return nil, GetRecipeOutput{}, fmt.Errorf("get recipe: recipe_id_or_slug is required")
		}

		recipe, err := getter.GetRecipe(ctx, identifier)
		if err != nil {
			return nil, GetRecipeOutput{}, fmt.Errorf("get recipe: %w", err)
		}

		output := GetRecipeOutput{
			ID:                  recipe.ID,
			Name:                recipe.Name,
			Slug:                recipe.Slug,
			Description:         recipe.Description,
			SourceURL:           recipe.OrgURL,
			PrepTime:            recipe.PrepTime,
			CookTime:            recipe.CookTime,
			PerformTime:         recipe.PerformTime,
			TotalTime:           recipe.TotalTime,
			RecipeServings:      recipe.RecipeServings,
			RecipeYieldQuantity: recipe.RecipeYieldQuantity,
			RecipeYield:         recipe.RecipeYield,
			Ingredients:         make([]GetRecipeIngredient, 0, len(recipe.Ingredients)),
			Instructions:        make([]GetRecipeInstruction, 0, len(recipe.Instructions)),
		}

		for _, ingredient := range recipe.Ingredients {
			text := strings.TrimSpace(ingredient.Display)
			if text == "" && ingredient.Note != nil {
				text = strings.TrimSpace(*ingredient.Note)
			}
			if text == "" && ingredient.OriginalText != nil {
				text = strings.TrimSpace(*ingredient.OriginalText)
			}

			result := GetRecipeIngredient{Text: text}
			if ingredient.Note != nil {
				result.Note = strings.TrimSpace(*ingredient.Note)
			}
			if ingredient.Quantity != nil && *ingredient.Quantity != 0 {
				result.Quantity = *ingredient.Quantity
			}
			if ingredient.Unit != nil {
				result.Unit = strings.TrimSpace(ingredient.Unit.Name)
				if result.Unit == "" {
					result.Unit = strings.TrimSpace(ingredient.Unit.Abbreviation)
				}
			}
			if ingredient.Food != nil {
				result.Food = strings.TrimSpace(ingredient.Food.Name)
			}
			output.Ingredients = append(output.Ingredients, result)
		}

		for _, instruction := range recipe.Instructions {
			output.Instructions = append(output.Instructions, GetRecipeInstruction{
				ID:      instruction.ID,
				Title:   instruction.Title,
				Summary: instruction.Summary,
				Text:    instruction.Text,
			})
		}

		return nil, output, nil
	}
}
