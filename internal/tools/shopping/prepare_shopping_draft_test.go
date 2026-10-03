package shopping_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/joeysaladino/homelab-mcp/internal/pantry"
	"github.com/joeysaladino/homelab-mcp/internal/server"
	shoppingdomain "github.com/joeysaladino/homelab-mcp/internal/shopping"
	shoppingtools "github.com/joeysaladino/homelab-mcp/internal/tools/shopping"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPrepareShoppingDraftToolOverMCP(t *testing.T) {
	quantity := 0.5
	builder := &fakeDraftBuilder{
		draft: shoppingdomain.ShoppingDraft{
			StartDate: mealieapi.Date("2026-10-05"),
			EndDate:   mealieapi.Date("2026-10-11"),
			Trips: []shoppingdomain.TripDraft{{
				Name:     "Run 1",
				Weekdays: []shoppingdomain.Weekday{shoppingdomain.Monday, shoppingdomain.Tuesday},
				Meals: []shoppingdomain.MealDraft{{
					Date:       mealieapi.Date("2026-10-05"),
					EntryType:  mealieapi.PlanEntryDinner,
					Title:      "Chicken dinner",
					RecipeID:   "recipe-id",
					RecipeName: "Chicken",
					Ingredients: []shoppingdomain.IngredientSource{{
						Text:     "1/2 tsp cumin",
						Quantity: &quantity,
						Unit:     "teaspoon",
					}},
				}},
			}},
			UnassignedMeals: []shoppingdomain.MealDraft{{
				Date:      mealieapi.Date("2026-10-10"),
				EntryType: mealieapi.PlanEntryBreakfast,
				Title:     "Simple breakfast",
			}},
			Pantry: pantry.Pantry{Items: []pantry.Item{{
				Name:  "cumin",
				State: pantry.StateHave,
			}}},
			Warnings: []string{"one source warning"},
		},
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(shoppingtools.NewModule(builder, builder))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = mcpServer.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "mealie.prepare_shopping_draft",
		Arguments: map[string]any{
			"start_date": " 2026-10-05 ",
			"end_date":   "2026-10-11",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", result.Content)
	}

	var output shoppingtools.PrepareShoppingDraftOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	if output.StartDate != "2026-10-05" || output.EndDate != "2026-10-11" || len(output.Trips) != 1 || len(output.UnassignedMeals) != 1 || len(output.Pantry) != 1 || len(output.Warnings) != 1 {
		t.Fatalf("output metadata = %+v, want range/trip/unassigned/pantry/warning", output)
	}
	if output.Trips[0].Name != "Run 1" || len(output.Trips[0].Weekdays) != 2 || len(output.Trips[0].Meals) != 1 {
		t.Fatalf("trip output = %+v, want configured meal grouping", output.Trips[0])
	}
	meal := output.Trips[0].Meals[0]
	if meal.RecipeName != "Chicken" || len(meal.Ingredients) != 1 || meal.Ingredients[0].Text != "1/2 tsp cumin" || meal.Ingredients[0].Quantity == nil || *meal.Ingredients[0].Quantity != 0.5 {
		t.Fatalf("meal output = %+v, want source ingredient details", meal)
	}
	if output.UnassignedMeals[0].Title != "Simple breakfast" || output.Pantry[0].State != "have" {
		t.Fatalf("unassigned/pantry output = %+v/%+v, want preserved context", output.UnassignedMeals[0], output.Pantry[0])
	}
	if !builder.called || builder.params.StartDate != mealieapi.Date("2026-10-05") || builder.params.EndDate != mealieapi.Date("2026-10-11") {
		t.Errorf("builder params = %+v, want normalized date range", builder.params)
	}
}

func TestPrepareShoppingDraftToolValidation(t *testing.T) {
	builder := &fakeDraftBuilder{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(shoppingtools.NewModule(builder, builder))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = mcpServer.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer session.Close()

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing start", args: map[string]any{"start_date": "", "end_date": "2026-10-11"}, want: "invalid start_date"},
		{name: "invalid end", args: map[string]any{"start_date": "2026-10-05", "end_date": "tomorrow"}, want: "invalid end_date"},
		{name: "reversed range", args: map[string]any{"start_date": "2026-10-11", "end_date": "2026-10-05"}, want: "start_date must not be after end_date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "mealie.prepare_shopping_draft",
				Arguments: tt.args,
			})
			if err != nil {
				t.Fatalf("CallTool() error = %v", err)
			}
			if !result.IsError {
				t.Fatal("CallTool() IsError = false, want true")
			}
			if len(result.Content) == 0 || !strings.Contains(contentText(t, result.Content[0]), tt.want) {
				t.Fatalf("error content = %+v, want substring %q", result.Content, tt.want)
			}
		})
	}
	if builder.called {
		t.Fatal("builder should not be called for invalid input")
	}
}

func TestPrepareShoppingDraftToolError(t *testing.T) {
	builder := &fakeDraftBuilder{err: errors.New("planner failed")}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(shoppingtools.NewModule(builder, builder))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = mcpServer.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "mealie.prepare_shopping_draft",
		Arguments: map[string]any{
			"start_date": "2026-10-05",
			"end_date":   "2026-10-11",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if !result.IsError || len(result.Content) == 0 || !strings.Contains(contentText(t, result.Content[0]), "planner failed") {
		t.Fatalf("result = %+v, want planner error", result)
	}
}

type fakeDraftBuilder struct {
	draft  shoppingdomain.ShoppingDraft
	params shoppingdomain.DraftParams
	called bool
	err    error
}

func (f *fakeDraftBuilder) BuildShoppingDraft(_ context.Context, params shoppingdomain.DraftParams) (shoppingdomain.ShoppingDraft, error) {
	f.called = true
	f.params = params
	return f.draft, f.err
}

func (f *fakeDraftBuilder) ApplyShoppingLists(_ context.Context, _ shoppingdomain.ShoppingListDraft) (shoppingdomain.ApplyShoppingListsResult, error) {
	return shoppingdomain.ApplyShoppingListsResult{}, nil
}

func contentText(t *testing.T, content mcp.Content) string {
	t.Helper()
	textContent, ok := content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want *mcp.TextContent", content)
	}
	return textContent.Text
}
