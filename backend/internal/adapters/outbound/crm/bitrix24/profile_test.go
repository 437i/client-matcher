package bitrix24

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/437i/client-matcher/backend/internal/domain/client"
	b24 "github.com/bitrix24/b24gosdk"
	"github.com/bitrix24/b24gosdk/b24test"
)

func TestSyntheticSearchProfile(t *testing.T) {
	const population = 25121
	rows := make([]map[string]any, 0, population)
	for i := 1; i <= population; i++ {
		rows = append(rows, map[string]any{"id": i, titleField: "Смирнов Борис", lastField: "Смирнов", firstField: "Борис", patronymicField: "Олегович"})
	}
	for i := range 575 {
		rows[i][nameFields[i%len(nameFields)]] = "Иванов"
	}
	for i := 575; i < 625; i++ {
		rows[i][nameFields[i%len(nameFields)]] = "Иванова"
	}
	for i, name := range []string{"Малинов", "Голубева", "Корнеев", "Березина"} {
		rows[population-1-i][titleField] = name
	}
	var calls, matched atomic.Int32
	var serverTime atomic.Int64
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		defer func() { serverTime.Add(int64(time.Since(started))) }()
		calls.Add(1)
		var req struct {
			Start  int
			Filter map[string]map[string]json.RawMessage
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		var conditions []predicate
		for key, raw := range req.Filter["0"] {
			if key == "logic" {
				continue
			}
			var term map[string]string
			if err := json.Unmarshal(raw, &term); err != nil {
				http.Error(w, "invalid filter", 400)
				return
			}
			for field, word := range term {
				conditions = append(conditions, predicate{strings.TrimPrefix(field, "%"), word})
			}
		}
		matches := make([]map[string]any, 0)
		for _, row := range rows {
			for _, term := range conditions {
				if strings.Contains(strings.ToLower(row[term.field].(string)), term.value) {
					matches = append(matches, row)
					break
				}
			}
		}
		matched.Store(int32(len(matches)))
		end := min(req.Start+b24.DefaultPageSize, len(matches))
		var next *int
		if end < len(matches) {
			next = &end
		}
		_, _ = w.Write(listPage(next, matches[req.Start:end]...))
	}))
	defer portal.Close()
	repo := NewWithClient(b24.NewClient(portal.URL+"/rest/1/synthetic/", b24.WithRetry(b24.RetryConfig{MaxAttempts: 1})))
	defaultBudget := repo.policy.maxPages
	records := make(map[string]client.CRMClient)
	remaining := defaultBudget
	if err := repo.collect(t.Context(), predicates(client.TokenizeSourceQuery("Иванов")), records, &remaining); err != nil {
		t.Fatal(err)
	}
	if len(records) != 625 || calls.Load() != 13 || remaining != 7 {
		t.Fatalf("collected %d unique records in %d calls, %d pages remaining", len(records), calls.Load(), remaining)
	}
	var exactIDs []string
	for i := range 625 {
		id := fmt.Sprint(i + 1)
		if records[id].ID != id {
			t.Fatalf("missing record %s", id)
		}
		if rows[i][titleField] == "Иванов" {
			exactIDs = append(exactIDs, id)
		}
	}
	slices.Sort(exactIDs)
	wantTop := exactIDs[:50]

	profile := func(query string, want error, width, pageCalls int) {
		t.Helper()
		var samples []time.Duration
		calls.Store(0)
		serverTime.Store(0)
		for range 20 {
			start := time.Now()
			got, err := repo.Search(t.Context(), query, 50)
			samples = append(samples, time.Since(start))
			if !errors.Is(err, want) || want != nil && got != nil || want == nil && len(got) != 50 {
				t.Fatalf("profile %q: %d records, %v", query, len(got), err)
			}
			for i, record := range got {
				if record.ID != wantTop[i] {
					t.Fatalf("profile %q: rank %d has ID %s, want %s", query, i, record.ID, wantTop[i])
				}
			}
		}
		if matched.Load() != int32(width) || calls.Load() != int32(pageCalls*len(samples)) {
			t.Fatalf("width=%d calls=%d", matched.Load(), calls.Load())
		}
		slices.Sort(samples)
		t.Logf("%s: OR width=%d, pages/attempts=%d, p50=%v p95=%v, mean fake server=%v", query, width, pageCalls, samples[9], samples[18], time.Duration(serverTime.Load())/time.Duration(len(samples)))
	}
	repo.policy.maxPages = 12
	profile("Иванов", errPages, 625, 12)
	repo.policy.maxPages = defaultBudget
	profile("Иванов", nil, 625, 13)
	queries := []struct{ value, id string }{{"Малиновв", fmt.Sprint(population)}, {"Галубева", fmt.Sprint(population - 1)}, {"Карнеев", fmt.Sprint(population - 2)}, {"Берзеина", fmt.Sprint(population - 3)}}
	for _, expand := range []bool{false, true} {
		repo.policy.expandTypos = expand
		calls.Store(0)
		var samples []time.Duration
		recovered := 0
		for range 20 {
			for _, query := range queries {
				start := time.Now()
				found, err := repo.Search(t.Context(), query.value, 50)
				samples = append(samples, time.Since(start))
				if err != nil {
					t.Fatal(err)
				}
				if slices.ContainsFunc(found, func(r client.CRMClient) bool { return r.ID == query.id }) {
					recovered++
				}
			}
		}
		slices.Sort(samples)
		wantCalls, wantRecovered := 80, 0
		if expand {
			wantCalls, wantRecovered = 160, 60
		}
		if calls.Load() != int32(wantCalls) || recovered != wantRecovered {
			t.Fatalf("expand=%t: %d calls, %d recovered", expand, calls.Load(), recovered)
		}
		t.Logf("typo expansion=%t: recovered=%d/%d, pages/attempts=%d, p50=%v p95=%v", expand, recovered, len(samples), calls.Load(), samples[39], samples[75])
	}
}

