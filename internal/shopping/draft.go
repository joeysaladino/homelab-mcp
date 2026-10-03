package shopping

import (
	"context"
	"fmt"
	"strings"
	"time"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/joeysaladino/homelab-mcp/internal/pantry"
)

const (
	draftDateLayout  = "2006-01-02"
	maxDraftDays     = 31
	maxMealPlanPages = 1000
	mealPlanPageSize = 50
)

// MealPlanReader is the smallest Mealie capability needed to build a draft.
type MealPlanReader interface {
	GetMealPlan(context.Context, mealieapi.MealPlanQuery) (mealieapi.MealPlanPage, error)
}

// RecipeReader is the smallest Mealie capability needed to expand recipe
// backed meal-plan entries into ingredient sources.
type RecipeReader interface {
	GetRecipe(context.Context, string) (mealieapi.Recipe, error)
}

// DraftBuilder is the use-case boundary consumed by the MCP adapter.
type DraftBuilder interface {
	BuildShoppingDraft(context.Context, DraftParams) (ShoppingDraft, error)
}

// DraftParams identifies the inclusive date range for a grocery draft.
type DraftParams struct {
	StartDate mealieapi.Date
	EndDate   mealieapi.Date
}

// ShoppingDraft is a read-only planning context. Ingredients remain attached
// to their source meal so a later LLM step can normalize them with context.
type ShoppingDraft struct {
	StartDate       mealieapi.Date
	EndDate         mealieapi.Date
	Trips           []TripDraft
	UnassignedMeals []MealDraft
	Pantry          pantry.Pantry
	Warnings        []string
}

// TripDraft groups meals according to the configured household grocery run.
type TripDraft struct {
	Name     string
	Weekdays []Weekday
	Meals    []MealDraft
}

// MealDraft preserves a meal-plan entry and its recipe-derived ingredient
// sources. Simple meals may have no ingredients.
type MealDraft struct {
	Date        mealieapi.Date
	EntryType   mealieapi.PlanEntryType
	Title       string
	Text        string
	RecipeID    string
	RecipeName  string
	RecipeSlug  string
	Ingredients []IngredientSource
}

// IngredientSource preserves parsed values when available while keeping the
// human-readable text authoritative for imported recipes.
type IngredientSource struct {
	Text     string
	Note     string
	Quantity *float64
	Unit     string
	Food     string
}

// Planner combines Mealie data with household pantry and trip configuration.
// It contains no MCP or HTTP concerns, which keeps the workflow testable and
// reusable by another transport later.
type Planner struct {
	mealPlans MealPlanReader
	recipes   RecipeReader
	pantry    pantry.Pantry
	trips     Plan
}

// NewPlanner constructs the read-only grocery-draft use case.
func NewPlanner(mealPlans MealPlanReader, recipes RecipeReader, pantryContext pantry.Pantry, tripPlan Plan) *Planner {
	if mealPlans == nil {
		panic("create shopping planner: nil meal-plan reader")
	}
	if recipes == nil {
		panic("create shopping planner: nil recipe reader")
	}
	return &Planner{
		mealPlans: mealPlans,
		recipes:   recipes,
		pantry:    pantryContext,
		trips:     tripPlan,
	}
}

// BuildShoppingDraft retrieves all meal-plan pages in the requested range,
// expands recipe-backed entries, and groups meals by configured weekday.
func (p *Planner) BuildShoppingDraft(ctx context.Context, params DraftParams) (ShoppingDraft, error) {
	startDate, endDate, err := validateDraftRange(params)
	if err != nil {
		return ShoppingDraft{}, fmt.Errorf("build shopping draft: %w", err)
	}

	entries, err := p.loadMealPlanEntries(ctx, startDate, endDate)
	if err != nil {
		return ShoppingDraft{}, fmt.Errorf("build shopping draft: %w", err)
	}

	draft := ShoppingDraft{
		StartDate:       startDate,
		EndDate:         endDate,
		Trips:           make([]TripDraft, 0, len(p.trips.Trips)),
		UnassignedMeals: make([]MealDraft, 0),
		Pantry:          p.pantry,
		Warnings:        make([]string, 0),
	}
	for _, trip := range p.trips.Trips {
		draft.Trips = append(draft.Trips, TripDraft{
			Name:     trip.Name,
			Weekdays: append([]Weekday(nil), trip.Weekdays...),
			Meals:    make([]MealDraft, 0),
		})
	}

	for _, entry := range entries {
		meal, warnings, err := p.expandMeal(ctx, entry)
		if err != nil {
			return ShoppingDraft{}, fmt.Errorf("build shopping draft: %w", err)
		}
		draft.Warnings = append(draft.Warnings, warnings...)

		weekday, err := weekdayForDate(entry.Date)
		if err != nil {
			return ShoppingDraft{}, fmt.Errorf("build shopping draft: %w", err)
		}
		trip, ok := p.trips.TripForWeekday(weekday)
		if !ok {
			draft.UnassignedMeals = append(draft.UnassignedMeals, meal)
			continue
		}
		for i := range draft.Trips {
			if draft.Trips[i].Name == trip.Name {
				draft.Trips[i].Meals = append(draft.Trips[i].Meals, meal)
				break
			}
		}
	}

	return draft, nil
}

