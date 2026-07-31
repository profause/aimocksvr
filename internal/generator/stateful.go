package generator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/profause/aimocksvr/internal/models"
	"github.com/profause/aimocksvr/internal/state"
)

const (
	msgResourceNotFound = `{"error":"resource not found"}`
	msgResourceConflict = `{"error":"resource already exists"}`
	msgBodyNotObject    = `{"error":"request body must be a JSON object"}`
)

// statefulGenerator serves stateful endpoints by persisting the resources
// created through POST and returning/updating/removing the same object on
// subsequent GET/PUT/PATCH/DELETE. Endpoints that are not stateful are
// delegated to the wrapped generator untouched.
//
// A stateful endpoint's path identifies its resource namespace: the trailing
// :param segment names the resource id (GET /users/:id), and the collection
// path without it (POST /users) is shared across endpoints of the same
// resource. Stateful endpoints without a path parameter only support POST
// (create); any other request is delegated to the wrapped generator.
type statefulGenerator struct {
	store state.Store
	inner Generator
	log   *zerolog.Logger
}

// NewStateful creates a Generator that adds persistent resources on top of
// inner. Requests to non-stateful endpoints pass through to inner unchanged.
func NewStateful(store state.Store, inner Generator, logger *zerolog.Logger) Generator {
	return &statefulGenerator{store: store, inner: inner, log: logger}
}

func (g *statefulGenerator) Generate(ctx context.Context, req *Request) (*Response, error) {
	if req.Endpoint == nil || !req.Endpoint.Stateful {
		return g.inner.Generate(ctx, req)
	}

	collection, paramName := resourceParts(req.Endpoint)
	resourceID := ""
	if paramName != "" {
		resourceID = req.PathParams[paramName]
	}

	switch req.Endpoint.Method {
	case http.MethodPost:
		return g.create(ctx, req, collection, resourceID)
	case http.MethodGet:
		if resourceID == "" {
			return g.inner.Generate(ctx, req)
		}
		return g.get(ctx, collection, resourceID)
	case http.MethodPut:
		if resourceID == "" {
			return g.inner.Generate(ctx, req)
		}
		return g.put(ctx, req, collection, resourceID)
	case http.MethodPatch:
		if resourceID == "" {
			return g.inner.Generate(ctx, req)
		}
		return g.patch(ctx, req, collection, resourceID)
	case http.MethodDelete:
		if resourceID == "" {
			return g.inner.Generate(ctx, req)
		}
		return g.delete(ctx, collection, resourceID)
	default:
		return g.inner.Generate(ctx, req)
	}
}

// create generates the resource body through the wrapped generator, assigns a
// resource id, and persists it. The id comes from the request body's "id"
// field when present, otherwise a UUID is generated and injected.
func (g *statefulGenerator) create(ctx context.Context, req *Request, collection, resourceID string) (*Response, error) {
	resp, err := g.inner.Generate(ctx, req)
	if err != nil {
		return nil, err
	}

	data, ok := parseObject(resp.Body)
	if !ok {
		data = map[string]any{}
	}
	if resourceID == "" {
		resourceID = idOf(data)
		if resourceID == "" {
			resourceID = uuid.NewString()
			data["id"] = resourceID
		}
	} else {
		data["id"] = resourceID
	}

	if err := g.store.Create(ctx, collection, resourceID, data); err != nil {
		if errors.Is(err, state.ErrConflict) {
			return &Response{Status: http.StatusConflict, Body: []byte(msgResourceConflict)}, nil
		}
		return nil, err
	}

	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &Response{Status: http.StatusCreated, Body: body}, nil
}

func (g *statefulGenerator) get(ctx context.Context, collection, resourceID string) (*Response, error) {
	data, found, err := g.store.Get(ctx, collection, resourceID)
	if err != nil {
		return nil, err
	}
	if !found {
		return &Response{Status: http.StatusNotFound, Body: []byte(msgResourceNotFound)}, nil
	}
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &Response{Status: http.StatusOK, Body: body}, nil
}