func TestExpansionSharedBudget(t *testing.T) {
	row := map[string]any{"id": 1, "title": "Малинов"}
	for _, tc := range []struct {
		name          string
		replies       [][]byte
		budget, calls int
		want          error
	}{
		{"deduplicate across filters", [][]byte{listPage(nil, map[string]any{"id": 1, "title": "Другой"}), listPage(nil, row, map[string]any{"id": 2, "title": "Малинов"})}, 2, 2, nil},
		{"exhaustion", [][]byte{listPage(nil), listPage(b24test.Ptr(50), row)}, 2, 2, errPages},
		{"error", [][]byte{listPage(nil), b24test.ErrorBody("ACCESS_DENIED", "private")}, 2, 2, errAccess},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := b24test.NewPortal(t).On(itemList, tc.replies...)
			repo := fakeRepo(p)
			repo.policy.expandTypos = true
			repo.policy.maxPages = tc.budget
			got, err := repo.Search(t.Context(), "Малиновв", 50)
			if !errors.Is(err, tc.want) || len(p.Calls()) != tc.calls || tc.want != nil && got != nil || tc.want == nil && (len(got) != 1 || got[0].ID != "2") {
				t.Fatalf("expansion: %v, %v, %d calls", got, err, len(p.Calls()))
			}
		})
	}
}

func TestExpansionDefaultSharedPageBudget(t *testing.T) {
	for _, tc := range []struct {
		name string
		next *int
		want error
	}{
		{"last page on shared budget", nil, nil},
		{"continuation on shared budget", b24test.Ptr(b24.DefaultPageSize), errPages},
	} {
		t.Run(tc.name, func(t *testing.T) {
			replies := make([][]byte, 20)
			for i := range 18 {
				replies[i] = listPage(b24test.Ptr((i + 1) * b24.DefaultPageSize))
			}
			replies[18] = listPage(nil)
			replies[19] = listPage(tc.next, map[string]any{"id": 1, "title": "Малинов"})
			p := b24test.NewPortal(t).On(itemList, replies...)
			repo := fakeRepo(p)
			repo.policy.expandTypos = true
			got, err := repo.Search(t.Context(), "Малиновв", 50)
			calls := p.CallsTo(itemList)
			if !errors.Is(err, tc.want) || (tc.want != nil && got != nil) || (tc.want == nil && (len(got) != 1 || got[0].ID != "1")) || len(calls) != 20 {
				t.Fatalf("expansion: %v, %v, %d calls", got, err, len(calls))
			}
			if calls[18].Params["start"] != float64(18*b24.DefaultPageSize) || calls[19].Params["start"] != nil {
				t.Fatal("expected 19 base pages followed by the first expansion page")
			}
		})
	}
}

func BenchmarkSyntheticLocalRanking(b *testing.B) {
	rows := make([]json.RawMessage, 625)
	for i := range rows {
		rows[i] = json.RawMessage(fmt.Sprintf(`{"id":%d,"title":"Иванов Иван Петрович","ufCrm7_1656048422":"2006-04-16T03:00:00+04:00"}`, i+1))
	}
	query := client.TokenizeSearchQuery("Иванов")
	b.ResetTimer()
	for range b.N {
		records := make(map[string]client.CRMClient, len(rows))
		for _, raw := range rows {
			record, err := decodeItem(raw)
			if err != nil {
				b.Fatal(err)
			}
			records[record.ID] = record
		}
		ranked, err := rank(b.Context(), query, records)
		if err != nil || len(ranked) != len(rows) {
			b.Fatalf("ranking: %d %v", len(ranked), err)
		}
	}
}
