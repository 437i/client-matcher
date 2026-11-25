package http_test

import (
	"context"
	"errors"
	tokenOutbound "github.com/437i/client-matcher/backend/internal/adapters/outbound"
	"github.com/437i/client-matcher/backend/internal/application/ports"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"

	web "github.com/437i/client-matcher/backend/internal/adapters/inbound/http"
	"github.com/437i/client-matcher/backend/internal/adapters/inbound/http/handlers"
	"github.com/437i/client-matcher/backend/internal/application/services"
	"github.com/437i/client-matcher/backend/internal/domain/client"
	"github.com/437i/client-matcher/backend/internal/domain/link"
	"github.com/437i/client-matcher/backend/openapi"
)

// Тесты сверяют спецификацию OpenAPI с поведением обработчиков.
//
// Статусы выведены из общего мэппинга ошибок handlers/response.go: отменённый
// или превышенный по времени запрос даёт 499 и 504, доменные ошибки — свои
// коды, всё остальное — 500. Если обработчик начнёт отдавать новый статус,
// спецификация перестанет совпадать с поведением и тест упадёт.

// errorSchemaRef — схема общего формата ошибки API.
const errorSchemaRef = "#/components/schemas/handlers.ErrorResponse"

// stubCRM, stubSite и stubLink — источники данных, которые отдают заданную
// ошибку или заданную запись.
type stubCRM struct {
	searchErr error
	getErr    error
	found     client.CRMClient
}

func (s *stubCRM) Search(context.Context, string, int) ([]client.CRMClient, error) {
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return []client.CRMClient{s.found}, nil
}

func (s *stubCRM) GetByID(context.Context, string) (client.CRMClient, error) {
	if s.getErr != nil {
		return client.CRMClient{}, s.getErr
	}
	return s.found, nil
}

type stubSite struct {
	getErr error
	found  client.SiteClient
}

func (s *stubSite) GetByID(context.Context, int64) (client.SiteClient, error) {
	if s.getErr != nil {
		return client.SiteClient{}, s.getErr
	}
	return s.found, nil
}

type stubLink struct {
	crmBusy, siteBusy bool
	createErr         error
	listErr           error
	decisions         []link.Decision
}

func (s *stubLink) Create(_ context.Context, decision link.Decision) (link.ClientLink, error) {
	if s.createErr != nil {
		return link.ClientLink{}, s.createErr
	}
	s.decisions = append(s.decisions, decision)
	return link.ClientLink{}, nil
}

func (s *stubLink) List(context.Context) ([]link.ClientLink, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return nil, nil
}

var (
	validCRM = client.CRMClient{FirstName: "Иван",
		ID:       "1",
		LastName: "Иванов",
		Birthday: &client.Date{Year: 1990, Month: 1, Day: 1},
	}
	validSite = client.SiteClient{ShortName: "Иван",
		ID:       1,
		LastName: "Иванов",
		Birthday: &client.Date{Year: 1990, Month: 1, Day: 1},
	}
)

// parityScenario — один проверяемый вызов: источники настроены так, чтобы
// обработчик вернул ожидаемый статус.
type parityScenario struct {
	name    string
	specKey string
	method  string
	path    string
	body    string
	setup   func(crm *stubCRM, site *stubSite, links *stubLink)
	health  error
	want    int
}

