//go:build integration

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	mealietools "github.com/joeysaladino/homelab-mcp/internal/tools/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const liveImportURL = "https://www.wholesomeyum.com/recipes/big-mac-salad-cheeseburger-salad-low-carb-gluten-free/"

func TestStdioServerRecipeTools(t *testing.T) {
	if os.Getenv("MEALIE_URL") == "" || os.Getenv("MEALIE_TOKEN") == "" {
		t.Skip("MEALIE_URL and MEALIE_TOKEN are required for the integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() could not locate the integration test")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "../.."))

	command := exec.Command("go", "run", "./cmd/homelab-mcp")
	command.Dir = repoRoot
	command.Stderr = os.Stderr

	transport := &mcp.CommandTransport{Command: command}
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "homelab-mcp-integration-test",
		Version: "0.1.0",
	}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect to stdio server: %v", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Logf("close MCP session: %v", err)
		}
	}()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "mealie.search_recipes",
		Arguments: map[string]any{
			"query": "chicken",
			"limit": 2,
			"page":  2,
		},
	})
	if err != nil {
		t.Fatalf("call mealie.search_recipes: %v", err)
	}
	if result.IsError {
		t.Fatalf("mealie.search_recipes returned a tool error: %+v", result.Content)
	}

	var output mealietools.SearchRecipesOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured result: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("decode structured result: %v", err)
	}

	if output.Query != "chicken" {
		t.Errorf("query = %q, want %q", output.Query, "chicken")
	}
	if output.Page != 2 {
		t.Errorf("page = %d, want 2", output.Page)
	}
	if output.PageSize != 2 {
		t.Errorf("page size = %d, want 2", output.PageSize)
	}
	if output.Total < len(output.Recipes) || output.Total == 0 {
		t.Errorf("total = %d, recipes returned = %d; want a non-empty valid result", output.Total, len(output.Recipes))
	}
	if len(output.Recipes) == 0 {
		t.Fatal("recipes = [], want at least one live Mealie result")
	}
	if output.Recipes[0].Slug == "" {
		t.Fatal("first recipe slug is empty, cannot exercise detail lookup")
	}

	detailResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "mealie.get_recipe",
		Arguments: map[string]any{
			"recipe_id_or_slug": output.Recipes[0].Slug,
		},
	})
	if err != nil {
		t.Fatalf("call mealie.get_recipe: %v", err)
	}
	if detailResult.IsError {
		t.Fatalf("mealie.get_recipe returned a tool error: %+v", detailResult.Content)
	}

	var detail mealietools.GetRecipeOutput
	detailStructured, err := json.Marshal(detailResult.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured recipe detail: %v", err)
	}
	if err := json.Unmarshal(detailStructured, &detail); err != nil {
		t.Fatalf("decode structured recipe detail: %v", err)
	}
	if detail.Slug != output.Recipes[0].Slug {
		t.Errorf("detail slug = %q, want %q", detail.Slug, output.Recipes[0].Slug)
	}
	if detail.Name == "" {
		t.Error("detail name is empty, want a live recipe name")
	}
	if len(detail.Ingredients) == 0 {
		t.Error("detail ingredients are empty, want live recipe ingredients")
	}
}

