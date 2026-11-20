## 1. Общие инструкции и CLI

- [x] 1.1 Перенести шесть skills в .agents/skills, сохранить references/OpenSpec и создать ссылки Claude/OpenCode; проверить целостность.
- [x] 1.2 Согласовать AGENTS.md, CLAUDE.md, rules/data-safety.md, frontend-инструкции и openspec/config.yaml с синтетическими данными и существующим frontend.
- [x] 1.3 Сделать шесть OpenCode commands и два agents адаптерами; согласовать opencode.json, проверить config/discovery или описать файловый fallback.

## 2. Модели и review

- [x] 2.1 Зафиксировать Sol medium, Luna high, лимит 2 без вложенности, контракт делегирования и локальный config в AGENTS.md/docs/agent-workflow.md.
- [x] 2.2 Описать Claude Plan review, проверку точных моделей, tools и отчёт; сохранить Auto Memory.

## 3. Wellflow

- [x] 3.1 Реализовать scripts/wellflow_usage.py: ключ, пагинация, schema/Decimal, timeout/retries/errors, фильтр, агрегация и атомарные private snapshots вне Git.
- [x] 3.2 Добавить unittest для пагинации, пустой истории, дробей, дубликатов, повторов cursor, JSON/schema, timeout, 401/403, 429/5xx, отсутствия и неразглашения ключа без реальной сети/auth.

## 4. Проверка

- [x] 4.1 Запустить strict OpenSpec, diff --check, unittest, ссылки/OpenSpec и доступное CLI discovery; записать результаты и ограничения в docs/agent-infrastructure-verification.md.
- [x] 4.2 Попытаться провести Claude review итоговой инфраструктуры при доступных CLI/model, исправить дефекты либо честно зафиксировать недоступность.

## Остаток и ограничения

Задача 1.1 выполнена: все шесть проектных skills находятся в .agents/skills; ссылки Claude/OpenCode, frontend references и OpenSpec workflows проверены. Повторный scripts/finalize_agent_skills.py --check подтверждает отсутствие оставшегося переноса.

4.1 выполнена с файловым fallback: runtime discovery Codex/OpenCode блокируется правами среды. 4.2 выполнена как проверка доступности: независимое review не состоялось, точные model IDs не подтверждены из-за недоступности каталога. Это не успешное Claude review. Подробности: docs/agent-infrastructure-verification.md. Перенос завершён; runtime discovery и независимый Claude Review остаются неподтверждёнными.
