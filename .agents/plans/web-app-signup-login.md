---
title: "Web App Signup and Login"
tags: [frontend, auth, signup, login, react]
status: draft
created: 2026-08-29
---

# Plan: Web App Signup and Login

> Project: `mocksvr-web` (React 19 + Vite + React Router v7 + Zustand + TanStack Query + Tailwind v4 + shadcn/ui).
> Status: draft — awaiting owner review before implementation.
> Backend: `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `GET /api/v1/auth/whoami` already exist and pass tests (`internal/account`, `internal/auth`). This plan is **frontend-only** and glues the SPA to those endpoints.

## 1. Objective

Replace the current hardcoded, non-interactive auth entry with real, account-based signup and login flows:

1. Users can **create an account** (email + password) via a signup page.
2. Users can **log in** to an existing account via a login page.
3. A valid account session is persisted and used to authorize all API calls.
4. Logged-in users can **log out**; the app shows who is signed in.
5. Existing manual/API-key workflows stay reachable (defensive), but the default entry point becomes the account forms.

## 2. Current State (relevant files under `web/src/`)

- **`main.tsx`** — `AuthGate` auto-mints a token from a **hardcoded API key** (`const API_KEY = 'sk_test_abc123xyz789'`) on first load via `mintToken(API_KEY)`, then renders the app. No login screen, no way to sign up.
- **`stores/auth-store.ts`** — Zustand store persisted to `localStorage` under `mocksvr-auth`. Holds `{ token, kind, name }` with `setToken`/`clearToken`. Missing: `account_id`, `email`, and a `loggedIn`-style flag.
- **`api/auth.ts`** — only `mintToken(apiKey)`. No `register`/`login`/`whoami` calls.
- **`api/client.ts`** — axios instance; request interceptor attaches `Authorization: Bearer <token>` from `useAuthStore.getState().token`. Many future pages will depend on this.
- **`routes/index.tsx`** — all routes wrapped in a single `DashboardLayout`; no auth guard / redirect.
- **`components/ui/`** — has `button`, `input`, `label`, `card`, `alert`, `separator`, `sonner` (toast) — enough to build the forms without new deps.
- **`pages/`** — no auth pages yet.

## 3. Backend contract (already implemented — reference only)

| Endpoint | Body | Success | Errors |
|---|---|---|---|
| `POST /api/v1/auth/register` | `{email, password}` | `201 {data:{account:{id,email}, token}}` | 400 `code=validation_error`, 409 `code=conflict` |
| `POST /api/v1/auth/login` | `{email, password}` | `200 {data:{account:{id,email}, token}}` | 400, 401 `code=unauthorized` (generic `invalid credentials`) |
| `GET /api/v1/auth/whoami` | — (Bearer) | `200 {data:{kind,account_id?,email?,name?}}` | 401 |

Notes:
- `register`/`login` return the **token in `data.token`** and the account in `data.account`.
- `whoami` returns an explicit map; `account_id`/`email` only appear for account JWTs (`internal/auth/handler.go`).
- Register/login are **public** (`isPublicPath`), so no credential is required to call them.

## 4. Proposed UI/UX

```
/ (unauthenticated) ──► /login ──► /signup
        │                        (link back to /login)
        ▼
login succeeds ──► redirect to / (dashboard)

