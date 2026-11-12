// Package openapi хранит сгенерированную спецификацию API и встраивает её
// в бинарник: отдаваемый документ побайтово совпадает с файлом в репозитории.
package openapi

import _ "embed"

// JSON — спецификация API в формате OpenAPI 3.1.
//
//go:embed swagger.json
var JSON []byte

// YAML — та же спецификация в формате YAML.
//
//go:embed swagger.yaml
var YAML []byte
