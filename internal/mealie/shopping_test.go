package mealie

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetShoppingLists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/api/households/shopping/lists" {
			t.Errorf("path = %s, want /api/households/shopping/lists", r.URL.Path)
		}
		for key, want := range map[string]string{
			"page":    "2",
			"perPage": "2",
		} {
			if got := r.URL.Query().Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want bearer token", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "page": 2,
            "per_page": 2,
            "total": 3,
            "total_pages": 2,
            "next": "https://mealie.example/api/households/shopping/lists?page=3",
            "previous": "https://mealie.example/api/households/shopping/lists?page=1",
            "items": [
                {
                    "id": "list-id",
                    "name": "Wed-Fri",
                    "householdId": "household-id",
                    "createdAt": "2026-09-28T12:00:00Z",
                    "updatedAt": "2026-09-29T12:00:00Z",
                    "futureField": "ignored"
                },
                {
                    "id": "second-list-id",
                    "name": null,
                    "householdId": "household-id"
                }
            ]
        }`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	got, err := client.GetShoppingLists(context.Background(), ShoppingListQuery{Page: 2, PerPage: 2})
	if err != nil {
		t.Fatalf("GetShoppingLists() error = %v", err)
	}
	if got.Page != 2 || got.PerPage != 2 || got.Total != 3 || got.TotalPages != 2 {
		t.Fatalf("pagination = %+v, want page 2/per-page 2/total 3/total-pages 2", got)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items length = %d, want 2", len(got.Items))
	}
	if got.Items[0].ID != "list-id" || got.Items[0].Name != "Wed-Fri" || got.Items[0].UpdatedAt == "" {
		t.Errorf("first list = %+v, want decoded list summary", got.Items[0])
	}
	if got.Items[1].Name != "" {
		t.Errorf("nullable list name = %q, want empty string", got.Items[1].Name)
	}
}

func TestGetShoppingList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/api/households/shopping/lists/list-id" {
			t.Errorf("path = %s, want /api/households/shopping/lists/list-id", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want bearer token", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "id": "list-id",
            "name": "Mon-Tue",
            "householdId": "household-id",
            "listItems": [
                {
                    "id": "item-id",
                    "shoppingListId": "list-id",
                    "quantity": 0,
                    "unit": null,
                    "food": null,
                    "note": "Ground cumin — check pantry",
                    "display": "Ground cumin — check pantry",
                    "checked": false,
                    "position": 0,
                    "foodId": null,
                    "unitId": null
                },
                {
                    "id": "parsed-item-id",
                    "shoppingListId": "list-id",
                    "quantity": 2,
                    "unit": {"name": "cup", "abbreviation": "c"},
                    "food": {"name": "rice"},
                    "note": null,
                    "display": "2 cups rice",
                    "checked": true,
                    "position": 1
                }
            ],
            "recipeReferences": []
        }`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	got, err := client.GetShoppingList(context.Background(), " list-id ")
	if err != nil {
		t.Fatalf("GetShoppingList() error = %v", err)
	}
	if got.ID != "list-id" || got.Name != "Mon-Tue" || len(got.Items) != 2 {
		t.Fatalf("list = %+v, want list with two items", got)
	}
	if got.Items[0].Display != "Ground cumin — check pantry" || got.Items[0].Quantity != 0 || got.Items[0].Checked {
		t.Errorf("unstructured shopping item = %+v, want display and zero quantity", got.Items[0])
	}
	if got.Items[1].Food == nil || got.Items[1].Food.Name != "rice" || got.Items[1].Unit == nil || got.Items[1].Unit.Name != "cup" || !got.Items[1].Checked {
		t.Errorf("structured shopping item = %+v, want food/unit/checked fields", got.Items[1])
	}
}

func TestGetShoppingListsValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be sent for invalid shopping-list parameters")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	tests := []struct {
		name   string
		params ShoppingListQuery
		want   string
	}{
		{name: "negative page", params: ShoppingListQuery{Page: -1}, want: "page must not be negative"},
		{name: "negative per-page", params: ShoppingListQuery{PerPage: -1}, want: "per-page must not be negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetShoppingLists(context.Background(), tt.params)
			if err == nil {
				t.Fatal("GetShoppingLists() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestGetShoppingListValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be sent for invalid shopping-list ID")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	tests := []struct {
		name string
		id   string
		want string
	}{
		{name: "missing ID", id: "   ", want: "list ID is required"},
		{name: "invalid path", id: "list/id", want: "invalid path character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetShoppingList(context.Background(), tt.id)
			if err == nil {
				t.Fatal("GetShoppingList() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}
