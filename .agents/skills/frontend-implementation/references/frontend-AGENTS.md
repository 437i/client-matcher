# Frontend — local agent instructions

## Scope

This directory contains the React + TypeScript + Vite single-page frontend for client-matcher. It is a presentation layer for the existing Go/Gin API, not a second backend.

## Before changes

1. Read the repository root `AGENTS.md`, `rules/frontend.md` and `rules/non-technical-ux.md`.
2. Inspect `package.json`, `vite.config.ts`, `src/`, tests, Dockerfile and Nginx config.
3. Read `../backend/openapi/swagger.yaml` and generated API types before changing API integration.
4. Check `git status --short`; preserve existing user changes.
Синтетические CSV проекта допустимы. Соблюдай rules/data-safety.md; не читай .env, credentials и секреты без необходимости.

## Architecture

- Keep UI code feature-oriented and small.
- Keep HTTP calls in the API layer; components use query/mutation hooks.
- Use OpenAPI-generated types; never maintain parallel API DTOs.
- Backend is authoritative for matching and validation.
- Use TanStack Query for server state and local React state for transient UI state.
- Reuse shadcn/ui primitives and a consistent design system.

## UX

The target users are fund staff with mixed and often low computer confidence. Follow `rules/non-technical-ux.md` as mandatory product requirements. Keep the UI in clear Russian, with one obvious primary workflow, visible field labels, explanations beside unfamiliar concepts, clear source labels and explicit next actions.

Show the CRM candidate and site record before save; make the pair and both IDs unmistakable. Never auto-select or auto-save. Preserve user entries on errors, explain recovery in plain language, prevent duplicate submits and show a persistent success confirmation. Implement initial, loading, no-results, empty, error, conflict, success and disabled states. Do not duplicate backend matching logic.

Use a calm, uncluttered visual system with readable text and comfortably large controls. Support keyboard-only use, visible focus and narrow screens. Avoid developer jargon, dashboard clutter, icon-only primary actions and decorative metrics. Perform a novice-style heuristic walkthrough; do not claim real user testing without actual staff participants.

## Commands

Use existing package scripts for typecheck, lint, tests, build and API type generation. Do not claim checks passed unless actually run. Do not modify backend, protected CSVs or Docker volumes. Do not commit or push unless explicitly asked.
