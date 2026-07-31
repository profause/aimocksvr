package router

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/profause/aimocksvr/internal/api"
)

// TestVersioningViaRegistryAPI exercises the full history -> diff -> rollback
// pipeline through the HTTP API, including how a rollback changes what the
// dynamic router serves.
func TestVersioningViaRegistryAPI(t *testing.T) {
	app := newImportApp(t)

	resp, err := app.Test(httptest.NewRequest("POST", "/api/v1/endpoints", strings.NewReader(`{
		"method": "get",
		"path": "/versioned",
		"prompt": "a versioned endpoint"
	}`)))
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d (body %q)", resp.StatusCode, body)
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	// A non-prompt modification (path change) bumps the version and moves the
	// served route.
	updateURL := "/api/v1/endpoints/" + created.Data.ID
	resp, err = app.Test(httptest.NewRequest("PUT", updateURL, strings.NewReader(`{
		"method": "get",
		"path": "/renamed",
		"prompt": "a versioned endpoint"
	}`)))
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on update, got %d", resp.StatusCode)
	}

	resp, err = app.Test(httptest.NewRequest("GET", "/renamed", nil))
	if err != nil {
		t.Fatalf("GET /renamed failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on /renamed, got %d", resp.StatusCode)
	}

	// History shows both snapshots with their full state.
	resp, err = app.Test(httptest.NewRequest("GET", updateURL+"/versions", nil))
	if err != nil {
		t.Fatalf("list versions failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on versions, got %d", resp.StatusCode)
	}
	var versions struct {
		Data struct {
			Versions []struct {
				Version int    `json:"version"`
				Method  string `json:"method"`
				Path    string `json:"path"`
			} `json:"versions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &versions); err != nil {
		t.Fatalf("decode versions response: %v", err)
	}
	if len(versions.Data.Versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions.Data.Versions))
	}
	if versions.Data.Versions[0].Version != 2 || versions.Data.Versions[0].Path != "/renamed" {
		t.Errorf("latest version snapshot wrong: %+v", versions.Data.Versions[0])
	}
	if versions.Data.Versions[1].Version != 1 || versions.Data.Versions[1].Path != "/versioned" {
		t.Errorf("v1 snapshot wrong: %+v", versions.Data.Versions[1])
	}

	// Diff v1 against the latest reports the path change.
	resp, err = app.Test(httptest.NewRequest("GET", updateURL+"/versions/1/diff", nil))
	if err != nil {
		t.Fatalf("diff failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on diff, got %d (body %q)", resp.StatusCode, body)
	}
	var diff struct {
		Data struct {
			Changes []struct {
				Field string `json:"field"`
				From  string `json:"from"`
				To    string `json:"to"`
			} `json:"changes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &diff); err != nil {
		t.Fatalf("decode diff response: %v", err)
	}
	pathChange := false
	for _, c := range diff.Data.Changes {
		if c.Field == "path" && c.From == "/versioned" && c.To == "/renamed" {
			pathChange = true
		}
	}
	if !pathChange {
		t.Errorf("expected path change in diff, got %+v", diff.Data.Changes)
	}

	// Rolling back to v1 restores the original route and records version 3.
	resp, err = app.Test(httptest.NewRequest("POST", updateURL+"/versions/1/rollback", nil))
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on rollback, got %d (body %q)", resp.StatusCode, body)
	}
	var rolled struct {
		Data struct {
			Path string `json:"path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &rolled); err != nil {
		t.Fatalf("decode rollback response: %v", err)
	}
	if rolled.Data.Path != "/versioned" {
		t.Fatalf("expected rollback to restore /versioned, got %q", rolled.Data.Path)
	}

	resp, err = app.Test(httptest.NewRequest("GET", "/versioned", nil))
	if err != nil {
		t.Fatalf("GET /versioned after rollback failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on /versioned after rollback, got %d", resp.StatusCode)
	}
	resp, err = app.Test(httptest.NewRequest("GET", "/renamed", nil))
	if err != nil {
		t.Fatalf("GET /renamed after rollback failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404 on /renamed after rollback, got %d", resp.StatusCode)
	}

	resp, err = app.Test(httptest.NewRequest("GET", updateURL+"/versions", nil))
	if err != nil {
		t.Fatalf("list versions after rollback failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), `"version":3`) {
		t.Errorf("expected rollback to record version 3, got %q", body)
	}

	// Rolling back to the latest version is rejected as a no-op.
	resp, err = app.Test(httptest.NewRequest("POST", updateURL+"/versions/3/rollback", nil))
	if err != nil {
		t.Fatalf("no-op rollback failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400 on no-op rollback, got %d", resp.StatusCode)
	}

	// Unknown versions are rejected for both rollback and diff.
	resp, err = app.Test(httptest.NewRequest("POST", updateURL+"/versions/99/rollback", nil))
	if err != nil {
		t.Fatalf("unknown rollback failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400 on unknown rollback, got %d", resp.StatusCode)
	}

	resp, err = app.Test(httptest.NewRequest("GET", updateURL+"/versions/99/diff", nil))
	if err != nil {
		t.Fatalf("unknown diff failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400 on unknown diff, got %d (body %q)", resp.StatusCode, body)
	}
	var fail struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &fail); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if fail.Error.Code != api.CodeValidationError {
		t.Errorf("expected %s code, got %q", api.CodeValidationError, fail.Error.Code)
	}
}
