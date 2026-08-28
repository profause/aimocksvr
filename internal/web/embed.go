// Package web embeds the frontend build and serves it via Fiber.
//
//go:build !skip_embed

package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gofiber/fiber/v3"
)

//go:embed all:dist/*
var distFS embed.FS

// spaRoutes are the embedded dashboard's client-side navigation paths. The
// static handler only falls back to index.html for these; any other path is
// passed through (c.Next()) so the dynamic mock catch-all can serve user-created
// mock endpoints at arbitrary paths.
var spaRoutes = map[string]bool{
	"/":          true,
	"/endpoints": true,
	"/logs":      true,
	"/scenarios": true,
	"/imports":   true,
	"/docs":      true,
	"/settings":  true,
}

// serveIndex returns the SPA bootstrap page, or false when the caller should
// continue to the next handler (c.Next()).
func serveIndex(c fiber.Ctx) bool {
	idx, readErr := distFS.ReadFile("dist/index.html")
	if readErr != nil {
		return false
	}
	c.Type("html")
	c.Send(idx)
	return true
}

// Register serves the embedded SPA. Non-API requests that match a static asset
// are served as files; requests that match a known client-side route fall back
// to index.html; everything else continues to the mock catch-all.
func Register(app *fiber.App) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("web: embed failed: " + err.Error())
	}

	app.Use(func(c fiber.Ctx) error {
		reqPath := c.Path()

		// Skip API and health routes
		if strings.HasPrefix(reqPath, "/api") || strings.HasPrefix(reqPath, "/health") {
			return c.Next()
		}

		// Try to serve the requested file
		filePath := strings.TrimPrefix(reqPath, "/")
		if filePath == "" {
			filePath = "index.html"
		}

		f, err := sub.Open(filePath)
		if err != nil {
			// Not a static file — serve the SPA bootstrap only for a known
			// client-side route; otherwise let the mock catch-all handle it.
			if spaRoutes[reqPath] && serveIndex(c) {
				return nil
			}
			return c.Next()
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil || info.IsDir() {
			if spaRoutes[reqPath] && serveIndex(c) {
				return nil
			}
			return c.Next()
		}

		ext := path.Ext(filePath)
		switch ext {
		case ".js":
			c.Type("js")
		case ".css":
			c.Type("css")
		case ".svg":
			c.Type("svg")
		case ".png":
			c.Type("png")
		case ".ico":
			c.Type("x-icon")
		case ".woff":
			c.Type("font/woff")
		case ".woff2":
			c.Type("font/woff2")
		case ".html":
			c.Type("html")
		}

		content, err := fs.ReadFile(sub, filePath)
		if err != nil {
			return c.Next()
		}

		return c.Send(content)
	})
}

// Handler returns a fiber handler that serves the embedded SPA.
// Use this when you want explicit control over the route.
func Handler() fiber.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("web: embed failed: " + err.Error())
	}

	return func(c fiber.Ctx) error {
		reqPath := strings.TrimPrefix(c.Path(), "/")
		if reqPath == "" {
			reqPath = "index.html"
		}

		f, err := sub.Open(reqPath)
		if err != nil {
			idx, readErr := distFS.ReadFile("dist/index.html")
			if readErr != nil {
				return c.Status(http.StatusNotFound).SendString("index.html not found")
			}
			c.Type("html")
			return c.Send(idx)
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil || info.IsDir() {
			idx, readErr := distFS.ReadFile("dist/index.html")
			if readErr != nil {
				return c.Status(http.StatusNotFound).SendString("index.html not found")
			}
			c.Type("html")
			return c.Send(idx)
		}

		ext := path.Ext(reqPath)
		switch ext {
		case ".js":
			c.Type("js")
		case ".css":
			c.Type("css")
		case ".svg":
			c.Type("svg")
		case ".png":
			c.Type("png")
		case ".ico":
			c.Type("x-icon")
		case ".woff":
			c.Type("font/woff")
		case ".woff2":
			c.Type("font/woff2")
		case ".html":
			c.Type("html")
		}

		content, err := fs.ReadFile(sub, reqPath)
		if err != nil {
			return c.Status(http.StatusNotFound).SendString("file not found")
		}

		return c.Send(content)
	}
}
