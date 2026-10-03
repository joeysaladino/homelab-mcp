package shopping_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/joeysaladino/homelab-mcp/internal/server"
	shoppingdomain "github.com/joeysaladino/homelab-mcp/internal/shopping"
	shoppingtools "github.com/joeysaladino/homelab-mcp/internal/tools/shopping"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestApplyShoppingListsToolPreviewsWithoutWriting(t *testing.T) {
	applier := &fakeShoppingListApplier{}
	ctx, session := connectShoppingSession(t, &fakeDraftBuilder{}, applier)

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "mealie.apply_shopping_lists",
		Arguments: map[string]any{
			"confirm": false,
			"trips": []any{map[string]any{
				"name": "  Run 1  ",
				"items": []any{map[string]any{
					"display": "  chicken — 1 lb  ",
				}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", result.Content)
	}

	var output shoppingtools.ApplyShoppingListsOutput
	decodeStructuredOutput(t, result, &output)
	if output.Applied || !output.ConfirmationRequired || output.TotalItems != 1 {
		t.Fatalf("output = %+v, want no-write preview with one item", output)
	}
	if len(output.Lists) != 1 || output.Lists[0].Name != "Run 1" || output.Lists[0].ID != "" || output.Lists[0].ItemCount != 1 {
		t.Fatalf("preview list = %+v, want normalized list without ID", output.Lists)
	}
	if len(output.Lists[0].Items) != 1 || output.Lists[0].Items[0].Display != "chicken — 1 lb" || output.Lists[0].Items[0].Note != "chicken — 1 lb" {
		t.Fatalf("preview items = %+v, want normalized display and default note", output.Lists[0].Items)
	}
	if applier.called {
		t.Fatal("applier was called for confirm=false preview")
	}
}

func TestApplyShoppingListsToolCreatesAfterConfirmation(t *testing.T) {
	applier := &fakeShoppingListApplier{
		result: shoppingdomain.ApplyShoppingListsResult{
			Lists:      []shoppingdomain.AppliedShoppingList{{Name: "Run 1", ID: "list-1", ItemCount: 1}},
			TotalItems: 1,
		},
	}
	ctx, session := connectShoppingSession(t, &fakeDraftBuilder{}, applier)

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "mealie.apply_shopping_lists",
		Arguments: map[string]any{
			"confirm": true,
			"trips": []any{map[string]any{
				"name": "Run 1",
				"items": []any{map[string]any{
					"display": "ground turkey — ~1.5 lb",
					"note":    "meat counter",
				}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %+v", result.Content)
	}

	var output shoppingtools.ApplyShoppingListsOutput
	decodeStructuredOutput(t, result, &output)
	if !output.Applied || output.ConfirmationRequired || output.TotalItems != 1 {
		t.Fatalf("output = %+v, want applied result", output)
	}
	if len(output.Lists) != 1 || output.Lists[0].ID != "list-1" || output.Lists[0].ItemCount != 1 {
		t.Fatalf("applied lists = %+v, want created list identity", output.Lists)
	}
	if !applier.called || len(applier.draft.Trips) != 1 || applier.draft.Trips[0].Items[0].Note != "meat counter" {
		t.Fatalf("applier draft = %+v, want normalized confirmed input", applier.draft)
	}
}

func TestApplyShoppingListsToolValidationAndError(t *testing.T) {
	tests := []struct {
		name      string
		arguments map[string]any
		applier   *fakeShoppingListApplier
		want      string
	}{
		{
			name: "blank trip name",
			arguments: map[string]any{
				"confirm": false,
				"trips": []any{map[string]any{
					"name":  " ",
					"items": []any{map[string]any{"display": "milk"}},
				}},
			},
			applier: &fakeShoppingListApplier{},
			want:    "trip 0: name is required",
		},
		{
			name: "applier error",
			arguments: map[string]any{
				"confirm": true,
				"trips": []any{map[string]any{
					"name":  "Run 1",
					"items": []any{map[string]any{"display": "milk"}},
				}},
			},
			applier: &fakeShoppingListApplier{err: errors.New("write failed")},
			want:    "write failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, session := connectShoppingSession(t, &fakeDraftBuilder{}, tt.applier)
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "mealie.apply_shopping_lists",
				Arguments: tt.arguments,
			})
			if err != nil {
				t.Fatalf("CallTool() error = %v", err)
			}
			if !result.IsError || len(result.Content) == 0 || !strings.Contains(contentText(t, result.Content[0]), tt.want) {
				var content string
				if len(result.Content) > 0 {
					content = contentText(t, result.Content[0])
				}
				t.Fatalf("result = %+v, content = %q, want tool error containing %q", result, content, tt.want)
			}
			if tt.name == "blank trip name" && tt.applier.called {
				t.Fatal("applier was called for invalid input")
			}
		})
	}
}

type fakeShoppingListApplier struct {
	draft  shoppingdomain.ShoppingListDraft
	result shoppingdomain.ApplyShoppingListsResult
	called bool
	err    error
}

func (f *fakeShoppingListApplier) ApplyShoppingLists(_ context.Context, draft shoppingdomain.ShoppingListDraft) (shoppingdomain.ApplyShoppingListsResult, error) {
	f.called = true
	f.draft = draft
	return f.result, f.err
}

func connectShoppingSession(t *testing.T, builder *fakeDraftBuilder, applier *fakeShoppingListApplier) (context.Context, *mcp.ClientSession) {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	mcpServer := server.New(shoppingtools.NewModule(builder, applier))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() {
		_ = mcpServer.Run(ctx, serverTransport)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return ctx, session
}

func decodeStructuredOutput(t *testing.T, result *mcp.CallToolResult, output any) {
	t.Helper()
	structured, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(structured, output); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
}
