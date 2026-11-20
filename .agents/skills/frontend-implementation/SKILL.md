---
name: frontend-implementation
description: Implement the client-matcher React frontend from the existing OpenAPI 3.1 contract, with accessible UI, typed API integration, tests and Docker deployment.
compatibility: Codex, Claude Code, OpenCode
---

# Frontend implementation — client-matcher

Use this skill for any frontend implementation, UI change, API integration, testing or frontend deployment work.

## Read first

1. Root `AGENTS.md`, `rules/frontend.md` and `rules/non-technical-ux.md`. Load and follow the `frontend-ux` skill before choosing the screen structure or writing UI copy.
2. `backend/openapi/swagger.yaml` and, when useful, the matching Go router/handlers/DTO/response mapping.
3. `docker-compose.yml`, `README.md`, `.gitignore` and backend Docker/health configuration.
4. Existing frontend files and frontend/AGENTS.md; the application already exists.
5. OpenSpec change artifacts supplied by the OpenSpec apply workflow.

Синтетические CSV проекта допустимы. Соблюдай rules/data-safety.md; не читай .env, credentials и секреты без необходимости.

## Modern Web Guidance

The project uses the official `modern-web-guidance` skill installed at
`.agents/skills/modern-web-guidance/SKILL.md`.

Follow this skill when implementing or changing frontend HTML, CSS,
client-side JavaScript or UI behavior, especially for:
- forms and form validation;
- search fields, autocomplete and typeahead;
- focus management and keyboard navigation;
- accessibility and screen reader behavior;
- responsive layouts and modern CSS;
- browser APIs and progressive enhancement;
- frontend performance.

Follow the official skill's search → retrieve → verify workflow.
Use its guidance before choosing a browser API, UI pattern or adding
a dependency. Check browser compatibility and apply recommended
fallbacks where necessary.

Do not invoke Modern Web Guidance for backend-only work,
infrastructure, tooling or trivial changes that do not affect
web platform behavior.

Project-specific requirements take precedence:
- `rules/frontend.md`;
- `rules/non-technical-ux.md`;
- `frontend/AGENTS.md`;
- `frontend-ux` skill;
- existing application architecture and dependencies.

Modern Web Guidance supplements these project rules; it does not
replace them or authorize unrelated architectural changes.

Do not introduce runtime dependencies merely because a guide
demonstrates a particular browser API.

Preserve the existing React, TypeScript, TanStack Query,
shadcn/ui and Radix UI architecture unless a concrete reason
justifies changing it.

## Architecture recommendation

- React + TypeScript + Vite SPA. Preserve the existing frontend and frontend/AGENTS.md; do not scaffold over the application.
- Tailwind CSS v4 + shadcn/ui + lucide-react.
- TanStack Query for server state.
- `openapi-typescript` + `openapi-fetch` for a typed API client generated from `backend/openapi/swagger.yaml`.
- Vitest + React Testing Library + user-event.
- Do not add React Router, Redux/Zustand, Axios, Next.js/SSR or extra abstraction layers for this one-screen tool.

Check official documentation and compatible stable versions before installing. Do not pin old versions from memory. Node supported by current Vite must be checked against the user's installed Node; Node 24 is suitable if it meets Vite's current engine requirement.

## Main user flow

1. Search CRM using `GET /api/v1/crm-clients?q=...`.
2. Display returned candidates and allow one CRM client to be selected.
3. Enter a known site ID and load it using `GET /api/v1/site-clients/{id}`.
4. Display selected CRM client and site client side-by-side, including birthday and surname.
5. Create the link using `POST /api/v1/links` with `{ crm_id, site_id }`.
6. On success, refresh `GET /api/v1/links` and show confirmation.
7. Display all stored links and provide CSV download at `/api/v1/links/export.csv`.

The backend remains the sole authority for surname/date matching. Do not reimplement `NormalizeName`, `ё/е` equivalence or birthday validation in TypeScript. If the backend rejects a pair, present its `ErrorResponse` message.

## Generated API types

Use the repository's OpenAPI document as the only API type source:

```bash
npx openapi-typescript ../backend/openapi/swagger.yaml -o src/api/schema.d.ts
```

Create package scripts for generation and typechecking. The generated file is committed or reproducibly generated as agreed in OpenSpec; never hand-edit it.

Initialize the typed client from the current origin and use paths as defined in OpenAPI (they already include `/api/v1`). Confirm in Vite proxy and Docker Nginx that `/api/` reaches backend without path duplication or CORS.

Use a direct anchor to `/api/v1/links/export.csv` for export. Do not parse the CSV endpoint as JSON.

## UI composition

Preserve one focused workflow screen:
- header with clear product purpose;
- primary card/stepper for CRM search, candidate selection, site lookup and confirmation;
- saved-links section with refresh, empty state and CSV download;
- clear loading/error/success states.

The interface should look like a polished internal operations tool, not a generic landing page. Choose a restrained palette, clear hierarchy, readable table, responsive layout, meaningful whitespace and consistent form controls. Avoid decorative dashboards, meaningless charts, gratuitous gradients and fake metrics.

## Async behavior

Use TanStack Query query/mutation hooks. Use stable query keys. Invalidate the links query after successful link creation. Disable duplicate submissions while pending. Prevent empty requests on the client, but do not duplicate backend business validation.

Centralize API error extraction. Prefer the backend `message` field and fall back to a clear Russian message for network errors or non-JSON failures. Never render raw stack traces, SQL or secrets.

## Docker integration

- Vite dev server proxy routes `/api` to `http://localhost:8080`.
- Docker frontend image builds `dist` with Node, then serves it with Nginx.
- Nginx proxies `/api/` to `http://backend:8080` and serves SPA routes with `try_files ... /index.html`.
- Preserve the existing Compose frontend service with a configurable host port defaulting to 3000. Keep existing backend port/Swagger and DB/migration services intact.
- Do not use `vite preview` as the production/demo server.
- Preserve the simple clean-clone start: `docker compose up --build -d`.

## Quality gates

- Strict TypeScript; no `any` for API payloads.
- Include `typecheck`, `lint`, `test`, `build`, `api:types` scripts.
- Tests for search, selection, site lookup, create success, backend validation/conflict errors, list refresh and CSV download link.
- Do not test using protected source CSV. Use inline synthetic fixtures or mocked HTTP.
- Verify keyboard support, visible focus, labels, responsive layout and accessible errors.
- After code, run all available frontend checks and Docker Compose config/build as appropriate. Report what was not run.
