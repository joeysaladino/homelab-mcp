package shopping

import (
	"context"
	"errors"
	"strings"
	"testing"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/joeysaladino/homelab-mcp/internal/pantry"
)

func TestBuildShoppingDraft(t *testing.T) {
	quantity := 1.0
	reader := &fakeDraftReader{
		mealPlanPages: map[int]mealieapi.MealPlanPage{
			1: {
				Page:       1,
				PerPage:    50,
				Total:      3,
				TotalPages: 2,
				Items: []mealieapi.MealPlanEntry{
					{
						Date:      mealieapi.Date("2026-10-05"),
						EntryType: mealieapi.PlanEntryDinner,
						Title:     "Cumin chicken",
						RecipeID:  stringPointer("recipe-one"),
					},
					{
						Date:      mealieapi.Date("2026-10-07"),
						EntryType: mealieapi.PlanEntryDinner,
						RecipeID:  stringPointer("recipe-two"),
					},
				},
			},
			2: {
				Page:       2,
				PerPage:    50,
				Total:      3,
				TotalPages: 2,
				Items: []mealieapi.MealPlanEntry{
					{
						Date:      mealieapi.Date("2026-10-10"),
						EntryType: mealieapi.PlanEntryBreakfast,
						Title:     "Sausage gravy and biscuits",
						Text:      "Simple breakfast",
					},
				},
			},
		},
		recipes: map[string]mealieapi.Recipe{
			"recipe-one": {
				ID:   "recipe-one",
				Name: "Cumin Chicken",
				Slug: "cumin-chicken",
				Ingredients: []mealieapi.RecipeIngredient{
					{Display: "1 teaspoon cumin", Quantity: &quantity},
					{Note: stringPointer("1 lime, juiced")},
				},
			},
			"recipe-two": {
				ID:   "recipe-two",
				Name: "Rice Bowls",
				Slug: "rice-bowls",
				Ingredients: []mealieapi.RecipeIngredient{
					{Food: &mealieapi.IngredientFood{Name: "rice"}, Display: "2 cups rice"},
				},
			},
		},
	}
	planner := NewPlanner(
		reader,
		reader,
		pantry.Pantry{Items: []pantry.Item{{Name: "cumin", State: pantry.StateHave}}},
		Plan{Trips: []Trip{
			{Name: "Run 1", Weekdays: []Weekday{Monday, Tuesday}},
			{Name: "Run 2", Weekdays: []Weekday{Wednesday, Thursday, Friday}},
		}},
	)

	draft, err := planner.BuildShoppingDraft(context.Background(), DraftParams{
		StartDate: mealieapi.Date("2026-10-05"),
		EndDate:   mealieapi.Date("2026-10-11"),
	})
	if err != nil {
		t.Fatalf("BuildShoppingDraft() error = %v", err)
	}
	if draft.StartDate != mealieapi.Date("2026-10-05") || draft.EndDate != mealieapi.Date("2026-10-11") || len(draft.Trips) != 2 || len(draft.Pantry.Items) != 1 {
		t.Fatalf("draft metadata = %+v, want range, two trips, and pantry context", draft)
	}
	if len(draft.Trips[0].Meals) != 1 || draft.Trips[0].Meals[0].RecipeName != "Cumin Chicken" {
		t.Fatalf("run 1 meals = %+v, want Cumin Chicken", draft.Trips[0].Meals)
	}
	if len(draft.Trips[0].Meals[0].Ingredients) != 2 || draft.Trips[0].Meals[0].Ingredients[1].Text != "1 lime, juiced" {
		t.Fatalf("run 1 ingredients = %+v, want display/note fallback", draft.Trips[0].Meals[0].Ingredients)
	}
	if len(draft.Trips[1].Meals) != 1 || draft.Trips[1].Meals[0].Ingredients[0].Food != "rice" {
		t.Fatalf("run 2 meals = %+v, want structured rice source", draft.Trips[1].Meals)
	}
	if len(draft.UnassignedMeals) != 1 || draft.UnassignedMeals[0].Date != mealieapi.Date("2026-10-10") || draft.UnassignedMeals[0].RecipeID != "" {
		t.Fatalf("unassigned meals = %+v, want Saturday simple meal", draft.UnassignedMeals)
	}
	if len(reader.mealPlanCalls) != 2 || len(reader.recipeCalls) != 2 {
		t.Errorf("reader calls = %d meal-plan pages/%d recipes, want 2/2", len(reader.mealPlanCalls), len(reader.recipeCalls))
	}
}

