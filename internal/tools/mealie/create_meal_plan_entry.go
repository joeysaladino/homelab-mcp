package mealie

import (
	"context"
	"fmt"
	"strings"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MealPlanWriter is the small capability this tool needs from the Mealie
// integration.
type MealPlanWriter interface {
	CreateMealPlanEntry(context.Context, mealieapi.CreateMealPlanEntryParams) (mealieapi.MealPlanEntry, error)
}

// CreateMealPlanEntryInput describes an additive meal-plan write. A recipe ID
// is optional when the meal is represented by title and/or text.
type CreateMealPlanEntryInput struct {
	Date      string `json:"date" jsonschema:"meal date in YYYY-MM-DD format"`
	EntryType string `json:"entry_type" jsonschema:"meal type: breakfast, lunch, dinner, side, snack, drink, or dessert"`
	Title     string `json:"title,omitempty" jsonschema:"short title for a simple meal"`
	Text      string `json:"text,omitempty" jsonschema:"optional description for a simple meal"`
	RecipeID  string `json:"recipe_id,omitempty" jsonschema:"Mealie recipe UUID from search or recipe detail; omit for a simple meal"`
}

// CreateMealPlanEntryOutput reports the created entry returned by Mealie.
type CreateMealPlanEntryOutput struct {
	Created bool             `json:"created"`
	Entry   GetMealPlanEntry `json:"entry"`
}

// RegisterCreateMealPlanEntry adds the additive mealie.create_meal_plan_entry
// tool to an MCP server.
func RegisterCreateMealPlanEntry(server *mcp.Server, writer MealPlanWriter) {
	if server == nil {
		panic("register mealie create meal plan tool: nil server")
	}
	if writer == nil {
		panic("register mealie create meal plan tool: nil writer")
	}

	destructive := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "mealie.create_meal_plan_entry",
		Title:       "Add a meal to the Mealie plan",
		Description: "Add one recipe-backed or simple meal to Mealie's household plan. This is an additive write: it creates a new plan entry and does not replace or delete existing entries. Use an existing recipe UUID when recipe_id is provided.",
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: &destructive,
			ReadOnlyHint:    false,
			Title:           "Add a meal to the Mealie plan",
		},
	}, createMealPlanEntryHandler(writer))
}

func createMealPlanEntryHandler(writer MealPlanWriter) mcp.ToolHandlerFor[CreateMealPlanEntryInput, CreateMealPlanEntryOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input CreateMealPlanEntryInput) (*mcp.CallToolResult, CreateMealPlanEntryOutput, error) {
		date, err := mealieapi.ParseDate(input.Date)
		if err != nil {
			return nil, CreateMealPlanEntryOutput{}, fmt.Errorf("create meal plan entry: invalid date: %w", err)
		}

		entryTypeValue := strings.ToLower(strings.TrimSpace(input.EntryType))
		if entryTypeValue == "" {
			return nil, CreateMealPlanEntryOutput{}, fmt.Errorf("create meal plan entry: entry_type is required")
		}
		entryType := mealieapi.PlanEntryType(entryTypeValue)
		if !entryType.Valid() {
			return nil, CreateMealPlanEntryOutput{}, fmt.Errorf("create meal plan entry: unsupported entry_type %q", input.EntryType)
		}

		title := strings.TrimSpace(input.Title)
		text := strings.TrimSpace(input.Text)
		recipeIDValue := strings.TrimSpace(input.RecipeID)
		if title == "" && text == "" && recipeIDValue == "" {
			return nil, CreateMealPlanEntryOutput{}, fmt.Errorf("create meal plan entry: provide title, text, or recipe_id")
		}

		var recipeID *string
		if recipeIDValue != "" {
			recipeID = &recipeIDValue
		}

		entry, err := writer.CreateMealPlanEntry(ctx, mealieapi.CreateMealPlanEntryParams{
			Date:      date,
			EntryType: entryType,
			Title:     title,
			Text:      text,
			RecipeID:  recipeID,
		})
		if err != nil {
			return nil, CreateMealPlanEntryOutput{}, fmt.Errorf("create meal plan entry: %w", err)
		}

		return nil, CreateMealPlanEntryOutput{
			Created: true,
			Entry:   mealPlanEntryOutput(entry),
		}, nil
	}
}
