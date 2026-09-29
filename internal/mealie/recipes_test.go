package mealie

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joeysaladino/homelab-mcp/internal/config"
)

func TestSearchRecipes(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "recipe page",
			body: `{
                "page": 2,
                "per_page": 10,
                "total": 1,
                "total_pages": 1,
                "next": null,
                "previous": null,
                "items": [{
                    "id": "recipe-id",
                    "name": "Chicken Tikka",
                    "slug": "chicken-tikka",
                    "description": "A weeknight recipe",
                    "orgURL": "https://recipes.example/chicken-tikka",
                    "ignoredFutureField": true
                }]
            }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("method = %s, want %s", r.Method, http.MethodGet)
				}
				if r.URL.Path != "/api/recipes" {
					t.Errorf("path = %s, want /api/recipes", r.URL.Path)
				}
				if got := r.URL.Query().Get("search"); got != "chicken" {
					t.Errorf("search = %q, want chicken", got)
				}
				if got := r.URL.Query().Get("page"); got != "2" {
					t.Errorf("page = %q, want 2", got)
				}
				if got := r.URL.Query().Get("perPage"); got != "10" {
					t.Errorf("perPage = %q, want 10", got)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
					t.Errorf("Authorization = %q, want bearer token", got)
				}
				if got := r.Header.Get("Accept"); got != "application/json" {
					t.Errorf("Accept = %q, want application/json", got)
				}

				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := newTestClient(t, server.URL, "test-token")
			got, err := client.SearchRecipes(context.Background(), RecipeSearchParams{
				Query:   " chicken ",
				Page:    2,
				PerPage: 10,
			})
			if err != nil {
				t.Fatalf("SearchRecipes() error = %v", err)
			}

			if got.Page != 2 || got.PerPage != 10 || got.Total != 1 || got.TotalPages != 1 {
				t.Fatalf("pagination = %+v, want page 2/per-page 10/total 1/total-pages 1", got)
			}
			if len(got.Items) != 1 {
				t.Fatalf("items length = %d, want 1", len(got.Items))
			}
			if got.Items[0].Name != "Chicken Tikka" || got.Items[0].Slug != "chicken-tikka" {
				t.Errorf("recipe = %+v, want Chicken Tikka/chicken-tikka", got.Items[0])
			}
		})
	}
}

func TestGetRecipe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/api/recipes/chicken-tikka" {
			t.Errorf("path = %s, want /api/recipes/chicken-tikka", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want bearer token", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "id": "recipe-id",
            "name": "Chicken Tikka",
            "slug": "chicken-tikka",
            "description": "A weeknight recipe",
            "orgURL": "https://recipes.example/chicken-tikka",
            "prepTime": "PT15M",
            "cookTime": "PT20M",
            "totalTime": "PT35M",
            "recipeServings": 4,
            "recipeYieldQuantity": 4,
            "recipeYield": "servings",
            "recipeIngredient": [
                {
                    "quantity": 0.0,
                    "unit": null,
                    "food": null,
                    "note": "1 teaspoon ground cumin",
                    "display": "1 teaspoon ground cumin",
                    "originalText": null
                },
                {
                    "quantity": 2.0,
                    "unit": {"id": "unit-id", "name": "cup", "abbreviation": "c"},
                    "food": {"id": "food-id", "name": "rice"},
                    "note": null,
                    "display": "2 cups rice",
                    "originalText": "2 cups rice"
                }
            ],
            "recipeInstructions": [{
                "id": "step-id",
                "title": "",
                "summary": "",
                "text": "Cook the rice."
            }]
        }`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	got, err := client.GetRecipe(context.Background(), " chicken-tikka ")
	if err != nil {
		t.Fatalf("GetRecipe() error = %v", err)
	}

	if got.ID != "recipe-id" || got.Name != "Chicken Tikka" || got.Slug != "chicken-tikka" {
		t.Fatalf("recipe identity = %+v, want recipe-id/Chicken Tikka/chicken-tikka", got)
	}
	if got.PrepTime != "PT15M" || got.CookTime != "PT20M" || got.TotalTime != "PT35M" {
		t.Errorf("times = prep %q/cook %q/total %q, want PT15M/PT20M/PT35M", got.PrepTime, got.CookTime, got.TotalTime)
	}
	if len(got.Ingredients) != 2 {
		t.Fatalf("ingredients length = %d, want 2", len(got.Ingredients))
	}
	if got.Ingredients[0].Display != "1 teaspoon ground cumin" || got.Ingredients[0].Unit != nil || got.Ingredients[0].Food != nil {
		t.Errorf("unstructured ingredient = %+v, want display text with nil unit and food", got.Ingredients[0])
	}
	if got.Ingredients[0].Quantity == nil || *got.Ingredients[0].Quantity != 0 {
		t.Errorf("unstructured quantity = %v, want pointer to zero", got.Ingredients[0].Quantity)
	}
	if got.Ingredients[1].Quantity == nil || *got.Ingredients[1].Quantity != 2 || got.Ingredients[1].Unit.Name != "cup" || got.Ingredients[1].Food.Name != "rice" {
		t.Errorf("structured ingredient = %+v, want quantity 2/cup/rice", got.Ingredients[1])
	}
	if len(got.Instructions) != 1 || got.Instructions[0].Text != "Cook the rice." {
		t.Errorf("instructions = %+v, want one cooking step", got.Instructions)
	}
}

func TestGetRecipeValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be sent for invalid recipe identifier")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	tests := []struct {
		name       string
		identifier string
		want       string
	}{
		{name: "missing identifier", want: "slug or id is required"},
		{name: "path separator", identifier: "recipes/chicken", want: "invalid path character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetRecipe(context.Background(), tt.identifier)
			if err == nil {
				t.Fatal("GetRecipe() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestImportRecipeURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/api/recipes/create/url" {
			t.Errorf("path = %s, want /api/recipes/create/url", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want bearer token", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}

		var body struct {
			URL               string `json:"url"`
			IncludeTags       bool   `json:"includeTags"`
			IncludeCategories bool   `json:"includeCategories"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		if body.URL != "https://recipes.example/chicken-tikka" || !body.IncludeTags || body.IncludeCategories {
			t.Errorf("request body = %+v, want URL and includeTags=true/includeCategories=false", body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode("recipe-imported")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	got, err := client.ImportRecipeURL(context.Background(), ImportRecipeParams{
		URL:               " https://recipes.example/chicken-tikka ",
		IncludeTags:       true,
		IncludeCategories: false,
	})
	if err != nil {
		t.Fatalf("ImportRecipeURL() error = %v", err)
	}
	if got != "recipe-imported" {
		t.Errorf("result = %q, want recipe-imported", got)
	}
}

func TestImportRecipeURLValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be sent for invalid import URL")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "missing URL", want: "URL is required"},
		{name: "unsupported scheme", url: "ftp://recipes.example/chicken", want: "http or https"},
		{name: "missing host", url: "https:///chicken", want: "http or https"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.ImportRecipeURL(context.Background(), ImportRecipeParams{URL: tt.url})
			if err == nil {
				t.Fatal("ImportRecipeURL() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestSearchRecipesValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be sent for invalid parameters")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	tests := []struct {
		name   string
		params RecipeSearchParams
		want   string
	}{
		{name: "missing query", want: "query is required"},
		{name: "negative page", params: RecipeSearchParams{Query: "chicken", Page: -1}, want: "page must not be negative"},
		{name: "negative per-page", params: RecipeSearchParams{Query: "chicken", PerPage: -1}, want: "per-page must not be negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.SearchRecipes(context.Background(), tt.params)
			if err == nil {
				t.Fatal("SearchRecipes() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestSearchRecipesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"invalid token"}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	_, err := client.SearchRecipes(context.Background(), RecipeSearchParams{Query: "chicken"})
	if err == nil {
		t.Fatal("SearchRecipes() error = nil, want error")
	}

	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error = %T (%v), want *HTTPError", err, err)
	}
	if httpErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("status code = %d, want %d", httpErr.StatusCode, http.StatusUnauthorized)
	}
	if !strings.Contains(err.Error(), "invalid token") {
		t.Errorf("error = %q, want response detail", err)
	}
	if strings.Contains(err.Error(), "test-token") {
		t.Errorf("error contains server-side token: %q", err)
	}
}

func TestNewClientValidation(t *testing.T) {
	_, err := NewClient(config.MealieConfig{}, nil)
	if err == nil {
		t.Fatal("NewClient() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "base URL is required") {
		t.Errorf("error = %q, want base URL validation", err)
	}
}

func newTestClient(t *testing.T, baseURL, token string) *Client {
	t.Helper()

	cfg, err := config.LoadFromEnv(mapLookup(map[string]string{
		"MEALIE_URL":   baseURL,
		"MEALIE_TOKEN": token,
	}))
	if err != nil {
		t.Fatalf("config.LoadFromEnv() error = %v", err)
	}

	client, err := NewClient(cfg.Mealie, nil)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
