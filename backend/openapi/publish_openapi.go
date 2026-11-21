//go:build ignore

// publish_openapi.go проверяет сгенерированную спецификацию и публикует её.
//
// Генератор swag складывает документы в .stage, поэтому оба файла готовы и
// согласованы до попадания в репозиторий. Шаг проверяет, что:
//
//   - JSON и YAML — валидные документы OpenAPI 3.1;
//   - они описывают одинаковые операции, статусы, типы содержимого и схемы;
//   - типы содержимого совпадают с поведением обработчиков: у успешного ответа
//     выгрузки CSV — text/csv, у остальных ответов — application/json, а ошибки
//     /api/v1 ссылаются на схему handlers.ErrorResponse.
//
// Только после проверки файлы публикуются: сначала оба записываются во
// временные файлы и переименовываются на место, чтобы неудачная запись
// второго файла не оставила репозиторий с расхождением. Очистка временного
// каталога выполняется после публикации и не влияет на её результат: если
// удалить каталог не удалось, шаг сообщает об этом предупреждением и
// завершается успешно.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	yaml "go.yaml.in/yaml/v4"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	stageDir = ".stage"

	jsonFile = "swagger.json"
	yamlFile = "swagger.yaml"

	exportPath = "/api/v1/links/export.csv"

	csvMediaType  = "text/csv"
	jsonMediaType = "application/json"

	errorSchemaRef = "#/components/schemas/handlers.ErrorResponse"

	apiPrefix = "/api/v1"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "publish openapi:", err)
		os.Exit(1)
	}
}

func run() error {
	stagedJSON, err := os.ReadFile(filepath.Join(stageDir, jsonFile))
	if err != nil {
		return fmt.Errorf("read staged %s: %w", jsonFile, err)
	}
	stagedYAML, err := os.ReadFile(filepath.Join(stageDir, yamlFile))
	if err != nil {
		return fmt.Errorf("read staged %s: %w", yamlFile, err)
	}

	stagedJSON, err = normalizeNullable(stagedJSON, json.Unmarshal, func(value any) ([]byte, error) { return json.MarshalIndent(value, "", "    ") })
	if err != nil {
		return fmt.Errorf("normalize JSON nullable fields: %w", err)
	}
	stagedYAML, err = normalizeNullable(stagedYAML, yaml.Unmarshal, yaml.Marshal)
	if err != nil {
		return fmt.Errorf("normalize YAML nullable fields: %w", err)
	}

	jsonDoc, err := load(stagedJSON)
	if err != nil {
		return fmt.Errorf("%s: %w", jsonFile, err)
	}
	yamlDoc, err := load(stagedYAML)
	if err != nil {
		return fmt.Errorf("%s: %w", yamlFile, err)
	}

	if err := checkContentTypes(jsonDoc); err != nil {
		return fmt.Errorf("%s: %w", jsonFile, err)
	}
	if err := checkContentTypes(yamlDoc); err != nil {
		return fmt.Errorf("%s: %w", yamlFile, err)
	}
	if err := checkSameShape(jsonDoc, yamlDoc); err != nil {
		return err
	}

	if err := publish(jsonFile, stagedJSON, yamlFile, stagedYAML); err != nil {
		return err
	}
	if err := os.RemoveAll(stageDir); err != nil {
		// Артефакты уже опубликованы: сбой очистки временного каталога не должен
		// выглядеть как неудачная генерация.
		fmt.Fprintf(os.Stderr, "publish openapi: warning: remove %s: %v\n", stageDir, err)
	}
	return nil
}

// Swag emits x-nullable; OpenAPI 3.1 requires a JSON Schema type union.
func normalizeNullable(data []byte, decode func([]byte, any) error, encode func(any) ([]byte, error)) ([]byte, error) {
	var doc map[string]any
	if err := decode(data, &doc); err != nil {
		return nil, err
	}
	components, _ := doc["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	for _, value := range schemas {
		schema, _ := value.(map[string]any)
		props, _ := schema["properties"].(map[string]any)
		for _, value := range props {
			prop, _ := value.(map[string]any)
			if prop["x-nullable"] != true {
				continue
			}
			typ, ok := prop["type"].(string)
			if !ok {
				return nil, fmt.Errorf("x-nullable requires a single schema type")
			}
			prop["type"] = []string{typ, "null"}
			delete(prop, "x-nullable")
		}
	}
	return encode(doc)
}

// load разбирает документ и проверяет его как OpenAPI 3.1.
func load(data []byte) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.1") {
		return nil, fmt.Errorf("версия OpenAPI %q, ожидалась 3.1.x", doc.OpenAPI)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return nil, err
	}
	return doc, nil
}

