package router

import (
	"strings"

	"github.com/profause/aimocksvr/internal/models"
)

// matchPath matches a stored endpoint pattern (e.g. /users/:id) against a
// concrete request path (e.g. /users/123), returning the captured parameters.
// Patterns support :name segments; static segments must match exactly.
func matchPath(pattern, path string) (map[string]string, bool) {
	patternSegs := splitPath(pattern)
	pathSegs := splitPath(path)
	if len(patternSegs) != len(pathSegs) {
		return nil, false
	}

	params := make(map[string]string)
	for i, seg := range patternSegs {
		if strings.HasPrefix(seg, ":") {
			params[strings.TrimPrefix(seg, ":")] = pathSegs[i]
			continue
		}
		if seg != pathSegs[i] {
			return nil, false
		}
	}
	return params, true
}

// bestMatch picks the most specific matching endpoint. Specificity is measured
// by the number of static segments, so /users/me wins over /users/:id for a
// request to /users/me.
func bestMatch(endpoints []models.Endpoint, path string) (*models.Endpoint, map[string]string, bool) {
	var (
		best       *models.Endpoint
		bestParams map[string]string
		bestStatic = -1
	)

	for i := range endpoints {
		params, ok := matchPath(endpoints[i].Path, path)
		if !ok {
			continue
		}
		static := staticSegments(endpoints[i].Path)
		if static > bestStatic {
			best = &endpoints[i]
			bestParams = params
			bestStatic = static
		}
	}

	if best == nil {
		return nil, nil, false
	}
	return best, bestParams, true
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func staticSegments(pattern string) int {
	count := 0
	for _, seg := range splitPath(pattern) {
		if !strings.HasPrefix(seg, ":") {
			count++
		}
	}
	return count
}
