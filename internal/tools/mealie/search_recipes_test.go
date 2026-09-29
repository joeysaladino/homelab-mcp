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

func TestSearchRecipesToolOverMCP(t *testing.T) {
	searcher := &fakeRecipeSearcher{
		page: mealieapi.RecipePage{
			Page:       2,
			PerPage:    2,
			Total:      10,
			TotalPages: 5,
			Items: []mealieapi.RecipeSummary{
				{ID: "one", Name: "Chicken Tikka", Slug: "chicken-tikka", Description: "Curry", OrgURL: "https://example.test/one"},
				{ID: "two", Name: "Chicken Soup", Slug: "chicken-soup"},
			},
		},
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(searcher, searcher, searcher, searcher, searcher)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- mcpServer.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "mealie.search_recipes",
		Arguments: map[string]any{
			"query": " chicken ",
			"limit": 2,
			"page":  2,
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", result.Content)
	}

	var output mealietools.SearchRecipesOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	if output.Query != "chicken" || output.Page != 2 || output.PageSize != 2 || output.Total != 10 || output.TotalPages != 5 || !output.HasMore || output.NextPage == nil || *output.NextPage != 3 || len(output.Recipes) != 2 {
		t.Fatalf("output = %+v, want page metadata and two recipes", output)
	}
	if output.Recipes[0].SourceURL != "https://example.test/one" {
		t.Errorf("SourceURL = %q, want source URL", output.Recipes[0].SourceURL)
	}

	if searcher.got.Query != "chicken" || searcher.got.Page != 2 || searcher.got.PerPage != 2 {
		t.Errorf("search params = %+v, want query=chicken page=2 per-page=2", searcher.got)
	}
}

func TestSearchRecipesToolValidation(t *testing.T) {
	searcher := &fakeRecipeSearcher{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(searcher, searcher, searcher, searcher, searcher)

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
		{name: "missing query", args: map[string]any{"query": "   "}, want: "query is required"},
		{name: "negative page", args: map[string]any{"query": "chicken", "page": -1}, want: "page must be at least 1"},
		{name: "oversized limit", args: map[string]any{"query": "chicken", "limit": 51}, want: "limit must be between 1 and 50"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "mealie.search_recipes",
				Arguments: tt.args,
			})
			if err != nil {
				t.Fatalf("CallTool() error = %v", err)
			}
			if !result.IsError {
				t.Fatal("CallTool() IsError = false, want true")
			}
			if len(result.Content) == 0 {
				t.Fatal("CallTool() returned no error content")
			}
			if !strings.Contains(contentText(t, result.Content[0]), tt.want) {
				t.Errorf("error content = %+v, want %q", result.Content[0], tt.want)
			}
		})
	}
	if searcher.called {
		t.Fatal("searcher was called for invalid input")
	}
}

type fakeRecipeSearcher struct {
	page                mealieapi.RecipePage
	recipe              mealieapi.Recipe
	mealPlan            mealieapi.MealPlanPage
	mealPlanParams      mealieapi.MealPlanQuery
	mealPlanEntry       mealieapi.MealPlanEntry
	mealPlanWrite       mealieapi.CreateMealPlanEntryParams
	got                 mealieapi.RecipeSearchParams
	importParams        mealieapi.ImportRecipeParams
	called              bool
	importCalled        bool
	mealPlanCalled      bool
	mealPlanWriteCalled bool
	err                 error
}

func (f *fakeRecipeSearcher) SearchRecipes(_ context.Context, params mealieapi.RecipeSearchParams) (mealieapi.RecipePage, error) {
	f.called = true
	f.got = params
	return f.page, f.err
}

func (f *fakeRecipeSearcher) GetRecipe(_ context.Context, _ string) (mealieapi.Recipe, error) {
	return f.recipe, f.err
}

func (f *fakeRecipeSearcher) ImportRecipeURL(_ context.Context, params mealieapi.ImportRecipeParams) (string, error) {
	f.importCalled = true
	f.importParams = params
	return "imported-recipe", f.err
}

func (f *fakeRecipeSearcher) GetMealPlan(_ context.Context, params mealieapi.MealPlanQuery) (mealieapi.MealPlanPage, error) {
	f.mealPlanCalled = true
	f.mealPlanParams = params
	return f.mealPlan, f.err
}

func (f *fakeRecipeSearcher) CreateMealPlanEntry(_ context.Context, params mealieapi.CreateMealPlanEntryParams) (mealieapi.MealPlanEntry, error) {
	f.mealPlanWriteCalled = true
	f.mealPlanWrite = params
	return f.mealPlanEntry, f.err
}

func contentText(t *testing.T, content mcp.Content) string {
	t.Helper()

	textContent, ok := content.(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want *mcp.TextContent", content)
	}
	return textContent.Text
}