TopNav: shows signed-in email/account + Logout button when authenticated, else Sign in / Sign up links.
```

- **`/login`** — email + password form, "Sign in" submit, error alert on 401 (generic message), link to `/signup`. On success: persist session, redirect to `/`.
- **`/signup`** — email + password (+ confirm password) form, client-side validation, link to `/login`. On success: persist session, redirect to `/`.
- **Logout button** — clears the persisted store and returns the user to `/login`.

## 5. Implementation Tasks

### PHASE-A — Auth API client
Extend `web/src/api/auth.ts`:
- `register(email, password)` → `POST /api/v1/auth/register`
- `login(email, password)` → `POST /api/v1/auth/login`
- `whoami()` → `GET /api/v1/auth/whoami`
- Shared response type: `{ account: { id, email }, token }`; parse `data` from the `ApiSuccess` envelope.

### PHASE-B — Auth store additions
Extend `stores/auth-store.ts`:
- Add optional `accountId: string` and `email: string` fields.
- `setAccountSession(accountId, email, token)` sets all three (preserve existing `setToken` for the legacy API-key path so nothing else breaks).
- Persist the new fields in the existing `mocksvr-auth` store entry (or a schema-migration/shape guard for old persisted values).
- The client `/own` identity: derive `isAuthenticated = Boolean(token)`.

### PHASE-C — Auth pages
- `pages/login-page.tsx` — react-hook-form + zod schema, calls `login`, on success `setAccountSession`, `toast`, `navigate('/')`.
- `pages/signup-page.tsx` — same pattern with `register`; client-side password-min-length + confirm-match validation.
- Keep both minimal and consistent with existing shadcn/ui usage (see `components/ui/`).
- Add an `auth-layout` or reuse a centered card layout (can live under `layouts/` or inline).

### PHASE-D — Routing & guards
Update `routes/index.tsx`:
- Add `/login` and `/signup` routes **outside** `DashboardLayout`.
- Add a guard around the dashboard routes: unauthenticated → redirect to `/login`.
- `/login` and `/signup` redirect to `/` when already authenticated.
- Replace **`main.tsx` `AuthGate`**: remove the hardcoded `API_KEY` auto-mint. New behavior:
  - If a token exists, optionally validate with `whoami()` (on failure, clear session → `/login`).
  - If no token, render the auth routes (no forced API-key mint).

### PHASE-E — Session UI
- `layouts/top-nav.tsx` — show signed-in email (from store) and a Logout action (calls `clearToken`, redirects to `/login`) when authenticated; show Sign in / Sign up links otherwise.
- Optional: surface account email on `pages/settings-page.tsx`.

### PHASE-F — Verification
- `npm run typecheck`, `npm run lint`, `npm run build` in `web/` all pass.
- Manual/dev smoke: fresh load → redirected to `/login`; signup → lands on `/`; logout → back to `/login`; login with wrong password shows generic error; existing API-key minting path still functions.
- Update `FRONTEND.md` if it documents the old auto-mint flow.

## 6. Risks / Open Decisions

1. **Removing the auto-mint breaks the "zero-config dashboard" UX.** Today the SPA works with no account. Replacing `AuthGate` means a fresh deploy requires signup before any page loads. Confirm this is the intent (it aligns with the backend account plan).
2. **`whoami()` on load** adds a network round-trip and a dependency on backend availability; if the backend is down, a stored token would be cleared even though it is valid. Recommendation: treat `whoami()` failure as non-fatal (keep session, show app) and only clear on explicit 401 — or skip on-load validation entirely for v1. Owner decision.
3. **Password handling in the client.** Only validate client-side shape; never log/stash passwords. Confirm-password field is optional — include only if wanted.
4. **Mixed legacy session:** the store can hold either an API-key JWT (`kind=api_key`, no account) or an account JWT. The guard should treat any valid token as authenticated (both mint valid tokens server-side); account UI (email in nav) shows only when `email` is present.
5. **Auth-store schema migration** for users with an old persisted `mocksvr-auth` entry — keep it lenient (missing fields default to `''`).

## 7. Acceptance Criteria

- [ ] Unauthenticated visit to `/` redirects to `/login`; visiting `/login` while authenticated redirects to `/`.
- [ ] Signup with a new email → authenticated → redirected to `/`; duplicate email shows a 409 error.
- [ ] Login with correct credentials → authenticated → redirected to `/`; wrong password / unknown email show a generic "invalid credentials" error (no enumeration).
- [ ] Logout clears the persisted session and returns to `/login`.
- [ ] All API calls after login carry `Authorization: Bearer <token>` (existing client interceptor).
- [ ] Dashboard top nav shows the signed-in email when authenticated.
- [ ] No hardcoded `API_KEY` in `main.tsx`; no passwords stored/logged.
- [ ] `npm run typecheck`, `npm run lint`, `npm run build` all pass.
