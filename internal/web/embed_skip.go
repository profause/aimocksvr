// Package web is a no-op when the frontend is not embedded.
//
//go:build skip_embed

package web

import "github.com/gofiber/fiber/v3"

// Register is a no-op when the skip_embed build tag is set.
func Register(app *fiber.App) {}
