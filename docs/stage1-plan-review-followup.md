## Plan Review (повторный): `matching-assessment-link-confirmation`

**Вердикт: READY WITH NOTES.** Critical и High нет. Прежние M1–M4 закрыты. Остались одна Medium и несколько Low.

### Проверка исправлений
- **M1 (blocked/invalid)** закрыт. `blocked_reasons` и 502 `invalid_source_client_data` описаны согласованно в design, обеих specs и tasks 4.1. Приоритет ошибок create задан явно.
- **M2 (fingerprint)** закрыт. Keyed HMAC, раздельные подключи, length-prefix и nil/present маркеры описаны. Токен не содержит PII, а tasks 2.1 это проверяют.
- **M3 (отчество Site)** закрыт. Отчество Site всегда unknown, эвристик нет. Spec-сценарий «Отчество сайта неизвестно» совпадает с design. В `site/csv/repo.go` `parseTitle` сейчас кладёт всё после первого слова в `LastName`, это совпадает с планом («сохранять как фамилию»).
- **M4 (CSV scope)** закрыт. Явно описаны `UnstructuredName`, unknown roles, поиск по таким записям и baseline 200/55. Сейчас `parseTitle` обоих CSV-адаптеров отвергает title из меньше чем двух слов. Строка попадает в warn-skip, так что изменение реально и осознанно. Тест `repo_dataset_test.go` уже фиксирует 200 CRM и 55 строк.

### Medium
- **MD1. Имена полей и коды не определены единообразно.**
  - Design говорит, что фамилия и имя сравниваются отдельно.
  - Whitelist аудита и DB CHECK используют только `name:*` и `birthday:*`.
  - Не сказано, какие значения принимает `FieldCheck.field` (`last_name`, `first_name`, `birthday`?).
  - Не сказано, как проверки фамилии и имени сворачиваются в `name:*` для `confirmation_codes`. Не сказано и то, как они сворачиваются в статус.
  - Нужно зафиксировать enum `field` в HTTP/OpenAPI и правило отображения в audit-код. Иначе тесты 1.2/3.2 и CHECK разойдутся с DTO.
  - Также не определено, какой код получают prefix/substring (по design это confirmation, вероятно `mismatch`).

### Low (существенные)
- **L1. Противоречие spec и design про некорректный ID.** Spec matching-assessment («Невозможная связь заблокирована») говорит «SHALL блокировать некорректный ID». Design и последний requirement возвращают 400 `invalid_id` без assessment. Уберите «некорректный ID» из первого requirement.
- **L2. Как адаптер сигнализирует invalid source, не указано.** CSV такие строки пропускает, и тогда выходит not found. Поэтому 502 через CSV недостижим, и тест нужен на mock-порту. Стоит назвать sentinel-ошибку в ports, иначе приоритет «повторное чтение/invalid/not-found» нельзя реализовать однозначно.
- **L3. Порядок проверок в assessment не определён для случая, когда источник отсутствует и ID занят.** Нужно сказать, выдаются ли оба blocked_reasons или lookup пропускается.

Риски вне scope (multi-instance, TTL, потеря audit в Down) описаны адекватно.

STAGE1_PLAN_REVIEW_DONE
