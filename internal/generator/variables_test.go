package generator

import (
	"testing"

	"github.com/google/uuid"
	"github.com/profause/aimocksvr/internal/models"
)

func testRequest() *Request {
	return &Request{
		Endpoint: &models.Endpoint{ID: uuid.New()},
		PathParams: map[string]string{
			"id":      "42",
			"Country": "us",
		},
		Query: map[string]string{
			"country": "fr",
		},
		Headers: map[string][]string{
			"Authorization": {"Bearer tok"},
			"Cookie":        {"session=abc123; theme=dark"},
		},
		Body: []byte(`{"email":"a@b.co","user":{"name":"Ada","score":42.5},"tags":["x","y"],"active":true,"note":null}`),
	}
}

func TestRenderVariables_Path(t *testing.T) {
	req := testRequest()
	got := RenderVariables("get user {{path.id}} from {{PATH.COUNTRY}}", req)
	want := "get user 42 from us"
	if got != want {
		t.Fatalf("RenderVariables = %q, want %q", got, want)
	}
}

func TestRenderVariables_Query(t *testing.T) {
	got := RenderVariables("country is {{query.country}}", testRequest())
	if want := "country is fr"; got != want {
		t.Fatalf("RenderVariables = %q, want %q", got, want)
	}
}

func TestRenderVariables_Headers(t *testing.T) {
	req := testRequest()
	got := RenderVariables("token {{headers.authorization}}", req)
	if want := "token Bearer tok"; got != want {
		t.Fatalf("RenderVariables = %q, want %q", got, want)
	}
}

func TestRenderVariables_Cookies(t *testing.T) {
	req := testRequest()
	got := RenderVariables("session {{cookies.session}} theme {{cookies.THEME}}", req)
	if want := "session abc123 theme dark"; got != want {
		t.Fatalf("RenderVariables = %q, want %q", got, want)
	}
}

func TestRenderVariables_Body(t *testing.T) {
	req := testRequest()
	got := RenderVariables("email {{body.email}}", req)
	if want := "email a@b.co"; got != want {
		t.Fatalf("RenderVariables = %q, want %q", got, want)
	}
}

func TestRenderVariables_BodyNested(t *testing.T) {
	req := testRequest()
	got := RenderVariables("{{body.user.name}} scored {{body.user.score}}", req)
	if want := "Ada scored 42.5"; got != want {
		t.Fatalf("RenderVariables = %q, want %q", got, want)
	}
}

func TestRenderVariables_BodyScalarTypes(t *testing.T) {
	req := testRequest()
	got := RenderVariables("active={{body.active}} tags={{body.tags}} note=[{{body.note}}]", req)
	want := `active=true tags=["x","y"] note=[]`
	if got != want {
		t.Fatalf("RenderVariables = %q, want %q", got, want)
	}
}

func TestRenderVariables_Multiple(t *testing.T) {
	req := testRequest()
	got := RenderVariables("{{path.id}}-{{query.country}}-{{body.email}}", req)
	if want := "42-fr-a@b.co"; got != want {
		t.Fatalf("RenderVariables = %q, want %q", got, want)
	}
}

func TestRenderVariables_MissingRendersEmpty(t *testing.T) {
	got := RenderVariables("user [{{path.nope}}] flag [{{query.missing}}]", testRequest())
	if want := "user [] flag []"; got != want {
		t.Fatalf("RenderVariables = %q, want %q", got, want)
	}
}

func TestRenderVariables_NoPlaceholders(t *testing.T) {
	prompt := "return a random user"
	if got := RenderVariables(prompt, testRequest()); got != prompt {
		t.Fatalf("RenderVariables = %q, want %q", got, prompt)
	}
}

func TestRenderVariables_MalformedPlaceholderLeftAlone(t *testing.T) {
	prompt := "{{path.id and {{unclosed"
	if got := RenderVariables(prompt, testRequest()); got != prompt {
		t.Fatalf("RenderVariables = %q, want %q", got, prompt)
	}
}

func TestRenderVariables_NonJSONBody(t *testing.T) {
	req := testRequest()
	req.Body = []byte("not json")
	got := RenderVariables("email [{{body.email}}]", req)
	if want := "email []"; got != want {
		t.Fatalf("RenderVariables = %q, want %q", got, want)
	}
}

func TestRenderVariables_NilRequest(t *testing.T) {
	prompt := "plain {{path.id}}"
	if got := RenderVariables(prompt, nil); got != prompt {
		t.Fatalf("RenderVariables = %q, want %q", got, prompt)
	}
}