func parityScenarios() []parityScenario {
	return []parityScenario{
		{
			name: "поиск: успех", specKey: "GET /api/v1/crm-clients",
			method: http.MethodGet, path: "/api/v1/crm-clients?q=иван", want: http.StatusOK,
		},
		{
			name: "поиск: пустой запрос", specKey: "GET /api/v1/crm-clients",
			method: http.MethodGet, path: "/api/v1/crm-clients?q=%20", want: http.StatusBadRequest,
		},
		{
			name: "поиск: запрос отменён", specKey: "GET /api/v1/crm-clients",
			method: http.MethodGet, path: "/api/v1/crm-clients?q=иван", want: 499,
			setup: func(crm *stubCRM, _ *stubSite, _ *stubLink) {
				crm.searchErr = context.Canceled
			},
		},
		{
			name: "поиск: время ожидания истекло", specKey: "GET /api/v1/crm-clients",
			method: http.MethodGet, path: "/api/v1/crm-clients?q=иван", want: 504,
			setup: func(crm *stubCRM, _ *stubSite, _ *stubLink) {
				crm.searchErr = context.DeadlineExceeded
			},
		},
		{
			name: "поиск: источник недоступен", specKey: "GET /api/v1/crm-clients",
			method: http.MethodGet, path: "/api/v1/crm-clients?q=иван", want: http.StatusInternalServerError,
			setup: func(crm *stubCRM, _ *stubSite, _ *stubLink) {
				crm.searchErr = errors.New("crm недоступен")
			},
		},

		{
			name: "клиент сайта: успех", specKey: "GET /api/v1/site-clients/{id}",
			method: http.MethodGet, path: "/api/v1/site-clients/1", want: http.StatusOK,
		},
		{
			name: "клиент сайта: пустой id", specKey: "GET /api/v1/site-clients/{id}",
			method: http.MethodGet, path: "/api/v1/site-clients/%20", want: http.StatusBadRequest,
		},
		{
			name: "клиент сайта: не найден", specKey: "GET /api/v1/site-clients/{id}",
			method: http.MethodGet, path: "/api/v1/site-clients/1", want: http.StatusNotFound,
			setup: func(_ *stubCRM, site *stubSite, _ *stubLink) {
				site.getErr = client.ErrSiteNotFound
			},
		},
		{
			name: "клиент сайта: запрос отменён", specKey: "GET /api/v1/site-clients/{id}",
			method: http.MethodGet, path: "/api/v1/site-clients/1", want: 499,
			setup: func(_ *stubCRM, site *stubSite, _ *stubLink) {
				site.getErr = context.Canceled
			},
		},
		{
			name: "клиент сайта: время ожидания истекло", specKey: "GET /api/v1/site-clients/{id}",
			method: http.MethodGet, path: "/api/v1/site-clients/1", want: 504,
			setup: func(_ *stubCRM, site *stubSite, _ *stubLink) {
				site.getErr = context.DeadlineExceeded
			},
		},
		{
			name: "клиент сайта: источник недоступен", specKey: "GET /api/v1/site-clients/{id}",
			method: http.MethodGet, path: "/api/v1/site-clients/1", want: http.StatusInternalServerError,
			setup: func(_ *stubCRM, site *stubSite, _ *stubLink) {
				site.getErr = errors.New("сайт недоступен")
			},
		},

		{
			name: "клиент сайта: некорректные данные", specKey: "GET /api/v1/site-clients/{id}",
			method: http.MethodGet, path: "/api/v1/site-clients/1", want: http.StatusBadGateway,
			setup: func(_ *stubCRM, site *stubSite, _ *stubLink) { site.getErr = client.ErrInvalidSourceData },
		},

		{
			name: "связь: успех", specKey: "POST /api/v1/links",
			method: http.MethodPost, path: "/api/v1/links", body: `{"crm_id":"1","site_id":"1"}`,
			want: http.StatusCreated,
		},
		{
			name: "связь: некорректное тело", specKey: "POST /api/v1/links",
			method: http.MethodPost, path: "/api/v1/links", body: `{`, want: http.StatusBadRequest,
		},
		{
			name: "связь: пустой id", specKey: "POST /api/v1/links",
			method: http.MethodPost, path: "/api/v1/links", body: `{"crm_id":" ","site_id":"1"}`,
			want: http.StatusBadRequest,
		},
		{
			name: "связь: клиента CRM нет", specKey: "POST /api/v1/links",
			method: http.MethodPost, path: "/api/v1/links", body: `{"crm_id":"1","site_id":"1"}`,
			want: http.StatusNotFound,
			setup: func(crm *stubCRM, _ *stubSite, _ *stubLink) {
				crm.getErr = client.ErrCRMNotFound
			},
		},
		{
			name: "связь: клиента сайта нет", specKey: "POST /api/v1/links",
			method: http.MethodPost, path: "/api/v1/links", body: `{"crm_id":"1","site_id":"1"}`,
			want: http.StatusNotFound,
			setup: func(_ *stubCRM, site *stubSite, _ *stubLink) {
				site.getErr = client.ErrSiteNotFound
			},
		},
		{
			name: "связь: фамилии не совпадают", specKey: "POST /api/v1/links",
			method: http.MethodPost, path: "/api/v1/links", body: `{"crm_id":"1","site_id":"1"}`,
			want: http.StatusUnprocessableEntity,
			setup: func(_ *stubCRM, site *stubSite, _ *stubLink) {
				site.found = client.SiteClient{
					ID: 1, LastName: "Петров",
					Birthday: &client.Date{Year: 1990, Month: 1, Day: 1},
				}
			},
		},
		{
			name: "связь: пустая фамилия", specKey: "POST /api/v1/links",
			method: http.MethodPost, path: "/api/v1/links", body: `{"crm_id":"1","site_id":"1"}`,
			want: 422,
			setup: func(_ *stubCRM, site *stubSite, _ *stubLink) {
				site.found = client.SiteClient{ID: 1, Birthday: &client.Date{Year: 1990, Month: 1, Day: 1}}
			},
		},
		{
			name: "связь: некорректная дата рождения", specKey: "POST /api/v1/links",
			method: http.MethodPost, path: "/api/v1/links", body: `{"crm_id":"1","site_id":"1"}`,
			want: http.StatusBadGateway,
			setup: func(crm *stubCRM, _ *stubSite, _ *stubLink) {
				crm.found.Birthday = &client.Date{}
			},
		},
		{
			name: "связь: уже связана", specKey: "POST /api/v1/links",
			method: http.MethodPost, path: "/api/v1/links", body: `{"crm_id":"1","site_id":"1"}`,
			want: http.StatusConflict,
			setup: func(_ *stubCRM, _ *stubSite, links *stubLink) {
				links.createErr = link.ErrCRMAlreadyLinked
			},
		},
		{
			name: "связь: запрос отменён", specKey: "POST /api/v1/links",
			method: http.MethodPost, path: "/api/v1/links", body: `{"crm_id":"1","site_id":"1"}`,
			want: 499,
			setup: func(crm *stubCRM, _ *stubSite, _ *stubLink) {
				crm.getErr = context.Canceled
			},
		},
		{
			name: "связь: время ожидания истекло", specKey: "POST /api/v1/links",
			method: http.MethodPost, path: "/api/v1/links", body: `{"crm_id":"1","site_id":"1"}`,
			want: 504,
			setup: func(crm *stubCRM, _ *stubSite, _ *stubLink) {
				crm.getErr = context.DeadlineExceeded
			},
		},
		{
			name: "связь: база недоступна", specKey: "POST /api/v1/links",
			method: http.MethodPost, path: "/api/v1/links", body: `{"crm_id":"1","site_id":"1"}`,
			want: http.StatusInternalServerError,
			setup: func(_ *stubCRM, _ *stubSite, links *stubLink) {
				links.createErr = errors.New("база недоступна")
			},
		},

		{
			name: "список связей: успех", specKey: "GET /api/v1/links",
			method: http.MethodGet, path: "/api/v1/links", want: http.StatusOK,
		},
		{
			name: "список связей: запрос отменён", specKey: "GET /api/v1/links",
			method: http.MethodGet, path: "/api/v1/links", want: 499,
			setup: func(_ *stubCRM, _ *stubSite, links *stubLink) {
				links.listErr = context.Canceled
			},
		},
		{
			name: "список связей: время ожидания истекло", specKey: "GET /api/v1/links",
			method: http.MethodGet, path: "/api/v1/links", want: 504,
			setup: func(_ *stubCRM, _ *stubSite, links *stubLink) {
				links.listErr = context.DeadlineExceeded
			},
		},
		{
			name: "список связей: база недоступна", specKey: "GET /api/v1/links",
			method: http.MethodGet, path: "/api/v1/links", want: http.StatusInternalServerError,
			setup: func(_ *stubCRM, _ *stubSite, links *stubLink) {
				links.listErr = errors.New("база недоступна")
			},
		},

		{
			name: "выгрузка CSV: успех", specKey: "GET /api/v1/links/export.csv",
			method: http.MethodGet, path: "/api/v1/links/export.csv", want: http.StatusOK,
		},
		{
			name: "выгрузка CSV: запрос отменён", specKey: "GET /api/v1/links/export.csv",
			method: http.MethodGet, path: "/api/v1/links/export.csv", want: 499,
			setup: func(_ *stubCRM, _ *stubSite, links *stubLink) {
				links.listErr = context.Canceled
			},
		},
		{
			name: "выгрузка CSV: время ожидания истекло", specKey: "GET /api/v1/links/export.csv",
			method: http.MethodGet, path: "/api/v1/links/export.csv", want: 504,
			setup: func(_ *stubCRM, _ *stubSite, links *stubLink) {
				links.listErr = context.DeadlineExceeded
			},
		},
		{
			name: "выгрузка CSV: база недоступна", specKey: "GET /api/v1/links/export.csv",
			method: http.MethodGet, path: "/api/v1/links/export.csv", want: http.StatusInternalServerError,
			setup: func(_ *stubCRM, _ *stubSite, links *stubLink) {
				links.listErr = errors.New("база недоступна")
			},
		},

		{
			name: "healthz: база доступна", specKey: "GET /healthz",
			method: http.MethodGet, path: "/healthz", want: http.StatusOK,
		},
		{
			name: "healthz: база недоступна", specKey: "GET /healthz",
			method: http.MethodGet, path: "/healthz", want: http.StatusServiceUnavailable,
			health: errors.New("база недоступна"),
		},
	}
}

