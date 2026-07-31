package endpoint

// CreateEndpointParams is the input for creating an endpoint. Method, Path and
// Prompt are required; the remaining fields fall back to sensible defaults.
type CreateEndpointParams struct {
	Method        string `json:"method"`
	Path          string `json:"path"`
	Description   string `json:"description"`
	Prompt        string `json:"prompt"`
	ResponseType  string `json:"response_type"`
	Stateful      bool   `json:"stateful"`
	Status        string `json:"status"`
	RequestSchema string `json:"request_schema"`
	ErrorSim      string `json:"error_sim"`
	Public        *bool  `json:"public"`
}

// UpdateEndpointParams is the input for replacing an endpoint. PUT semantics:
// all provided values replace the existing ones; an omitted stateful field
// keeps the current value.
type UpdateEndpointParams struct {
	Method        string  `json:"method"`
	Path          string  `json:"path"`
	Description   string  `json:"description"`
	Prompt        string  `json:"prompt"`
	ResponseType  string  `json:"response_type"`
	Stateful      *bool   `json:"stateful"`
	Status        string  `json:"status"`
	RequestSchema *string `json:"request_schema"`
	ErrorSim      *string `json:"error_sim"`
	Public        *bool   `json:"public"`
}

// ListParams controls pagination of List responses.
type ListParams struct {
	Page  int
	Limit int
}
