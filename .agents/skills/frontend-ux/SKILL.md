---
name: frontend-ux
description: Design and review a clear, calm, accessible interface for fund staff with low computer confidence; use before frontend proposal and implementation.
compatibility: Codex, Claude Code, OpenCode
---

# Frontend UX for non-technical staff

Use this skill before designing screens, writing interface copy, reviewing user flows or accepting frontend implementation.

## Read first

- `rules/non-technical-ux.md` (mandatory product brief)
- `rules/frontend.md`
- `backend/openapi/swagger.yaml` for facts about available data and operations

## Design method

1. Define the user's goal in one plain-language sentence.
2. List the minimum decisions and inputs the user must provide. Do not add extra steps or information fields.
3. Write the user journey in everyday Russian before designing components.
4. For every screen/state, answer: what does the user see, what should they do next, what happens after the action, and how can they recover from a mistake?
5. Keep a single obvious primary action. Use progressive disclosure for secondary explanations, not for essential identity or status information.
6. Show source context. A person selected from “Основная база” and a record loaded from “База сайта” must never look interchangeable.
7. Add a final review of the pair before the explicit save action. Do not auto-match or auto-save.
8. Preserve user work during errors and make retry/correction clear.
9. Design empty, loading, no-result, invalid, conflict, success and network-failure states before declaring the flow complete.
10. Perform the novice-style walkthrough in `rules/non-technical-ux.md`; report it as a heuristic, never as real user testing.

## Content rules

- Russian, short sentences, familiar verbs.
- Explain “ID” as the number of a record the employee received when creating the site entry.
- Do not expose developer terms, raw status codes or backend internals.
- Every error says what happened and one safe next step.
- Never use placeholder-only labels or icon-only primary actions.

## Visual and accessibility rules

- Calm, trustworthy, restrained visual design; no generic dashboard clutter.
- Body text normally 16 CSS px or larger; helper text preferably 14 CSS px or larger.
- Interactive controls should aim for at least 44×44 CSS px where practical.
- Target WCAG 2.2 AA, including text contrast, keyboard access, visible focus, labels and non-colour status cues.
- Layout must remain understandable at narrow widths and 200% zoom.
- Do not rely on hover, drag, colour alone, animation or sound.

## Output when used for proposal

Provide:
- primary persona and task;
- step-by-step user journey;
- one-screen information architecture;
- short Russian labels and helper copy for every input/action;
- a state matrix (initial/loading/success/empty/error/conflict);
- design direction and accessibility decisions;
- novice walkthrough plan;
- acceptance criteria that can be tested.

Do not start frontend implementation during a proposal-only phase.
