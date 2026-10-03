package mealie

import (
	"context"
	"encoding/json"
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

func TestCreateShoppingList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/api/households/shopping/lists" {
			t.Errorf("path = %s, want /api/households/shopping/lists", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want bearer token", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}

		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		if body.Name != "Mon-Tue" {
			t.Errorf("request body = %+v, want trimmed list name", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
            "id": "list-id",
            "name": "Mon-Tue",
            "householdId": "household-id",
            "listItems": []
        }`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	got, err := client.CreateShoppingList(context.Background(), CreateShoppingListParams{Name: " Mon-Tue "})
	if err != nil {
		t.Fatalf("CreateShoppingList() error = %v", err)
	}
	if got.ID != "list-id" || got.Name != "Mon-Tue" {
		t.Fatalf("created list = %+v, want list-id/Mon-Tue", got)
	}
}

func TestCreateShoppingListItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/api/households/shopping/items/create-bulk" {
			t.Errorf("path = %s, want /api/households/shopping/items/create-bulk", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want bearer token", got)
		}

		var body []struct {
			Quantity       float64 `json:"quantity"`
			Note           string  `json:"note"`
			Display        string  `json:"display"`
			ShoppingListID string  `json:"shoppingListId"`
			Checked        bool    `json:"checked"`
			Position       int     `json:"position"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if len(body) != 2 {
			t.Fatalf("request body length = %d, want 2", len(body))
		}
		if body[0].Quantity != 0 || body[0].Display != "Ground cumin — check pantry" || body[0].Note != "Ground cumin — check pantry" || body[0].ShoppingListID != "list-id" || body[0].Checked || body[0].Position != 0 {
			t.Errorf("first request item = %+v, want clean zero-quantity item", body[0])
		}
		if body[1].Quantity != 0 || body[1].Display != "2 cups rice" || body[1].Note != "Rice for dinner" || body[1].Position != 1 {
			t.Errorf("second request item = %+v, want zero quantity and explicit note", body[1])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
            "createdItems": [
                {
                    "id": "item-one",
                    "shoppingListId": "list-id",
                    "quantity": 0,
                    "note": "Ground cumin — check pantry",
                    "display": "Ground cumin — check pantry",
                    "checked": false,
                    "position": 0
                },
                {
                    "id": "item-two",
                    "shoppingListId": "list-id",
                    "quantity": 0,
                    "note": "Rice for dinner",
                    "display": "2 cups rice",
                    "checked": false,
                    "position": 1
                }
            ],
            "updatedItems": [],
            "deletedItems": []
        }`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	got, err := client.CreateShoppingListItems(context.Background(), []CreateShoppingListItemParams{
		{ShoppingListID: " list-id ", Display: " Ground cumin — check pantry "},
		{ShoppingListID: "list-id", Display: "2 cups rice", Note: " Rice for dinner ", Position: 1},
	})
	if err != nil {
		t.Fatalf("CreateShoppingListItems() error = %v", err)
	}
	if len(got.CreatedItems) != 2 || got.CreatedItems[1].Display != "2 cups rice" || got.CreatedItems[1].Note != "Rice for dinner" {
		t.Fatalf("created items = %+v, want two decoded items", got.CreatedItems)
	}
}

func TestCreateShoppingListValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be sent for invalid shopping-list creation")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	_, err := client.CreateShoppingList(context.Background(), CreateShoppingListParams{Name: "   "})
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("CreateShoppingList() error = %v, want required name error", err)
	}
}

func TestCreateShoppingListItemsValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be sent for invalid shopping-item creation")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	tests := []struct {
		name   string
		params []CreateShoppingListItemParams
		want   string
	}{
		{name: "empty items", want: "at least one item is required"},
		{name: "missing list ID", params: []CreateShoppingListItemParams{{Display: "Rice"}}, want: "list ID is required"},
		{name: "missing display", params: []CreateShoppingListItemParams{{ShoppingListID: "list-id"}}, want: "display is required"},
		{name: "negative position", params: []CreateShoppingListItemParams{{ShoppingListID: "list-id", Display: "Rice", Position: -1}}, want: "position must not be negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.CreateShoppingListItems(context.Background(), tt.params)
			if err == nil {
				t.Fatal("CreateShoppingListItems() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}
