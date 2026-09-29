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

func TestGetRecipeToolOverMCP(t *testing.T) {
	zero := 0.0
	two := 2.0
	getter := &fakeRecipeSearcher{
		recipe: mealieapi.Recipe{
			ID:          "recipe-id",
			Name:        "Chicken Tikka",
			Slug:        "chicken-tikka",
			Description: "A weeknight recipe",
			OrgURL:      "https://recipes.example/chicken-tikka",
			Ingredients: []mealieapi.RecipeIngredient{
				{Quantity: &zero, Note: stringPointer("1 teaspoon ground cumin"), Display: "1 teaspoon ground cumin"},
				{
					Quantity: &two,
					Unit:     &mealieapi.IngredientUnit{Name: "cup"},
					Food:     &mealieapi.IngredientFood{Name: "rice"},
					Display:  "2 cups rice",
				},
			},
			Instructions: []mealieapi.RecipeInstruction{{ID: "step-id", Text: "Cook the rice."}},
		},
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(getter, getter, getter)

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
		Name: "mealie.get_recipe",
		Arguments: map[string]any{
			"recipe_id_or_slug": " chicken-tikka ",
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", result.Content)
	}

	var output mealietools.GetRecipeOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}

	if output.ID != "recipe-id" || output.Name != "Chicken Tikka" || output.Slug != "chicken-tikka" {
		t.Fatalf("identity = %+v, want recipe-id/Chicken Tikka/chicken-tikka", output)
	}
	if len(output.Ingredients) != 2 {
		t.Fatalf("ingredients length = %d, want 2", len(output.Ingredients))
	}
	if output.Ingredients[0].Text != "1 teaspoon ground cumin" || output.Ingredients[0].Note != "1 teaspoon ground cumin" || output.Ingredients[0].Quantity != 0 {
		t.Errorf("unstructured ingredient = %+v, want human text and no parsed quantity", output.Ingredients[0])
	}
	if output.Ingredients[1].Text != "2 cups rice" || output.Ingredients[1].Quantity != 2 || output.Ingredients[1].Unit != "cup" || output.Ingredients[1].Food != "rice" {
		t.Errorf("structured ingredient = %+v, want text/quantity/unit/food", output.Ingredients[1])
	}
	if len(output.Instructions) != 1 || output.Instructions[0].Text != "Cook the rice." {
		t.Errorf("instructions = %+v, want one cooking step", output.Instructions)
	}
}

func TestGetRecipeToolValidation(t *testing.T) {
	getter := &fakeRecipeSearcher{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(getter, getter, getter)

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
		Name:      "mealie.get_recipe",
		Arguments: map[string]any{"recipe_id_or_slug": "   "},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("CallTool() IsError = false, want true")
	}
	if len(result.Content) == 0 || !strings.Contains(contentText(t, result.Content[0]), "recipe_id_or_slug is required") {
		t.Fatalf("error content = %+v, want required identifier message", result.Content)
	}
	if getter.called {
		t.Fatal("getter should not be called for invalid input")
	}
}

func stringPointer(value string) *string {
	return &value
}
