package mealie

import (
	"context"
	"fmt"
	"strings"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultMealPlanLimit = 50
	maxMealPlanLimit     = 50
)

// MealPlanReader is the small capability this tool needs from the Mealie
// integration.
type MealPlanReader interface {
	GetMealPlan(context.Context, mealieapi.MealPlanQuery) (mealieapi.MealPlanPage, error)
}

// GetMealPlanInput requests an inclusive date range from Mealie.
type GetMealPlanInput struct {
	StartDate string `json:"start_date" jsonschema:"inclusive start date in YYYY-MM-DD format"`
	EndDate   string `json:"end_date" jsonschema:"inclusive end date in YYYY-MM-DD format"`
	Limit     int    `json:"limit,omitempty" jsonschema:"maximum number of entries to return; defaults to 50 and cannot exceed 50"`
	Page      int    `json:"page,omitempty" jsonschema:"page of entries to return, starting at 1; use next_page when has_more is true"`
}

// GetMealPlanOutput is the stable MCP-facing result shape for a meal-plan
// range. Recipe is nil for simple meals.
type GetMealPlanOutput struct {
	StartDate  string             `json:"start_date"`
	EndDate    string             `json:"end_date"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
	Total      int                `json:"total"`
	TotalPages int                `json:"total_pages"`
	HasMore    bool               `json:"has_more"`
	NextPage   *int               `json:"next_page,omitempty"`
	Entries    []GetMealPlanEntry `json:"entries"`
}

// GetMealPlanEntry is one human-usable plan entry.
type GetMealPlanEntry struct {
	ID        int                `json:"id"`
	Date      string             `json:"date"`
	EntryType string             `json:"entry_type"`
	Title     string             `json:"title,omitempty"`
	Text      string             `json:"text,omitempty"`
	RecipeID  string             `json:"recipe_id,omitempty"`
	Recipe    *GetMealPlanRecipe `json:"recipe,omitempty"`
}

// GetMealPlanRecipe is the compact recipe projection embedded in a plan
// entry.
type GetMealPlanRecipe struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	SourceURL   string `json:"source_url,omitempty"`
}

// RegisterGetMealPlan adds the read-only mealie.get_meal_plan tool to an MCP
// server.
func RegisterGetMealPlan(server *mcp.Server, reader MealPlanReader) {
	if server == nil {
		panic("register mealie get meal plan tool: nil server")
	}
	if reader == nil {
		panic("register mealie get meal plan tool: nil reader")
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "mealie.get_meal_plan",
		Title:       "Get a Mealie meal plan",
		Description: "Retrieve Mealie meal-plan entries for an inclusive date range. Recipe-backed meals include a compact recipe summary; simple meals preserve their title and text. This is read-only and does not modify the plan.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
			Title:        "Get a Mealie meal plan",
		},
	}, getMealPlanHandler(reader))
}

func getMealPlanHandler(reader MealPlanReader) mcp.ToolHandlerFor[GetMealPlanInput, GetMealPlanOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input GetMealPlanInput) (*mcp.CallToolResult, GetMealPlanOutput, error) {
		startDate, err := mealieapi.ParseDate(input.StartDate)
		if err != nil {
			return nil, GetMealPlanOutput{}, fmt.Errorf("get meal plan: invalid start_date: %w", err)
		}
		endDate, err := mealieapi.ParseDate(input.EndDate)
		if err != nil {
			return nil, GetMealPlanOutput{}, fmt.Errorf("get meal plan: invalid end_date: %w", err)
		}
		if startDate > endDate {
			return nil, GetMealPlanOutput{}, fmt.Errorf("get meal plan: start_date must not be after end_date")
		}

		limit := input.Limit
		if limit == 0 {
			limit = defaultMealPlanLimit
		}
		if limit < 0 || limit > maxMealPlanLimit {
			return nil, GetMealPlanOutput{}, fmt.Errorf("get meal plan: limit must be between 1 and %d", maxMealPlanLimit)
		}
		pageNumber := input.Page
		if pageNumber == 0 {
			pageNumber = 1
		}
		if pageNumber < 1 {
			return nil, GetMealPlanOutput{}, fmt.Errorf("get meal plan: page must be at least 1")
		}

		page, err := reader.GetMealPlan(ctx, mealieapi.MealPlanQuery{
			StartDate: startDate,
			EndDate:   endDate,
			Page:      pageNumber,
			PerPage:   limit,
		})
		if err != nil {
			return nil, GetMealPlanOutput{}, fmt.Errorf("get meal plan: %w", err)
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

		output := GetMealPlanOutput{
			StartDate:  string(startDate),
			EndDate:    string(endDate),
			Page:       page.Page,
			PageSize:   pageSize,
			Total:      page.Total,
			TotalPages: page.TotalPages,
			HasMore:    hasMore,
			NextPage:   nextPage,
			Entries:    make([]GetMealPlanEntry, 0, len(page.Items)),
		}

		for _, entry := range page.Items {
			output.Entries = append(output.Entries, mealPlanEntryOutput(entry))
		}

		return nil, output, nil
	}
}

func mealPlanEntryOutput(entry mealieapi.MealPlanEntry) GetMealPlanEntry {
	result := GetMealPlanEntry{
		ID:        entry.ID,
		Date:      string(entry.Date),
		EntryType: string(entry.EntryType),
		Title:     entry.Title,
		Text:      entry.Text,
	}
	if entry.RecipeID != nil {
		result.RecipeID = strings.TrimSpace(*entry.RecipeID)
	}
	if entry.Recipe != nil {
		result.Recipe = &GetMealPlanRecipe{
			ID:          entry.Recipe.ID,
			Name:        entry.Recipe.Name,
			Slug:        entry.Recipe.Slug,
			Description: entry.Recipe.Description,
			SourceURL:   entry.Recipe.OrgURL,
		}
	}
	return result
}
