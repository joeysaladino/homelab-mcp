package shopping

import (
	"context"
	"fmt"
	"strings"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	shoppingdomain "github.com/joeysaladino/homelab-mcp/internal/shopping"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PrepareShoppingDraftBuilder is the use case this MCP adapter exposes.
type PrepareShoppingDraftBuilder interface {
	BuildShoppingDraft(context.Context, shoppingdomain.DraftParams) (shoppingdomain.ShoppingDraft, error)
}

// PrepareShoppingDraftInput identifies the inclusive meal-plan range.
type PrepareShoppingDraftInput struct {
	StartDate string `json:"start_date" jsonschema:"inclusive start date in YYYY-MM-DD format"`
	EndDate   string `json:"end_date" jsonschema:"inclusive end date in YYYY-MM-DD format"`
}

// PrepareShoppingDraftOutput is a read-only context for LLM grocery
// normalization. Ingredients remain attached to their source meals instead of
// being silently merged by the server.
type PrepareShoppingDraftOutput struct {
	StartDate       string                      `json:"start_date"`
	EndDate         string                      `json:"end_date"`
	Trips           []PrepareShoppingTrip       `json:"trips"`
	UnassignedMeals []PrepareShoppingMeal       `json:"unassigned_meals"`
	Pantry          []PrepareShoppingPantryItem `json:"pantry"`
	Warnings        []string                    `json:"warnings"`
}

// PrepareShoppingTrip is one configured grocery run and its meals.
type PrepareShoppingTrip struct {
	Name     string                `json:"name"`
	Weekdays []string              `json:"weekdays"`
	Meals    []PrepareShoppingMeal `json:"meals"`
}

// PrepareShoppingMeal is one meal-plan entry with recipe-derived ingredients.
type PrepareShoppingMeal struct {
	Date        string                      `json:"date"`
	EntryType   string                      `json:"entry_type"`
	Title       string                      `json:"title,omitempty"`
	Text        string                      `json:"text,omitempty"`
	RecipeID    string                      `json:"recipe_id,omitempty"`
	RecipeName  string                      `json:"recipe_name,omitempty"`
	RecipeSlug  string                      `json:"recipe_slug,omitempty"`
	Ingredients []PrepareShoppingIngredient `json:"ingredients"`
}

// PrepareShoppingIngredient retains both human text and any parsed Mealie
// values. Text is authoritative when imported recipe structure is incomplete.
type PrepareShoppingIngredient struct {
	Text     string   `json:"text"`
	Note     string   `json:"note,omitempty"`
	Quantity *float64 `json:"quantity,omitempty"`
	Unit     string   `json:"unit,omitempty"`
	Food     string   `json:"food,omitempty"`
}

// PrepareShoppingPantryItem is the stable MCP-facing pantry projection.
type PrepareShoppingPantryItem struct {
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	State    string `json:"state"`
	Note     string `json:"note,omitempty"`
}

// RegisterPrepareShoppingDraft adds the read-only
// mealie.prepare_shopping_draft tool.
func RegisterPrepareShoppingDraft(server *mcp.Server, builder PrepareShoppingDraftBuilder) {
	if server == nil {
		panic("register prepare shopping draft tool: nil server")
	}
	if builder == nil {
		panic("register prepare shopping draft tool: nil builder")
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mealie.prepare_shopping_draft",
		Title:       "Prepare a Mealie shopping draft",
		Description: "Read the Mealie meal plan for a date range, expand recipe-backed meals into human-readable ingredient sources, group meals by configured grocery run, and include pantry context. This is read-only: it does not normalize, create, merge, or modify shopping lists.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
			Title:        "Prepare a Mealie shopping draft",
		},
	}, prepareShoppingDraftHandler(builder))
}

