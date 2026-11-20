---
name: backend-review
description: Провести read-only аудит Go backend или Git diff, найти конкретные ошибки и риски без изменения файлов.
compatibility: Codex, Claude Code, OpenCode
---

# Backend review

1. Выполни `git status --short` и изучи `git diff`; не трогай изменения пользователя.
2. Проследи затронутый сценарий по всем слоям: router → handler/DTO → service → ports → adapter → domain/repository.
3. Ищи реальные ошибки, нарушения контракта, race conditions, утечки ресурсов, некорректную обработку context/errors и недостатки тестов.
4. Для API сверяй документацию с реальными handler/DTO/status mapping и tests.
5. Синтетические CSV проекта допустимы. Соблюдай rules/data-safety.md; не читай .env, credentials и секреты без необходимости.
6. Ничего не изменяй и не создавай. Не выполняй форматтеры или генераторы.
7. Если нужно запустить тесты, делай это только безопасными командами и отмечай результат.
8. Отчёт: сначала findings по severity, затем подтверждения, затем непроверенные области. Не перечисляй косметические предпочтения как дефекты.