// checkContentTypes сверяет типы содержимого ответов с поведением обработчиков:
// выгрузка CSV отдаёт text/csv только при успехе, остальные ответы —
// application/json, а ошибки /api/v1 — со схемой handlers.ErrorResponse.
func checkContentTypes(doc *openapi3.T) error {
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			for status, resp := range op.Responses.Map() {
				if resp == nil || resp.Value == nil {
					return fmt.Errorf("%s %s: ответ %s без описания", method, path, status)
				}

				want := jsonMediaType
				if path == exportPath && status == "200" {
					want = csvMediaType
				}
				if got := mediaType(resp.Value); got != want {
					return fmt.Errorf("%s %s: ответ %s отдаёт %q, ожидался %q",
						method, path, status, got, want)
				}

				if isError(status) && strings.HasPrefix(path, apiPrefix) && schemaRef(resp.Value) != errorSchemaRef {
					return fmt.Errorf("%s %s: ответ %s ссылается на схему %q, ожидалась %q",
						method, path, status, schemaRef(resp.Value), errorSchemaRef)
				}
			}
		}
	}
	return nil
}

// checkSameShape сверяет JSON и YAML: оба файла описывают один и тот же
// контракт, поэтому расхождение означает, что публиковать нечего.
func checkSameShape(jsonDoc, yamlDoc *openapi3.T) error {
	jsonShape := shape(jsonDoc)
	yamlShape := shape(yamlDoc)
	if !reflect.DeepEqual(jsonShape, yamlShape) {
		return fmt.Errorf("%s и %s описывают разный контракт:\njson: %v\nyaml: %v",
			jsonFile, yamlFile, jsonShape, yamlShape)
	}
	return nil
}

// publish записывает оба файла во временные, а затем переименовывает их на
// место. Если второе переименование не удалось, первый файл возвращается к
// прежнему содержимому.
func publish(jsonName string, jsonData []byte, yamlName string, yamlData []byte) error {
	previousJSON, err := os.ReadFile(jsonName)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", jsonName, err)
	}

	jsonTmp := jsonName + ".tmp"
	yamlTmp := yamlName + ".tmp"
	if err := os.WriteFile(jsonTmp, jsonData, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", jsonTmp, err)
	}
	if err := os.WriteFile(yamlTmp, yamlData, 0o644); err != nil {
		os.Remove(jsonTmp)
		return fmt.Errorf("write %s: %w", yamlTmp, err)
	}

	if err := os.Rename(jsonTmp, jsonName); err != nil {
		os.Remove(jsonTmp)
		os.Remove(yamlTmp)
		return fmt.Errorf("publish %s: %w", jsonName, err)
	}
	if err := os.Rename(yamlTmp, yamlName); err != nil {
		os.Remove(yamlTmp)
		if restoreErr := restore(jsonName, previousJSON); restoreErr != nil {
			return fmt.Errorf("publish %s: %w (и не удалось вернуть прежний %s: %v)",
				yamlName, err, jsonName, restoreErr)
		}
		return fmt.Errorf("publish %s: %w", yamlName, err)
	}
	return nil
}

// restore возвращает файл к прежнему содержимому.
func restore(name string, previous []byte) error {
	if previous == nil {
		if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(name, previous, 0o644)
}

// mediaType возвращает единственный тип содержимого ответа.
func mediaType(resp *openapi3.Response) string {
	if len(resp.Content) != 1 {
		return ""
	}
	for mediaType := range resp.Content {
		return mediaType
	}
	return ""
}

// schemaRef возвращает ссылку на схему ответа.
func schemaRef(resp *openapi3.Response) string {
	for _, mediaType := range resp.Content {
		if mediaType.Schema == nil {
			return ""
		}
		return mediaType.Schema.Ref
	}
	return ""
}

// isError сообщает, что статус относится к ошибке.
func isError(status string) bool {
	return strings.HasPrefix(status, "4") || strings.HasPrefix(status, "5")
}

// shape описывает контракт документа: операции, статусы, типы содержимого и
// ссылки на схемы.
func shape(doc *openapi3.T) []string {
	var out []string
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			for status, resp := range op.Responses.Map() {
				media, schema := "", ""
				if resp != nil && resp.Value != nil {
					media = mediaType(resp.Value)
					schema = schemaRef(resp.Value)
				}
				out = append(out, fmt.Sprintf("%s %s %s %s %s", method, path, status, media, schema))
			}
		}
	}
	sort.Strings(out)
	return out
}