func TestSpecDocumentsHandlerStatuses(t *testing.T) {
	scenarios := append(parityScenarios(), assessmentParityScenarios()...)

	observed := map[string]map[string]bool{}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			crm := &stubCRM{found: validCRM}
			site := &stubSite{found: validSite}
			links := &stubLink{}
			if sc.setup != nil {
				sc.setup(crm, site, links)
			}

			srv := newParityServer(t, crm, site, links, sc.health)
			rec := do(t, srv, sc.method, sc.path, sc.body)
			if rec.Code != sc.want {
				t.Errorf("статус %d, ожидался %d (тело %s)", rec.Code, sc.want, rec.Body.String())
			}

			if observed[sc.specKey] == nil {
				observed[sc.specKey] = map[string]bool{}
			}
			observed[sc.specKey][strconv.Itoa(sc.want)] = true
		})
	}

	documented := documentedStatuses(t)
	for specKey, statuses := range observed {
		docStatuses, ok := documented[specKey]
		if !ok {
			t.Errorf("в спецификации нет операции %s", specKey)
			continue
		}
		if !sameStatuses(statuses, docStatuses) {
			t.Errorf("операция %s: обработчик возвращает %v, спецификация описывает %v",
				specKey, sortedKeys(statuses), sortedKeys(docStatuses))
		}
	}
}

