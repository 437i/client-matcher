**Вердикт: подтверждённых блокирующих Critical, High и Medium замечаний нет.** Прежний MD1 закрыт.

Что проверено по четырём файлам (`design.md`, `tasks.md` и две spec):

- **Enum и порядок `FieldCheck.field`:** `last_name`, `first_name`, `birthday` в одном и том же порядке заданы в design (раздел «Единые поля…») и в spec («Единые проверки…»). Тест порядка есть в задаче 1.2. Отчество сайта неизвестно и отдельного check не создаёт.
- **Агрегация статуса:** любой `missing`, `alias_match`, `probable_match` или `mismatch` даёт `requires_confirmation`. `ready` возможен только при `exact` или `normalized_match` по всем трём полям. Бизнес-блокировка даёт `blocked` с пустыми `checks`. Это согласовано между design, spec и задачей 1.2.
- **Mapping field→audit codes:** `last_name` и `first_name` сводятся в `name:code`, `birthday` в `birthday:code`, коды дедуплицируются и сортируются. Whitelist в DB CHECK соответствует достижимым кодам. `alias_match` возможен только для `first_name`, а у даты нет `alias_match` и `probable_match`. Любой `requires_confirmation` даёт хотя бы один код, поэтому `cardinality>0` не противоречит. Тест mapping есть в задаче 3.1.
- **Prefix/substring:** без полного согласования компонента даётся `mismatch`. Это есть в design, в spec (сценарий «Частичное совпадение имени») и в задаче 1.2.
- **`client.ErrInvalidSourceData`:** sentinel и проверка `errors.Is` ведут к 502 `invalid_source_client_data`. Сценарий проверяется mock-портом, потому что CSV пропускает malformed-строки. Это отражено в задаче 2.2.
- **Missing source и occupied lookup:** порядок задан как ID → CRM → Site → (если обе записи есть) lookup. При любом not-found lookup не выполняется. Это есть в design и в spec «Источник отсутствует и ID занят». Приоритет на create согласуется с приоритетом при assessment.

Оставшиеся Low (отдельного цикла не требуют):

- Если CRM вернул invalid source, а Site отсутствует, результатом будет 502 из-за порядка CRM→Site. Стоит одной строкой зафиксировать это в тестах 2.2.
- Правила для `UnstructuredName` («обе роли неизвестны → оба check `missing`») стоит явно перечислить в тестовых кейсах 1.1/1.2.

Apply не разрешён: нужно отдельное решение пользователя.

STAGE1_PLAN_REVIEW_DONE
