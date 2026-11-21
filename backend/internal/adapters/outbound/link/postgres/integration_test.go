package postgres

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/437i/client-matcher/backend/internal/domain/link"
)

// The DSN must point to a disposable, empty synthetic database.
func TestPostgresLinkAudit(t *testing.T) {
	dsn := os.Getenv("CLIENT_MATCHER_TEST_DSN")
	if dsn == "" {
		t.Skip("set CLIENT_MATCHER_TEST_DSN to an empty disposable PostgreSQL database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var empty bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('client_links') IS NULL AND to_regclass('goose_db_version') IS NULL").Scan(&empty); err != nil || !empty {
		t.Fatalf("need empty database: %v", err)
	}
	migrate := func(args ...string) {
		t.Helper()
		base := []string{"-dir", "../../../../../migrations", "postgres", dsn}
		out, err := exec.CommandContext(ctx, "goose", append(base, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("Goose %s: %v\n%s", args[0], err, out)
		}
	}
	sql := func(query string) {
		t.Helper()
		if _, err := pool.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	migrate("up-to", "20261109141005")
	sql("INSERT INTO client_links(crm_id,site_id) VALUES ('old-c','old-s')")
	migrate("up")
	var defaults bool
	err = pool.QueryRow(ctx, "SELECT NOT manually_confirmed AND confirmation_reason IS NULL AND confirmed_at IS NULL AND assessment_rule_version IS NULL AND cardinality(confirmation_codes)=0 FROM client_links WHERE crm_id='old-c'").Scan(&defaults)
	if err != nil || !defaults {
		t.Fatalf("legacy defaults: %v %v", defaults, err)
	}
	for _, query := range []string{
		"UPDATE client_links SET confirmation_reason='unexpected' WHERE crm_id='old-c'",
		"UPDATE client_links SET manually_confirmed=true,confirmation_reason='test',confirmed_at=now(),confirmation_codes=ARRAY['name:mismatch'] WHERE crm_id='old-c'",
		"UPDATE client_links SET manually_confirmed=true,confirmation_reason=' ',confirmed_at=now(),assessment_rule_version=1,confirmation_codes=ARRAY['name:mismatch'] WHERE crm_id='old-c'",
	} {
		if _, err := pool.Exec(ctx, query); !checkViolation(err) {
			t.Fatalf("expected CHECK violation: %v", err)
		}
	}
	repo := NewPGLinkRepo(pool)
	ready, err := repo.Create(ctx, link.Decision{CRMID: "ready-c", SiteID: "ready-s", RuleVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, "SELECT NOT manually_confirmed AND confirmation_reason IS NULL AND confirmed_at IS NULL AND assessment_rule_version IS NULL AND cardinality(confirmation_codes)=0 FROM client_links WHERE id=$1", ready.ID).Scan(&defaults)
	if err != nil || !defaults {
		t.Fatalf("ready audit: %v %v", defaults, err)
	}
	decision := link.Decision{CRMID: "crm", SiteID: "site", RuleVersion: 1, Confirmation: &link.Confirmation{Reason: "Проверено", At: time.Now().UTC().Truncate(time.Microsecond), Codes: []string{"birthday:missing", "name:mismatch"}}}
	saved, err := repo.Create(ctx, decision)
	if err != nil || saved.CRMID != "crm" || saved.SiteID != "site" {
		t.Fatalf("create: %+v %v", saved, err)
	}
	var manual bool
	var reason string
	var codes []string
	var at time.Time
	var version int
	err = pool.QueryRow(ctx, "SELECT manually_confirmed,confirmation_reason,confirmation_codes,confirmed_at,assessment_rule_version FROM client_links WHERE id=$1", saved.ID).Scan(&manual, &reason, &codes, &at, &version)
	if err != nil || !manual || reason != decision.Confirmation.Reason || !slices.Equal(codes, decision.Confirmation.Codes) || !at.Equal(decision.Confirmation.At) || version != 1 {
		t.Fatalf("audit mismatch: %v", err)
	}
	var columns []string
	err = pool.QueryRow(ctx, "SELECT array_agg(column_name::text ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='client_links'").Scan(&columns)
	if err != nil || !slices.Equal(columns, []string{"id", "crm_id", "site_id", "created_at", "manually_confirmed", "confirmation_reason", "confirmation_codes", "confirmed_at", "assessment_rule_version"}) {
		t.Fatalf("audit columns: %v %v", columns, err)
	}
	crm, site, err := repo.LookupBySourceIDs(ctx, "crm", "old-s")
	if err != nil || !crm || !site {
		t.Fatalf("lookup: %v %v %v", crm, site, err)
	}
	for _, tc := range []struct {
		c, s string
		want error
	}{{"crm", "free-s", link.ErrCRMAlreadyLinked}, {"free-c", "site", link.ErrSiteAlreadyLinked}, {"crm", "site", nil}} {
		_, err = repo.Create(ctx, link.Decision{CRMID: tc.c, SiteID: tc.s, RuleVersion: 1})
		if tc.want == nil {
			if !errors.Is(err, link.ErrCRMAlreadyLinked) && !errors.Is(err, link.ErrSiteAlreadyLinked) {
				t.Fatal(err)
			}
		} else if !errors.Is(err, tc.want) {
			t.Fatalf("unique: %v", err)
		}
	}
	invalid := decision
	invalid.CRMID, invalid.SiteID = "bad-c", "bad-s"
	invalid.Confirmation = &link.Confirmation{Reason: "test", At: time.Now(), Codes: []string{"bad:code"}}
	if _, err := repo.Create(ctx, invalid); !checkViolation(err) {
		t.Fatalf("expected CHECK: %v", err)
	}
	crm, site, err = repo.LookupBySourceIDs(ctx, "bad-c", "bad-s")
	if err != nil || crm || site {
		t.Fatalf("partial insert: %v %v %v", crm, site, err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, id := range []string{"race-s1", "race-s2"} {
		go func() {
			<-start
			_, err := repo.Create(ctx, link.Decision{CRMID: "race-c", SiteID: id, RuleVersion: 1, Confirmation: decision.Confirmation})
			results <- err
		}()
	}
	close(start)
	a, b := <-results, <-results
	if !(a == nil && errors.Is(b, link.ErrCRMAlreadyLinked) || b == nil && errors.Is(a, link.ErrCRMAlreadyLinked)) {
		t.Fatalf("concurrent insert: %v %v", a, b)
	}
	migrate("down")
	var count int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM client_links").Scan(&count)
	if err != nil || count != 4 {
		t.Fatalf("down row preservation: %d %v", count, err)
	}
	err = pool.QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='client_links'").Scan(&count)
	if err != nil || count != 4 {
		t.Fatalf("down columns: %d %v", count, err)
	}
}

func checkViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}
