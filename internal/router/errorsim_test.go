package router

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/profause/aimocksvr/internal/api"
	"github.com/profause/aimocksvr/internal/models"
)

func newSimEndpoint(method, path, config string) models.Endpoint {
	e := newEndpoint(method, path)
	e.ErrorSim = config
	return e
}

func TestDynamicHandlerSimulatesStatusCodes(t *testing.T) {
	for _, tc := range []struct {
		status int
	}{
		{500},
		{404},
		{429},
		{503},
	} {
		t.Run(string(rune(tc.status)), func(t *testing.T) {
			config, _ := json.Marshal(map[string]any{"status": tc.status})
			store := &fakeStore{
				endpoints: []models.Endpoint{newSimEndpoint("GET", "/boom", string(config))},
			}
			app := newDynamicApp(t, store)

			resp, err := app.Test(httptest.NewRequest("GET", "/boom", nil))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.status {
				t.Fatalf("expected %d, got %d", tc.status, resp.StatusCode)
			}
			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("expected error envelope, got non-JSON body")
			}
			if code, _ := body["error"].(map[string]any)["code"].(string); code != api.CodeErrorSimulation {
				t.Errorf("expected code %q, got %v", api.CodeErrorSimulation, code)
			}
		})
	}
}

func TestDynamicHandlerSimulatesMalformedJSON(t *testing.T) {
	store := &fakeStore{
		endpoints: []models.Endpoint{newSimEndpoint("GET", "/broken", `{"malformed_json":true}`)},
	}
	app := newDynamicApp(t, store)

	resp, err := app.Test(httptest.NewRequest("GET", "/broken", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var v any
	if err := json.Unmarshal(body, &v); err == nil {
		t.Fatalf("expected malformed JSON body, got valid JSON: %q", body)
	}
}

func TestDynamicHandlerSimulatesLatency(t *testing.T) {
	store := &fakeStore{
		endpoints: []models.Endpoint{newSimEndpoint("GET", "/slow", `{"latency_ms":150}`)},
	}
	app := newDynamicApp(t, store)

	start := time.Now()
	resp, err := app.Test(httptest.NewRequest("GET", "/slow", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	elapsed := time.Since(start)

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if elapsed < 150*time.Millisecond {
		t.Errorf("expected at least 150ms latency, took %s", elapsed)
	}
}

func TestDynamicHandlerSimulatesDropConnection(t *testing.T) {
	store := &fakeStore{
		endpoints: []models.Endpoint{newSimEndpoint("GET", "/drop", `{"drop_connection":true}`)},
	}
	app := newDynamicApp(t, store)

	_, err := app.Test(httptest.NewRequest("GET", "/drop", nil))
	if err == nil {
		t.Fatal("expected an empty response for a dropped connection, got a normal response")
	}
}

func TestDynamicHandlerSimulatesTimeout(t *testing.T) {
	store := &fakeStore{
		endpoints: []models.Endpoint{newSimEndpoint("GET", "/timeout", `{"timeout_ms":200}`)},
	}
	app := newDynamicApp(t, store)

	start := time.Now()
	_, err := app.Test(httptest.NewRequest("GET", "/timeout", nil))
	elapsed := time.Since(start)

	if elapsed < 200*time.Millisecond {
		t.Errorf("timeout should hold the connection at least 200ms, took %s", elapsed)
	}
	if err == nil {
		t.Log("connection was dropped, timeout simulated")
	}
}

func TestDynamicHandlerSimulationPrecedence(t *testing.T) {
	// Timeout wins over status when both are configured.
	store := &fakeStore{
		endpoints: []models.Endpoint{newSimEndpoint("GET", "/t", `{"timeout_ms":150,"status":500}`)},
	}
	app := newDynamicApp(t, store)

	start := time.Now()
	_, _ = app.Test(httptest.NewRequest("GET", "/t", nil))
	if time.Since(start) < 150*time.Millisecond {
		t.Errorf("timeout behavior should take precedence, elapsed %s", time.Since(start))
	}
}

func TestDynamicHandlerSimulationWithoutFailureRateFailsAlways(t *testing.T) {
	// A status without failure_rate (0) always applies.
	store := &fakeStore{
		endpoints: []models.Endpoint{newSimEndpoint("GET", "/always", `{"status":418}`)},
	}
	app := newDynamicApp(t, store)

	for i := 0; i < 5; i++ {
		resp, err := app.Test(httptest.NewRequest("GET", "/always", nil))
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 418 {
			t.Fatalf("request %d: expected 418, got %d", i, resp.StatusCode)
		}
	}
}

func TestDynamicHandlerCorruptErrorSimServesNormally(t *testing.T) {
	store := &fakeStore{
		endpoints: []models.Endpoint{newSimEndpoint("GET", "/corrupt", `{not json`)},
	}
	app := newDynamicApp(t, store)

	resp, err := app.Test(httptest.NewRequest("GET", "/corrupt", nil))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 despite corrupt error_sim, got %d", resp.StatusCode)
	}
}

func TestErrorSimulationViaRegistryAPI(t *testing.T) {
	app := newImportApp(t)

	// Create an endpoint with error simulation through the registry API.
	resp, err := app.Test(httptest.NewRequest("POST", "/api/v1/endpoints", strings.NewReader(`{
		"method": "get",
		"path": "/flaky",
		"prompt": "a flaky endpoint",
		"error_sim": "{\"status\": 500, \"failure_rate\": 0}"
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
			ID       string `json:"id"`
			ErrorSim string `json:"error_sim"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if !strings.Contains(created.Data.ErrorSim, `"status"`) || !strings.Contains(created.Data.ErrorSim, "500") {
		t.Errorf("expected error_sim stored, got %q", created.Data.ErrorSim)
	}

	// The endpoint now fails with the simulated status.
	resp, err = app.Test(httptest.NewRequest("GET", "/flaky", nil))
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 500 {
		t.Fatalf("expected simulated 500, got %d", resp.StatusCode)
	}

	// Clearing error_sim restores normal serving.
	clearURL := "/api/v1/endpoints/" + created.Data.ID
	resp, err = app.Test(httptest.NewRequest("PUT", clearURL, strings.NewReader(`{
		"method": "get",
		"path": "/flaky",
		"prompt": "a flaky endpoint",
		"error_sim": ""
	}`)))
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 on update, got %d", resp.StatusCode)
	}

	resp, err = app.Test(httptest.NewRequest("GET", "/flaky", nil))
	if err != nil {
		t.Fatalf("GET after clear failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200 after clearing error_sim, got %d", resp.StatusCode)
	}
}
