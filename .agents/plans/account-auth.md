# Plan: Account-Based Authentication (Email/Password + JWT + account_id Scoping)

> Branch: `dockerise`. Status: draft, awaiting owner review before implementation.
> Builds on the current model (Fiber v3, Bun ORM, PostgreSQL, uber-fx, golang-migrate) and the existing `internal/auth` JWT machinery.

## 1. Objective

1. Users create an **account** with an email and password (`accounts` table).
2. Users **login** with email and password.
3. On login the server assigns a **JWT token** for subsequent requests.
4. Every resource created afterwards belongs to an **account** (`account_id`), and all read/write access is scoped to that account.

## 2. Current Architecture (relevant parts)

- **Auth today** (`internal/auth`): credential-based only.
  - `Service.Authenticate` accepts API keys / workspace tokens (configured in env, stored as SHA-256 hashes) or an HS256 JWT minted from one of those credentials by `POST /api/v1/auth/token`.
  - `MintJWT` signs `{iss, aud, sub, kind, iat, exp}`; `Identity = {Kind, Name}`; middleware stores it under `auth.IdentityKey`.
  - There is **no account concept**, no email/password, no per-resource ownership.
- **Resources** that will need `account_id`:
  - `endpoints`, `endpoint_versions` (`endpoint_id` FK), `request_history` (`endpoint_id` FK), `mock_resources` (stateful store).
  - Uniqueness today: `endpoints (method, path)` unique; `mock_resources (collection, resource_id)` unique.
- **Registration** (`internal/router`): `app.Use(auth.Middleware())`, control plane under `/api/v1`, dynamic mock catch-all resolves `method → all active endpoints` and best-matches the path — no account dimension.
- **Password tooling**: `golang.org/x/crypto` already in `go.mod` (indirect) — `bcrypt` available without a new module.

## 3. Proposed Architecture

```
POST /api/v1/auth/register  ─┐
POST /api/v1/auth/login     ─┼─► internal/account (repo + service, bcrypt hashing)
GET  /api/v1/auth/whoami    ─┘          │
                                        ▼
                                  internal/auth (MintJWT, middleware)
                                        │  Identity{Kind, Name, AccountID}
                                        ▼
Control plane (endpoints/import) ◄── stamped + scoped by AccountID
Dynamic mock router ◄── resolves endpoints/stateful resources per AccountID
```

- New `internal/account` package: owns the `accounts` row, bcrypt hashing, register/login business rules. Depends on `auth` to mint JWTs (one-directional).
- `auth` is extended without rewiring: JWT claims gain the account id (`sub = <account uuid>`), `Identity` gains `AccountID`, `/whoami` returns it. Legacy API keys / workspace tokens keep working as today (they mint empty-account identities).
- `endpoint` service + `state` store stamp `AccountID` from the authenticated identity on create and filter by it on every read/write, including the dynamic router's endpoint resolution and stateful resource CRUD.

## 4. Database Changes

### 000010_create_accounts (.up/.down)
```
accounts:
  id uuid PK
  email varchar(320) NOT NULL UNIQUE                 -- lower-cased before insert
  password_hash varchar(255) NOT NULL                -- bcrypt
  created_at timestamptz DEFAULT now()
  updated_at timestamptz DEFAULT now()
```

### 000011_add_account_id (.up/.down)
Add nullable `account_id uuid` to **endpoints**, **endpoint_versions**, **request_history**, **mock_resources** (uuid type, `REFERENCES accounts(id)` for endpoints/versions/history; enforced app-side and FK for direct resources), plus per-table index on `account_id`.

**Backfill (safe, non-destructive):** create one synthetic **`legacy` account** (random password hash, reserved email `legacy@local`) and assign it to all existing rows:
- `endpoints.account_id = legacy`
- `endpoint_versions.account_id = legacy` (join via `endpoint_id`)
- `request_history.account_id = legacy` (join via `endpoint_id`)
- `mock_resources.account_id = legacy`

