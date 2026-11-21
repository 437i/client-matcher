package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/437i/client-matcher/backend/internal/adapters/inbound/http/handlers"
	"github.com/437i/client-matcher/backend/internal/domain/client"
)

const assessmentPath = "/api/v1/link-assessments"
const linksPath = "/api/v1/links"
const pairJSON = "{\"crm_id\":\"1\",\"site_id\":\"1\"}"

type assessmentClock struct{ now time.Time }

func (c *assessmentClock) Now() time.Time { return c.now }

func TestLinkAssessmentHTTP(t *testing.T) {
	crm := &stubCRM{found: validCRM}
	site := &stubSite{found: validSite}
	links := &stubLink{}
	clock := &assessmentClock{now: time.Unix(2000, 0).UTC()}
	srv := newParityServerWithClock(t, crm, site, links, nil, clock)
	assess := func() handlers.AssessmentResponse {
		t.Helper()
		rec := do(t, srv, http.MethodPost, assessmentPath, pairJSON)
		var a handlers.AssessmentResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil || rec.Code != 200 {
			t.Fatalf("assessment %d: %s (%v)", rec.Code, rec.Body, err)
		}
		return a
	}
	ready := assess()
	if ready.Status != "ready" || ready.ConfirmationRequired || ready.Token == nil || ready.ExpiresAt == nil || len(ready.Checks) != 3 || ready.BlockedReasons == nil {
		t.Fatalf("ready: %+v", ready)
	}
	site.found.ShortName = "Ваня"
	site.found.Birthday = nil
	a := assess()
	if a.Status != "requires_confirmation" || !a.ConfirmationRequired || len(a.Checks) != 3 || a.Checks[1].Code != "alias_match" || a.Checks[2].SiteValue != nil || a.Token == nil {
		t.Fatalf("assessment: %+v", a)
	}
	rec := do(t, srv, http.MethodGet, "/api/v1/site-clients/1", "")
	if !strings.Contains(rec.Body.String(), "\"birthday\":null") {
		t.Fatalf("birthday: %s", rec.Body)
	}
	create := func(token *string, confirmed bool, reason string, status int, code string) {
		t.Helper()
		body, err := json.Marshal(handlers.CreateRequest{CRMID: "1", SiteID: "1", AssessmentToken: token, Confirmed: &confirmed, Reason: reason})
		if err != nil {
			t.Fatal(err)
		}
		rec := do(t, srv, http.MethodPost, linksPath, string(body))
		if rec.Code != status || code != "" && errCode(t, rec) != code {
			t.Fatalf("want %d %s, got %d %s", status, code, rec.Code, rec.Body)
		}
	}
	create(nil, true, "Проверено", 422, "confirmation_required")
	create(a.Token, true, "  ", 400, "invalid_confirmation")
	create(a.Token, true, strings.Repeat("я", 1001), 400, "invalid_confirmation")
	create(a.Token, true, "Проверено", 201, "")
	site.found = validSite
	create(a.Token, true, "Проверено", 409, "assessment_stale")
	create(ready.Token, true, "Проверено", 400, "invalid_confirmation")
	clock.now = clock.now.Add(5 * time.Minute)
	create(ready.Token, false, "", 409, "assessment_expired")
	crm.getErr = client.ErrCRMNotFound
	bad := "invalid-token"
	create(&bad, false, "", 400, "invalid_assessment_token")
	site.getErr = client.ErrSiteNotFound
	links.crmBusy = true
	blocked := assess()
	if blocked.Status != "blocked" || blocked.Checks == nil || len(blocked.Checks) != 0 || blocked.Token != nil || blocked.ExpiresAt != nil || blocked.ConfirmationRequired || strings.Join(blocked.BlockedReasons, ",") != "crm_not_found,site_not_found" {
		t.Fatalf("blocked: %+v", blocked)
	}
	crm.getErr = nil
	site.getErr = nil
	links.siteBusy = true
	blocked = assess()
	if strings.Join(blocked.BlockedReasons, ",") != "crm_already_linked,site_already_linked" {
		t.Fatalf("occupied: %+v", blocked)
	}
	for _, path := range []string{assessmentPath, linksPath} {
		for _, size := range []int{16 * 1024, 16*1024 + 1} {
			want := 413
			if size == 16*1024 {
				want = 409
				if path == assessmentPath {
					want = 200
				}
			}
			rec := do(t, srv, http.MethodPost, path, pairJSON+strings.Repeat(" ", size-len(pairJSON)))
			if rec.Code != want {
				t.Fatalf("%s %d bytes: %d %s", path, size, rec.Code, rec.Body)
			}
		}
	}
}

func TestLinkWorkflowSafeLogs(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	crm := &stubCRM{found: validCRM}
	site := &stubSite{found: validSite}
	links := &stubLink{}
	site.found.ShortName = "Ваня"
	srv := newParityServer(t, crm, site, links, nil)
	rec := do(t, srv, http.MethodPost, assessmentPath, pairJSON)
	var assessment handlers.AssessmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &assessment); err != nil || assessment.Token == nil {
		t.Fatalf("assessment: %s", rec.Body)
	}
	const reason = "SYNTHETIC_REASON"
	links.createErr = errors.New(reason + " " + *assessment.Token + " " + crm.found.FirstName + " " + crm.found.Birthday.String())
	yes := true
	body, err := json.Marshal(handlers.CreateRequest{CRMID: "1", SiteID: "1", AssessmentToken: assessment.Token, Confirmed: &yes, Reason: reason})
	if err != nil {
		t.Fatal(err)
	}
	rec = do(t, srv, http.MethodPost, linksPath, string(body))
	if rec.Code != 500 || !strings.Contains(logs.String(), "internal_error") {
		t.Fatalf("response %d, log missing error code", rec.Code)
	}
	for _, s := range []string{reason, *assessment.Token, crm.found.FirstName, crm.found.Birthday.String()} {
		if strings.Contains(logs.String(), s) || strings.Contains(rec.Body.String(), s) {
			t.Fatal("sensitive workflow data in logs or error response")
		}
	}
}

func assessmentParityScenarios() []parityScenario {
	const key = "POST " + assessmentPath
	var out []parityScenario
	for _, tc := range []struct {
		status int
		err    error
		body   string
	}{
		{200, nil, pairJSON}, {400, nil, "{}"}, {413, nil, strings.Repeat(" ", 16*1024+1)},
		{499, context.Canceled, pairJSON}, {500, errors.New("source unavailable"), pairJSON},
		{502, client.ErrInvalidSourceData, pairJSON}, {504, context.DeadlineExceeded, pairJSON},
	} {
		out = append(out, parityScenario{name: "оценка " + http.StatusText(tc.status), specKey: key, method: http.MethodPost, path: assessmentPath, body: tc.body, want: tc.status, setup: func(c *stubCRM, _ *stubSite, _ *stubLink) { c.getErr = tc.err }})
	}
	return append(out, parityScenario{name: "связь: слишком большой запрос", specKey: "POST " + linksPath, method: http.MethodPost, path: linksPath, body: strings.Repeat(" ", 16*1024+1), want: 413})
}
