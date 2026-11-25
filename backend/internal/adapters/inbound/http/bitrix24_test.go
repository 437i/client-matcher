package http_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/437i/client-matcher/backend/internal/adapters/inbound/http/handlers"
	"github.com/437i/client-matcher/backend/internal/adapters/outbound/crm/bitrix24"
	b24 "github.com/bitrix24/b24gosdk"
	"github.com/bitrix24/b24gosdk/b24test"
)

func TestBitrixWorkflow(t *testing.T) {
	const get = "crm.item.get"
	const list = "crm.item.list"
	const readyTitle = "Иванов Иван"
	const extraTitle = "Иванов Иван Лишнее"
	row := func(title string) map[string]any {
		return map[string]any{"id": 1, "title": title, "ufCrm7_1656048402": "Пётр", "ufCrm7_1656048422": "1990-01-01T00:00:00+04:00"}
	}
	card := func(title string) []byte { return b24test.Result(map[string]any{"item": row(title)}) }
	portal := b24test.NewPortal(t).On(list, b24test.WrappedListResult("items", []any{row(readyTitle)}, nil, nil))
	crm := bitrix24.NewWithClient(portal.Client(b24.WithRetry(b24.RetryConfig{MaxAttempts: 1})))
	links := &stubLink{}
	srv := newParityServer(t, crm, &stubSite{found: validSite}, links, nil)
	search := do(t, srv, http.MethodGet, "/api/v1/crm-clients?q=Иванов", "")
	var found []handlers.CRMClientResponse
	if search.Code != 200 || json.Unmarshal(search.Body.Bytes(), &found) != nil || len(found) != 1 || found[0].FullName != readyTitle || found[0].FirstName != "Пётр" {
		t.Fatalf("search: %d %s", search.Code, search.Body)
	}
	assess := func() handlers.AssessmentResponse {
		t.Helper()
		rec := do(t, srv, http.MethodPost, assessmentPath, pairJSON)
		var a handlers.AssessmentResponse
		if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &a) != nil || a.Token == nil {
			t.Fatalf("assessment: %d %s", rec.Code, rec.Body)
		}
		return a
	}
	create := func(token *string, confirmed bool, want int, code string) {
		t.Helper()
		req := handlers.CreateRequest{CRMID: "1", SiteID: "1", AssessmentToken: token}
		if confirmed {
			req.Confirmed = &confirmed
			req.Reason = "Проверено"
		}
		raw, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		rec := do(t, srv, http.MethodPost, linksPath, string(raw))
		if rec.Code != want || code != "" && errCode(t, rec) != code {
			t.Fatalf("create: %d %s", rec.Code, rec.Body)
		}
	}
	for _, path := range []string{assessmentPath, linksPath} {
		rec := do(t, srv, http.MethodPost, path, strings.Replace(pairJSON, `"1"`, `"001"`, 1))
		if rec.Code != 400 || errCode(t, rec) != "invalid_id" {
			t.Fatalf("invalid id: %d %s", rec.Code, rec.Body)
		}
	}
	if len(portal.CallsTo(get)) != 0 || len(links.decisions) != 0 {
		t.Fatal("invalid ID reached a source")
	}
	portal.On(get, card(readyTitle), card(" "+readyTitle), card(readyTitle), card(extraTitle), card(extraTitle))
	ready := assess()
	if ready.Status != "ready" {
		t.Fatalf("ready: %+v", ready)
	}
	create(ready.Token, false, 409, "assessment_stale")
	if len(links.decisions) != 0 {
		t.Fatal("stale create wrote a link")
	}
	create(nil, false, 201, "")
	extra := assess()
	if extra.Status != "requires_confirmation" || len(extra.Checks) != 3 {
		t.Fatalf("confirmation: %+v", extra)
	}
	last := extra.Checks[0]
	if last.Field != "last_name" || last.Code != "mismatch" || last.CRMValue == nil || *last.CRMValue != extraTitle || last.SiteValue == nil || *last.SiteValue != "Иван Иванов" {
		t.Fatalf("compatible check: %+v", last)
	}
	create(extra.Token, true, 201, "")
	if len(links.decisions) != 2 || links.decisions[0].Confirmation != nil || links.decisions[1].Confirmation == nil || !slices.Equal(links.decisions[1].Confirmation.Codes, []string{"name:mismatch"}) {
		t.Fatalf("audit: %+v", links.decisions)
	}
	if len(portal.CallsTo(get)) != 5 {
		t.Fatal("each assess and create must read a fresh card")
	}

	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	sensitive := portal.URL() + " PRIVATE_SYNTHETIC_RESPONSE Иван Иванов 1990-01-01"
	for _, code := range []string{"NOT_FOUND", "ACCESS_DENIED"} {
		portal.OnError(get, code, sensitive)
		create(extra.Token, true, 500, "internal_error")
		rec := do(t, srv, http.MethodPost, assessmentPath, pairJSON)
		if rec.Code != 500 || errCode(t, rec) != "internal_error" || strings.Contains(rec.Body.String(), "token") {
			t.Fatalf("technical assessment: %d %s", rec.Code, rec.Body)
		}
		for _, raw := range []string{rec.Body.String(), logs.String()} {
			for _, secret := range []string{portal.URL(), "testwebhooksecret", "PRIVATE_SYNTHETIC_RESPONSE", "Иван Иванов", "1990-01-01"} {
				if strings.Contains(raw, secret) {
					t.Fatal("unsafe HTTP error or log")
				}
			}
		}
	}
	if len(portal.CallsTo(get)) != 9 || len(links.decisions) != 2 {
		t.Fatal("failed read must be fresh and must not write a link")
	}
}
