---
name: frontend-review
description: Independently review client-matcher frontend for correctness, API contract drift, UX, accessibility, security, tests and deployment regressions without modifying files.
compatibility: Codex, Claude Code, OpenCode
---

# Frontend review

Work read-only. Do not create, edit, format, generate or delete files.

1. Start with `git status --short --untracked-files=all`, staged diff and unstaged diff. Include new/untracked frontend files; ordinary `git diff` alone is insufficient.
2. Read root `AGENTS.md`, `rules/frontend.md`, `rules/non-technical-ux.md`, `frontend/AGENTS.md`, OpenSpec change artifacts and frontend package scripts.
3. Compare frontend requests and response handling against `backend/openapi/swagger.yaml` and actual backend routes/handlers when necessary.
4. Check all user flows: CRM search, candidate selection, site ID lookup, pair creation, list refresh after success, persistent saved links and CSV export.
5. Check API typing is generated from the committed OpenAPI spec; no hand-maintained duplicate DTOs or invented endpoints.
6. Check error states for 400/404/409/422/499/500/502/504 as present in OpenAPI; user-friendly fallback on network failures.
7. Check duplicate-submit prevention, async loading, query invalidation, empty states and no stale selected candidate/site record after changing inputs.
8. Check usability for staff with low computer confidence: can a first-time user identify the next action, understand the site ID field, distinguish similar CRM candidates, review both records before saving, recover from each error, and find the saved result without training?
9. Check content and visual design: plain Russian, familiar action labels, no unexplained technical jargon, no cluttered dashboard, clear hierarchy, readable text, appropriately large controls, no placeholder-only labels or icon-only primary actions.
10. Check accessibility against WCAG 2.2 AA: semantic labels, keyboard navigation, visible focus, accessible errors/statuses, table semantics, contrast and responsive layouts.
11. Check security: no secrets in VITE_* variables, no tokens or credentials in browser bundle, no direct database access, no protected CSV reads, no raw server internals in UI.
12. Check Vite proxy and Docker Nginx same-origin proxy preserve `/api/v1` exactly and avoid CORS/path duplication. Check SPA fallback and frontend service wiring in Compose.
13. Run safe checks only when useful and permitted. Never start/stop containers or delete volumes in a read-only audit unless explicitly instructed.

Do not claim actual user testing unless representative staff actually participated; distinguish a heuristic novice walkthrough from user research.

Report only concrete findings, ordered Critical/High/Medium/Low. For each finding include file/line, reproducible scenario, impact and minimal correction. Separate confirmed defects from hypotheses. Mention tests/checks actually run and not run. End with READY / READY WITH NOTES / NOT READY.