func TestStdioServerMealPlan(t *testing.T) {
	if os.Getenv("MEALIE_URL") == "" || os.Getenv("MEALIE_TOKEN") == "" {
		t.Skip("MEALIE_URL and MEALIE_TOKEN are required for the integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() could not locate the integration test")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "../.."))

	command := exec.Command("go", "run", "./cmd/homelab-mcp")
	command.Dir = repoRoot
	command.Stderr = os.Stderr

	transport := &mcp.CommandTransport{Command: command}
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "homelab-mcp-meal-plan-integration-test",
		Version: "0.1.0",
	}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect to stdio server: %v", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Logf("close MCP session: %v", err)
		}
	}()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "mealie.get_meal_plan",
		Arguments: map[string]any{
			"start_date": "2026-09-28",
			"end_date":   "2026-10-02",
		},
	})
	if err != nil {
		t.Fatalf("call mealie.get_meal_plan: %v", err)
	}
	if result.IsError {
		t.Fatalf("mealie.get_meal_plan returned a tool error: %+v", result.Content)
	}

	var output mealietools.GetMealPlanOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal meal-plan result: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("decode meal-plan result: %v", err)
	}
	if output.StartDate != "2026-09-28" || output.EndDate != "2026-10-02" {
		t.Errorf("date range = %s through %s, want 2026-09-28 through 2026-10-02", output.StartDate, output.EndDate)
	}
	if output.Total == 0 || len(output.Entries) == 0 {
		t.Fatalf("meal plan = total %d, entries %d; want live entries", output.Total, len(output.Entries))
	}

	hasRecipeBackedEntry := false
	hasSimpleEntry := false
	for _, entry := range output.Entries {
		if entry.Recipe != nil {
			hasRecipeBackedEntry = true
		}
		if entry.Recipe == nil && (entry.Title != "" || entry.Text != "") {
			hasSimpleEntry = true
		}
	}
	if !hasRecipeBackedEntry {
		t.Error("meal plan has no recipe-backed entry")
	}
	if !hasSimpleEntry {
		t.Error("meal plan has no simple title/text entry")
	}
}

func TestStdioServerCreateMealPlanEntry(t *testing.T) {
	if os.Getenv("MEALIE_URL") == "" || os.Getenv("MEALIE_TOKEN") == "" {
		t.Skip("MEALIE_URL and MEALIE_TOKEN are required for the integration test")
	}
	date := strings.TrimSpace(os.Getenv("HOMELAB_MCP_LIVE_PLAN_DATE"))
	entryType := strings.TrimSpace(os.Getenv("HOMELAB_MCP_LIVE_PLAN_ENTRY_TYPE"))
	title := strings.TrimSpace(os.Getenv("HOMELAB_MCP_LIVE_PLAN_TITLE"))
	text := strings.TrimSpace(os.Getenv("HOMELAB_MCP_LIVE_PLAN_TEXT"))
	recipeID := strings.TrimSpace(os.Getenv("HOMELAB_MCP_LIVE_PLAN_RECIPE_ID"))
	if date == "" || entryType == "" || (title == "" && text == "" && recipeID == "") {
		t.Skip("set HOMELAB_MCP_LIVE_PLAN_DATE, HOMELAB_MCP_LIVE_PLAN_ENTRY_TYPE, and meal content to run the live write test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() could not locate the integration test")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "../.."))

	command := exec.Command("go", "run", "./cmd/homelab-mcp")
	command.Dir = repoRoot
	command.Stderr = os.Stderr

	transport := &mcp.CommandTransport{Command: command}
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "homelab-mcp-plan-write-integration-test",
		Version: "0.1.0",
	}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect to stdio server: %v", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Logf("close MCP session: %v", err)
		}
	}()

	existingResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "mealie.get_meal_plan",
		Arguments: map[string]any{
			"start_date": date,
			"end_date":   date,
		},
	})
	if err != nil {
		t.Fatalf("check existing meal-plan entries: %v", err)
	}
	if existingResult.IsError {
		t.Fatalf("existing meal-plan check returned a tool error: %+v", existingResult.Content)
	}
	var existing mealietools.GetMealPlanOutput
	existingStructured, err := json.Marshal(existingResult.StructuredContent)
	if err != nil {
		t.Fatalf("marshal existing meal-plan result: %v", err)
	}
	if err := json.Unmarshal(existingStructured, &existing); err != nil {
		t.Fatalf("decode existing meal-plan result: %v", err)
	}
	for _, entry := range existing.Entries {
		if entry.EntryType != entryType || (recipeID != "" && entry.RecipeID != recipeID) || (title != "" && entry.Title != title) || (text != "" && entry.Text != text) {
			continue
		}
		t.Skipf("matching meal-plan entry already exists with ID %d; skipping duplicate write", entry.ID)
	}

	arguments := map[string]any{
		"date":       date,
		"entry_type": entryType,
	}
	if title != "" {
		arguments["title"] = title
	}
	if text != "" {
		arguments["text"] = text
	}
	if recipeID != "" {
		arguments["recipe_id"] = recipeID
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "mealie.create_meal_plan_entry",
		Arguments: arguments,
	})
	if err != nil {
		t.Fatalf("call mealie.create_meal_plan_entry: %v", err)
	}
	if result.IsError {
		t.Fatalf("mealie.create_meal_plan_entry returned a tool error: %+v", result.Content)
	}
	var output mealietools.CreateMealPlanEntryOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal created meal-plan result: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("decode created meal-plan result: %v", err)
	}
	if !output.Created || output.Entry.Date != date || output.Entry.EntryType != entryType {
		t.Fatalf("created entry = %+v, want created %s %s entry", output.Entry, date, entryType)
	}
	if recipeID != "" && output.Entry.RecipeID != recipeID {
		t.Errorf("created recipe ID = %q, want %q", output.Entry.RecipeID, recipeID)
	}
}

