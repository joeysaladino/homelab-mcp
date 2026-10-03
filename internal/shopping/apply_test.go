package shopping

import (
	"context"
	"errors"
	"strings"
	"testing"

	mealieapi "github.com/joeysaladino/homelab-mcp/internal/mealie"
)

func TestNormalizeShoppingListDraft(t *testing.T) {
	tests := []struct {
		name    string
		input   ShoppingListDraft
		want    ShoppingListDraft
		wantErr string
	}{
		{
			name: "trims values and defaults note",
			input: ShoppingListDraft{Trips: []ShoppingTripInput{{
				Name: "  Run 1  ",
				Items: []ShoppingItemInput{
					{Display: "  chicken — 1 lb  "},
					{Display: " onions — 2 ", Note: "  produce section  "},
				},
			}}},
			want: ShoppingListDraft{Trips: []ShoppingTripInput{{
				Name: "Run 1",
				Items: []ShoppingItemInput{
					{Display: "chicken — 1 lb", Note: "chicken — 1 lb"},
					{Display: "onions — 2", Note: "produce section"},
				},
			}}},
		},
		{name: "requires a trip", input: ShoppingListDraft{}, wantErr: "at least one shopping trip"},
		{
			name: "rejects duplicate trip names",
			input: ShoppingListDraft{Trips: []ShoppingTripInput{
				{Name: "Run 1", Items: []ShoppingItemInput{{Display: "milk"}}},
				{Name: " run 1 ", Items: []ShoppingItemInput{{Display: "bread"}}},
			}},
			wantErr: "duplicate name",
		},
		{
			name:    "requires items",
			input:   ShoppingListDraft{Trips: []ShoppingTripInput{{Name: "Run 1"}}},
			wantErr: "at least one item",
		},
		{
			name:    "requires item display",
			input:   ShoppingListDraft{Trips: []ShoppingTripInput{{Name: "Run 1", Items: []ShoppingItemInput{{Display: " "}}}}},
			wantErr: "display is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeShoppingListDraft(tt.input)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NormalizeShoppingListDraft() error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeShoppingListDraft() error = %v", err)
			}
			if !equalShoppingListDraft(got, tt.want) {
				t.Fatalf("NormalizeShoppingListDraft() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestApplyShoppingLists(t *testing.T) {
	creator := &fakeShoppingListCreator{
		lists: []mealieapi.ShoppingList{
			{ID: "list-1", Name: "Run 1"},
			{ID: "list-2", Name: "Run 2"},
		},
	}
	writer := &fakeShoppingListItemWriter{
		createdCounts: []int{2, 1},
	}
	applier := NewApplier(creator, writer)

	got, err := applier.ApplyShoppingLists(context.Background(), ShoppingListDraft{Trips: []ShoppingTripInput{
		{Name: " Run 1 ", Items: []ShoppingItemInput{{Display: " chicken — 1 lb "}, {Display: "onion", Note: "produce"}}},
		{Name: "Run 2", Items: []ShoppingItemInput{{Display: "rice"}}},
	}})
	if err != nil {
		t.Fatalf("ApplyShoppingLists() error = %v", err)
	}
	if len(creator.params) != 2 || creator.params[0].Name != "Run 1" || creator.params[1].Name != "Run 2" {
		t.Fatalf("created list params = %+v, want normalized names", creator.params)
	}
	if len(writer.params) != 2 {
		t.Fatalf("item write calls = %d, want 2", len(writer.params))
	}
	wantItems := []mealieapi.CreateShoppingListItemParams{
		{ShoppingListID: "list-1", Display: "chicken — 1 lb", Note: "chicken — 1 lb", Position: 0},
		{ShoppingListID: "list-1", Display: "onion", Note: "produce", Position: 1},
	}
	if !equalShoppingItemParams(writer.params[0], wantItems) {
		t.Fatalf("first item write = %+v, want %+v", writer.params[0], wantItems)
	}
	if len(writer.params[1]) != 1 || writer.params[1][0].ShoppingListID != "list-2" {
		t.Fatalf("second item write = %+v, want list-2 item", writer.params[1])
	}
	if len(got.Lists) != 2 || got.Lists[0].ID != "list-1" || got.Lists[0].ItemCount != 2 || got.Lists[1].ItemCount != 1 || got.TotalItems != 3 {
		t.Fatalf("apply result = %+v, want two lists and three items", got)
	}
}

func TestApplyShoppingListsReturnsPartialResultOnLaterFailure(t *testing.T) {
	creator := &fakeShoppingListCreator{
		lists: []mealieapi.ShoppingList{{ID: "list-1", Name: "Run 1"}, {ID: "list-2", Name: "Run 2"}},
	}
	writer := &fakeShoppingListItemWriter{
		createdCounts: []int{1},
		errAtCall:     1,
		err:           errors.New("upstream unavailable"),
	}
	applier := NewApplier(creator, writer)

	got, err := applier.ApplyShoppingLists(context.Background(), ShoppingListDraft{Trips: []ShoppingTripInput{
		{Name: "Run 1", Items: []ShoppingItemInput{{Display: "milk"}}},
		{Name: "Run 2", Items: []ShoppingItemInput{{Display: "bread"}}},
	}})
	if err == nil || !strings.Contains(err.Error(), `list "Run 2" (list-2)`) || !strings.Contains(err.Error(), "upstream unavailable") {
		t.Fatalf("ApplyShoppingLists() error = %v, want failed list context", err)
	}
	if len(got.Lists) != 1 || got.Lists[0].ID != "list-1" || got.TotalItems != 1 {
		t.Fatalf("partial result = %+v, want first list only", got)
	}
}

type fakeShoppingListCreator struct {
	params []mealieapi.CreateShoppingListParams
	lists  []mealieapi.ShoppingList
}

func (f *fakeShoppingListCreator) CreateShoppingList(_ context.Context, params mealieapi.CreateShoppingListParams) (mealieapi.ShoppingList, error) {
	f.params = append(f.params, params)
	list := f.lists[0]
	f.lists = f.lists[1:]
	return list, nil
}

type fakeShoppingListItemWriter struct {
	params        [][]mealieapi.CreateShoppingListItemParams
	createdCounts []int
	errAtCall     int
	err           error
}

func (f *fakeShoppingListItemWriter) CreateShoppingListItems(_ context.Context, params []mealieapi.CreateShoppingListItemParams) (mealieapi.ShoppingListItemsCollection, error) {
	call := len(f.params)
	f.params = append(f.params, params)
	if f.err != nil && call == f.errAtCall {
		return mealieapi.ShoppingListItemsCollection{}, f.err
	}
	count := f.createdCounts[call]
	return mealieapi.ShoppingListItemsCollection{CreatedItems: make([]mealieapi.ShoppingListItem, count)}, nil
}

func equalShoppingListDraft(got, want ShoppingListDraft) bool {
	if len(got.Trips) != len(want.Trips) {
		return false
	}
	for i := range got.Trips {
		if got.Trips[i].Name != want.Trips[i].Name || len(got.Trips[i].Items) != len(want.Trips[i].Items) {
			return false
		}
		for j := range got.Trips[i].Items {
			if got.Trips[i].Items[j] != want.Trips[i].Items[j] {
				return false
			}
		}
	}
	return true
}

func equalShoppingItemParams(got, want []mealieapi.CreateShoppingListItemParams) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
