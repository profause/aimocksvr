````markdown
# AGENT.md

# Execution Rules

- Never implement multiple phases in a single iteration unless explicitly requested.
- Before starting any phase, explain the implementation plan.
- After completing a phase, summarize the completed work and wait for approval before proceeding.
- If an architectural decision is unclear, choose the simplest production-ready solution and document the rationale.
- Do not introduce unnecessary dependencies.
- Favor maintainability and extensibility over premature optimization.

## Project

AI Mock Server

## Mission

Build an AI-powered API mock server that enables developers to create realistic REST APIs without writing backend code.

The server should dynamically generate responses using AI while supporting deterministic behavior, stateful resources, OpenAPI imports, configurable testing scenarios, and reusable mock data.

The project should be modular, production-ready, and designed to support both self-hosted deployments and a future SaaS offering.

---

# Core Principles

- Always produce clean, idiomatic Go code.
- Prefer composition over inheritance.
- Keep business logic independent from HTTP handlers.
- Follow SOLID principles.
- Use dependency injection.
- Avoid global mutable state.
- Every feature should be testable.
- Every public API should be documented.
- Every package should have a single responsibility.
- Keep functions small and readable.
- Avoid unnecessary abstractions.

---

# Tech Stack

## Backend

- Go 1.26+
- Fiber v3
- Bun ORM
- PostgreSQL
- Redis
- OpenAI-compatible providers
- Docker

## Frontend (Future)

- React
- TailwindCSS
- shadcn/ui
- TanStack Query

---

# Architecture

```
Client
    │
    ▼
Fiber Router
    │
    ▼
Dynamic Endpoint Resolver
    │
    ▼
Response Generator
    ├── Faker
    ├── AI Provider
    ├── JSON Schema Validator
    └── Stateful Storage
    │
    ▼
HTTP Response
```

---

# Project Structure

```
cmd/
    server/

internal/
    ai/
    api/
    auth/
    cache/
    config/
    database/
    endpoint/
    generator/
    importer/
    middleware/
    models/
    prompts/
    router/
    scenarios/
    state/
    storage/
    validator/

pkg/

configs/

docs/

migrations/
```

---

# Development Roadmap

## Phase 1 — Core Server

Implement:

- HTTP server
- Health endpoint
- Configuration
- Logging
- Database connection
- Graceful shutdown

Deliverable

```
GET /health

{
    "status": "ok"
}
```

---

## Phase 2 — Endpoint Registry

Create models for:

### Endpoint

Fields

- id
- method
- path
- description
- prompt
- response_type
- status
- created_at
- updated_at

### EndpointVersion

Fields

- id
- endpoint_id
- prompt
- schema
- version

### RequestHistory

Fields

- id
- endpoint_id
- request
- response
- latency
- created_at

Expose CRUD endpoints.

---

## Phase 3 — Dynamic Router

Do not register routes statically.

Every request should:

1. Resolve endpoint from database
2. Load configuration
3. Generate response
4. Return JSON

No restart should ever be required after creating an endpoint.

---

## Phase 4 — AI Providers

Create an abstraction.

```go
type AIProvider interface {
    GenerateSchema(...)
    GenerateResponse(...)
    GeneratePrompt(...)
}
```

Support:

- OpenAI
- Ollama
- OpenRouter

Switch providers through configuration.

---

## Phase 5 — Prompt Templates

Create reusable prompts.

System prompt:

- Return valid JSON
- Never return Markdown
- Respect JSON Schema
- Produce deterministic output when possible

Endpoint prompts should describe business behavior instead of JSON syntax.

---

## Phase 6 — JSON Schema

When an endpoint is created:

Prompt

↓

AI generates schema

↓

Store schema

↓

Validate every future response

Only regenerate when the prompt changes.

---

## Phase 7 — Response Generation

Flow

```
Request
    ↓
Load Endpoint
    ↓
Load Schema
    ↓
Inject Variables
    ↓
Generate Response
    ↓
Validate
    ↓
Return JSON
```

Retry once if validation fails.

---

