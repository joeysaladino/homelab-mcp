package mealie

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const mealPlanDateLayout = "2006-01-02"

// Date is a calendar date in the format used by Mealie's API. It deliberately
// does not carry a time zone because meal-plan entries are date-only values.
type Date string

// PlanEntryType is one of Mealie's supported meal-plan categories.
type PlanEntryType string

const (
	PlanEntryBreakfast PlanEntryType = "breakfast"
	PlanEntryLunch     PlanEntryType = "lunch"
	PlanEntryDinner    PlanEntryType = "dinner"
	PlanEntrySide      PlanEntryType = "side"
	PlanEntrySnack     PlanEntryType = "snack"
	PlanEntryDrink     PlanEntryType = "drink"
	PlanEntryDessert   PlanEntryType = "dessert"
)

// Valid reports whether the plan entry type is supported by Mealie.
func (t PlanEntryType) Valid() bool {
	switch t {
	case PlanEntryBreakfast, PlanEntryLunch, PlanEntryDinner, PlanEntrySide, PlanEntrySnack, PlanEntryDrink, PlanEntryDessert:
		return true
	default:
		return false
	}
}

// ParseDate validates and normalizes an ISO-8601 calendar date.
func ParseDate(value string) (Date, error) {
	date := strings.TrimSpace(value)
	if date == "" {
		return "", fmt.Errorf("date is required")
	}
	if _, err := time.Parse(mealPlanDateLayout, date); err != nil {
		return "", fmt.Errorf("date must use YYYY-MM-DD: %w", err)
	}
	return Date(date), nil
}

func validateDate(value Date, field string) error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse(mealPlanDateLayout, string(value)); err != nil {
		return fmt.Errorf("%s must use YYYY-MM-DD: %w", field, err)
	}
	return nil
}

// MealPlanQuery controls a request to Mealie's meal-plan endpoint. Dates are
// inclusive, and zero Page/PerPage values leave pagination defaults to Mealie.
type MealPlanQuery struct {
	StartDate Date
	EndDate   Date
	Page      int
	PerPage   int
}

// CreateMealPlanEntryParams describes an additive meal-plan write. RecipeID
// may be nil for a simple meal represented by title and/or text.
type CreateMealPlanEntryParams struct {
	Date      Date
	EntryType PlanEntryType
	Title     string
	Text      string
	RecipeID  *string
}

// MealPlanPage is the paginated response returned by GET
// /api/households/mealplans.
type MealPlanPage struct {
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
	Total      int             `json:"total"`
	TotalPages int             `json:"total_pages"`
	Items      []MealPlanEntry `json:"items"`
	Next       *string         `json:"next"`
	Previous   *string         `json:"previous"`
}

// MealPlanEntry is the useful projection of a Mealie plan entry. Recipe is
// nil for simple meals that have only title/text.
type MealPlanEntry struct {
	Date      Date           `json:"date"`
	EntryType PlanEntryType  `json:"entryType"`
	Title     string         `json:"title"`
	Text      string         `json:"text"`
	RecipeID  *string        `json:"recipeId"`
	ID        int            `json:"id"`
	Recipe    *RecipeSummary `json:"recipe"`
}

// GetMealPlan retrieves meal-plan entries in an optional inclusive date
// range.
func (c *Client) GetMealPlan(ctx context.Context, params MealPlanQuery) (MealPlanPage, error) {
	if err := validateDate(params.StartDate, "start date"); err != nil {
		return MealPlanPage{}, fmt.Errorf("get mealie meal plan: %w", err)
	}
	if err := validateDate(params.EndDate, "end date"); err != nil {
		return MealPlanPage{}, fmt.Errorf("get mealie meal plan: %w", err)
	}
	if params.StartDate != "" && params.EndDate != "" && params.StartDate > params.EndDate {
		return MealPlanPage{}, fmt.Errorf("get mealie meal plan: start date must not be after end date")
	}
	if params.Page < 0 {
		return MealPlanPage{}, fmt.Errorf("get mealie meal plan: page must not be negative")
	}
	if params.PerPage < 0 {
		return MealPlanPage{}, fmt.Errorf("get mealie meal plan: per-page must not be negative")
	}

	query := url.Values{}
	if params.StartDate != "" {
		query.Set("start_date", string(params.StartDate))
	}
	if params.EndDate != "" {
		query.Set("end_date", string(params.EndDate))
	}
	if params.Page > 0 {
		query.Set("page", strconv.Itoa(params.Page))
	}
	if params.PerPage > 0 {
		query.Set("perPage", strconv.Itoa(params.PerPage))
	}

	var page MealPlanPage
	if err := c.getJSON(ctx, "/api/households/mealplans", query, &page); err != nil {
		return MealPlanPage{}, fmt.Errorf("get mealie meal plan: %w", err)
	}
	return page, nil
}

// CreateMealPlanEntry adds one recipe-backed or simple meal to Mealie's
// household meal plan.
func (c *Client) CreateMealPlanEntry(ctx context.Context, params CreateMealPlanEntryParams) (MealPlanEntry, error) {
	if err := validateDate(params.Date, "date"); err != nil {
		return MealPlanEntry{}, fmt.Errorf("create mealie meal-plan entry: %w", err)
	}
	if params.Date == "" {
		return MealPlanEntry{}, fmt.Errorf("create mealie meal-plan entry: date is required")
	}
	if !params.EntryType.Valid() {
		return MealPlanEntry{}, fmt.Errorf("create mealie meal-plan entry: unsupported entry type %q", params.EntryType)
	}

	payload := struct {
		Date      Date          `json:"date"`
		EntryType PlanEntryType `json:"entryType"`
		Title     string        `json:"title"`
		Text      string        `json:"text"`
		RecipeID  *string       `json:"recipeId"`
	}{
		Date:      params.Date,
		EntryType: params.EntryType,
		Title:     strings.TrimSpace(params.Title),
		Text:      strings.TrimSpace(params.Text),
		RecipeID:  params.RecipeID,
	}

	var entry MealPlanEntry
	if err := c.postJSON(ctx, "/api/households/mealplans", payload, &entry); err != nil {
		return MealPlanEntry{}, fmt.Errorf("create mealie meal-plan entry: %w", err)
	}
	return entry, nil
}
