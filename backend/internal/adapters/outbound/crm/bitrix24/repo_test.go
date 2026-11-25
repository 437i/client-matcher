package bitrix24

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/437i/client-matcher/backend/internal/domain/client"
	b24 "github.com/bitrix24/b24gosdk"
	"github.com/bitrix24/b24gosdk/b24test"
)

func fakeRepo(p *b24test.Portal) *Repo {
	return NewWithClient(p.Client(b24.WithRetry(b24.RetryConfig{MaxAttempts: 1})))
}

func TestDecodeItem(t *testing.T) {
	for _, id := range []string{"42", `"42"`, `"0042"`} {
		row := fmt.Sprintf(`{"id":%s,"title":" Иванов Иван ","ufCrm7_1657182997586":"Петров","ufCrm7_1656048402":"Иван","ufCrm7_1656048409":"Петрович","clinic":"ignored"}`, id)
		got, err := decodeItem(json.RawMessage(row))
		if err != nil || got.ID != "42" || got.SourceFullName != " Иванов Иван " || got.LastName != "Петров" || got.FirstName != "Иван" || got.Patronymic != "Петрович" || got.Clinic != "" {
			t.Fatalf("mapped card: %+v, %v", got, err)
		}
	}
	for _, raw := range []string{`{}`, `{"id":null}`, `{"id":0}`, `{"id":-1}`, `{"id":9223372036854775808}`, `{"id":"wrong"}`, `{"id":""}`} {
		if _, err := decodeItem(json.RawMessage(raw)); !errors.Is(err, client.ErrInvalidSourceData) {
			t.Fatalf("bad ID: %s: %v", raw, err)
		}
	}
	for _, tc := range []struct{ raw, want string }{
		{`"2006-04-16"`, "2006-04-16"}, {`"2006-04-16T03:00:00+04:00"`, "2006-04-16"}, {`"2006-04-16T23:00:00-04:00"`, "2006-04-16"},
		{"null", ""}, {`""`, ""}, {"42", "invalid"}, {`"2023-02-29"`, "invalid"}, {`"2006-04-16T03:00:00+04:00junk"`, "invalid"},
		{`"2006-04-16T03:00:00+24:00"`, "invalid"}, {`"2006-04-16T03:00:00-04:60"`, "invalid"},
	} {
		got, err := decodeItem(json.RawMessage(fmt.Sprintf(`{"id":42,"%s":%s}`, birthdayField, tc.raw)))
		if tc.want == "invalid" {
			if !errors.Is(err, client.ErrInvalidSourceData) {
				t.Fatalf("invalid birthday accepted: %s", tc.raw)
			}
			continue
		}
		date := ""
		if got.Birthday != nil {
			date = got.Birthday.String()
		}
		if err != nil || date != tc.want || got.SourceFullName != "" {
			t.Fatalf("birthday %s: %v, %v", tc.raw, date, err)
		}
	}
}

func cardReply(id any, title string) []byte {
	return b24test.Result(map[string]any{"item": map[string]any{"id": id, "title": title}})
}

func TestGetFreshAndCanonical(t *testing.T) {
	p := b24test.NewPortal(t).On(itemGet, cardReply(42, "Первое"), cardReply("0042", "Второе"), cardReply(43, "Другая"), b24test.Result(map[string]any{}))
	r := fakeRepo(p)
	for _, bad := range []string{"", "0", "-1", "00123", "+123", " 42", "42 ", "abc", "9223372036854775808"} {
		if _, err := r.GetByID(t.Context(), bad); !errors.Is(err, client.ErrInvalidCRMID) {
			t.Fatalf("ID %q: %v", bad, err)
		}
	}
	if len(p.Calls()) != 0 {
		t.Fatal("invalid IDs caused I/O")
	}
	for _, title := range []string{"Первое", "Второе"} {
		got, err := r.GetByID(t.Context(), "42")
		if err != nil || got.SourceFullName != title {
			t.Fatalf("fresh get: %+v, %v", got, err)
		}
	}
	for range 2 {
		if _, err := r.GetByID(t.Context(), "42"); !errors.Is(err, client.ErrInvalidSourceData) {
			t.Fatalf("invalid get response: %v", err)
		}
	}
	for _, call := range p.CallsTo(itemGet) {
		if !reflect.DeepEqual(call.Params, map[string]any{"id": float64(42), "entityTypeId": float64(135)}) {
			t.Fatalf("get payload: %#v", call.Params)
		}
	}
	if len(p.Calls()) != 4 {
		t.Fatalf("get count: %d", len(p.Calls()))
	}
}