func TestSpecContentTypesMatchHandlers(t *testing.T) {
	doc := loadOpenAPIDoc(t)

	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			for status, resp := range op.Responses.Map() {
				want := "application/json"
				if path == "/api/v1/links/export.csv" && status == "200" {
					want = "text/csv"
				}
				got := contentTypes(resp.Value)
				if len(got) != 1 || got[0] != want {
					t.Errorf("%s %s %s: типы содержимого %v, ожидался %q", method, path, status, got, want)
				}
				if isErrorStatus(status) && strings.HasPrefix(path, "/api/v1") && schemaRefOf(resp.Value) != errorSchemaRef {
					t.Errorf("%s %s %s: схема %q, ожидалась %q",
						method, path, status, schemaRefOf(resp.Value), errorSchemaRef)
				}
			}
		}
	}
}

func loadOpenAPIDoc(t *testing.T) *openapi3.T {
	t.Helper()

	doc, err := openapi3.NewLoader().LoadFromData(openapi.JSON)
	if err != nil {
		t.Fatalf("разобрать спецификацию: %v", err)
	}
	return doc
}

// documentedStatuses собирает статусы, описанные в спецификации, по ключу
// "METHOD path".
func documentedStatuses(t *testing.T) map[string]map[string]bool {
	t.Helper()

	doc := loadOpenAPIDoc(t)
	out := map[string]map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			key := method + " " + path
			if out[key] == nil {
				out[key] = map[string]bool{}
			}
			for status := range op.Responses.Map() {
				out[key][status] = true
			}
		}
	}
	return out
}