// put replaces the stored resource with the request body, keeping the id from
// the path so lookups stay consistent. When the body omits the id, the
// existing id value (and its type) is preserved.
func (g *statefulGenerator) put(ctx context.Context, req *Request, collection, resourceID string) (*Response, error) {
	current, found, err := g.store.Get(ctx, collection, resourceID)
	if err != nil {
		return nil, err
	}
	if !found {
		return &Response{Status: http.StatusNotFound, Body: []byte(msgResourceNotFound)}, nil
	}

	data, ok := parseObject(req.Body)
	if !ok {
		return &Response{Status: http.StatusBadRequest, Body: []byte(msgBodyNotObject)}, nil
	}
	if _, hasID := data["id"]; !hasID {
		if existing, has := current["id"]; has {
			data["id"] = existing
		} else {
			data["id"] = resourceID
		}
	} else {
		ensureID(data, resourceID)
	}

	if err := g.store.Update(ctx, collection, resourceID, data); err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return &Response{Status: http.StatusNotFound, Body: []byte(msgResourceNotFound)}, nil
		}
		return nil, err
	}

	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &Response{Status: http.StatusOK, Body: body}, nil
}

// patch deep-merges the request body into the stored resource, preserving the
// id from the path.
func (g *statefulGenerator) patch(ctx context.Context, req *Request, collection, resourceID string) (*Response, error) {
	current, found, err := g.store.Get(ctx, collection, resourceID)
	if err != nil {
		return nil, err
	}
	if !found {
		return &Response{Status: http.StatusNotFound, Body: []byte(msgResourceNotFound)}, nil
	}

	patch, ok := parseObject(req.Body)
	if !ok {
		return &Response{Status: http.StatusBadRequest, Body: []byte(msgBodyNotObject)}, nil
	}

	merged := mergeObjects(current, patch)
	ensureID(merged, resourceID)
	if err := g.store.Update(ctx, collection, resourceID, merged); err != nil {
		return nil, err
	}

	body, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	return &Response{Status: http.StatusOK, Body: body}, nil
}

func (g *statefulGenerator) delete(ctx context.Context, collection, resourceID string) (*Response, error) {
	found, err := g.store.Delete(ctx, collection, resourceID)
	if err != nil {
		return nil, err
	}
	if !found {
		return &Response{Status: http.StatusNotFound, Body: []byte(msgResourceNotFound)}, nil
	}
	return &Response{Status: http.StatusNoContent}, nil
}

// resourceParts derives the collection path and trailing :param name from an
// endpoint's path pattern. The collection path is shared across endpoints of
// the same resource, so POST /users and GET /users/:id both belong to /users.
func resourceParts(e *models.Endpoint) (collection, paramName string) {
	trimmed := strings.Trim(e.Path, "/")
	if trimmed == "" {
		return "/", ""
	}
	segs := strings.Split(trimmed, "/")
	last := segs[len(segs)-1]
	if strings.HasPrefix(last, ":") {
		return "/" + strings.Join(segs[:len(segs)-1], "/"), last[1:]
	}
	return "/" + strings.Join(segs, "/"), ""
}

// parseObject decodes a JSON object body. Non-object bodies yield ok=false.
func parseObject(body []byte) (map[string]any, bool) {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, false
	}
	return data, true
}

// idOf returns the string form of a resource object's "id" field, or "" when
// it is absent or empty.
func idOf(data map[string]any) string {
	id, ok := data["id"]
	if !ok {
		return ""
	}
	switch v := id.(type) {
	case string:
		return v
	case float64:
		// FormatFloat with -1 precision renders whole numbers without a
		// decimal part: 42.0 becomes "42".
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

// ensureID makes the resource id match the id from the path. When the data
// already carries an id with the same string form (for example a numeric 1
// that equals path id "1"), it is left untouched so its original type is
// preserved; otherwise it is replaced.
func ensureID(data map[string]any, resourceID string) {
	if _, ok := data["id"]; ok && idOf(map[string]any{"id": data["id"]}) == resourceID {
		return
	}
	data["id"] = resourceID
}

// mergeObjects deep-merges src into dst: nested objects are merged
// recursively, everything else is replaced. dst is mutated and returned.
func mergeObjects(dst, src map[string]any) map[string]any {
	for key, value := range src {
		if srcMap, ok := value.(map[string]any); ok {
			if dstMap, ok := dst[key].(map[string]any); ok {
				dst[key] = mergeObjects(dstMap, srcMap)
				continue
			}
		}
		dst[key] = value
	}
	return dst
}