func validateDraftRange(params DraftParams) (mealieapi.Date, mealieapi.Date, error) {
	startDate, err := mealieapi.ParseDate(string(params.StartDate))
	if err != nil {
		return "", "", fmt.Errorf("invalid start date: %w", err)
	}
	endDate, err := mealieapi.ParseDate(string(params.EndDate))
	if err != nil {
		return "", "", fmt.Errorf("invalid end date: %w", err)
	}
	start, _ := time.Parse(draftDateLayout, string(startDate))
	end, _ := time.Parse(draftDateLayout, string(endDate))
	if start.After(end) {
		return "", "", fmt.Errorf("start date must not be after end date")
	}
	days := int(end.Sub(start)/(24*time.Hour)) + 1
	if days > maxDraftDays {
		return "", "", fmt.Errorf("date range cannot exceed %d days", maxDraftDays)
	}
	return startDate, endDate, nil
}

func (p *Planner) loadMealPlanEntries(ctx context.Context, startDate, endDate mealieapi.Date) ([]mealieapi.MealPlanEntry, error) {
	entries := make([]mealieapi.MealPlanEntry, 0)
	for pageNumber := 1; pageNumber <= maxMealPlanPages; pageNumber++ {
		page, err := p.mealPlans.GetMealPlan(ctx, mealieapi.MealPlanQuery{
			StartDate: startDate,
			EndDate:   endDate,
			Page:      pageNumber,
			PerPage:   mealPlanPageSize,
		})
		if err != nil {
			return nil, fmt.Errorf("read meal plan page %d: %w", pageNumber, err)
		}
		entries = append(entries, page.Items...)

		currentPage := page.Page
		if currentPage == 0 {
			currentPage = pageNumber
		}
		if page.TotalPages <= currentPage || len(page.Items) == 0 {
			return entries, nil
		}
	}
	return nil, fmt.Errorf("meal plan exceeded %d pages", maxMealPlanPages)
}

func (p *Planner) expandMeal(ctx context.Context, entry mealieapi.MealPlanEntry) (MealDraft, []string, error) {
	meal := MealDraft{
		Date:        entry.Date,
		EntryType:   entry.EntryType,
		Title:       strings.TrimSpace(entry.Title),
		Text:        strings.TrimSpace(entry.Text),
		Ingredients: make([]IngredientSource, 0),
	}
	warnings := make([]string, 0)

	identifier := recipeIdentifier(entry)
	if identifier == "" {
		return meal, warnings, nil
	}

	recipe, err := p.recipes.GetRecipe(ctx, identifier)
	if err != nil {
		return MealDraft{}, nil, fmt.Errorf("read recipe %q for %s: %w", identifier, entry.Date, err)
	}
	meal.RecipeID = recipe.ID
	meal.RecipeName = strings.TrimSpace(recipe.Name)
	meal.RecipeSlug = strings.TrimSpace(recipe.Slug)
	for i, ingredient := range recipe.Ingredients {
		text := ingredient.HumanText()
		if text == "" {
			warnings = append(warnings, fmt.Sprintf("recipe %q ingredient %d has no human-readable text", recipe.Name, i))
			continue
		}
		source := IngredientSource{Text: text}
		if ingredient.Note != nil {
			source.Note = strings.TrimSpace(*ingredient.Note)
		}
		if ingredient.Quantity != nil {
			source.Quantity = ingredient.Quantity
		}
		if ingredient.Unit != nil {
			source.Unit = strings.TrimSpace(ingredient.Unit.Name)
			if source.Unit == "" {
				source.Unit = strings.TrimSpace(ingredient.Unit.Abbreviation)
			}
		}
		if ingredient.Food != nil {
			source.Food = strings.TrimSpace(ingredient.Food.Name)
		}
		meal.Ingredients = append(meal.Ingredients, source)
	}
	return meal, warnings, nil
}

func recipeIdentifier(entry mealieapi.MealPlanEntry) string {
	if entry.RecipeID != nil && strings.TrimSpace(*entry.RecipeID) != "" {
		return strings.TrimSpace(*entry.RecipeID)
	}
	if entry.Recipe == nil {
		return ""
	}
	if strings.TrimSpace(entry.Recipe.ID) != "" {
		return strings.TrimSpace(entry.Recipe.ID)
	}
	return strings.TrimSpace(entry.Recipe.Slug)
}

func weekdayForDate(date mealieapi.Date) (Weekday, error) {
	parsed, err := time.Parse(draftDateLayout, string(date))
	if err != nil {
		return "", fmt.Errorf("parse meal-plan date %q: %w", date, err)
	}
	switch parsed.Weekday() {
	case time.Monday:
		return Monday, nil
	case time.Tuesday:
		return Tuesday, nil
	case time.Wednesday:
		return Wednesday, nil
	case time.Thursday:
		return Thursday, nil
	case time.Friday:
		return Friday, nil
	case time.Saturday:
		return Saturday, nil
	case time.Sunday:
		return Sunday, nil
	default:
		return "", fmt.Errorf("unsupported weekday for date %q", date)
	}
}
