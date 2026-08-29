package importer

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/api"
	"github.com/profause/aimocksvr/internal/config"
	"github.com/profause/aimocksvr/internal/endpoint"
)

func newTestApp(es *fakeEndpointService) *fiber.App {
	logger := zerolog.Nop()
	h := NewHandler(NewService(es, &logger), &config.Config{}, &logger)

	app := fiber.New()
	h.Register(app)
	return app
}

func TestHandlerImportRawBody(t *testing.T) {
	es := &fakeEndpointService{result: endpoint.ImportResult{Created: 3}}
	app := newTestApp(es)

	req := httptest.NewRequest("POST", "/imports/openapi", bytes.NewReader([]byte(v3Spec)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Parsed  int `json:"parsed"`
			Created int `json:"created"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Data.Parsed != 3 || body.Data.Created != 3 {
		t.Errorf("unexpected response body: %+v", body)
	}
	if len(es.items) != 3 {
		t.Errorf("expected 3 import items, got %d", len(es.items))
	}
}

func TestHandlerImportMultipart(t *testing.T) {
	es := &fakeEndpointService{result: endpoint.ImportResult{Created: 1}}
	app := newTestApp(es)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "openapi.json")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write([]byte(v2Spec)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	mw.Close()

	req := httptest.NewRequest("POST", "/imports/openapi", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if len(es.items) != 1 || es.items[0].Method != "GET" || es.items[0].Path != "/pets" {
		t.Errorf("unexpected items: %+v", es.items)
	}
}

func TestHandlerImportRejectsEmptyBody(t *testing.T) {
	app := newTestApp(&fakeEndpointService{})

	req := httptest.NewRequest("POST", "/imports/openapi", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandlerImportRejectsInvalidSpec(t *testing.T) {
	app := newTestApp(&fakeEndpointService{})

	req := httptest.NewRequest("POST", "/imports/openapi", bytes.NewReader([]byte("not: [openapi")))
	req.Header.Set("Content-Type", "application/yaml")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandlerImportRejectsMissingMultipartField(t *testing.T) {
	app := newTestApp(&fakeEndpointService{})

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("other", "value")
	mw.Close()

	req := httptest.NewRequest("POST", "/imports/openapi", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandlerImportReturnsEnvelopeOnSuccess(t *testing.T) {
	es := &fakeEndpointService{result: endpoint.ImportResult{Created: 1}}
	app := newTestApp(es)

	req := httptest.NewRequest("POST", "/imports/openapi", bytes.NewReader([]byte(v3YAML)))
	req.Header.Set("Content-Type", "application/yaml")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	var body api.Response
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Success || body.Data == nil {
		t.Errorf("expected success envelope with data, got %+v", body)
	}
}

func TestHandlerImportPostmanRawBody(t *testing.T) {
	es := &fakeEndpointService{result: endpoint.ImportResult{Created: 1}}
	app := newTestApp(es)

	req := httptest.NewRequest("POST", "/imports/postman", bytes.NewReader([]byte(postmanCollection)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if len(es.items) != 3 {
		t.Fatalf("expected 3 import items, got %d", len(es.items))
	}
}

func TestHandlerImportPostmanMultipart(t *testing.T) {
	es := &fakeEndpointService{result: endpoint.ImportResult{Created: 1}}
	app := newTestApp(es)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "collection.postman_collection")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write([]byte(postmanCollection)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	mw.Close()

	req := httptest.NewRequest("POST", "/imports/postman", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if len(es.items) != 3 || es.items[0].Path != "/v2/users" {
		t.Errorf("unexpected items: %+v", es.items)
	}
}

func TestHandlerImportPostmanRejectsInvalidCollection(t *testing.T) {
	app := newTestApp(&fakeEndpointService{})

	req := httptest.NewRequest("POST", "/imports/postman", bytes.NewBufferString(`{"info":{"name":"T"}}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
