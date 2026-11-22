package postgres

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
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
	sql("INSERT INTO client_links(crm_id,site_id) VALUES ('old-c','1')")
	migrate("up-to", "20261121180000")
	for _, tc := range []struct{ id, diagnostic string }{
		{"invalid-synthetic", "invalid decimal values"},
		{"9223372036854775808", "outside BIGINT range"},
		{"-9223372036854775809", "outside BIGINT range"},
		{" +01 ", "numeric duplicates"},
	} {
		if _, err := pool.Exec(ctx, "INSERT INTO client_links(crm_id,site_id) VALUES ('bad-migration',$1)", tc.id); err != nil {
			t.Fatal(err)
		}
		snapshot := func() [3]string {
			t.Helper()
			var values [3]string
			err := pool.QueryRow(ctx, "SELECT (SELECT jsonb_agg(to_jsonb(l) ORDER BY crm_id)::text FROM client_links l), (SELECT array_agg(pg_get_constraintdef(oid) ORDER BY conname)::text FROM pg_constraint WHERE conrelid='client_links'::regclass), (SELECT pg_typeof(site_id)::text FROM client_links LIMIT 1)").Scan(&values[0], &values[1], &values[2])
			if err != nil {
				t.Fatal(err)
			}
			return values
		}
		before := snapshot()
		out, err := exec.CommandContext(ctx, "goose", "-dir", "../../../../../migrations", "postgres", dsn, "up").CombinedOutput()
		if err == nil || !strings.Contains(string(out), tc.diagnostic) || strings.Contains(string(out), tc.id) {
			t.Fatalf("expected safe migration failure for %s: %s", tc.diagnostic, out)
		}
		if after := snapshot(); after != before {
			t.Fatal("failed migration changed data or schema")
		}
		sql("DELETE FROM client_links WHERE crm_id='bad-migration'")
	}
	sql("INSERT INTO client_links(crm_id,site_id,manually_confirmed,confirmation_reason,confirmation_codes,confirmed_at,assessment_rule_version) VALUES ('min-c',E'\\013-9223372036854775808\\t',true,'synthetic',ARRAY['name:mismatch'],'2026-01-01 00:00:00+00',1)")
	sql("INSERT INTO client_links(crm_id,site_id) VALUES ('max-c',' +09223372036854775807 ')")

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
	ready, err := repo.Create(ctx, link.Decision{CRMID: "ready-c", SiteID: 2, RuleVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, "SELECT NOT manually_confirmed AND confirmation_reason IS NULL AND confirmed_at IS NULL AND assessment_rule_version IS NULL AND cardinality(confirmation_codes)=0 FROM client_links WHERE id=$1", ready.ID).Scan(&defaults)
	if err != nil || !defaults {
		t.Fatalf("ready audit: %v %v", defaults, err)
	}
	decision := link.Decision{CRMID: "crm", SiteID: 9007199254740993, RuleVersion: 1, Confirmation: &link.Confirmation{Reason: "Проверено", At: time.Now().UTC().Truncate(time.Microsecond), Codes: []string{"birthday:missing", "name:mismatch"}}}
	saved, err := repo.Create(ctx, decision)
	if err != nil || saved.CRMID != "crm" || saved.SiteID != decision.SiteID {
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
	crm, site, err := repo.LookupBySourceIDs(ctx, "crm", 1)
	if err != nil || !crm || !site {
		t.Fatalf("lookup: %v %v %v", crm, site, err)
	}
	for _, tc := range []struct {
		c    string
		s    int64
		want error
	}{{"crm", 4, link.ErrCRMAlreadyLinked}, {"free-c", decision.SiteID, link.ErrSiteAlreadyLinked}, {"crm", decision.SiteID, nil}} {
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
	invalid.CRMID, invalid.SiteID = "bad-c", 5
	invalid.Confirmation = &link.Confirmation{Reason: "test", At: time.Now(), Codes: []string{"bad:code"}}
	if _, err := repo.Create(ctx, invalid); !checkViolation(err) {
		t.Fatalf("expected CHECK: %v", err)
	}
	crm, site, err = repo.LookupBySourceIDs(ctx, "bad-c", 5)
	if err != nil || crm || site {
		t.Fatalf("partial insert: %v %v %v", crm, site, err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, id := range []int64{6, 7} {
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
	listed, err := repo.List(ctx)
	if err != nil || len(listed) != 6 {
		t.Fatalf("list: %v", err)
	}
	for i := 1; i < len(listed); i++ {
		a, b := listed[i-1], listed[i]
		if a.CreatedAt.After(b.CreatedAt) || (a.CreatedAt.Equal(b.CreatedAt) && a.ID.String() >= b.ID.String()) {
			t.Fatal("unexpected list order")
		}
	}
	migrate("down")
	var preserved int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM client_links WHERE (crm_id='min-c' AND site_id='-9223372036854775808' AND manually_confirmed AND confirmation_reason='synthetic' AND confirmation_codes=ARRAY['name:mismatch'] AND confirmed_at='2026-01-01 00:00:00+00' AND assessment_rule_version=1) OR (crm_id='max-c' AND site_id='9223372036854775807')").Scan(&preserved)
	if err != nil || preserved != 2 {
		t.Fatalf("Down did not preserve boundary IDs and audit: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE client_links SET site_id=' ' WHERE crm_id='min-c'"); !checkViolation(err) {
		t.Fatalf("Down must restore text CHECK: %v", err)
	}
	migrate("down-to", "20261109141005")
	var count int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM client_links").Scan(&count)
	if err != nil || count != 6 {
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
