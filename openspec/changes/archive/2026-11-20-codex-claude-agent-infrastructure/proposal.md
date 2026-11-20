## Why

Нужны общие процедуры Codex/Claude/OpenCode, простой независимый review и локальный учёт расходов.

## What Changes

- AGENTS.md — общий источник; CLAUDE.md импортирует его.
- Шесть проектных skills в .agents/skills; ссылки Claude/OpenCode; сохранение OpenSpec, commands и agents.
- Согласование синтетических CSV, существующего frontend и границы proposal/apply.
- Sol medium, Luna high, максимум 2 без вложенности; полный контракт делегирования.
- Claude Plan review без записи/shell/MCP; проверка точных Haiku 5.5/Sonnet 5.5 IDs, сохранение Auto Memory.
- Python Wellflow Collector: пагинация, Decimal, ошибки, сводка, атомарные полные snapshots вне Git, unittest.

## Capabilities

### New Capabilities
- shared-agent-guidance: общие инструкции и skills трёх CLI.
- independent-agent-review: модели, review и Wellflow Collector.

### Modified Capabilities
Нет.

## Impact

Только инфраструктура, документация и синтетические тесты. Apply явно разрешён текущим запросом. Backend/frontend бизнес-код, API/OpenAPI, БД/миграции, пользовательские данные и глобальные настройки сохраняются. Без orchestrator, security gates, изоляционных стендов, commit/push и destructive Git. Секреты не публикуются, credentials без необходимости не читаются, реальные клиентские данные не добавляются. Collector самостоятельно получает ключ при отдельном запуске пользователем; реальный Wellflow запрос при реализации не выполняется.
