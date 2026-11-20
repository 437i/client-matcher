# Testing and verification rules

- Сначала определи baseline: текущий Git diff и релевантные существующие тесты.
- Новую бизнес-логику покрывай table-driven unit tests; ошибки проверяй через `errors.Is`.
- Для PostgreSQL repository нужны интеграционные тесты с реальным PostgreSQL/Testcontainers, если это уместно и доступно; не выдавай unit-тест мока за интеграционный.
- Для HTTP-контрактов проверяй method/path, request binding, статус, JSON schema/ключи и Content-Type.
- Для CSV проверяй заголовок, число колонок, escaping и пустую выборку. Не печатай реальные входные данные в тестовые логи.
- После правок Go-кода запускай `go test ./...`, `go vet ./...`, `go build ./...` из `backend/`.
- После любых правок проверяй `git diff --check`.
- Docker-проверки запускай только если Docker доступен. Не выполняй `docker compose down -v` и не удаляй volume.
- В итоговом отчёте перечисляй точные команды и их результат. Не утверждай, что проверка выполнена, если её не запускали.