func newParityServer(t *testing.T, crm ports.CRMReader, site *stubSite, links *stubLink, healthErr error) *gin.Engine {
	t.Helper()

	return newParityServerWithClock(t, crm, site, links, healthErr, ports.SystemClock{})
}

func newParityServerWithClock(t *testing.T, crm ports.CRMReader, site *stubSite, links *stubLink, healthErr error, clock ports.Clock) *gin.Engine {
	t.Helper()
	svc := services.NewClientLinkService(crm, site, links, tokenOutbound.NewHMACToken([32]byte{1}, clock), clock)
	srv, err := web.NewRouter(
		handlers.NewClientHandler(services.NewClientSearchService(crm, site)),
		handlers.NewLinkHandler(svc),
		handlers.NewAssessmentHandler(svc),
		handlers.NewHealthHandler(func(context.Context) error { return healthErr }),
	)
	if err != nil {
		t.Fatalf("собрать роутер: %v", err)
	}
	return srv
}

func sameStatuses(left, right map[string]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for key := range left {
		if !right[key] {
			return false
		}
	}
	return true
}

func sortedKeys(statuses map[string]bool) []string {
	out := make([]string, 0, len(statuses))
	for key := range statuses {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func contentTypes(resp *openapi3.Response) []string {
	out := make([]string, 0, len(resp.Content))
	for mediaType := range resp.Content {
		out = append(out, mediaType)
	}
	sort.Strings(out)
	return out
}

func schemaRefOf(resp *openapi3.Response) string {
	for _, mediaType := range resp.Content {
		if mediaType.Schema == nil {
			return ""
		}
		return mediaType.Schema.Ref
	}
	return ""
}

func isErrorStatus(status string) bool {
	return strings.HasPrefix(status, "4") || strings.HasPrefix(status, "5")
}

func (s *stubLink) LookupBySourceIDs(context.Context, string, int64) (bool, bool, error) {
	return s.crmBusy, s.siteBusy, nil
}

func TestNullableResponseSchemas(t *testing.T) {
	doc := loadOpenAPIDoc(t)
	for schema, fields := range map[string][]string{
		"handlers.CRMClientResponse":       {"birthday"},
		"handlers.SiteClientResponse":      {"birthday"},
		"handlers.AssessmentResponse":      {"token", "expires_at"},
		"handlers.AssessmentCheckResponse": {"crm_value", "site_value"},
	} {
		for _, field := range fields {
			if !doc.Components.Schemas[schema].Value.Properties[field].Value.Type.IncludesNull() {
				t.Errorf("%s.%s must allow null", schema, field)
			}
		}
	}
}
