package mealie

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Date
		err   string
	}{
		{name: "valid with whitespace", input: " 2026-09-28 ", want: Date("2026-09-28")},
		{name: "missing", err: "date is required"},
		{name: "invalid format", input: "09/28/2026", err: "date must use YYYY-MM-DD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDate(tt.input)
			if tt.err == "" {
				if err != nil {
					t.Fatalf("ParseDate() error = %v", err)
				}
				if got != tt.want {
					t.Errorf("ParseDate() = %q, want %q", got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatal("ParseDate() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.err) {
				t.Errorf("error = %q, want substring %q", err, tt.err)
			}
		})
	}
}

func TestGetMealPlan(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want %s", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/api/households/mealplans" {
			t.Errorf("path = %s, want /api/households/mealplans", r.URL.Path)
		}
		for key, want := range map[string]string{
			"start_date": "2026-09-28",
			"end_date":   "2026-10-02",
			"page":       "2",
			"perPage":    "2",
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
            "next": "https://mealie.example/api/households/mealplans?page=3",
            "previous": "https://mealie.example/api/households/mealplans?page=1",
            "items": [
                {
                    "date": "2026-09-28",
                    "entryType": "dinner",
                    "title": "",
                    "text": "",
                    "recipeId": "recipe-id",
                    "id": 123,
                    "recipe": {
                        "id": "recipe-id",
                        "name": "Chicken Tikka",
                        "slug": "chicken-tikka",
                        "description": "A weeknight recipe",
                        "orgURL": "https://recipes.example/chicken-tikka"
                    }
                },
                {
                    "date": "2026-09-29",
                    "entryType": "dinner",
                    "title": "Steak Night",
                    "text": "NY Strip with roasted vegetables",
                    "recipeId": null,
                    "id": 124,
                    "recipe": null
                }
            ]
        }`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	got, err := client.GetMealPlan(context.Background(), MealPlanQuery{
		StartDate: Date("2026-09-28"),
		EndDate:   Date("2026-10-02"),
		Page:      2,
		PerPage:   2,
	})
	if err != nil {
		t.Fatalf("GetMealPlan() error = %v", err)
	}
	if got.Page != 2 || got.PerPage != 2 || got.Total != 3 || got.TotalPages != 2 {
		t.Fatalf("pagination = %+v, want page 2/per-page 2/total 3/total-pages 2", got)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items length = %d, want 2", len(got.Items))
	}
	if got.Items[0].Date != Date("2026-09-28") || got.Items[0].RecipeID == nil || got.Items[0].Recipe == nil || got.Items[0].Recipe.Name != "Chicken Tikka" {
		t.Errorf("recipe-backed entry = %+v, want recipe summary", got.Items[0])
	}
	if got.Items[1].Title != "Steak Night" || got.Items[1].Text == "" || got.Items[1].RecipeID != nil || got.Items[1].Recipe != nil {
		t.Errorf("simple entry = %+v, want title/text and no recipe", got.Items[1])
	}
}

func TestGetMealPlanValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be sent for invalid meal-plan parameters")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	tests := []struct {
		name   string
		params MealPlanQuery
		want   string
	}{
		{name: "invalid start date", params: MealPlanQuery{StartDate: Date("09/28/2026")}, want: "start date must use YYYY-MM-DD"},
		{name: "invalid end date", params: MealPlanQuery{EndDate: Date("tomorrow")}, want: "end date must use YYYY-MM-DD"},
		{name: "reversed range", params: MealPlanQuery{StartDate: Date("2026-10-02"), EndDate: Date("2026-09-28")}, want: "start date must not be after end date"},
		{name: "negative page", params: MealPlanQuery{Page: -1}, want: "page must not be negative"},
		{name: "negative per-page", params: MealPlanQuery{PerPage: -1}, want: "per-page must not be negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetMealPlan(context.Background(), tt.params)
			if err == nil {
				t.Fatal("GetMealPlan() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestGetMealPlanIgnoresUnknownFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"page": 1, "per_page": 50, "total": 0, "total_pages": 0,
			"items": []any{}, "futureField": "ignored",
		})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL, "test-token")
	if _, err := client.GetMealPlan(context.Background(), MealPlanQuery{}); err != nil {
		t.Fatalf("GetMealPlan() error = %v", err)
	}
}
