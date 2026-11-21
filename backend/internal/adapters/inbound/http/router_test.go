package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	tokenOutbound "github.com/437i/client-matcher/backend/internal/adapters/outbound"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	web "github.com/437i/client-matcher/backend/internal/adapters/inbound/http"
	"github.com/437i/client-matcher/backend/internal/adapters/inbound/http/handlers"
	crmCSV "github.com/437i/client-matcher/backend/internal/adapters/outbound/crm/csv"
	siteCSV "github.com/437i/client-matcher/backend/internal/adapters/outbound/site/csv"
	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/application/services"
	"github.com/437i/client-matcher/backend/internal/domain/link"
)

const crmData = `id,title,birthday,clinic
1352,Иванов Иван Бек кызы,1982-04-15,Наименование Организации
1353,Иванова Мария,1990-01-02,"ООО ""Рога, и копыта"""
1354,Королёв Пётр Сергеевич,2001-09-09,НИИ им. Денисова
`

const siteData = `id,title,birthday
3586,Иван Иванов,1982-04-15
3587,Маша Иванова,1990-01-02
3588,Пётр Королев,2001-09-09
3589,Ваня Петров,1982-04-15
3590,Ваня Иванов,1983-01-01
`

// memLinkRepo — in-memory реализация ports.ClientLinkRepo (вместо PostgreSQL).
type memLinkRepo struct {
	mu        sync.Mutex
	links     []link.ClientLink
	createErr error
	listErr   error
}

var _ ports.ClientLinkRepo = (*memLinkRepo)(nil)

func (m *memLinkRepo) Create(_ context.Context, data link.Decision) (link.ClientLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return link.ClientLink{}, m.createErr
	}
	result := link.ClientLink{ID: uuid.New(), CRMID: data.CRMID, SiteID: data.SiteID, CreatedAt: time.Now().UTC()}
	m.links = append(m.links, result)
	return result, nil
}

func (m *memLinkRepo) List(context.Context) ([]link.ClientLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listErr != nil {
		return nil, m.listErr
	}
	return append([]link.ClientLink{}, m.links...), nil
}

func (m *memLinkRepo) GetByID(context.Context, uuid.UUID) (link.ClientLink, error) {
	return link.ClientLink{}, link.ErrNotFound
}
func (m *memLinkRepo) GetByCRMID(context.Context, string) (link.ClientLink, error) {
	return link.ClientLink{}, link.ErrNotFound
}
func (m *memLinkRepo) GetBySiteID(context.Context, string) (link.ClientLink, error) {
	return link.ClientLink{}, link.ErrNotFound
}
func (m *memLinkRepo) Delete(context.Context, uuid.UUID) error { return link.ErrNotFound }

func newTestServer(t *testing.T, linkRepo *memLinkRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	crmPath, sitePath := filepath.Join(dir, "crm.csv"), filepath.Join(dir, "site.csv")
	if err := os.WriteFile(crmPath, []byte(crmData), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sitePath, []byte(siteData), 0o600); err != nil {
		t.Fatal(err)
	}

	crmRepo, err := crmCSV.NewCSVCRMRepo(crmPath)
	if err != nil {
		t.Fatal(err)
	}
	siteRepo, err := siteCSV.NewCSVSiteRepo(sitePath)
	if err != nil {
		t.Fatal(err)
	}

	clock := ports.SystemClock{}
	svc := services.NewClientLinkService(crmRepo, siteRepo, linkRepo, tokenOutbound.NewHMACToken([32]byte{1}, clock), clock)
	router, err := web.NewRouter(
		handlers.NewClientHandler(services.NewClientSearchService(crmRepo, siteRepo)),
		handlers.NewLinkHandler(svc),
		handlers.NewAssessmentHandler(svc),
		handlers.NewHealthHandler(func(context.Context) error { return nil }),
	)
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func do(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var resp handlers.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error response %q: %v", rec.Body.String(), err)
	}
	return resp.Code
}

func TestSearch(t *testing.T) {
	srv := newTestServer(t, &memLinkRepo{})

	rec := do(t, srv, http.MethodGet, "/api/v1/crm-clients?q=%D0%B8%D0%B2%D0%B0%D0%BD%D0%BE%D0%B2%20%D0%B8%D0%B2%D0%B0%D0%BD", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var list []handlers.CRMClientResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "1352" || list[0].FullName != "Иванов Иван Бек кызы" || list[0].Birthday == nil || *list[0].Birthday != "1982-04-15" {
		t.Errorf("unexpected result: %+v", list)
	}

	// пустой результат — [] а не null
	rec = do(t, srv, http.MethodGet, "/api/v1/crm-clients?q=zzz", "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("want 200 [], got %d %q", rec.Code, rec.Body)
	}

	if rec = do(t, srv, http.MethodGet, "/api/v1/crm-clients", ""); rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_query" {
		t.Errorf("missing q: got %d %s", rec.Code, rec.Body)
	}
	if rec = do(t, srv, http.MethodGet, "/api/v1/crm-clients?q=%20%20", ""); rec.Code != http.StatusBadRequest || errCode(t, rec) != "query_empty" {
		t.Errorf("blank q: got %d %s", rec.Code, rec.Body)
	}
}