func TestStdioServerImportRecipeURL(t *testing.T) {
	if os.Getenv("MEALIE_URL") == "" || os.Getenv("MEALIE_TOKEN") == "" {
		t.Skip("MEALIE_URL and MEALIE_TOKEN are required for the integration test")
	}
	if os.Getenv("HOMELAB_MCP_LIVE_IMPORT") != "1" {
		t.Skip("set HOMELAB_MCP_LIVE_IMPORT=1 to run the live write test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() could not locate the integration test")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "../.."))

	command := exec.Command("go", "run", "./cmd/homelab-mcp")
	command.Dir = repoRoot
	command.Stderr = os.Stderr

	transport := &mcp.CommandTransport{Command: command}
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "homelab-mcp-import-integration-test",
		Version: "0.1.0",
	}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect to stdio server: %v", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Logf("close MCP session: %v", err)
		}
	}()

	searchResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "mealie.search_recipes",
		Arguments: map[string]any{
			"query": "wholesomeyum.com",
			"limit": 50,
			"page":  1,
		},
	})
	if err != nil {
		t.Fatalf("check for existing imported recipe: %v", err)
	}
	if searchResult.IsError {
		t.Fatalf("existing recipe check returned a tool error: %+v", searchResult.Content)
	}

	var existing mealietools.SearchRecipesOutput
	searchStructured, err := json.Marshal(searchResult.StructuredContent)
	if err != nil {
		t.Fatalf("marshal existing recipe search: %v", err)
	}
	if err := json.Unmarshal(searchStructured, &existing); err != nil {
		t.Fatalf("decode existing recipe search: %v", err)
	}
	for _, recipe := range existing.Recipes {
		if recipe.SourceURL == liveImportURL {
			t.Skipf("recipe already exists in Mealie as %q; skipping duplicate import", recipe.Slug)
		}
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "mealie.import_recipe_url",
		Arguments: map[string]any{
			"url": liveImportURL,
		},
	})
	if err != nil {
		t.Fatalf("call mealie.import_recipe_url: %v", err)
	}
	if result.IsError {
		t.Fatalf("mealie.import_recipe_url returned a tool error: %+v", result.Content)
	}

	var output mealietools.ImportRecipeURLOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal import result: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("decode import result: %v", err)
	}
	if !output.Created {
		t.Fatal("import result created = false, want true")
	}
	if output.URL != liveImportURL {
		t.Errorf("import URL = %q, want supplied URL", output.URL)
	}
	if output.MealieResult == "" {
		t.Error("Mealie result is empty, want the API response string")
	}

	verifyResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "mealie.search_recipes",
		Arguments: map[string]any{
			"query": "big mac salad",
			"limit": 50,
			"page":  1,
		},
	})
	if err != nil {
		t.Fatalf("verify imported recipe: %v", err)
	}
	if verifyResult.IsError {
		t.Fatalf("import verification returned a tool error: %+v", verifyResult.Content)
	}
	var verified mealietools.SearchRecipesOutput
	verifyStructured, err := json.Marshal(verifyResult.StructuredContent)
	if err != nil {
		t.Fatalf("marshal import verification: %v", err)
	}
	if err := json.Unmarshal(verifyStructured, &verified); err != nil {
		t.Fatalf("decode import verification: %v", err)
	}
	found := false
	for _, recipe := range verified.Recipes {
		if recipe.SourceURL == liveImportURL {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("imported source URL was not found in follow-up search results")
	}
}
