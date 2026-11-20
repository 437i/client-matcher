# UX rules — staff with low computer confidence

These requirements are product requirements, not optional visual polish. The application will be used by fund staff with different levels of computer confidence. Assume some users rarely work with unfamiliar software and may be worried about making an irreversible mistake.

## Primary outcome

A first-time user should be able to complete the normal task without reading technical documentation, knowing what an API is, or asking a colleague which button to press:

1. Find a person in the main database.
2. Choose the correct record using full name, date of birth and organization.
3. Enter the record number received after adding the person to the site.
4. Review both records together.
5. Save the relationship and understand that it was saved.
6. Find previously saved relationships and download the CSV when needed.

Do not claim that real users tested the interface unless actual sessions with such users took place. You may perform and report a clearly labelled novice-style heuristic walkthrough.

## Language and copy

- The entire interface is in clear, natural Russian. Use short sentences and familiar verbs.
- Write for a colleague, not for a developer. Prefer “Основная база”, “База сайта”, “Номер записи”, “Найти клиента”, “Проверить данные”, “Сохранить связь”.
- If the acronym CRM is useful, introduce it as “Основная база (CRM)”; do not use unexplained technical abbreviations.
- Avoid technical words in user-facing copy: endpoint, API, JSON, HTTP, запрос, payload, сервер, mutation, cache, UUID, status code, backend, frontend.
- Explain the site ID beside the field, e.g. “Введите номер записи, который вы получили при добавлении клиента на сайт”. Do not assume the user knows what an ID means.
- Labels must remain visible above inputs. Placeholder text is an example, never the only label.
- Button text must describe the result: “Найти клиента”, “Найти запись на сайте”, “Сохранить связь”, “Скачать CSV”. Avoid “OK”, “Submit”, “Продолжить” without context.
- Do not expose raw error codes, stack traces, SQL text or developer-oriented details. Map known API error codes to calm, actionable Russian messages; preserve the real meaning and do not invent a recovery path that the system cannot perform.
- Do not blame the user. Prefer “Запись с таким номером не найдена. Проверьте номер и попробуйте ещё раз” over terse or accusatory messages.

## Information architecture

- Prefer one focused screen with a clearly visible primary task and a separate section for saved relationships.
- Present the workflow as a small number of visible steps. The user should always know what to do now and what happens next.
- Keep one visually dominant primary action at a time. Secondary actions must look secondary.
- Do not make the user navigate between multiple pages or tabs for the normal matching task.
- Do not add dashboards, charts, activity feeds, decorative statistics, sidebars, settings pages, onboarding tours or multi-step wizards unless evidence shows they solve a real user problem.
- Use progressive disclosure only for optional explanations. Never hide the selected person, site record, key identifiers, validation result or next required action.
- Do not rely on icons alone. Pair meaningful icons with visible text; icon-only controls are exceptional and need accessible names and a tooltip.

## Safe and predictable workflow

- CRM search is an explicit, understandable action. Enter submits the search; the user can also press a clearly labelled button. Do not send a request on every keystroke without a deliberate, tested reason.
- Search results must be easy to distinguish. Show full name prominently, then date of birth and organization. Keep the record ID visible but secondary. Never select a candidate automatically.
- Make the selected CRM record unmistakable and provide a simple “Изменить выбор” action.
- Site lookup requires an explicit click or Enter. Show the returned record before saving.
- Before saving, show a compact review with both records and both IDs. Use clear labels (“Основная база” and “База сайта”) so the user can see exactly which records will be linked.
- Never create a relationship automatically, based only on a search result, or on a frontend-side guess. The user explicitly saves; the backend remains the authority for validation.
- Prevent duplicate submissions while saving. Show progress text such as “Сохраняем связь…” and disable the save action until the request completes.
- Do not clear inputs or selections when a request fails. Preserve the user's work so they can correct a value and retry.
- After changing an input that invalidates a previously loaded record, visibly mark that record as needing lookup again. Do not leave stale data appearing current.
- After success, show a persistent, understandable confirmation that names the two saved IDs. Refresh the saved-links list. Do not rely only on a short-lived toast.
- Avoid confirmation dialogs for routine, reversible navigation. For the create action, a clear review of the pair immediately above the “Сохранить связь” button is the confirmation step.

