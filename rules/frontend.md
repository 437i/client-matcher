# Frontend rules — client-matcher

Read and follow `rules/non-technical-ux.md`. Its requirements are mandatory product requirements, not optional polish.

## Product scope

Frontend — небольшое русскоязычное SPA для сотрудника фонда. Основной сценарий:
1. Ввести имя/фамилию или фрагмент ФИО и найти кандидатов в CRM.
2. Выбрать конкретного CRM-клиента.
3. Ввести известный `site_id` и получить данные записи сайта для визуальной сверки.
4. Отправить пару ID на backend для проверки и сохранения.
5. Видеть ранее сохранённые связи после перезапуска и скачать CSV со всеми связями.

Это внутренний рабочий инструмент, а не публичный сайт, CRM-клон или полноценная админ-платформа.

## Source of truth: backend API

- OpenAPI contract: `backend/openapi/swagger.yaml` (OpenAPI 3.1).
- Verify it against `backend/internal/adapters/inbound/http/router.go`, handlers, `handlers/dto.go`, `handlers/response.go` before coding.
- Do not invent endpoints, fields, request bodies, statuses, or business rules.
- Do not change Go backend or OpenAPI contract to make frontend implementation easier unless a separately approved OpenSpec change explicitly requires it.
- Generate TypeScript API types from the local OpenAPI file; do not manually duplicate DTO types.
- Backend is authoritative for name/date matching. Do not implement a second matching algorithm in the browser. Show source data side-by-side and display backend validation errors.

## Existing API (verify before implementation)

- `GET /api/v1/crm-clients?q=...` — CRM search; query param `q` is required; returns an array of CRM client DTOs.
- `GET /api/v1/site-clients/{id}` — loads a site record by known ID.
- `POST /api/v1/links` — JSON body `{ crm_id, site_id }`; creates a link after backend validation.
- `GET /api/v1/links` — returns saved ID pairs and creation timestamps.
- `GET /api/v1/links/export.csv` — downloads CSV (`crm_id,site_id`). Use normal browser download/navigation, not JSON parsing.
- `GET /healthz` — operational endpoint, not a user workflow requirement.

Actual DTO fields are defined by OpenAPI. In the checked-in contract:
- CRM: `id`, `full_name`, `last_name`, `first_name`, optional `patronymic`, `birthday`, `clinic`.
- Site: `id`, `short_name`, `last_name`, `birthday`.
- Link: `id`, `crm_id`, `site_id`, `created_at`.
- Error: `code`, `message`.

The task describes an organization name; the current API field is `clinic`. Present it in Russian UI with an appropriate user-facing label (e.g. «Организация/клиника») without changing the API field name.

## Recommended stack

- React + strict TypeScript + Vite.
- Tailwind CSS v4 + shadcn/ui; use accessible generated primitives rather than building bespoke controls unnecessarily.
- Lucide icons.
- TanStack Query for remote/server state, caching, loading/error states, and invalidating saved links after creation.
- `openapi-typescript` (dev dependency) to generate `src/api/schema.d.ts` from `../backend/openapi/swagger.yaml`.
- `openapi-fetch` for type-safe calls using generated `paths`.
- Vitest + React Testing Library + user-event for component/workflow tests.
- npm is the package manager unless repository evidence gives a reason to use another one.

Avoid: Next.js/SSR, React Router for this one-screen app, Redux, Zustand, Axios, GraphQL, UI component mega-frameworks, extra form-state libraries, and unnecessary runtime validation layers. Add a dependency only when its purpose is clear.

## API integration

- Use same-origin relative API paths beginning with `/api/v1`.
- In Vite dev mode proxy `/api` to `http://localhost:8080`.
- In Docker, frontend Nginx proxies `/api/` to the `backend:8080` Compose service. This avoids CORS and avoids exposing a separate frontend API origin.
- `openapi-fetch` client should use the current origin as base URL. Confirm generated paths and avoid duplicating `/api/v1`.
- Keep API calls in a small central API module; do not scatter raw fetch calls through components.
- Handle all error responses using the actual `{ code, message }` payload. Show understandable Russian feedback; do not swallow errors or display raw stack/SQL details.
- Use query/mutation hooks; after successful link creation invalidate/refetch `GET /api/v1/links`.
- Disable duplicate submissions while a mutation is pending.
- The CSV export should use an ordinary link/button to `/api/v1/links/export.csv`, allowing the browser to download the attachment.

## UX requirements

- The target users are fund staff with mixed and often low computer confidence. Design for a first-time user who should not need training or developer help.
- Follow `rules/non-technical-ux.md` for Russian plain-language copy, guided workflow, visual hierarchy, safe actions, recovery states, accessibility and novice walkthrough.
- Keep one focused screen and a clearly visible normal flow: search the main database → select a person → enter and look up the site record number → review both records → explicitly save.
- Show a separate saved-links section with a clear empty state and CSV export.
- Do not use unexplained technical vocabulary, icon-only controls, dense dashboards, unnecessary tabs, nested navigation or decorative metrics.
- Make the selected CRM record and loaded site record unmistakable. Never auto-select a candidate or save a relationship without explicit action.
- Before save, show the two records and both IDs in a review area. The save button must communicate exactly what it does.
- Preserve user input on errors. Explain errors in plain Russian and suggest a safe next action. Do not show raw status codes, stack traces or server internals.
- Provide visible loading, no-results, empty, error, success and disabled states. After success, confirm the saved pair and refresh the saved list.
- Use clear hierarchy, adequate text size and large controls. Do not rely on colour alone. Target WCAG 2.2 AA and aim for 44×44 CSS px interaction targets where practical.
- Responsive layout is required, but desktop workflow is the primary layout. Support keyboard-only operation and visible focus.
- Do not claim real-user usability testing unless actual representative users participated. Perform and report a novice-style heuristic walkthrough instead.

## Deployment

- Local development: `cd frontend && npm run dev`; Vite proxy targets the backend on `localhost:8080`.
- Demo/compose: add a `frontend` service that builds static assets and serves them with Nginx.
- Nginx serves the SPA and proxies `/api/` to `http://backend:8080` on the Compose network.
- Publish the frontend on a separate configurable host port, default `3000`; preserve the existing backend/Swagger port and service.
- Do not modify backend business logic, routes, database schema, or CSV source paths.
- Do not use a development Vite server as the production/demo container server; serve the built `dist/` directory.

## Tests and verification

- TypeScript must be strict; use `noUncheckedIndexedAccess` where compatible.
- Provide scripts for `typecheck`, `lint`, `test`, `build`, and `api:types`.
- Unit/component tests cover search, candidate selection, site lookup, link creation, backend errors, list refresh and empty states.
- Mock HTTP in UI tests; confirmed synthetic CSV fixtures are allowed under rules/data-safety.md.
- Verify generated API types are reproducible from the committed OpenAPI file.
- Verify frontend Docker build, Compose config, SPA fallback, same-origin API proxy and browser workflows when tools are available.
- Never mark checks successful unless actually run.
