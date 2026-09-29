package mealie_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/joeysaladino/homelab-mcp/internal/server"
	mealietools "github.com/joeysaladino/homelab-mcp/internal/tools/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGetMealPlanToolOverMCP(t *testing.T) {
	recipeID := "recipe-id"
	reader := &fakeRecipeSearcher{
		mealPlan: mealieapi.MealPlanPage{
			Page:       2,
			PerPage:    2,
			Total:      5,
			TotalPages: 3,
			Items: []mealieapi.MealPlanEntry{
				{
					ID:        123,
					Date:      mealieapi.Date("2026-09-28"),
					EntryType: "dinner",
					RecipeID:  &recipeID,
					Recipe: &mealieapi.RecipeSummary{
						ID:          recipeID,
						Name:        "Chicken Tikka",
						Slug:        "chicken-tikka",
						Description: "A weeknight recipe",
						OrgURL:      "https://recipes.example/chicken-tikka",
					},
				},
				{
					ID:        124,
					Date:      mealieapi.Date("2026-09-29"),
					EntryType: "dinner",
					Title:     "Steak Night",
					Text:      "NY Strip with roasted vegetables",
				},
			},
		},
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(reader, reader, reader, reader, reader)

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
		Name: "mealie.get_meal_plan",
		Arguments: map[string]any{
			"start_date": " 2026-09-28 ",
			"end_date":   "2026-10-02",
			"limit":      2,
			"page":       2,
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", result.Content)
	}

	var output mealietools.GetMealPlanOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}

	if output.StartDate != "2026-09-28" || output.EndDate != "2026-10-02" || output.Page != 2 || output.PageSize != 2 || output.Total != 5 || output.TotalPages != 3 || !output.HasMore || output.NextPage == nil || *output.NextPage != 3 {
		t.Fatalf("metadata = %+v, want normalized range and next page", output)
	}
	if len(output.Entries) != 2 {
		t.Fatalf("entries length = %d, want 2", len(output.Entries))
	}
	if output.Entries[0].Recipe == nil || output.Entries[0].Recipe.Name != "Chicken Tikka" || output.Entries[0].RecipeID != recipeID {
		t.Errorf("recipe-backed entry = %+v, want recipe summary and ID", output.Entries[0])
	}
	if output.Entries[1].Title != "Steak Night" || output.Entries[1].Text == "" || output.Entries[1].Recipe != nil || output.Entries[1].RecipeID != "" {
		t.Errorf("simple entry = %+v, want title/text and no recipe", output.Entries[1])
	}
	if !reader.mealPlanCalled || reader.mealPlanParams.StartDate != mealieapi.Date("2026-09-28") || reader.mealPlanParams.EndDate != mealieapi.Date("2026-10-02") || reader.mealPlanParams.Page != 2 || reader.mealPlanParams.PerPage != 2 {
		t.Errorf("meal plan params = %+v, want normalized dates/page/per-page", reader.mealPlanParams)
	}
}

func TestGetMealPlanToolValidation(t *testing.T) {
	reader := &fakeRecipeSearcher{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(reader, reader, reader, reader, reader)

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
		{name: "missing start date", args: map[string]any{"start_date": "", "end_date": "2026-10-02"}, want: "invalid start_date"},
		{name: "invalid end date", args: map[string]any{"start_date": "2026-09-28", "end_date": "tomorrow"}, want: "invalid end_date"},
		{name: "reversed range", args: map[string]any{"start_date": "2026-10-02", "end_date": "2026-09-28"}, want: "start_date must not be after end_date"},
		{name: "oversized limit", args: map[string]any{"start_date": "2026-09-28", "end_date": "2026-10-02", "limit": 51}, want: "limit must be between 1 and 50"},
		{name: "negative page", args: map[string]any{"start_date": "2026-09-28", "end_date": "2026-10-02", "page": -1}, want: "page must be at least 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "mealie.get_meal_plan",
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
	if reader.mealPlanCalled {
		t.Fatal("reader should not be called for invalid input")
	}
}
