package router

import (
	"testing"

	"github.com/profause/aimocksvr/internal/models"
)

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		wantOK  bool
		want    map[string]string
	}{
		{name: "static exact", pattern: "/users", path: "/users", wantOK: true, want: map[string]string{}},
		{name: "static mismatch", pattern: "/users", path: "/orders", wantOK: false},
		{name: "single param", pattern: "/users/:id", path: "/users/123", wantOK: true, want: map[string]string{"id": "123"}},
		{name: "multiple params", pattern: "/orgs/:org/users/:id", path: "/orgs/acme/users/42", wantOK: true, want: map[string]string{"org": "acme", "id": "42"}},
		{name: "missing segment", pattern: "/users/:id", path: "/users", wantOK: false},
		{name: "extra segment", pattern: "/users", path: "/users/123", wantOK: false},
		{name: "static vs param", pattern: "/users/me", path: "/users/me", wantOK: true, want: map[string]string{}},
		{name: "trailing slash", pattern: "/users/", path: "/users", wantOK: true, want: map[string]string{}},
		{name: "empty param value", pattern: "/users/:id", path: "/users/", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := matchPath(tt.pattern, tt.path)
			if ok != tt.wantOK {
				t.Fatalf("matchPath(%q, %q) ok = %v, want %v", tt.pattern, tt.path, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("params = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("param %q = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestBestMatchPreference(t *testing.T) {
	endpoints := []models.Endpoint{
		{Method: "GET", Path: "/users/:id"},
		{Method: "GET", Path: "/users/me"},
	}

	e, params, ok := bestMatch(endpoints, "/users/me")
	if !ok {
		t.Fatal("expected a match")
	}
	if e.Path != "/users/me" {
		t.Errorf("expected static segment to win, got %q", e.Path)
	}
	if params["id"] != "" {
		t.Errorf("did not expect id param, got %v", params)
	}

	e, params, ok = bestMatch(endpoints, "/users/42")
	if !ok {
		t.Fatal("expected a match")
	}
	if e.Path != "/users/:id" {
		t.Errorf("expected param pattern, got %q", e.Path)
	}
	if params["id"] != "42" {
		t.Errorf("expected id=42, got %v", params)
	}
}

func TestBestMatchNone(t *testing.T) {
	endpoints := []models.Endpoint{
		{Method: "GET", Path: "/users/:id"},
	}

	if _, _, ok := bestMatch(endpoints, "/orders/42"); ok {
		t.Fatal("did not expect a match")
	}
}
