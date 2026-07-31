// Package api provides helpers that produce the consistent JSON response
// envelope used by every HTTP endpoint of the server.
package api

import "github.com/gofiber/fiber/v3"

// Error codes used across the API. Codes are stable and safe for clients to
// branch on, unlike human-readable messages.
const (
	CodeInvalidJSON     = "INVALID_JSON"
	CodeInvalidID       = "INVALID_ID"
	CodeValidationError = "VALIDATION_ERROR"
	CodeNotFound        = "NOT_FOUND"
	CodeConflict        = "CONFLICT"
	CodeInternalError   = "INTERNAL_ERROR"
	CodeErrorSimulation = "ERROR_SIMULATION"
	CodeUnauthorized    = "UNAUTHORIZED"
)

// Response is the envelope for successful responses.
type Response struct {
	Success bool `json:"success"`
	Data    any  `json:"data,omitempty"`
}

// ErrorResponse is the envelope for failed responses.
type ErrorResponse struct {
	Success bool  `json:"success"`
	Error   Error `json:"error"`
}

// Error describes a single failure returned to the client.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OK writes a 200 response with data.
func OK(c fiber.Ctx, data any) error {
	return c.Status(fiber.StatusOK).JSON(Response{Success: true, Data: data})
}

// Created writes a 201 response with data.
func Created(c fiber.Ctx, data any) error {
	return c.Status(fiber.StatusCreated).JSON(Response{Success: true, Data: data})
}

// Fail writes an error response with the given status, code and message.
func Fail(c fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(ErrorResponse{
		Success: false,
		Error:   Error{Code: code, Message: message},
	})
}
