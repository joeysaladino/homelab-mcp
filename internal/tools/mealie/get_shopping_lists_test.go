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

func TestGetShoppingListsToolOverMCP(t *testing.T) {
	lister := &fakeRecipeSearcher{
		shoppingLists: mealieapi.ShoppingListPage{
			Page:       2,
			PerPage:    2,
			Total:      5,
			TotalPages: 3,
			Items: []mealieapi.ShoppingListSummary{
				{ID: "list-one", Name: " Mon-Tue ", CreatedAt: "2026-09-28T12:00:00Z", UpdatedAt: "2026-09-29T12:00:00Z"},
				{ID: "list-two", Name: "Wed-Fri"},
			},
		},
	}

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(mealietools.NewModule(lister))

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
		Name: "mealie.get_shopping_lists",
		Arguments: map[string]any{
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

	var output mealietools.GetShoppingListsOutput
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(structured, &output); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	if output.Page != 2 || output.PageSize != 2 || output.Total != 5 || output.TotalPages != 3 || !output.HasMore || output.NextPage == nil || *output.NextPage != 3 {
		t.Fatalf("metadata = %+v, want page metadata and next page", output)
	}
	if len(output.Lists) != 2 || output.Lists[0].ID != "list-one" || output.Lists[0].Name != "Mon-Tue" {
		t.Fatalf("lists = %+v, want two normalized summaries", output.Lists)
	}
	if !lister.shoppingListsCalled || lister.shoppingListQuery.Page != 2 || lister.shoppingListQuery.PerPage != 2 {
		t.Errorf("shopping-list query = %+v, want page 2/per-page 2", lister.shoppingListQuery)
	}
}

func TestGetShoppingListsToolValidation(t *testing.T) {
	lister := &fakeRecipeSearcher{}
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(mealietools.NewModule(lister))

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
		{name: "oversized limit", args: map[string]any{"limit": 51}, want: "limit must be between 1 and 50"},
		{name: "negative page", args: map[string]any{"page": -1}, want: "page must be at least 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "mealie.get_shopping_lists",
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
	if lister.shoppingListsCalled {
		t.Fatal("lister should not be called for invalid input")
	}
}
