package openapi

// Генерация спецификации OpenAPI 3.1 из аннотаций HTTP-обработчиков.
// Запуск: go generate ./openapi (из каталога backend).
//
// Клиент генератора намеренно не добавлен в go.mod: версия закреплена здесь,
// а готовые swagger.json и swagger.yaml лежат в репозитории.
//
// Генератор пишет документы в .stage, а publish_openapi.go проверяет оба
// формата и публикует их в пакет: так в репозиторий попадает только
// согласованная и валидная спецификация.
//
//go:generate go run github.com/swaggo/swag/v2/cmd/swag@v2.0.0-rc6 init --v3.1 --quiet -g openapi.go -d ../internal/adapters/inbound/http/handlers -o .stage --outputTypes json,yaml
//go:generate go run publish_openapi.go
