package mealie_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeysaladino/homelab-mcp/internal/server"
	mealietools "github.com/joeysaladino/homelab-mcp/internal/tools/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestImportRecipeURLToolOverMCP(t *testing.T) {
	importer := &fakeRecipeSearcher{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(importer, importer, importer, importer)

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
		Name: "mealie.import_recipe_url",
		Arguments: map[string]any{
			"url":                " https://www.wholesomeyum.com/recipes/big-mac-salad-cheeseburger-salad-low-carb-gluten-free/ ",
			"include_tags":       true,
			"include_categories": true,
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", result.Content)
	}

	var output mealietools.ImportRecipeURLOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	if !output.Created || output.MealieResult != "imported-recipe" {
		t.Errorf("output = %+v, want created=true and Mealie result", output)
	}
	if output.URL != "https://www.wholesomeyum.com/recipes/big-mac-salad-cheeseburger-salad-low-carb-gluten-free/" {
		t.Errorf("URL = %q, want trimmed source URL", output.URL)
	}
	if !importer.importCalled {
		t.Fatal("importer was not called")
	}
	if importer.importParams.URL != output.URL || !importer.importParams.IncludeTags || !importer.importParams.IncludeCategories {
		t.Errorf("import params = %+v, want trimmed URL and both options enabled", importer.importParams)
	}
}

func TestImportRecipeURLToolValidation(t *testing.T) {
	importer := &fakeRecipeSearcher{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(importer, importer, importer, importer)

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
		Name:      "mealie.import_recipe_url",
		Arguments: map[string]any{"url": "   "},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want true")
	}
	if len(result.Content) == 0 || !strings.Contains(contentText(t, result.Content[0]), "url is required") {
		t.Fatalf("error content = %+v, want required URL message", result.Content)
	}
	if importer.importCalled {
		t.Fatal("importer should not be called for invalid input")
	}
}
