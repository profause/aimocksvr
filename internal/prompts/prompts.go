// Package prompts builds the reusable chat messages sent to AI providers.
//
// Prompt templates live here instead of in the ai client so that every
// generation task shares one authoritative system prompt and message shape.
// Endpoint prompts are expected to describe business behavior (e.g. "return a
// user with a full name and email") rather than JSON syntax; the templates
// below keep the model focused on producing machine-readable JSON.
package prompts

import "strings"

// Message is a single chat message.
type Message struct {
	Role    string
	Content string
}

const (
	RoleSystem = "system"
	RoleUser   = "user"
)

// System returns the base system prompt shared by every generation task. It
// encodes the invariants the whole mock server relies on: only JSON leaves the
// model, Markdown is never emitted, schemas are honored, and output is as
// deterministic as the backend allows.
func System() string {
	return "You are a mock API server. " +
		"Return only valid JSON. " +
		"Never return Markdown. " +
		"Respect any JSON Schema you are given. " +
		"Produce deterministic output when possible."
}

// SchemaRequest returns the messages that ask a model to infer the JSON Schema
// of an endpoint's response from its behavior prompt.
func SchemaRequest(endpointPrompt string) []Message {
	return []Message{
		{Role: RoleSystem, Content: System()},
		{Role: RoleUser, Content: "Describe the JSON response for this mock endpoint.\n" +
			"Return only a JSON Schema object.\n\nEndpoint prompt:\n" + endpointPrompt},
	}
}

// ResponseRequest returns the messages that ask a model to produce a JSON
// response body for a mock endpoint described by its behavior prompt. When a
// JSON Schema is provided, the model is told to match it exactly so the stored
// schema and the generated responses stay consistent.
func ResponseRequest(endpointPrompt, schema string) []Message {
	user := "Return the JSON response for this mock endpoint.\n"
	if strings.TrimSpace(schema) != "" {
		user += "Match this JSON Schema exactly:\n" + schema + "\n"
	}
	user += "\nEndpoint prompt:\n" + endpointPrompt
	return []Message{
		{Role: RoleSystem, Content: System()},
		{Role: RoleUser, Content: user},
	}
}
