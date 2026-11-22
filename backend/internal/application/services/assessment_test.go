package services

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/437i/client-matcher/backend/internal/adapters/outbound"
	"github.com/437i/client-matcher/backend/internal/application/ports"
	"github.com/437i/client-matcher/backend/internal/domain/client"
	"github.com/437i/client-matcher/backend/internal/domain/link"
	"github.com/437i/client-matcher/backend/internal/domain/matcher"
)

type fixture struct {
	crm                        client.CRMClient
	site                       client.SiteClient
	crmErr, siteErr, lookupErr error
	crmBusy, siteBusy          bool
	calls                      []string
	decision                   link.Decision
	now                        time.Time
}

func (f *fixture) Now() time.Time { return f.now }
func (f *fixture) Search(context.Context, string, int) ([]client.CRMClient, error) {
	panic("unexpected search")
}
func (f *fixture) GetByID(context.Context, string) (client.CRMClient, error) {
	f.calls = append(f.calls, "crm")
	return f.crm, f.crmErr
}

type fixtureSite struct{ f *fixture }

func (s fixtureSite) GetByID(context.Context, int64) (client.SiteClient, error) {
	s.f.calls = append(s.f.calls, "site")
	return s.f.site, s.f.siteErr
}
func (f *fixture) LookupBySourceIDs(context.Context, string, int64) (bool, bool, error) {
	f.calls = append(f.calls, "lookup")
	return f.crmBusy, f.siteBusy, f.lookupErr
}
func (f *fixture) Create(_ context.Context, d link.Decision) (link.ClientLink, error) {
	f.calls = append(f.calls, "create")
	f.decision = d
	return link.ClientLink{CRMID: d.CRMID, SiteID: d.SiteID}, nil
}
func (f *fixture) List(context.Context) ([]link.ClientLink, error) { panic("unexpected list") }
func setup() (*fixture, *ClientLinkService) {
	d := &client.Date{Year: 2001, Month: 2, Day: 3}
	f := &fixture{crm: client.CRMClient{ID: "c", FirstName: "Иван", LastName: "Тестов", Birthday: d}, site: client.SiteClient{ID: 9007199254740993, ShortName: "Иван", LastName: "Тестов", Birthday: d}, now: time.Unix(1000, 0)}
	return f, NewClientLinkService(f, fixtureSite{f}, f, outbound.NewHMACToken([32]byte{42}, f), f)
}
func TestAssessmentPriorities(t *testing.T) {
	for _, tc := range []struct {
		name           string
		setup          func(*fixture)
		reasons, calls []string
		want           error
	}{
		{"ready", func(*fixture) {}, nil, []string{"crm", "site", "lookup"}, nil},
		{"both missing before lookup", func(f *fixture) {
			f.crmErr = client.ErrCRMNotFound
			f.siteErr = client.ErrSiteNotFound
			f.crmBusy = true
		}, []string{"crm_not_found", "site_not_found"}, []string{"crm", "site"}, nil},
		{"both occupied", func(f *fixture) { f.crmBusy = true; f.siteBusy = true }, []string{"crm_already_linked", "site_already_linked"}, []string{"crm", "site", "lookup"}, nil},
		{"invalid CRM before missing site", func(f *fixture) { f.crmErr = client.ErrInvalidSourceData; f.siteErr = client.ErrSiteNotFound }, nil, []string{"crm"}, client.ErrInvalidSourceData},
		{"site timeout", func(f *fixture) { f.siteErr = context.DeadlineExceeded }, nil, []string{"crm", "site"}, context.DeadlineExceeded},
		{"CRM canceled", func(f *fixture) { f.crmErr = context.Canceled }, nil, []string{"crm"}, context.Canceled},
		{"lookup error", func(f *fixture) { f.lookupErr = context.DeadlineExceeded }, nil, []string{"crm", "site", "lookup"}, context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, s := setup()
			tc.setup(f)
			r, err := s.Assess(context.Background(), " c ", 9007199254740993)
			if !errors.Is(err, tc.want) || !slices.Equal(f.calls, tc.calls) || !slices.Equal(r.BlockedReasons, tc.reasons) {
				t.Fatalf("assessment %+v err=%v calls=%v", r, err, f.calls)
			}
			if tc.want != nil || len(tc.reasons) > 0 {
				if r.Token != nil || r.ExpiresAt != nil || len(r.Checks) != 0 || r.ConfirmationRequired {
					t.Fatal("unexpected assessment data")
				}
				if tc.want == nil && r.Status != matcher.StatusBlocked {
					t.Fatal(r.Status)
				}
			} else if r.Status != matcher.StatusReady || r.Token == nil || !r.ExpiresAt.Equal(f.now.Add(ports.TokenTTL)) {
				t.Fatalf("ready: %+v", r)
			}
		})
	}
}
func TestCreateRechecksAndAudits(t *testing.T) {
	for _, tc := range []struct {
		name             string
		ready            bool
		alter            func(*fixture, *string)
		without, confirm bool
		reason           string
		want             error
		calls            []string
	}{
		{name: "legacy", ready: true, without: true},
		{name: "manual", confirm: true, reason: "  Проверено  "},
		{name: "unicode limit", confirm: true, reason: strings.Repeat("я", 1000)},
		{name: "missing token", without: true, confirm: true, reason: "Проверено", want: ErrConfirmationRequired},
		{name: "not confirmed", want: ErrConfirmationRequired},
		{name: "blank reason", confirm: true, reason: "\u2003", want: ErrInvalidConfirmation},
		{name: "long reason", confirm: true, reason: strings.Repeat("я", 1001), want: ErrInvalidConfirmation},
		{name: "ready confirmed", ready: true, confirm: true, want: ErrInvalidConfirmation},
		{name: "ready reason", ready: true, reason: "Проверено", want: ErrInvalidConfirmation},
		{name: "invalid before reading", ready: true, alter: func(f *fixture, p *string) { *p = ""; f.crmErr = client.ErrCRMNotFound }, want: ports.ErrInvalidAssessmentToken, calls: []string{}},
		{name: "expired", alter: func(f *fixture, _ *string) { f.now = f.now.Add(ports.TokenTTL) }, want: ports.ErrAssessmentExpired, calls: []string{}},
		{name: "missing before stale", alter: func(f *fixture, _ *string) { f.siteErr = client.ErrSiteNotFound }, want: client.ErrSiteNotFound, calls: []string{"crm", "site"}},
		{name: "invalid source", alter: func(f *fixture, _ *string) { f.crmErr = client.ErrInvalidSourceData }, want: client.ErrInvalidSourceData, calls: []string{"crm"}},
		{name: "cosmetic before occupied", alter: func(f *fixture, _ *string) { f.crm.LastName = "ТЕСТОВ"; f.crmBusy = true }, want: ErrAssessmentStale, calls: []string{"crm", "site"}},
		{name: "became ready", alter: func(f *fixture, _ *string) { f.site.ShortName = f.crm.FirstName }, want: ErrAssessmentStale, calls: []string{"crm", "site"}},
		{name: "occupied before confirmation", alter: func(f *fixture, _ *string) { f.siteBusy = true }, want: link.ErrSiteAlreadyLinked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, s := setup()
			ctx := context.Background()
			if !tc.ready {
				f.site.ShortName = "Ваня"
			}
			r, err := s.Assess(ctx, "c", 9007199254740993)
			if err != nil || r.Token == nil {
				t.Fatalf("assessment: %v", err)
			}
			tok := r.Token
			if tc.without {
				tok = nil
			}
			if tc.alter != nil {
				tc.alter(f, tok)
			}
			f.calls = nil
			_, err = s.CreateLink(ctx, " c ", 9007199254740993, tok, tc.confirm, tc.reason)
			calls := tc.calls
			if calls == nil {
				calls = []string{"crm", "site", "lookup"}
				if tc.want == nil {
					calls = append(calls, "create")
				}
			}
			if !errors.Is(err, tc.want) || !slices.Equal(f.calls, calls) {
				t.Fatalf("err=%v calls=%v, want %v %v", err, f.calls, tc.want, calls)
			}
			if tc.want == nil {
				audit := f.decision.Confirmation
				if tc.ready {
					if audit != nil {
						t.Fatal("unexpected audit")
					}
				} else if audit == nil || audit.Reason != strings.TrimSpace(tc.reason) || !audit.At.Equal(f.now) || !slices.Equal(audit.Codes, []string{"name:alias_match"}) {
					t.Fatalf("audit: %+v", audit)
				}
			}
		})
	}
}
