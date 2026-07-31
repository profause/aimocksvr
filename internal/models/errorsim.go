package models

import (
	"encoding/json"
	"strings"
)

// ErrorSimulation configures per-endpoint error behavior (Phase 13). It is
// stored on the endpoint as JSON text; a zero/absent value disables
// simulation.
//
// LatencyMs adds an artificial delay to every served request. The remaining
// fields are failures gated by FailureRate (0 or 100 means the configured
// failure always applies, otherwise it applies in the given percentage of
// requests). When several failures are configured, the first of TimeoutMs,
// DropConnection, MalformedJSON and Status wins.
type ErrorSimulation struct {
	LatencyMs      int  `json:"latency_ms"`
	TimeoutMs      int  `json:"timeout_ms"`
	Status         int  `json:"status"`
	MalformedJSON  bool `json:"malformed_json"`
	DropConnection bool `json:"drop_connection"`
	FailureRate    int  `json:"failure_rate"`
}

// UnmarshalErrorSim decodes an endpoint's stored error_sim JSON text. Empty
// or blank input yields no simulation; invalid JSON returns an error so
// callers can fall back to serving normally.
func UnmarshalErrorSim(text string) (*ErrorSimulation, error) {
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}
	var sim ErrorSimulation
	if err := json.Unmarshal([]byte(text), &sim); err != nil {
		return nil, err
	}
	return &sim, nil
}

// ShouldFail reports whether the configured failure triggers for the given
// roll (0-99). A FailureRate of 0 (unset) or 100 means the failure always
// applies; otherwise it applies when roll is below the rate.
func (s *ErrorSimulation) ShouldFail(roll int) bool {
	if s.FailureRate == 0 || s.FailureRate >= 100 {
		return true
	}
	return roll < s.FailureRate
}