func TestAccessLogDoesNotContainSearchQuery(t *testing.T) {
	previousWriter := gin.DefaultWriter
	var logs bytes.Buffer
	gin.DefaultWriter = &logs
	t.Cleanup(func() { gin.DefaultWriter = previousWriter })

	srv := newTestServer(t, &memLinkRepo{})
	marker := "ПерсональныеФИОДляПроверки"
	rec := do(t, srv, http.MethodGet, "/api/v1/crm-clients?q="+marker, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(logs.String(), marker) {
		t.Fatalf("access log contains the search query: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "/api/v1/crm-clients") {
		t.Fatalf("access log does not contain the route: %s", logs.String())
	}
}

func TestGetSiteClient(t *testing.T) {
	srv := newTestServer(t, &memLinkRepo{})

	rec := do(t, srv, http.MethodGet, "/api/v1/site-clients/3586", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var cl handlers.SiteClientResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &cl)
	if cl.ID != "3586" || cl.ShortName != "Иван" || cl.LastName != "Иванов" || cl.Birthday == nil || *cl.Birthday != "1982-04-15" {
		t.Errorf("unexpected: %+v", cl)
	}

	if rec = do(t, srv, http.MethodGet, "/api/v1/site-clients/9999", ""); rec.Code != http.StatusNotFound || errCode(t, rec) != "site_client_not_found" {
		t.Errorf("got %d %s", rec.Code, rec.Body)
	}
}

func TestCreateLinkErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		createErr  error
		wantStatus int
		wantCode   string
	}{
		{"crm not found", `{"crm_id":"0","site_id":"3586"}`, nil, 404, "crm_client_not_found"},
		{"site not found", `{"crm_id":"1352","site_id":"0"}`, nil, 404, "site_client_not_found"},
		{"last name mismatch", `{"crm_id":"1352","site_id":"3589"}`, nil, 422, "confirmation_required"},
		{"birthday mismatch", `{"crm_id":"1352","site_id":"3590"}`, nil, 422, "confirmation_required"},
		{"crm already linked", `{"crm_id":"1352","site_id":"3586"}`, link.ErrCRMAlreadyLinked, 409, "crm_already_linked"},
		{"site already linked", `{"crm_id":"1352","site_id":"3586"}`, link.ErrSiteAlreadyLinked, 409, "site_already_linked"},
		{"internal error is not leaked", `{"crm_id":"1352","site_id":"3586"}`, errors.New("pq: password=secret"), 500, "internal_error"},
		{"missing field", `{"crm_id":"1352"}`, nil, 400, "invalid_id"},
		{"blank id", `{"crm_id":"  ","site_id":"3586"}`, nil, 400, "invalid_id"},
		{"broken json", `{`, nil, 400, "invalid_body"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(t, &memLinkRepo{createErr: tc.createErr})
			rec := do(t, srv, http.MethodPost, "/api/v1/links", tc.body)
			if rec.Code != tc.wantStatus || errCode(t, rec) != tc.wantCode {
				t.Errorf("got %d %s, want %d %s", rec.Code, rec.Body, tc.wantStatus, tc.wantCode)
			}
			if strings.Contains(rec.Body.String(), "secret") {
				t.Error("internal error details leaked to the client")
			}
		})
	}
}

func TestCreateListAndExport(t *testing.T) {
	srv := newTestServer(t, &memLinkRepo{})

	// ё/е: «Королёв» в CRM и «Королев» на сайте — это один человек; ID с пробелами обрезаются
	for _, body := range []string{
		`{"crm_id":"1352","site_id":"3586"}`,
		`{"crm_id":" 1354 ","site_id":" 3588 "}`,
	} {
		rec := do(t, srv, http.MethodPost, "/api/v1/links", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s: %d %s", body, rec.Code, rec.Body)
		}
	}

	rec := do(t, srv, http.MethodGet, "/api/v1/links", "")
	var links []handlers.LinkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &links); err != nil || len(links) != 2 {
		t.Fatalf("list: %v %s", err, rec.Body)
	}
	if links[1].CRMID != "1354" || links[1].SiteID != "3588" {
		t.Errorf("IDs must be trimmed: %+v", links[1])
	}

	rec = do(t, srv, http.MethodGet, "/api/v1/links/export.csv", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("export status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, `attachment; filename="client_links_`) {
		t.Errorf("Content-Disposition = %q", cd)
	}
	want := "crm_id,site_id\n1352,3586\n1354,3588\n"
	if rec.Body.String() != want {
		t.Errorf("export body = %q, want %q", rec.Body.String(), want)
	}
}

func TestExportEmpty(t *testing.T) {
	rec := do(t, newTestServer(t, &memLinkRepo{}), http.MethodGet, "/api/v1/links/export.csv", "")
	if rec.Code != http.StatusOK || rec.Body.String() != "crm_id,site_id\n" {
		t.Errorf("got %d %q", rec.Code, rec.Body)
	}
}

func TestExportErrorIsJSON(t *testing.T) {
	repo := &memLinkRepo{listErr: errors.New("pq: password=secret")}
	rec := do(t, newTestServer(t, repo), http.MethodGet, "/api/v1/links/export.csv", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if code := errCode(t, rec); code != "internal_error" {
		t.Errorf("code = %q, want internal_error", code)
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Error("internal error details leaked to the client")
	}
}

func TestHealth(t *testing.T) {
	rec := do(t, newTestServer(t, &memLinkRepo{}), http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK {
		t.Errorf("got %d", rec.Code)
	}
}

func (m *memLinkRepo) LookupBySourceIDs(_ context.Context, crmID, siteID string) (bool, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var crm, site bool
	for _, record := range m.links {
		crm = crm || record.CRMID == crmID
		site = site || record.SiteID == siteID
	}
	return crm, site, nil
}