## Phase 8 — Variable Injection

Support variables.

Examples

```
{{path.id}}

{{query.country}}

{{body.email}}

{{headers.authorization}}

{{cookies.session}}
```

Variables should be available inside prompts.

---

## Phase 9 — Stateful Resources

Support persistent mock APIs.

Example

```
POST /users

creates

↓

GET /users/:id

returns same object

↓

DELETE

removes object
```

Support

- POST
- GET
- PUT
- PATCH
- DELETE

---

## Phase 10 — Faker Integration

AI defines structure.

Faker generates values.

Supported generators

- Person
- Company
- Email
- Phone
- Address
- UUID
- Bank
- Credit Card
- Date
- Currency
- Country

---

## Phase 11 — OpenAPI Import

Support

- swagger.json
- swagger.yaml

Workflow

```
Upload

↓

Parse

↓

Generate Endpoints

↓

Generate Prompts

↓

Store
```

---

## Phase 12 — Postman Import

Support

- collection.json

Infer

- methods
- paths
- requests
- responses

---

## Phase 13 — Error Simulation

Per endpoint support

- latency
- timeout
- HTTP 500
- HTTP 404
- HTTP 429
- malformed JSON
- dropped connection
- random failure percentage

---

## Phase 14 — Scenarios

Allow chained workflows.

Example

```
Login

↓

Create Customer

↓

Create Wallet

↓

Transfer

↓

Logout
```

---

## Phase 15 — Versioning

Every endpoint modification creates a new version.

Support

- history
- rollback
- diff

---

## Phase 16 — Authentication

Support

- Public endpoints
- API Keys
- JWT
- Workspace tokens

---

## Phase 17 — Dashboard

Features

- Endpoint management
- Prompt editor
- Schema editor
- Request logs
- Version history
- Live testing

# Required Reading

Before making any changes, read the following documents in order.

`FRONTEND.md`
   - React dashboard architecture
   - UI implementation plan
   - Component guidelines

---

## Phase 18 — CLI

Commands

```
mocksvr serve

mocksvr import swagger.yaml

mocksvr export

mocksvr login

mocksvr doctor
```

---

## Phase 19 — Docker

Provide

```
docker compose up
```

Should start

- API
- PostgreSQL
- Redis

---

## Phase 20 — Testing

### Unit Tests

- Generator
- Router
- Validator
- Importers
- Storage

### Integration Tests

- CRUD flow
- Stateful endpoints
- AI generation
- Error simulation

Benchmark

- Response latency
- AI latency
- Cache hit rate

---

# Future Features

- GraphQL mocking
- gRPC mocking
- SOAP mocking
- WebSocket mocking
- Webhook simulator
- Traffic recorder
- Traffic replay
- Contract testing
- Git synchronization
- VS Code extension
- MCP server
- SaaS workspaces
- Team collaboration
- CI/CD integration

---

# Coding Standards

## Code Quality

- Keep packages cohesive.
- Keep functions under ~50 lines where practical.
- Avoid deeply nested conditionals.
- Prefer explicit code over clever code.

## Error Handling

- Never ignore returned errors.
- Wrap errors with context.
- Return structured API errors.

## Database

- Use Bun ORM.
- Use transactions where appropriate.
- Never build SQL using string concatenation.

## HTTP

- Return consistent JSON responses.
- Validate requests.
- Validate generated responses against stored JSON Schemas.

## AI

- Never trust AI output without validation.
- Cache generated schemas.
- Minimize AI calls.
- Use deterministic prompts when possible.

## Testing

Every new feature should include:

- Unit tests
- Integration tests (where applicable)

---

# Definition of Done

A task is complete only if:

- Code builds successfully.
- Tests pass.
- Documentation is updated.
- Public APIs are documented.
- No linting issues remain.
- Feature works end-to-end.
- Code follows project conventions.

---

# Long-Term Goal

Build the best AI-powered API virtualization platform that enables frontend developers, mobile developers, QA engineers, and integration teams to simulate production-quality APIs without requiring a live backend.
````
