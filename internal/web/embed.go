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

// Register serves the embedded SPA. Non-API requests that don't match a
// static asset fall back to index.html for client-side routing.
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
			// File not found — serve index.html for SPA routing
			idx, readErr := distFS.ReadFile("dist/index.html")
			if readErr != nil {
				return c.Next()
			}
			c.Type("html")
			return c.Send(idx)
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil || info.IsDir() {
			idx, readErr := distFS.ReadFile("dist/index.html")
			if readErr != nil {
				return c.Next()
			}
			c.Type("html")
			return c.Send(idx)
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
