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

func TestCreateMealPlanEntryToolOverMCP(t *testing.T) {
	recipeID := "recipe-id"
	writer := &fakeRecipeSearcher{
		mealPlanEntry: mealieapi.MealPlanEntry{
			ID:        456,
			Date:      mealieapi.Date("2026-10-05"),
			EntryType: mealieapi.PlanEntryDinner,
			RecipeID:  &recipeID,
			Recipe: &mealieapi.RecipeSummary{
				ID:   recipeID,
				Name: "Chicken Tikka",
				Slug: "chicken-tikka",
			},
		},
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(writer, writer, writer, writer, writer)

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
		Name: "mealie.create_meal_plan_entry",
		Arguments: map[string]any{
			"date":       " 2026-10-05 ",
			"entry_type": "DINNER",
			"recipe_id":  " recipe-id ",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", result.Content)
	}

	var output mealietools.CreateMealPlanEntryOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	if !output.Created || output.Entry.ID != 456 || output.Entry.RecipeID != recipeID || output.Entry.Recipe == nil || output.Entry.Recipe.Name != "Chicken Tikka" {
		t.Errorf("output = %+v, want created recipe-backed entry", output)
	}
	if !writer.mealPlanWriteCalled || writer.mealPlanWrite.Date != mealieapi.Date("2026-10-05") || writer.mealPlanWrite.EntryType != mealieapi.PlanEntryDinner || writer.mealPlanWrite.RecipeID == nil || *writer.mealPlanWrite.RecipeID != recipeID {
		t.Errorf("write params = %+v, want normalized date/type/recipe ID", writer.mealPlanWrite)
	}
}

func TestCreateSimpleMealPlanEntryToolOverMCP(t *testing.T) {
	writer := &fakeRecipeSearcher{
		mealPlanEntry: mealieapi.MealPlanEntry{
			ID:        457,
			Date:      mealieapi.Date("2026-10-09"),
			EntryType: mealieapi.PlanEntryDinner,
			Title:     "Steak Night",
			Text:      "NY Strip with roasted vegetables",
		},
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(writer, writer, writer, writer, writer)

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
		Name: "mealie.create_meal_plan_entry",
		Arguments: map[string]any{
			"date":       "2026-10-09",
			"entry_type": "dinner",
			"title":      " Steak Night ",
			"text":       " NY Strip with roasted vegetables ",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", result.Content)
	}
	var output mealietools.CreateMealPlanEntryOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	if !output.Created || output.Entry.Title != "Steak Night" || output.Entry.Text == "" || output.Entry.Recipe != nil || output.Entry.RecipeID != "" {
		t.Errorf("output = %+v, want created simple meal", output)
	}
	if writer.mealPlanWrite.RecipeID != nil || writer.mealPlanWrite.Title != "Steak Night" || writer.mealPlanWrite.Text != "NY Strip with roasted vegetables" {
		t.Errorf("write params = %+v, want trimmed simple meal fields", writer.mealPlanWrite)
	}
}

func TestCreateMealPlanEntryToolValidation(t *testing.T) {
	writer := &fakeRecipeSearcher{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(writer, writer, writer, writer, writer)

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
		{name: "missing date", args: map[string]any{"date": "", "entry_type": "dinner", "title": "Dinner"}, want: "invalid date"},
		{name: "missing entry type", args: map[string]any{"date": "2026-10-05", "entry_type": "", "title": "Dinner"}, want: "entry_type is required"},
		{name: "unsupported entry type", args: map[string]any{"date": "2026-10-05", "entry_type": "supper", "title": "Dinner"}, want: "unsupported entry_type"},
		{name: "empty meal", args: map[string]any{"date": "2026-10-05", "entry_type": "dinner"}, want: "provide title, text, or recipe_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "mealie.create_meal_plan_entry",
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
	if writer.mealPlanWriteCalled {
		t.Fatal("writer should not be called for invalid input")
	}
}