This keeps the invariant "every resource belongs to an account" true for pre-existing data and keeps the currently deployed dashboard working. **Owner decision:** if legacy/unauth access should instead be dropped, we keep the column nullable and treat `NULL` as "legacy namespace". Default recommendation: legacy account.

### 000012_per_account_uniqueness (.up/.down) — only after backfill
- `DROP CONSTRAINT endpoints_method_path_key` → `ADD CONSTRAINT endpoints_account_method_path_key UNIQUE (account_id, method, path)`.
- `DROP CONSTRAINT mock_resources_collection_resource_id_key` → `ADD CONSTRAINT mock_resources_account_collection_resource_id_key UNIQUE (account_id, collection, resource_id)`.

Consequence: two accounts can both own `POST /users`. Destructive DDL is confined to constraint replacement; backfill runs first so no conflict with existing rows.

## 5. API Changes

| Endpoint | Body | Success | Errors |
|---|---|---|---|
| `POST /api/v1/auth/register` | `{email, password}` | `201 {account:{id,email}, token}` | 400 validation, 409 email exists |
| `POST /api/v1/auth/login` | `{email, password}` | `200 {account:{id,email}, token}` | 400 validation, 401 invalid credentials |
| `GET /api/v1/auth/whoami` | — | `{kind:"jwt", name/account_id, email}` | 401 |

Validation: email matches a standard pattern; password min 8 chars (both reported consistently as 400 `code=validation_error`). Login failure is a single generic 401 (`invalid credentials`) — no account enumeration.

Existing `POST /api/v1/auth/token` (API-key → JWT) is unchanged for backward compatibility; those JWTs carry no account and therefore cannot create resources.

## 6. Implementation Tasks (phased, per AGENT.md "one phase per iteration")

- PHASE-1 (DB): migrations 000010–000012 + down files
- PHASE-2 (Model/Repo): `internal/models/account.go`; `internal/account` repository (Create/FindByEmail)
- PHASE-3 (Service+routes): account service (register, login, bcrypt, mint via `auth.Service`); extend `auth` (claims `sub`=account id, `Identity.AccountID`, `/whoami`, middleware unchanged); wire routes + fx in `cmd/server/main.go`. Three mechanical changes, all required for ownership to flow through the middleware:
  - (a) add `AccountID uuid.UUID` to `Identity` (internal/auth/service.go);
  - (b) in `Authenticate`, map an account `sub` (UUID) into `Identity.AccountID` kind-based, and keep `Identity.Name` populated only for legacy API-key/workspace-token JWTs (today it copies `claims.Subject` → `Name`, which would swallow a UUID subject);
  - (c) mint path for accounts must set `sub` = account UUID (today `MintJWT` sets `Subject: id.Name`, which is empty for fresh accounts) — introduce an account-specific mint (e.g. `MintAccountJWT(accountID, email)`) reusing the existing `signJWT`.
- PHASE-4 (Ownership): `endpoint` model + repo + service stamp/filter `account_id`; `state` store scoped by account; importer stamps account
- PHASE-5 (Router scoping): `ListActiveByMethod` per account; dynamic resolver uses authenticated account, falls back to legacy namespace for unauthenticated calls. **`hasIdentity` (internal/router/dynamic.go:193) must accept account identities** (`Kind == KindAccount && AccountID != uuid.Nil` — it currently requires `Name != ""`, which account JWTs do not carry) or a private mock endpoint will 401 a valid account JWT. [Review finding Q1, recorded 2026-08-29]
- PHASE-6 (Frontend, optional follow-up): dashboard register/login screens, bearer-token storage
- PHASE-7 (Tests + docs): unit + integration + `README.md`/`FRONTEND.md` updates

Dependencies: PHASE-2 → 1, PHASE-3 → 2, PHASE-4 → 3, PHASE-5 → 4, PHASE-7 → all.

## 7. Testing Strategy

- Unit: bcrypt round-trip, duplicate email, weak password, login failure paths (`internal/account`); JWT round-trip with account id, token expiry, legacy keys still authenticate (`internal/auth`).
- Integration (Postgres-backed, following `repository_integration_test.go` pattern):
  - register → create endpoint → owned by account A only
  - account A cannot read/update/delete account B's endpoint or version/history
  - `POST /users` stateful resource scoped per account; no cross-account reads
  - legacy rows accessible via the legacy account after migration
  - two accounts can each own `GET /users` without conflict
  - unauthenticated mock call resolves legacy namespace only
