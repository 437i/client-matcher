package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/437i/client-matcher/backend/openapi"
)

// Служебные маршруты документации не описывают сами себя в спецификации.
var docsRoutes = map[string]bool{
	"/openapi.json": true,
	"/openapi.yaml": true,
	"/docs":         true,
	"/docs/*any":    true,
}

type specDocument struct {
	OpenAPI string `json:"openapi"`
	Servers []struct {
		URL string `json:"url"`
	} `json:"servers"`
	Paths map[string]map[string]json.RawMessage `json:"paths"`
}

func loadSpec(t *testing.T) specDocument {
	t.Helper()

	var doc specDocument
	if err := json.Unmarshal(openapi.JSON, &doc); err != nil {
		t.Fatalf("decode spec: %v", err)
	}
	return doc
}

func TestSpecEndpointsServeCommittedArtifacts(t *testing.T) {
	srv := newTestServer(t, &memLinkRepo{})

	tests := []struct {
		target      string
		contentType string
		want        []byte
	}{
		{"/openapi.json", "application/json; charset=utf-8", openapi.JSON},
		{"/openapi.yaml", "application/yaml; charset=utf-8", openapi.YAML},
	}

	for _, tc := range tests {
		rec := do(t, srv, http.MethodGet, tc.target, "")
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", tc.target, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); ct != tc.contentType {
			t.Errorf("%s: Content-Type = %q, want %q", tc.target, ct, tc.contentType)
		}
		if rec.Body.String() != string(tc.want) {
			t.Errorf("%s: body differs from the committed artifact", tc.target)
		}
	}
}

func TestSpecIsValidOpenAPI31(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(openapi.JSON)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}

	if !strings.HasPrefix(doc.OpenAPI, "3.1") {
		t.Errorf("openapi version = %q, want 3.1.x", doc.OpenAPI)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Errorf("validate spec: %v", err)
	}
}

// YAML-версия спецификации тоже должна быть валидным документом OpenAPI 3.1
// и описывать те же операции, что и JSON.
func TestSpecYAMLIsValidOpenAPI31(t *testing.T) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(openapi.YAML)
	if err != nil {
		t.Fatalf("load yaml spec: %v", err)
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.1") {
		t.Errorf("openapi version = %q, want 3.1.x", doc.OpenAPI)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Errorf("validate yaml spec: %v", err)
	}

	jsonSpec := loadSpec(t)
	yamlOps := make([]string, 0)
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			yamlOps = append(yamlOps, strings.ToUpper(method)+" "+path)
		}
	}
	jsonOps := make([]string, 0)
	for path, item := range jsonSpec.Paths {
		for method := range item {
			jsonOps = append(jsonOps, strings.ToUpper(method)+" "+path)
		}
	}
	sort.Strings(yamlOps)
	sort.Strings(jsonOps)

	if !slices.Equal(yamlOps, jsonOps) {
		t.Errorf("yaml operations %v differ from json operations %v", yamlOps, jsonOps)
	}
}

func TestSpecPathsMatchRouter(t *testing.T) {
	srv := newTestServer(t, &memLinkRepo{})

	spec := loadSpec(t)
	specOps := make(map[string]bool)
	for path, item := range spec.Paths {
		for method := range item {
			specOps[strings.ToUpper(method)+" "+path] = true
		}
	}

	routerOps := make(map[string]bool)
	for _, route := range srv.Routes() {
		if docsRoutes[route.Path] {
			continue
		}
		routerOps[route.Method+" "+openAPIPath(route.Path)] = true
	}

	for op := range routerOps {
		if !specOps[op] {
			t.Errorf("route %s is not documented in the spec", op)
		}
	}
	for op := range specOps {
		if !routerOps[op] {
			t.Errorf("spec documents %s, but the route does not exist", op)
		}
	}
}

func TestSpecServersDoNotDuplicateBasePath(t *testing.T) {
	spec := loadSpec(t)

	if len(spec.Servers) != 1 {
		t.Fatalf("servers = %d, want 1", len(spec.Servers))
	}
	if url := spec.Servers[0].URL; strings.Contains(url, "/api/v1") {
		t.Errorf("server url %q duplicates the /api/v1 prefix", url)
	}
}

