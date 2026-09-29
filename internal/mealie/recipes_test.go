package mealie

import (
	"context"
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