func prepareShoppingDraftHandler(builder PrepareShoppingDraftBuilder) mcp.ToolHandlerFor[PrepareShoppingDraftInput, PrepareShoppingDraftOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input PrepareShoppingDraftInput) (*mcp.CallToolResult, PrepareShoppingDraftOutput, error) {
		startDate, err := mealieapi.ParseDate(input.StartDate)
		if err != nil {
			return nil, PrepareShoppingDraftOutput{}, fmt.Errorf("prepare shopping draft: invalid start_date: %w", err)
		}
		endDate, err := mealieapi.ParseDate(input.EndDate)
		if err != nil {
			return nil, PrepareShoppingDraftOutput{}, fmt.Errorf("prepare shopping draft: invalid end_date: %w", err)
		}
		if startDate > endDate {
			return nil, PrepareShoppingDraftOutput{}, fmt.Errorf("prepare shopping draft: start_date must not be after end_date")
		}

		draft, err := builder.BuildShoppingDraft(ctx, shoppingdomain.DraftParams{
			StartDate: startDate,
			EndDate:   endDate,
		})
		if err != nil {
			return nil, PrepareShoppingDraftOutput{}, fmt.Errorf("prepare shopping draft: %w", err)
		}

		return nil, shoppingDraftOutput(draft), nil
	}
}

func shoppingDraftOutput(draft shoppingdomain.ShoppingDraft) PrepareShoppingDraftOutput {
	output := PrepareShoppingDraftOutput{
		StartDate:       string(draft.StartDate),
		EndDate:         string(draft.EndDate),
		Trips:           make([]PrepareShoppingTrip, 0, len(draft.Trips)),
		UnassignedMeals: make([]PrepareShoppingMeal, 0, len(draft.UnassignedMeals)),
		Pantry:          make([]PrepareShoppingPantryItem, 0, len(draft.Pantry.Items)),
		Warnings:        make([]string, 0, len(draft.Warnings)),
	}
	output.Warnings = append(output.Warnings, draft.Warnings...)
	for _, trip := range draft.Trips {
		result := PrepareShoppingTrip{
			Name:     trip.Name,
			Weekdays: make([]string, 0, len(trip.Weekdays)),
			Meals:    make([]PrepareShoppingMeal, 0, len(trip.Meals)),
		}
		for _, weekday := range trip.Weekdays {
			result.Weekdays = append(result.Weekdays, string(weekday))
		}
		for _, meal := range trip.Meals {
			result.Meals = append(result.Meals, shoppingMealOutput(meal))
		}
		output.Trips = append(output.Trips, result)
	}
	for _, meal := range draft.UnassignedMeals {
		output.UnassignedMeals = append(output.UnassignedMeals, shoppingMealOutput(meal))
	}
	for _, item := range draft.Pantry.Items {
		output.Pantry = append(output.Pantry, PrepareShoppingPantryItem{
			Name:     item.Name,
			Category: item.Category,
			State:    string(item.State),
			Note:     item.Note,
		})
	}
	return output
}

func shoppingMealOutput(meal shoppingdomain.MealDraft) PrepareShoppingMeal {
	result := PrepareShoppingMeal{
		Date:        string(meal.Date),
		EntryType:   string(meal.EntryType),
		Title:       strings.TrimSpace(meal.Title),
		Text:        strings.TrimSpace(meal.Text),
		RecipeID:    strings.TrimSpace(meal.RecipeID),
		RecipeName:  strings.TrimSpace(meal.RecipeName),
		RecipeSlug:  strings.TrimSpace(meal.RecipeSlug),
		Ingredients: make([]PrepareShoppingIngredient, 0, len(meal.Ingredients)),
	}
	for _, ingredient := range meal.Ingredients {
		result.Ingredients = append(result.Ingredients, PrepareShoppingIngredient{
			Text:     ingredient.Text,
			Note:     ingredient.Note,
			Quantity: ingredient.Quantity,
			Unit:     ingredient.Unit,
			Food:     ingredient.Food,
		})
	}
	return result
}
