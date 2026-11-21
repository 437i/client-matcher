# Работа агентов

Общий источник — AGENTS.md и применимые вложенные инструкции. Канонический каталог skills — .agents/skills; Claude и OpenCode должны использовать относительные ссылки на него. Перенос завершён; CLI-каталоги используют ссылки на .agents/skills.

Основной Codex: gpt-6.1-sol, reasoning medium. Он отвечает за архитектуру, задачи, интеграцию, проверку результатов субагентов, тесты и итоговый результат. Субагенты: gpt-6-luna, reasoning high; максимум 2 одновременно, вложенные субагенты запрещены. Контракт задачи включает цель, типы данных, инварианты, ограничения, граничные значения, область изменения и критерии результата. Предположения явно отделяются от требований. Пересекающиеся изменения выполняются последовательно. DeepSeek, GLM и динамический маршрутизатор не добавляются.

Рекомендуемый локальный фрагмент Codex config (применяет пользователь; глобальные настройки и sol.config.toml агент не меняет):

```toml
model = "gpt-6.1-sol"
model_reasoning_effort = "medium"

[agents]
default_subagent_model = "gpt-6-luna"
default_subagent_reasoning_effort = "high"
max_concurrent_threads_per_session = 2
```

Это рекомендуемые настройки; фактическая поддержка зависит от установленной версии Codex и провайдера. Они не меняют модель уже работающей сессии.

## Полный цикл

Preflight → Explore → Propose → Plan Review → User Approval → Apply → Verify → Code Review → Fix/Re-verify → Close.

Перед работой читать docs/roadmap.md и docs/PROJECT_STATUS.md. Preflight: cwd, ветка, HEAD, status с untracked, unstaged/staged diff и локальный upstream ref без fetch. Сохранять пользовательские изменения; обновлять PROJECT_STATUS при смене фазы и на границах этапов.

Для существенных изменений OpenSpec фиксирует proposal, specs, design (включая Domain Concepts) и tasks. Plan Review обязателен после Explore/Propose и до запроса разрешения на Apply. Пользователь подтверждает начало Apply; технические детали в согласованном scope решает Codex. После разрешения Codex выполняет весь tasks.md, не останавливаясь после каждой задачи.

После реализации выполнить основные проверки и обязательный независимый Code Review. Codex проверяет findings: гипотезы не считаются дефектами. Подтверждённые Critical/High/Medium в scope исправляются с повторными проверками; Claude повторно проверяет изменённую область. Необязательные Low не создают бесконечный цикл. Close — только по фактическим критериям готовности. Sync specs/Archive — отдельные действия, без автоматической архивации. Не переходить к следующему этапу при открытом gate.

## Headless Claude Review

Пользователь подтвердил интерактивную работу точного model ID claude-sonnet-5-5, effort medium. Headless smoke test из Codex также прошёл 2026-11-20; подробности в agent-infrastructure-verification.md. Это основной reviewer архитектуры, matching, API, БД и безопасности.

Codex самостоятельно запускает из корня проекта: claude --print --model claude-sonnet-5-5 --effort medium --permission-mode plan --tools "Read,Grep,Glob" --disallowedTools "Bash,PowerShell,Edit,Write,NotebookEdit,Agent" --strict-mcp-config --mcp-config '{"mcpServers":{}}' --settings '{"disableAllHooks":true,"useAutoModeDuringPlan":false}' --permission-prompts none --max-turns 20 'ЗАДАНИЕ REVIEW С ТОЧНЫМ SCOPE'.

Plan Review получает proposal, specs, design, tasks и необходимые инструкции проекта. Проверяет полноту требований, доменную модель, архитектуру, ограничения, риски и критерии готовности.

Code Review получает точный список изменённых файлов, включая untracked, соответствующий OpenSpec change, фактический diff и результаты команд. Если change отсутствует для локальной корректировки, явно указать это. Не передавать несвязанный пользовательский diff.

Инструменты Claude — только Read, Grep, Glob. Bash, Edit, Write, Agent и MCP недоступны; hooks отключены для данного запуска. Запрещён --dangerously-skip-permissions. Не читать секреты, .env, credentials, auth.json и реальные данные. Auto Memory сохраняется; --bare/--safe-mode не использовать, глобальную защиту и настройки не менять.

Фиксировать exit code, ошибки модели/авторизации, фактический ответ и результаты проверок. Для контроля tool calls использовать --output-format stream-json --verbose; не задавать fallback-model. Отчёт: findings по severity с файлом/строкой, сценарием, влиянием, минимальным исправлением; выполненные/пропущенные проверки, ограничения, вердикт. Codex проверяет замечания, исправляет подтверждённые дефекты и повторно запускает Claude для изменённой области.

Если reviewer недоступен, завершить подготовительную работу, записать точную причину и оставить gate открытым. Предложить безопасный ручной запуск этой же команды или повторную попытку. Недоступность review не является успешным прохождением. Существенный Apply требует Plan Review и отдельного разрешения пользователя; принятие этапа требует Code Review.

Commit/push не выполняются автоматически. CLAUDE.md остаётся минимальным импортом AGENTS.md.

Headless review требует сети к настроенному провайдеру. При sandbox ENOTFOUND запросить разрешение на конкретный запуск. Scope ограничивать исходниками, документацией и подтверждённой синтетикой; реальные данные и секреты исключать.