func TestGetByIDDecodeErrorContext(t *testing.T) {
	for _, tc := range []struct {
		name string
		want error
	}{
		{"active", client.ErrInvalidSourceData},
		{"canceled", context.Canceled},
		{"deadline", context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				if tc.want == context.DeadlineExceeded {
					deadline, stop := context.WithTimeout(ctx, time.Second)
					defer stop()
					ctx = deadline
				}
				// The SDK closes the body after decoding the REST envelope.
				transport := cancelAfterRead{
					base: roundTripFunc(func(*http.Request) (*http.Response, error) {
						return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"result":[]}`))}, nil
					}),
					cancel: func() {
						switch tc.want {
						case context.Canceled:
							cancel()
						case context.DeadlineExceeded:
							<-ctx.Done()
						}
					},
				}
				repo, err := newHTTPS("https://example.invalid/rest/1/synthetic/", &http.Client{Transport: transport})
				if err != nil {
					t.Fatal(err)
				}
				got, err := repo.GetByID(ctx, "42")
				if got != (client.CRMClient{}) || !errors.Is(err, tc.want) {
					t.Fatalf("GetByID: %+v, %v; want %v", got, err, tc.want)
				}
			})
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func listPage(next *int, rows ...map[string]any) []byte {
	return b24test.WrappedListResult("items", rows, next, nil)
}

func TestSearchWireAndRanking(t *testing.T) {
	p := b24test.NewPortal(t).On(itemList,
		listPage(b24test.Ptr(50), map[string]any{"id": 1, "title": "Семёнов Сашенька", firstField: "Сашенька"}, map[string]any{"id": 2, "title": "Другой", firstField: "Саша"}),
		listPage(nil, map[string]any{"id": "001", "title": "Переопределение"}, map[string]any{"id": 9, "title": "Саша Семёнов"}, map[string]any{"id": 10, "title": "Саша Семёнов"}, map[string]any{"id": 3, "title": "Семёнов Александр", firstField: "Александр"}))
	r := fakeRepo(p)
	for _, query := range []string{"", " — "} {
		if got, err := r.Search(t.Context(), query, 3); err != nil || len(got) != 0 {
			t.Fatal(err)
		}
	}
	if _, err := r.Search(t.Context(), "Семёнов", 0); err != nil || len(p.Calls()) != 0 {
		t.Fatal("empty search made network calls")
	}
	got, err := r.Search(t.Context(), "Семёнов Саша", 3)
	if err != nil || len(got) != 3 || got[0].ID != "10" || got[1].ID != "9" || got[2].ID != "3" {
		t.Fatalf("ranked results: %+v; %v", got, err)
	}
	calls := p.CallsTo(itemList)
	if len(p.Calls()) != 2 || len(calls) != 2 || calls[1].Params["start"] != float64(50) {
		t.Fatalf("paging: %#v", calls)
	}
	params := calls[0].Params
	if len(params) != 4 || params["entityTypeId"] != float64(135) || !reflect.DeepEqual(params["order"], map[string]any{"id": "ASC"}) || !reflect.DeepEqual(params["select"], []any{"id", titleField, lastField, firstField, patronymicField, birthdayField}) {
		t.Fatalf("list params: %#v", params)
	}
	filter := params["filter"].(map[string]any)
	group := filter["0"].(map[string]any)
	if len(filter) != 1 || group["logic"] != "OR" {
		t.Fatalf("filter: %#v", filter)
	}
	expected := map[predicate]bool{}
	for _, field := range nameFields {
		for _, word := range []string{"семенов", "семёнов", "саша"} {
			expected[predicate{"%" + field, word}] = true
		}
	}
	for _, field := range []string{titleField, firstField} {
		for _, word := range []string{"александр", "александра"} {
			expected[predicate{"%" + field, word}] = true
		}
	}
	for key, v := range group {
		if key == "logic" {
			continue
		}
		cond := v.(map[string]any)
		if len(cond) != 1 {
			t.Fatal(cond)
		}
		for field, value := range cond {
			pair := predicate{field, value.(string)}
			if !expected[pair] {
				t.Fatalf("unexpected predicate: %+v", pair)
			}
			delete(expected, pair)
		}
	}
	if len(expected) != 0 {
		t.Fatalf("missing predicates: %+v", expected)
	}
}

func TestSearchCursorAndBudget(t *testing.T) {
	row := map[string]any{"id": 1, "title": "Иванов"}
	for _, tc := range []struct {
		name          string
		replies       [][]byte
		budget, calls int
		want          error
	}{
		{"last page on budget", [][]byte{listPage(nil, row)}, 1, 1, nil},
		{"budget", [][]byte{listPage(b24test.Ptr(50), row)}, 1, 1, errPages},
		{"empty page", [][]byte{listPage(b24test.Ptr(50)), listPage(nil, row)}, 2, 2, nil},
		{"zero", [][]byte{listPage(b24test.Ptr(0), row)}, 3, 1, errCursor},
		{"negative", [][]byte{listPage(b24test.Ptr(-1), row)}, 3, 1, errCursor},
		{"regression", [][]byte{listPage(b24test.Ptr(50), row), listPage(b24test.Ptr(25), row)}, 3, 2, errCursor},
		{"repeat", [][]byte{listPage(b24test.Ptr(50), row), listPage(b24test.Ptr(50), row)}, 3, 2, errCursor},
		{"late API error", [][]byte{listPage(b24test.Ptr(50), row), b24test.ErrorBody("ACCESS_DENIED", "private")}, 3, 2, errAccess},
		{"late invalid row", [][]byte{listPage(b24test.Ptr(50), row), listPage(nil, map[string]any{"id": 1, birthdayField: "invalid"})}, 3, 2, errRows},
		{"malformed raw next indistinguishable from absent", [][]byte{[]byte(`{"result":{"items":[{"id":1,"title":"Иванов"}]},"next":"bad"}`)}, 1, 1, nil},
		{"null raw next", [][]byte{[]byte(`{"result":{"items":[{"id":1,"title":"Иванов"}]},"next":null}`)}, 1, 1, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := b24test.NewPortal(t).On(itemList, tc.replies...)
			r := fakeRepo(p)
			r.policy.maxPages = tc.budget
			got, err := r.Search(t.Context(), "Иванов", 50)
			if !errors.Is(err, tc.want) || len(p.Calls()) != tc.calls || tc.want != nil && got != nil || tc.want == nil && len(got) != 1 {
				t.Fatalf("got %+v, %v, %d calls", got, err, len(p.Calls()))
			}
		})
	}
	p := b24test.NewPortal(t)
	r := fakeRepo(p)
	r.policy.maxConditions = 3
	if got, err := r.Search(t.Context(), "Иванов", 50); got != nil || !errors.Is(err, errConditions) || len(p.Calls()) != 0 {
		t.Fatalf("condition budget: %v %v", got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := rank(ctx, nil, map[string]client.CRMClient{"1": {ID: "1"}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("rank cancellation: %v", err)
	}
	expired, stop := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer stop()
	if got, err := rank(expired, nil, nil); got != nil || !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errPages) {
		t.Fatalf("ranking deadline: %v, %v", got, err)
	}
}

func TestSearchDefaultPageBudget(t *testing.T) {
	for _, tc := range []struct {
		name string
		next *int
		want error
	}{
		{"last page on budget", nil, nil},
		{"continuation on budget", b24test.Ptr(20 * b24.DefaultPageSize), errPages},
	} {
		t.Run(tc.name, func(t *testing.T) {
			replies := make([][]byte, 20)
			for i := range replies {
				next := b24test.Ptr((i + 1) * b24.DefaultPageSize)
				if i == len(replies)-1 {
					next = tc.next
				}
				replies[i] = listPage(next, map[string]any{"id": i + 1, "title": "Иванов"})
			}
			p := b24test.NewPortal(t).On(itemList, replies...)
			got, err := fakeRepo(p).Search(t.Context(), "Иванов", 50)
			if !errors.Is(err, tc.want) || (tc.want != nil && got != nil) || (tc.want == nil && len(got) != 20) || len(p.Calls()) != 20 {
				t.Fatalf("got %d records, %v, %d calls", len(got), err, len(p.Calls()))
			}
		})
	}
}

func TestHTTPSAndSafeErrors(t *testing.T) {
	marker := "PRIVATE_SYNTHETIC_RESPONSE"
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"success", 200, string(cardReply(42, "Иванов")), nil},
		{"quota", 503, string(b24test.ErrorBody("QUERY_LIMIT_EXCEEDED", marker)), errQuota},
		{"operation time", 429, string(b24test.ErrorBody("OPERATION_TIME_LIMIT", marker)), errQuota},
		{"denied", 403, string(b24test.ErrorBody("ACCESS_DENIED", marker)), errAccess},
		{"not found", 404, marker, errUnavailable},
		{"ambiguous", 200, string(b24test.ErrorBody("NOT_FOUND", marker)), errUnavailable},
		{"invalid JSON", 200, marker, errUnavailable},
		{"empty error", 200, `{"error":""}`, client.ErrInvalidSourceData},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			webhook := server.URL + "/rest/1/synthetic/"
			repo, err := newHTTPS(webhook, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = repo.GetByID(t.Context(), "42")
			if !errors.Is(err, tc.want) || calls.Load() != 1 {
				t.Fatalf("GET: %v, calls %d", err, calls.Load())
			}
			if err != nil && (strings.Contains(err.Error(), marker) || strings.Contains(err.Error(), webhook) || strings.Contains(err.Error(), "synthetic") || errors.Is(err, client.ErrCRMNotFound)) {
				t.Fatal("unsafe or ambiguous classification")
			}
		})
	}
	var forwarded atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { forwarded.Add(1) }))
	defer target.Close()
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	repo, err := newHTTPS(redirect.URL+"/rest/1/synthetic/", redirect.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(t.Context(), "42"); !errors.Is(err, errUnavailable) || forwarded.Load() != 0 {
		t.Fatalf("redirect: %v", err)
	}
}

func TestInFlightAndBetweenPageCancellation(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline", "between"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			started := make(chan struct{})
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if mode == "between" {
					_, _ = w.Write(listPage(b24test.Ptr(50), map[string]any{"id": 42, "title": "Иванов"}))
					return
				}
				close(started)
				select {
				case <-r.Context().Done():
				case <-time.After(time.Second):
				}
			}))
			defer server.Close()
			hc := server.Client()
			var want error = context.Canceled
			switch mode {
			case "deadline":
				hc.Timeout = 40 * time.Millisecond
				want = context.DeadlineExceeded
			case "cancel":
				go func() { <-started; cancel() }()
			case "between":
				hc.Transport = cancelAfterRead{base: http.DefaultTransport, cancel: cancel}
			}
			r := NewWithClient(b24.NewClient(server.URL+"/rest/1/synthetic/", b24.WithHTTPClient(hc), b24.WithRetry(b24.RetryConfig{MaxAttempts: 1})))
			got, err := r.Search(ctx, "Иванов", 50)
			if !errors.Is(err, want) || got != nil || calls.Load() != 1 {
				t.Fatalf("cancellation: %v, %v, %d", got, err, calls.Load())
			}
		})
	}
}

type cancelAfterRead struct {
	base   http.RoundTripper
	cancel context.CancelFunc
}

func (tr cancelAfterRead) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := tr.base.RoundTrip(req)
	if err == nil {
		res.Body = cancelOnClose{res.Body, tr.cancel}
	}
	return res, err
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c cancelOnClose) Close() error { err := c.ReadCloser.Close(); c.cancel(); return err }
