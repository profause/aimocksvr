````markdown
# Frontend Foundation (Milestone 0)

## Objective

Establish the frontend foundation for the MockSvr Dashboard within the existing Go project.

The dashboard must be implemented as a React application inside the repository under the `web/` directory. It should be independently runnable during development while being embedded into the Go binary for production deployments.

No dashboard features should be implemented during this milestone. The goal is to create a clean, scalable, and production-ready frontend architecture.

---

# Project Structure

Create the following structure:

```text
mocksvr/
├── cmd/
├── internal/
├── configs/
├── migrations/
├── web/
│   ├── public/
│   ├── src/
│   │   ├── api/
│   │   ├── assets/
│   │   ├── components/
│   │   ├── features/
│   │   ├── hooks/
│   │   ├── layouts/
│   │   ├── lib/
│   │   ├── pages/
│   │   ├── routes/
│   │   ├── services/
│   │   ├── stores/
│   │   ├── styles/
│   │   ├── types/
│   │   ├── utils/
│   │   ├── App.tsx
│   │   └── main.tsx
│   ├── index.html
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
└── Dockerfile
```

---

# Technology Stack

## Framework

- React 19
- TypeScript
- Vite

## Styling

- Tailwind CSS
- shadcn/ui
- Radix UI

## Routing

- React Router

## State Management

- Zustand

## Server State

- TanStack Query

## HTTP Client

- Axios

## Forms

- React Hook Form
- Zod

## Icons

- Lucide React

## Editors (Future)

- Monaco Editor

---

# Development Environment

Configure Vite development server.

Requirements

- Hot Module Reloading
- TypeScript support
- Tailwind support
- API proxy

Proxy configuration

```text
/api

↓

http://localhost:3000
```

The frontend should communicate with the Go backend without requiring CORS configuration.

---

# Tailwind CSS

Configure Tailwind CSS.

Requirements

- Dark mode support
- CSS variables
- Design tokens
- Responsive utilities

---

# shadcn/ui

Initialize shadcn/ui.

Install the default component library.

Configure

- theme
- aliases
- utilities

No custom components should be created yet.

---

# Routing

Configure React Router.

Initial routes

```text
/

↓

Dashboard Layout

↓

Placeholder Page
```

Future routes should be easy to register.

---

# State Management

Configure Zustand.

Create stores for

- UI state
- Theme
- Workspace
- Sidebar

Do not implement business logic.

---

# Server State

Configure TanStack Query.

Create

- QueryClient
- Provider
- DevTools (development only)

No API integrations yet.

---

# Axios

Create a reusable Axios client.

Requirements

- Base URL configuration
- JSON defaults
- Error interceptors
- Request interceptors

Do not implement authentication yet.

---

# Path Aliases

Configure aliases.

Example

```text
@/components

@/pages

@/features

@/hooks

@/stores

@/utils

@/lib

@/services

@/api
```

---

# Linting

Configure

- ESLint
- Prettier

Requirements

- Strict TypeScript
- Consistent formatting
- Import ordering

---

# Theme

Implement theme support.

Modes

- Light
- Dark
- System

Persist preference using local storage.

---

# Layout

Create the application shell only.

Structure

```text
+------------------------------------------------------+
| Top Navigation                                       |
+-------------+----------------------------------------+
| Sidebar     |                                        |
|             |                                        |
|             |          Workspace                     |
|             |                                        |
+-------------+----------------------------------------+
```

Use placeholder content.

Do not implement dashboard functionality.

---

# Error Handling

Create

- Error Boundary
- Not Found Page
- Loading Screen

---

# Environment Variables

Configure

```text
VITE_API_URL

VITE_APP_NAME

VITE_APP_VERSION
```

---

# Package Scripts

Provide

```bash
npm run dev

npm run build

npm run preview

npm run lint

npm run format

npm run typecheck
```

---

# Acceptance Criteria

The milestone is complete when:

- React application builds successfully.
- TypeScript reports zero errors.
- ESLint passes.
- Tailwind CSS is configured.
- shadcn/ui is installed.
- React Router is configured.
- Zustand is configured.
- TanStack Query is configured.
- Axios client is configured.
- Path aliases are working.
- Theme switching works.
- API proxy is configured.
- Placeholder application shell renders successfully.
- Project structure matches the specification.

---

# Deliverables

Upon completion, provide:

1. Directory structure.
2. Installed dependencies.
3. Configuration summary.
4. Development instructions.
5. Remaining work before Milestone 1.

Stop and wait for approval before implementing any dashboard features.
````