func TestBuildShoppingDraftValidation(t *testing.T) {
	reader := &fakeDraftReader{}
	planner := NewPlanner(reader, reader, pantry.Pantry{}, Plan{Trips: []Trip{{Name: "Run 1", Weekdays: []Weekday{Monday}}}})
	tests := []struct {
		name   string
		params DraftParams
		want   string
	}{
		{name: "missing start", params: DraftParams{EndDate: mealieapi.Date("2026-10-11")}, want: "invalid start date"},
		{name: "missing end", params: DraftParams{StartDate: mealieapi.Date("2026-10-05")}, want: "invalid end date"},
		{name: "reversed", params: DraftParams{StartDate: mealieapi.Date("2026-10-11"), EndDate: mealieapi.Date("2026-10-05")}, want: "start date must not be after end date"},
		{name: "too long", params: DraftParams{StartDate: mealieapi.Date("2026-10-01"), EndDate: mealieapi.Date("2026-11-01")}, want: "cannot exceed 31 days"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := planner.BuildShoppingDraft(context.Background(), tt.params)
			if err == nil {
				t.Fatal("BuildShoppingDraft() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
	if len(reader.mealPlanCalls) != 0 {
		t.Fatal("meal-plan reader called for invalid draft parameters")
	}
}

func TestBuildShoppingDraftRecipeError(t *testing.T) {
	reader := &fakeDraftReader{
		mealPlanPages: map[int]mealieapi.MealPlanPage{
			1: {
				Page:       1,
				TotalPages: 1,
				Items: []mealieapi.MealPlanEntry{{
					Date:     mealieapi.Date("2026-10-05"),
					RecipeID: stringPointer("missing-recipe"),
				}},
			},
		},
		recipeErr: errors.New("recipe unavailable"),
	}
	planner := NewPlanner(reader, reader, pantry.Pantry{}, Plan{Trips: []Trip{{Name: "Run 1", Weekdays: []Weekday{Monday}}}})

	_, err := planner.BuildShoppingDraft(context.Background(), DraftParams{
		StartDate: mealieapi.Date("2026-10-05"),
		EndDate:   mealieapi.Date("2026-10-05"),
	})
	if err == nil || !strings.Contains(err.Error(), "read recipe") || !strings.Contains(err.Error(), "recipe unavailable") {
		t.Fatalf("BuildShoppingDraft() error = %v, want recipe context", err)
	}
}

type fakeDraftReader struct {
	mealPlanPages map[int]mealieapi.MealPlanPage
	recipes       map[string]mealieapi.Recipe
	mealPlanCalls []mealieapi.MealPlanQuery
	recipeCalls   []string
	recipeErr     error
}

func (f *fakeDraftReader) GetMealPlan(_ context.Context, params mealieapi.MealPlanQuery) (mealieapi.MealPlanPage, error) {
	f.mealPlanCalls = append(f.mealPlanCalls, params)
	return f.mealPlanPages[params.Page], nil
}

func (f *fakeDraftReader) GetRecipe(_ context.Context, identifier string) (mealieapi.Recipe, error) {
	f.recipeCalls = append(f.recipeCalls, identifier)
	if f.recipeErr != nil {
		return mealieapi.Recipe{}, f.recipeErr
	}
	return f.recipes[identifier], nil
}

func stringPointer(value string) *string {
	return &value
}
