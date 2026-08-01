import { useState, useEffect, useRef } from 'react'
import { cn } from '@/lib/utils'

interface Section {
  id: string
  title: string
  content: React.ReactNode
}

function CodeBlock({ children, className }: { children: string; className?: string }) {
  return (
    <pre className={cn('overflow-x-auto rounded-lg bg-muted p-4 text-sm font-mono', className)}>
      <code>{children}</code>
    </pre>
  )
}

function InlineCode({ children }: { children: string }) {
  return (
    <code className="rounded bg-muted px-1.5 py-0.5 text-sm font-mono">{children}</code>
  )
}

function Heading({ id, children }: { id: string; children: React.ReactNode }) {
  return (
    <h2 id={id} className="scroll-mt-20 text-xl font-semibold tracking-tight">
      {children}
    </h2>
  )
}

function SubHeading({ id, children }: { id: string; children: React.ReactNode }) {
  return (
    <h3 id={id} className="scroll-mt-20 text-lg font-semibold tracking-tight">
      {children}
    </h3>
  )
}

function Table({ children }: { children: React.ReactNode }) {
  return (
    <div className="overflow-x-auto rounded-lg border">
      <table className="w-full text-sm">{children}</table>
    </div>
  )
}

const sections: Section[] = [
  {
    id: 'overview',
    title: 'Overview',
    content: (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">
          MockSvr is an AI-powered API mock server that dynamically generates REST API responses.
          Create mock endpoints without writing backend code, import existing specs, and simulate
          real-world API behavior.
        </p>
        <p className="text-muted-foreground">
          Every request to a mock endpoint is resolved from the database, generates a response
          on the fly, and returns JSON — no restart required after creating or modifying endpoints.
        </p>
      </div>
    ),
  },
  {
    id: 'response-format',
    title: 'Response Format',
    content: (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">
          All control plane API responses use a consistent JSON envelope.
        </p>
        <SubHeading id="success-response">Success</SubHeading>
        <CodeBlock>{`{ "success": true, "data": {} }`}</CodeBlock>
        <SubHeading id="error-response">Error</SubHeading>
        <CodeBlock>{`{ "success": false, "error": { "code": "...", "message": "..." } }`}</CodeBlock>
      </div>
    ),
  },
  {
    id: 'endpoint-registry',
    title: 'Endpoint Registry',
    content: (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">
          The endpoint registry is the core of MockSvr. Create, read, update, and delete mock
          endpoints through the control plane API.
        </p>
        <SubHeading id="routes">Routes</SubHeading>
        <Table>
          <thead className="bg-muted">
            <tr>
              <th className="px-3 py-2 text-left font-medium">Method</th>
              <th className="px-3 py-2 text-left font-medium">Path</th>
              <th className="px-3 py-2 text-left font-medium">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-t">
              <td className="px-3 py-2"><InlineCode>GET</InlineCode></td>
              <td className="px-3 py-2 font-mono text-xs">/api/v1/endpoints</td>
              <td className="px-3 py-2">List endpoints (paginated)</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2"><InlineCode>POST</InlineCode></td>
              <td className="px-3 py-2 font-mono text-xs">/api/v1/endpoints</td>
              <td className="px-3 py-2">Create endpoint</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2"><InlineCode>GET</InlineCode></td>
              <td className="px-3 py-2 font-mono text-xs">/api/v1/endpoints/:id</td>
              <td className="px-3 py-2">Get endpoint</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2"><InlineCode>PUT</InlineCode></td>
              <td className="px-3 py-2 font-mono text-xs">/api/v1/endpoints/:id</td>
              <td className="px-3 py-2">Replace endpoint</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2"><InlineCode>DELETE</InlineCode></td>
              <td className="px-3 py-2 font-mono text-xs">/api/v1/endpoints/:id</td>
              <td className="px-3 py-2">Delete endpoint</td>
            </tr>
          </tbody>
        </Table>
        <SubHeading id="create-example">Create an Endpoint</SubHeading>
        <CodeBlock>{`curl -X POST http://localhost:8080/api/v1/endpoints \\
  -H 'Content-Type: application/json' \\
  -d '{"method":"post","path":"/users","prompt":"create a user and return it"}'`}</CodeBlock>
        <p className="text-muted-foreground text-sm">
          Creating an endpoint records a new version (version 1 on create). Each version is a full
          snapshot — method, path, description, prompt, response type, stateful, status,
          request_schema, error_sim, public, and the response schema.
        </p>
      </div>
    ),
  },
  {
    id: 'versioning',
    title: 'Versioning',
    content: (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">
          Every endpoint modification creates a new version. Version history, diffs, and rollbacks
          are supported.
        </p>
        <Table>
          <thead className="bg-muted">
            <tr>
              <th className="px-3 py-2 text-left font-medium">Method</th>
              <th className="px-3 py-2 text-left font-medium">Path</th>
              <th className="px-3 py-2 text-left font-medium">Description</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-t">
              <td className="px-3 py-2"><InlineCode>GET</InlineCode></td>
              <td className="px-3 py-2 font-mono text-xs">/api/v1/endpoints/:id/versions</td>
              <td className="px-3 py-2">List endpoint versions</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2"><InlineCode>GET</InlineCode></td>
              <td className="px-3 py-2 font-mono text-xs">/api/v1/endpoints/:id/versions/:version/diff</td>
              <td className="px-3 py-2">Diff a version vs the latest</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2"><InlineCode>POST</InlineCode></td>
              <td className="px-3 py-2 font-mono text-xs">/api/v1/endpoints/:id/versions/:version/rollback</td>
              <td className="px-3 py-2">Roll back to a version</td>
            </tr>
          </tbody>
        </Table>
        <SubHeading id="diff-example">Diff Response</SubHeading>
        <CodeBlock>{`{
  "success": true,
  "data": {
    "version": 1,
    "changes": [
      {"field": "path", "from": "/v1", "to": "/v2"},
      ...
    ]
  }
}`}</CodeBlock>
        <p className="text-muted-foreground text-sm">
          Rollback itself becomes a new version, so it can be inspected or undone. Rolling back
          to the latest version or a non-existent version is rejected with
          <InlineCode>VALIDATION_ERROR</InlineCode>.
        </p>
      </div>
    ),
  },
  {
    id: 'request-history',
    title: 'Request History',
    content: (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">
          Every served request is recorded and visible via the history endpoint.
        </p>
        <CodeBlock>{`GET /api/v1/endpoints/:id/history`}</CodeBlock>
        <p className="text-muted-foreground text-sm">
          Returns the request body, response body, and latency for each recorded call.
        </p>
      </div>
    ),
  },
  {
    id: 'dynamic-routing',
    title: 'Dynamic Routing',
    content: (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">
          Registered mock endpoints are served without a restart. The router resolves every request
          against the database and generates a response on the fly. Routes outside the control plane
          (<InlineCode>/health</InlineCode>, <InlineCode>/api/v1/*</InlineCode>) are treated as
          mock endpoints.
        </p>
        <SubHeading id="path-parameters">Path Parameters</SubHeading>
        <p className="text-muted-foreground">
          Path patterns support <InlineCode>:param</InlineCode> segments. Static segments take
          precedence over parameters when both match.
        </p>
        <CodeBlock>{`# Create an endpoint with a path parameter
curl -X POST http://localhost:8080/api/v1/endpoints \\
  -H 'Content-Type: application/json' \\
  -d '{"method":"get","path":"/users/:id","prompt":"return a user by id"}'

# Call the mock endpoint
curl http://localhost:8080/users/42`}</CodeBlock>
        <p className="text-muted-foreground text-sm">
          Only <InlineCode>status: active</InlineCode> endpoints are served.
        </p>
      </div>
    ),
  },
  {
    id: 'variable-injection',
    title: 'Variable Injection',
    content: (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">
          Prompts support request variables. Before the prompt reaches the AI provider, placeholders
          are replaced with values from the incoming request:
        </p>
        <Table>
          <thead className="bg-muted">
            <tr>
              <th className="px-3 py-2 text-left font-medium">Placeholder</th>
              <th className="px-3 py-2 text-left font-medium">Source</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-t">
              <td className="px-3 py-2 font-mono text-xs">{'{{path.id}}'}</td>
              <td className="px-3 py-2">Path parameter</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2 font-mono text-xs">{'{{query.country}}'}</td>
              <td className="px-3 py-2">Query parameter</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2 font-mono text-xs">{'{{body.email}}'}</td>
              <td className="px-3 py-2">JSON body field (dot paths supported)</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2 font-mono text-xs">{'{{headers.auth}}'}</td>
              <td className="px-3 py-2">Request header (case-insensitive)</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2 font-mono text-xs">{'{{cookies.session}}'}</td>
              <td className="px-3 py-2">Request cookie</td>
            </tr>
          </tbody>
        </Table>
        <p className="text-muted-foreground text-sm">
          Names are case-insensitive and missing values render as an empty string.
          Dot paths like <InlineCode>{'{{body.user.name}}'}</InlineCode> are supported.
        </p>
      </div>
    ),
  },
  {
    id: 'request-schema',
    title: 'Request Schema Validation',
    content: (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">
          Endpoints can declare a <InlineCode>request_schema</InlineCode> — a JSON Schema describing
          the expected request body. Before any response is generated, the incoming body is validated
          against it.
        </p>
        <p className="text-muted-foreground">
          A non-conforming or missing body is rejected with
          <InlineCode>400 VALIDATION_ERROR</InlineCode>. Formats like <InlineCode>email</InlineCode>,
          <InlineCode>uuid</InlineCode>, and <InlineCode>date-time</InlineCode> are enforced.
          An empty schema disables validation for that endpoint.
        </p>
      </div>
    ),
  },
  {
    id: 'stateful-endpoints',
    title: 'Stateful Endpoints',
    content: (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">
          Endpoints flagged <InlineCode>"stateful": true</InlineCode> act as persistent mock APIs.
          A <InlineCode>POST</InlineCode> creates a resource, and the same object is returned by
          <InlineCode>GET</InlineCode>, updated by <InlineCode>PUT</InlineCode>/<InlineCode>PATCH</InlineCode>,
          and removed by <InlineCode>DELETE</InlineCode>.
        </p>
        <CodeBlock>{`# Create a stateful endpoint
curl -X POST http://localhost:8080/api/v1/endpoints \\
  -H 'Content-Type: application/json' \\
  -d '{"method":"post","path":"/users","prompt":"create a user","stateful":true}'

# Create a resource
curl -X POST http://localhost:8080/users \\
  -H 'Content-Type: application/json' \\
  -d '{"email":"a@b.co"}'
# => {"email":"a@b.co","id":"<uuid>",...}  201

# Retrieve the resource
curl http://localhost:8080/users/1

# Update the resource
curl -X PATCH http://localhost:8080/users/1 \\
  -H 'Content-Type: application/json' \\
  -d '{"name":"Ada"}'

# Delete the resource
curl -X DELETE http://localhost:8080/users/1`}</CodeBlock>
        <p className="text-muted-foreground text-sm">
          Resources are keyed by the endpoint's collection path plus the resource id from the path.
          Duplicate POST ids return <InlineCode>409</InlineCode>. Resources are stored in
          PostgreSQL so they survive server restarts.
        </p>
      </div>
    ),
  },
  {
    id: 'openapi-import',
    title: 'OpenAPI Import',
    content: (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">
          Existing APIs can be bootstrapped from their spec. Accepts a Swagger 2.0 or OpenAPI 3.x
          document in JSON or YAML, either as the raw request body or as a multipart upload.
        </p>
        <CodeBlock>{`curl -X POST http://localhost:8080/api/v1/imports/openapi \\
  -H 'Content-Type: application/json' \\
  --data @openapi.json

# Or as a file upload
curl -X POST http://localhost:8080/api/v1/imports/openapi \\
  -F file=@swagger.yaml`}</CodeBlock>
        <p className="text-muted-foreground text-sm">
          OpenAPI path templates become router parameters:
          {'/users/{id}'} → <InlineCode>/users/:id</InlineCode>.
          The operation summary and description become the endpoint's prompt. Re-importing is
          idempotent — operations whose method + path already exist are reported as
          <InlineCode>skipped</InlineCode>.
        </p>
      </div>
    ),
  },
  {
    id: 'postman-import',
    title: 'Postman Import',
    content: (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">
          Accepts a Postman Collection v2.0/v2.1 JSON document as raw body or multipart file upload.
        </p>
        <CodeBlock>{`curl -X POST http://localhost:8080/api/v1/imports/postman \\
  -H 'Content-Type: application/json' \\
  --data @collection.json`}</CodeBlock>
        <p className="text-muted-foreground text-sm">
          Every request in the collection becomes a mock endpoint. Postman path templates already
          use router syntax, and <InlineCode>{'{{variable}}'}</InlineCode> placeholders are
          converted to <InlineCode>:param</InlineCode>. When the request body is JSON, a JSON
          Schema is inferred and stored as <InlineCode>request_schema</InlineCode>.
        </p>
      </div>
    ),
  },
  {
    id: 'error-simulation',
    title: 'Error Simulation',
    content: (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">
          Any endpoint can simulate failure modes by setting <InlineCode>error_sim</InlineCode> to
          a JSON config string on create/update.
        </p>
        <CodeBlock>{`curl -X POST http://localhost:8080/api/v1/endpoints \\
  -H 'Content-Type: application/json' \\
  -d '{"method":"get","path":"/payments","prompt":"charge a card",
       "error_sim":"{\\"status\\":500,\\"failure_rate\\":30}"}'`}</CodeBlock>
        <SubHeading id="error-sim-fields">Supported Fields</SubHeading>
        <Table>
          <thead className="bg-muted">
            <tr>
              <th className="px-3 py-2 text-left font-medium">Field</th>
              <th className="px-3 py-2 text-left font-medium">Effect</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-t">
              <td className="px-3 py-2 font-mono text-xs">latency_ms</td>
              <td className="px-3 py-2">Artificial delay on every request</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2 font-mono text-xs">timeout_ms</td>
              <td className="px-3 py-2">Hold connection, then drop (timeout)</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2 font-mono text-xs">drop_connection</td>
              <td className="px-3 py-2">Close connection immediately</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2 font-mono text-xs">malformed_json</td>
              <td className="px-3 py-2">Respond 200 with invalid JSON</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2 font-mono text-xs">status</td>
              <td className="px-3 py-2">Respond with this status (400-599)</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2 font-mono text-xs">failure_rate</td>
              <td className="px-3 py-2">0-100 percentage of requests that fail</td>
            </tr>
          </tbody>
        </Table>
        <p className="text-muted-foreground text-sm">
          <InlineCode>latency_ms</InlineCode> always applies. Failure behaviors are rolled against
          <InlineCode>failure_rate</InlineCode>. When several are configured, priority is:
          timeout_ms &gt; drop_connection &gt; malformed_json &gt; status.
        </p>
      </div>
    ),
  },
  {
    id: 'authentication',
    title: 'Authentication',
    content: (
      <div className="flex flex-col gap-4">
        <p className="text-muted-foreground">
          Auth is disabled by default. Enable it with <InlineCode>MOCKSVR_AUTH_ENABLED=true</InlineCode>
          and configure credentials.
        </p>
        <SubHeading id="credential-types">Credential Types</SubHeading>
        <Table>
          <thead className="bg-muted">
            <tr>
              <th className="px-3 py-2 text-left font-medium">Type</th>
              <th className="px-3 py-2 text-left font-medium">Header</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-t">
              <td className="px-3 py-2">API Key</td>
              <td className="px-3 py-2 font-mono text-xs">Authorization: Bearer &lt;key&gt; or X-API-Key: &lt;key&gt;</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2">Workspace Token</td>
              <td className="px-3 py-2 font-mono text-xs">Authorization: Bearer &lt;token&gt; or X-Workspace-Token: &lt;token&gt;</td>
            </tr>
            <tr className="border-t">
              <td className="px-3 py-2">JWT</td>
              <td className="px-3 py-2 font-mono text-xs">Authorization: Bearer &lt;jwt&gt;</td>
            </tr>
          </tbody>
        </Table>
        <SubHeading id="minting-tokens">Minting a JWT</SubHeading>
        <CodeBlock>{`curl -X POST http://localhost:8080/api/v1/auth/token \\
  -H 'Content-Type: application/json' \\
  -d '{"api_key":"sk_test_123"}'
# => {"success":true,"data":{"token":"eyJ...","kind":"api_key","name":"dev"}}`}</CodeBlock>
        <SubHeading id="whoami">Who Am I</SubHeading>
        <CodeBlock>{`curl http://localhost:8080/api/v1/auth/whoami \\
  -H 'Authorization: Bearer eyJ...'
# => {"success":true,"data":{"kind":"jwt","name":"dev"}}`}</CodeBlock>
        <SubHeading id="private-endpoints">Private Endpoints</SubHeading>
        <p className="text-muted-foreground">
          Mock endpoints are public by default. Set <InlineCode>"public": false</InlineCode> when
          creating or updating an endpoint to require authentication.
        </p>
        <CodeBlock>{`curl -X POST http://localhost:8080/api/v1/endpoints \\
  -H 'Content-Type: application/json' \\
  -H 'X-API-Key: sk_test_123' \\
  -d '{"method":"get","path":"/secret","prompt":"a private endpoint","public":false}'

curl http://localhost:8080/secret                      # 401
curl http://localhost:8080/secret -H 'X-API-Key: sk_test_123'   # 200`}</CodeBlock>
      </div>
    ),
  },
]

export function DocsPage() {
  const [activeSection, setActiveSection] = useState('overview')
  const observerRef = useRef<IntersectionObserver | null>(null)

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            setActiveSection(entry.target.id)
          }
        })
      },
      { rootMargin: '-20% 0px -70% 0px' }
    )

    sections.forEach((section) => {
      const el = document.getElementById(section.id)
      if (el) observer.observe(el)
    })

    observerRef.current = observer
    return () => observer.disconnect()
  }, [])

  return (
    <div className="flex gap-8">
      {/* Table of Contents */}
      <aside className="hidden w-56 shrink-0 lg:block">
        <nav className="sticky top-20 flex flex-col gap-1">
          <p className="mb-2 text-sm font-semibold">On this page</p>
          {sections.map((section) => (
            <a
              key={section.id}
              href={`#${section.id}`}
              className={cn(
                'block rounded-md px-2 py-1 text-sm transition-colors',
                activeSection === section.id
                  ? 'bg-accent text-accent-foreground font-medium'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              {section.title}
            </a>
          ))}
        </nav>
      </aside>

      {/* Content */}
      <main className="min-w-0 flex-1">
        <div className="mb-8">
          <h1 className="text-2xl font-bold tracking-tight">Documentation</h1>
          <p className="text-muted-foreground mt-1">
            User guide and API reference for MockSvr.
          </p>
        </div>

        <div className="flex flex-col gap-12">
          {sections.map((section) => (
            <section key={section.id} id={section.id} className="flex flex-col gap-4">
              <Heading id={section.id}>{section.title}</Heading>
              {section.content}
            </section>
          ))}
        </div>
      </main>
    </div>
  )
}