// swag применяет @Produce ко всем ответам операции, поэтому шаг нормализации
// при генерации возвращает ошибке 500 у export.csv фактический тип JSON.
func TestSpecExportErrorResponseIsJSON(t *testing.T) {
	loader := openapi3.NewLoader()

	for name, data := range map[string][]byte{"json": openapi.JSON, "yaml": openapi.YAML} {
		t.Run(name, func(t *testing.T) {
			doc, err := loader.LoadFromData(data)
			if err != nil {
				t.Fatalf("load spec: %v", err)
			}

			item := doc.Paths.Find("/api/v1/links/export.csv")
			if item == nil || item.Get == nil {
				t.Fatal("export operation not found")
			}

			ok := item.Get.Responses.Status(http.StatusOK)
			if ok == nil || ok.Value == nil {
				t.Fatal("200 response not found")
			}
			if _, found := ok.Value.Content["text/csv"]; !found || len(ok.Value.Content) != 1 {
				t.Errorf("200: want only text/csv, got %d content type(s)", len(ok.Value.Content))
			}

			fail := item.Get.Responses.Status(http.StatusInternalServerError)
			if fail == nil || fail.Value == nil {
				t.Fatal("500 response not found")
			}
			media, found := fail.Value.Content["application/json"]
			if !found || len(fail.Value.Content) != 1 {
				t.Errorf("500: want only application/json, got %d content type(s)", len(fail.Value.Content))
			} else if media.Schema == nil || media.Schema.Ref != "#/components/schemas/handlers.ErrorResponse" {
				t.Errorf("500 schema = %+v, want handlers.ErrorResponse", media.Schema)
			}
		})
	}
}

func TestSwaggerUIServedLocally(t *testing.T) {
	srv := newTestServer(t, &memLinkRepo{})

	for _, target := range []string{"/docs", "/docs/"} {
		rec := do(t, srv, http.MethodGet, target, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", target, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "/openapi.json") {
			t.Errorf("%s: swagger UI does not load the spec from the backend", target)
		}
		for _, cdn := range []string{"cdn.jsdelivr.net", "unpkg.com", "cdnjs.cloudflare.com"} {
			if strings.Contains(body, cdn) {
				t.Errorf("%s: swagger UI references external CDN %q", target, cdn)
			}
		}
	}

	rec := do(t, srv, http.MethodGet, "/docs/swagger-ui.css", "")
	if rec.Code != http.StatusOK {
		t.Errorf("swagger UI assets: status %d", rec.Code)
	}
}

// Swagger UI только показывает спецификацию, поэтому изменяющие методы на
// маршрутах документации не регистрируются.
func TestSwaggerUIRejectsWriteMethods(t *testing.T) {
	srv := newTestServer(t, &memLinkRepo{})

	for _, target := range []string{"/docs", "/docs/"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			t.Run(method+" "+target, func(t *testing.T) {
				rec := do(t, srv, method, target, "")
				if rec.Code != http.StatusNotFound {
					t.Errorf("%s %s: status %d, want %d", method, target, rec.Code, http.StatusNotFound)
				}
			})
		}
	}
}

// Интерфейс только читает спецификацию, поэтому кроме GET доступен HEAD:
// ответ без тела, но с теми же статусом и заголовками.
func TestSwaggerUISupportsHead(t *testing.T) {
	srv := newTestServer(t, &memLinkRepo{})

	for _, target := range []string{"/docs", "/docs/"} {
		t.Run(target, func(t *testing.T) {
			rec := do(t, srv, http.MethodHead, target, "")
			if rec.Code != http.StatusOK {
				t.Errorf("%s: status %d, want %d", target, rec.Code, http.StatusOK)
			}
		})
	}
}

// openAPIPath переводит путь Gin (:id) в путь OpenAPI ({id}).
func openAPIPath(ginPath string) string {
	parts := strings.Split(ginPath, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
	}
	return strings.Join(parts, "/")
}
