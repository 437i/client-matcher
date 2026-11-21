# Проверка инфраструктуры агентов

Разделы до повторной headless-проверки — исторический отчёт; текущие результаты находятся ниже.

Дата: 2026-11-20. Реализация выполнена последовательно без субагентов, без commit/push, реальных DB/API запросов Wellflow и чтения настоящих credentials.

## Выполнено

- Пять planning-файлов существующего change сокращены; strict validation прошла до Apply. Удалены pre-Apply gates, изоляционные стенды и обязательные доказательства deny-read.
- Создан CLAUDE.md с импортом AGENTS.md. Согласованы общие инструкции, синтетические CSV и существующий frontend.
- Сохранены имена шести OpenCode commands и двух reviewer agents; процедуры вынесены в skills, адаптеры сокращены. Permissions больше не запрещают все CSV; auth.json и env остаются закрыты.
- Политика Sol medium / Luna high / максимум 2 без вложенности и полный контракт делегирования описаны в AGENTS.md и docs/agent-workflow.md. Глобальные настройки не менялись.
- Реализован Collector и документация запуска. 19 unittest прошли на mocks и временных synthetic fixtures без сети/auth. Проверены пагинация, дубликаты, Decimal, interval, schema/JSON, timeout, HTTP errors/retries, получение ключа, отсутствие утечек, атомарная запись и права.
- На временном синтетическом проекте проверена команда завершения переноса skills, включая повторный запуск.
- Файловая проверка: в .claude/skills и .opencode/skills разрешаются все 12 SKILL.md — шесть проектных и шесть OpenSpec. References frontend сохраняются. OpenSpec skills побайтово совпадают с исходным состоянием.
- Backend Go, go.mod и OpenAPI побайтово совпадают с baseline начала работы: пользовательская реализация CRM name search сохранена. Frontend business code не редактировался.
- opencode.json проходит json.loads / python3 -m json.tool. git diff --check прошёл.

## Завершение переноса и ограничения CLI

Перенос завершён. Файловая проверка 2026-11-20 подтверждает все шесть проектных skills в каноническом .agents/skills: safe-go-change, backend-review, openapi-swagger, frontend-implementation, frontend-review и frontend-ux. Относительные ссылки .claude/skills и .opencode/skills разрешаются в эти каталоги; frontend references и ссылки на OpenSpec workflows сохранены. Повторный python3 scripts/finalize_agent_skills.py --check завершился успешно без оставшихся операций переноса. Задача 1.1 отмечена выполненной.

Прежний запрет sandbox на запись в .agents/skills больше не означает незавершённый перенос: текущее состояние проверено по файлам. Runtime discovery этим не подтверждается.

Codex 0.170.0: прежний запуск debug prompt-input завершился Operation not permitted. Runtime discovery Codex остаётся неподтверждённым; после переноса канонический каталог содержит проектные и OpenSpec skills. Проверка файлов и ссылок не заменяет runtime discovery.

OpenCode 1.19.4: opencode --pure debug skill и debug config завершаются до загрузки проекта из-за запрета записи ~/.local/share/opencode/log/opencode.log. Runtime schema validation/discovery не подтверждены; JSON синтаксис и ссылки проверены отдельно. Не создавался временный профиль с auth. После завершённого переноса обе команды требуют локальной повторной проверки.

Claude Code 2.1.328: help подтверждает Plan mode, tools allowlist, disallowedTools, strict-mcp-config, settings и permission-prompts none. Отдельный CLI listing skills не обнаружен; файловые ссылки проверены. После запуска обычной сессии проверить наличие skills в её меню.

## Claude review

Независимое review НЕ проведено. CLI доступен, но точные Haiku 5.5/Sonnet 5.5 IDs и доступность моделей у провайдера не подтверждены. Попытка получить официальный каталог https://platform.claude.com/docs/en/about-claude/models/overview завершилась URLError в ограниченной сети. Модель не угадывалась, alias не выдавался за проверенный ID. Вызов модели не выполнялся; это не успешное review.

Для этой инфраструктуры нужен Sonnet 5.5, поскольку меняются permissions и обработка credentials. После подтверждения точного ID запустить Plan review по docs/agent-workflow.md для инфраструктуры, включая новые файлы, без backend/OpenAPI пользовательского diff. Auto Memory не отключалась и не очищалась; Claude-Mem не устанавливался.

Backend/frontend тесты и Docker не запускались: бизнес-код не входит в scope. Реальная совместимость Wellflow API, единица cost_cents и полный runtime discovery требуют локальной проверки. Перенос skills завершён; runtime discovery и независимый Claude Review остаются неподтверждёнными. Архивация не является подтверждением этих проверок.

## Повторная проверка headless из Codex — 2026-11-20

Историческая проверка выше не изменена. Пользователь подтвердил успешный интерактивный Claude Review с claude-sonnet-5-5, effort medium и вердиктом READY WITH NOTES.

Новая проверка: /Users/alexfisher/.local/bin/claude, Claude Code 2.1.329. Запущена указанная пользователем команда claude --print из корня проекта: model claude-sonnet-5-5, effort medium, permission-mode plan, tools Read/Grep/Glob, strict MCP с пустым конфигом, disableAllHooks=true, permission-prompts none, max-turns 5.

Первый запуск в sandbox: exit 1, API Error: Can't reach the API server — check your internet or DNS (ENOTFOUND).
Повторный конкретный запуск с одноразовым разрешением вне sandbox: exit 0, содержательный ответ о проектном review и маркер CLAUDE_HEADLESS_OK. Ошибок авторизации или модели нет, fallback-model не задан. CLI подтвердил маршрутизацию через api.wellflow.dev предупреждением о несовместимости будущего auto-mode classifier billing. Предупреждение не блокировало Plan/read-only вызов; auto mode не используется.

Вывод был текстовым: попыток shell/записи не наблюдалось, но трасса tool calls этим запуском не предоставлялась. Для последующих review используется stream-json с проверкой tool calls. Успех smoke test подтверждает headless-маршрут, но не заменяет review конкретного change. Глобальные настройки, credentials и Auto Memory не менялись; прежние ограничения Codex/OpenCode discovery автоматически устранёнными не считаются.

## Фактические независимые review после smoke test

Stage 0 Code Review и повторная проверка, Stage 1 Plan Review и две повторные проверки реально выполнены Codex через claude --print с claude-sonnet-5-5, effort medium. Все exit codes 0, is_error=false и итоговые маркеры присутствуют. Stream-json проверен: модель claude-sonnet-5-5, tool calls только Read/Grep/Glob, shell/write/Agent/MCP не вызывались. Итоговые отчёты: stage0-code-review.md и stage1-plan-review.md. История: stage0-code-review-initial.md, stage1-plan-review-initial.md, stage1-plan-review-followup.md. Это новые проверки; они не изменяют исторический результат первого отчёта. Smoke использовал max-turns=5, полноценные review — 20/12/8 в зависимости от scope.
