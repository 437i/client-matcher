//go:build tools

package openapi

// Файл фиксирует зависимость github.com/getkin/kin-openapi, которую использует
// публикация спецификации (publish_openapi.go, отдельная программа с тегом
// ignore). go mod tidy не видит импорты файлов с тегом ignore, поэтому без
// этого файла требование могло бы исчезнуть из go.mod.
//
// Тег tools не включается при обычной сборке, поэтому в бинарник backend
// зависимость не попадает.
import _ "github.com/getkin/kin-openapi/openapi3"