## States and recovery

Every meaningful async section must have designed states:

- Initial: explain what to enter and what the user will get.
- Loading: state what is happening in plain Russian; prevent duplicate actions.
- Results: show the number of results when useful and make selection clear.
- No results: explain that nothing was found and suggest a sensible next step, such as checking spelling or trying a shorter part of the name.
- Empty saved-links list: explain that saved relationships will appear here after the first successful save.
- Validation/conflict: explain what happened and what the user can do next. Do not imply that a conflict can be overwritten.
- Network/service failure: say that the application could not reach the required source, preserve entered data and offer a retry when retry is safe.
- Success: state exactly what was saved and refresh the list.

Use inline messages near the relevant control for field-specific problems. Use a page-level alert for broader failures. Give errors a readable title and a next step; colour alone is never enough.

## Visual design

- Aim for a calm, trustworthy, friendly internal work tool—not a developer console, generic admin template or marketing landing page.
- Use a restrained light theme by default, a neutral background, clear white or subtly tinted work surfaces, one consistent accent colour and semantic success/warning/error colours. Do not invent the foundation's brand identity or logo.
- Prioritise hierarchy, alignment, whitespace and readable text over decoration. Avoid excessive cards nested inside cards, strong gradients, glassmorphism, tiny metadata, dense borders and gratuitous animation.
- Use a consistent spacing scale and typography scale. Body text should normally be at least 16 CSS px; helper text should remain comfortably readable (prefer 14 CSS px or larger).
- Inputs and primary buttons should be comfortably large (aim for at least 44 CSS px height). Give interactive controls enough spacing to avoid accidental clicks.
- Full names, organization names and long Russian text must wrap without clipping or overlapping controls.
- Format dates consistently for Russian users (e.g. 15.04.1982), while preserving the actual calendar date from the API.
- Do not use colour as the sole way to communicate selected, successful, invalid or pending states. Combine colour with text, icon or border/shape.
- Avoid motion that is not useful. Respect reduced-motion preferences.

## Accessibility and interaction

- Use the W3C [WCAG 2.2 Recommendation](https://www.w3.org/TR/WCAG22/) as the accessibility reference.
- Target WCAG 2.2 AA as the accessibility baseline. Relevant W3C guidance includes text contrast of at least 4.5:1 for normal text and target size minimums; aim for larger 44×44 CSS px interaction targets where practical.
- Every input has a persistent visible label and programmatic label association. Help text and error text are programmatically associated where applicable.
- All functionality works with keyboard alone. Tab order is logical; Enter activates expected form actions; Escape only dismisses safe transient UI.
- Provide a clearly visible focus indicator. Never remove outline without an equally visible replacement.
- Use semantic headings, buttons, links, forms, tables and status/alert regions.
- Announce loading, result count, success and errors to assistive technology without moving focus unexpectedly.
- Do not rely on hover, colour, sound or drag-and-drop.
- At narrow widths, transform tables into readable cards or provide a deliberate horizontal-scroll treatment with clear context. Do not shrink text until it becomes difficult to read.
- Verify keyboard navigation and layouts at desktop, tablet and narrow mobile widths, including 200% zoom where practical.

## Usability acceptance

Before considering the UI complete, perform a novice-style walkthrough using synthetic mocked responses, without reading protected CSV files:

1. Can a first-time user identify where to start within a few seconds?
2. Is it obvious what information belongs in each field and where to find the site record number?
3. Can the user distinguish two CRM candidates with similar names?
4. Is it obvious which two records will be linked before save?
5. Does every error explain what happened and what to do next?
6. Is success unmistakable, and can the user find the saved relationship afterward?
7. Can the user download the CSV without understanding file formats or developer terminology?
8. Can the whole task be completed without a mouse?

Report this as a heuristic walkthrough, not as actual user research. If any step needs explanation from a developer, simplify the interface or its wording.
