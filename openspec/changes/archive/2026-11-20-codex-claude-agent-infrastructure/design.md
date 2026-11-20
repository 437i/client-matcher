## Context

Frontend уже существует. Шесть проектных skills находятся в .opencode/skills; шесть OpenSpec workflows — в .agents/skills. Пользовательский backend/OpenAPI diff сохраняется.

## Decisions

1. Перенести проектные skills вместе с references в .agents/skills; оба CLI-каталога используют относительные ссылки. OpenSpec не меняется. CLAUDE.md импортирует AGENTS.md. Commands и agents OpenCode сохраняют имена и делегируют общим skills. Proposal не запускает Apply без авторизации; текущая задача авторизацию имеет.
2. Синтетические CSV проекта разрешены. Секреты не публикуются, реальные клиентские данные не добавляются, credentials без необходимости не читаются, destructive Git запрещён. Permissions не объявляются полной изоляцией.
3. Основной Codex gpt-6.1-sol/medium отвечает за архитектуру, задачи, интеграцию, проверку субагентов, тесты и результат. Субагенты gpt-6-luna/high: максимум 2 без вложенности. Контракт: цель, типы, инварианты, ограничения, граничные значения, scope, критерии результата. Предположения явно отделяются от требований. Документируется локальный config, глобальные настройки и sol.config.toml не меняются. DeepSeek/GLM и динамический маршрутизатор не добавляются. Эта реализация последовательна без субагентов.
4. Claude review обязателен для архитектуры, business logic, API, DB, безопасности, matching и этой инфраструктуры; тривиальная документация допускает проверки Codex. Обычная модель Haiku 5.5, критичная Sonnet 5.5 после проверки точного ID. Plan mode, только Read/Grep/Glob, пустой MCP, без shell/edit и dangerously-skip-permissions. Auto Memory сохраняется, Claude-Mem не устанавливается. Отчёт: findings/severity/файл/строка/сценарий/влияние/минимальное исправление, выполненные и пропущенные проверки, ограничения. Исправления проверяет Codex; изменённый scope пересматривается. Недоступный CLI/model честно фиксируется без остановки остальных задач.
5. Collector использует urllib/json/Decimal/argparse стандартной библиотеки. GET https://api.wellflow.dev/v1/usage?limit=20, cursor до null. data содержит created_at, model, input_tokens, output_tokens, total_tokens, cost_cents. Проверяются структура, типы, timestamps и повтор cursor. Ключ: WELLFLOW_API_KEY, иначе OPENAI_API_KEY из CODEX_HOME/auth.json или ~/.codex/auth.json. Агент реальные auth не читает. Timeout 30 секунд; максимум 3 попытки timeout/429/5xx с ограниченным Retry-After. 401/403, JSON/schema ошибки завершают сбор без полного snapshot. Redirect не передаёт Bearer на другой адрес.
6. Каждый полный обход атомарно сохраняется отдельно в ~/Library/Application Support/client-matcher/wellflow-usage/, вне Git. macOS: каталог 0700, файлы 0600. Decimal сериализуется точно без float. Все записи и дубликаты сохраняются, request_id не создаётся. Фильтр [from, until) влияет на отчёт, snapshot полный. cost_cents выводится без долларов и атрибуции задачам.

## Verification

Strict OpenSpec, diff --check, unittest без сети/auth, ссылки и сохранность OpenSpec, доступное CLI discovery/config. Backend/frontend тесты не нужны. Реальный Wellflow запрос не выполняется. Недоступные опции CLI документируются с файловым fallback. Archive после проверки отдельным действием.