- Migrations: `Migrate()` up+down+up against a scratch database.
- `go test ./...` and `go vet` must pass; frontend `typecheck`/`lint`/`build` for PHASE-6.

## 8. Risks / Open Decisions (need owner sign-off before PHASE-1)

1. **Legacy namespace policy** (see §4): keep a synthetic `legacy` account so old/dashboard data keeps working vs. leaving `account_id` nullable. Recommend the legacy account.
2. **Unauthenticated mock resolution** in a multi-account world: unauthenticated `/foo` calls resolve the legacy namespace only; anything else requires a JWT of the owning account. This changes "anyone with the URL can hit a public mock" — confirm.
3. **Resource creation credentials**: creating endpoints/resources requires an account-bound JWT (API keys mint legacy JWTs that cannot create). Confirm this is acceptable for existing local/dev flows.
4. **JWT claim change**: `sub` becomes the account UUID for account JWTs. Existing API-key JWTs keep `sub=<api key name>`; both verify fine. Nothing breaks at runtime, but tokens minted by the old build and tokens minted by the new build differ in meaning.
5. Bcrypt cost default (e.g. `DefaultCost`=10) is fine for a mock server; configurable later if needed.

### Deployment lockstep (PHASE-1 finding, recorded 2026-08-29)

Migrations 000010–000012 must **not** be applied to any environment running the current binary. With the v12 schema, `internal/endpoint` `Create`/`Update` use `Returning("*")` (repository.go:73,80) and bun fails to scan the new `account_id` column into the `Endpoint` model, which has no such field until PHASE-4 → `does not have column account_id`. Lists/gets/deletes and mock serving keep working; endpoint creation/update breaks. Deploy 000010–000012 in lockstep with the PHASE-4 model fields (Endpoints/EndpointVersion/RequestHistory/MockResource). Also: the two DB-backed integration packages share `MOCKSVR_TEST_DATABASE_URL`; run them serialized (`go test -p 1 ./...`) so their drop-and-migrate helpers do not race.

### Review follow-ups (TASK-001→003 review, recorded 2026-08-29)

- **Account email in token/whoami**: keep plan §5 contract — account JWTs carry an `email` claim; `Identity` gains `Email`; `/whoami` returns it. (Fixes the currently-dead `email` param on `MintAccountJWT`.)
- **whoami zero-UUID leak**: `Identity.AccountID` is `json:"account_id"` and `uuid.UUID` cannot be suppressed by `omitempty` (it is a `[16]byte` array). whoami handler must build its response map and emit `account_id` only when set, or `Identity` needs a custom `MarshalJSON`.
- **`MintJWT` guard**: refuse `KindAccount` identities (empty `sub` would mint a never-authenticating token).
- **Login timing side channel**: run a dummy bcrypt compare on `ErrNotFound` so unknown-email and wrong-password take similar time.
- **Rate limiting** (SUGGESTION, deferred): the two new public routes have no rate limit — acceptable for a mock tool; review before an internet-facing deploy.

## 9. Acceptance Criteria

- [ ] `buzz` verification: register → login → create endpoint → list/get/update/delete all return only that account's resources (negative test against a second account returns 404).
- [ ] Every new row in `endpoints`, `endpoint_versions`, `request_history`, `mock_resources` carries a valid `account_id`.
- [ ] Migration is up/down reversible and backfills existing rows to the legacy account.
- [ ] Two accounts can own the same `method+path` mock and stateful collection without conflict.
- [ ] `go test ./...`, `go vet`, and (if PHASE-6 done) frontend build all pass.
- [ ] `README.md` documents the new endpoints and env/config requirements (`auth.jwt_secret` must be set for account JWTs).
- [ ] No API contract/breaking change for existing `/api/v1/auth/token` and anonymous mock serving of legacy data.