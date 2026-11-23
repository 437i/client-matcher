package db

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	linkPG "github.com/437i/client-matcher/backend/internal/adapters/outbound/link/postgres"
	sitePG "github.com/437i/client-matcher/backend/internal/adapters/outbound/site/postgres"
	"github.com/437i/client-matcher/backend/internal/domain/link"
	"github.com/437i/client-matcher/backend/internal/platform/config"
	"github.com/437i/client-matcher/backend/internal/platform/sshtunnel"
	"github.com/437i/client-matcher/backend/internal/platform/sshtunnel/sshtest"
	"github.com/437i/client-matcher/backend/migrations"
)

func TestProductionPostgres(t *testing.T) {
	if os.Getenv("CLIENT_MATCHER_STAGE3_INTEGRATION") != "1" {
		t.Skip("requires disposable SITE 10.23 and OD 16.15 on loopback ports 55410 and 55416")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 55*time.Second)
	defer cancel()
	siteAdmin := testAdmin(t, ctx, 55410, 100023)
	odAdmin := testAdmin(t, ctx, 55416, 160015)
	exec := func(p *pgxpool.Pool, q string) {
		t.Helper()
		if _, err := p.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	exec(siteAdmin, "CREATE ROLE stage3_reader LOGIN PASSWORD 'synthetic'; CREATE TABLE public.children(id bigint PRIMARY KEY,title text,birthday timestamp, private_note text); INSERT INTO public.children VALUES (9007199254740993,'Иван Петров','2020-01-01 22:30',NULL),(2,'Анна Петрова',NULL,NULL); GRANT SELECT(id,title,birthday) ON public.children TO stage3_reader")
	exec(odAdmin, "CREATE ROLE stage3_od LOGIN PASSWORD 'synthetic'; CREATE TABLE public.client_links(marker text); INSERT INTO public.client_links VALUES ('untouched'); CREATE TABLE public.goose_db_version(marker text); INSERT INTO public.goose_db_version VALUES ('untouched')")
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := siteAdmin.Exec(cleanup, "DROP TABLE public.children; DROP ROLE stage3_reader"); err != nil {
			t.Error(err)
		}
		if _, err := odAdmin.Exec(cleanup, "DROP SCHEMA IF EXISTS apps CASCADE; DROP TABLE public.client_links,public.goose_db_version; DROP ROLE stage3_od"); err != nil {
			t.Error(err)
		}
	})
	sitePeer := sshtest.New(t, "127.0.0.1:0", "127.0.0.1:55410", false)
	odPeer := sshtest.New(t, "127.0.0.1:0", "127.0.0.1:55416", true)
	hosts := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(hosts, []byte(sitePeer.KnownHost+odPeer.KnownHost), 0600); err != nil {
		t.Fatal(err)
	}
	open := func(peer *sshtest.Peer, user string, apps bool) *pgxpool.Pool {
		t.Helper()
		tunnel, err := sshtunnel.New(peer.Config, "localhost:5432", hosts)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(tunnel.Close)
		pool, err := NewTunnelPool(ctx, testDBConfig(user), tunnel, apps)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { tunnel.Close(); pool.Close() })
		if err := tunnel.Start(ctx, pool.Reset, func() { t.Error("unexpected startup loss") }); err != nil {
			t.Fatal(err)
		}
		if err := sshtunnel.EnableRecovery(tunnel); err != nil {
			t.Fatal(err)
		}
		if err := pool.Ping(ctx); err != nil {
			t.Fatal(err)
		}
		return pool
	}
	site := open(sitePeer, "stage3_reader", false)
	od := open(odPeer, "stage3_od", true)
	requireValue[int](t, ctx, site, "SELECT current_setting('server_version_num')::int", 100023)
	requireValue[int](t, ctx, od, "SELECT current_setting('server_version_num')::int", 160015)
	siteRepo := sitePG.NewSiteRepo(site)
	checkSite := func(t *testing.T) {
		t.Helper()
		dated, err := siteRepo.GetByID(ctx, 9007199254740993)
		if err != nil || dated.ID != 9007199254740993 || dated.ShortName != "Иван" || dated.LastName != "Петров" || dated.Birthday == nil || dated.Birthday.String() != "2020-01-02" {
			t.Fatalf("dated SITE record: %+v: %v", dated, err)
		}
		undated, err := siteRepo.GetByID(ctx, 2)
		if err != nil || undated.ID != 2 || undated.ShortName != "Анна" || undated.LastName != "Петрова" || undated.Birthday != nil {
			t.Fatalf("NULL SITE record: %+v: %v", undated, err)
		}
	}
	checkSite(t)
	if _, err := site.Exec(ctx, "SELECT private_note FROM public.children"); err == nil {
		t.Fatal("SITE can read unrelated columns")
	}
	if _, err := site.Exec(ctx, "UPDATE public.children SET title='changed'"); err == nil {
		t.Fatal("SITE can write")
	}
	t.Log("SITE PostgreSQL 10.23 (100023) through SSH: ID, title, date at midnight boundary and NULL verified with SiteReader")

	for _, setup := range []string{"", "CREATE SCHEMA apps; GRANT CREATE ON SCHEMA apps TO stage3_od", "REVOKE CREATE ON SCHEMA apps FROM stage3_od; GRANT USAGE ON SCHEMA apps TO stage3_od"} {
		if setup != "" {
			exec(odAdmin, setup)
		}
		if err := MigrateOD(ctx, od); err == nil {
			t.Fatal("accepted invalid schema permissions")
		}
	}
	exec(odAdmin, "GRANT CREATE ON SCHEMA apps TO stage3_od")
	requireValue[string](t, ctx, od, "SHOW search_path", "apps")
	directConfig := testDBConfig("postgres")
	directConfig.Host = "127.0.0.1"
	directConfig.Port = "55416"
	directPool, err := NewPool(ctx, directConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer directPool.Close()
	requireValue[string](t, ctx, directPool, "SHOW search_path", "\"$user\", public")
	otherOD := open(odPeer, "stage3_od", true)
	concurrent := make(chan error, 1)
	go func() { concurrent <- MigrateOD(ctx, otherOD) }()
	err = MigrateOD(ctx, od)
	if otherErr := <-concurrent; err != nil || otherErr != nil {
		t.Fatalf("concurrent Up: %v / %v", err, otherErr)
	}
	if err := MigrateOD(ctx, od); err != nil {
		t.Fatal(err)
	}
	requireValue[int](t, ctx, od, "SELECT count(*) FROM apps.goose_db_version WHERE is_applied AND version_id>0", 3)
	requireValue[string](t, ctx, od, "SELECT pg_typeof((NULL::apps.client_links).site_id)::text", "bigint")
	requireValue[string](t, ctx, odAdmin, "SELECT marker FROM public.client_links", "untouched")
	requireValue[string](t, ctx, odAdmin, "SELECT marker FROM public.goose_db_version", "untouched")
	requireValue[bool](t, ctx, siteAdmin, "SELECT to_regclass('public.client_links') IS NULL AND to_regclass('public.goose_db_version') IS NULL", true)
	repo := linkPG.NewPGLinkRepo(od)
	decision := link.Decision{CRMID: "synthetic", SiteID: 9007199254740993, RuleVersion: 1, Confirmation: &link.Confirmation{Reason: "synthetic", Codes: []string{"name:mismatch"}, At: time.Now()}}
	if _, err := repo.Create(ctx, decision); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, link.Decision{CRMID: "duplicate", SiteID: decision.SiteID}); err == nil {
		t.Fatal("SITE ID uniqueness missing")
	}
	requireValue[bool](t, ctx, odAdmin, "SELECT manually_confirmed AND confirmation_reason='synthetic' AND confirmation_codes=ARRAY['name:mismatch'] FROM apps.client_links", true)
	checkOD := func(t *testing.T) {
		t.Helper()
		rows, err := repo.List(ctx)
		if err != nil || len(rows) != 1 {
			t.Fatalf("list links: %d: %v", len(rows), err)
		}
		crm, site, err := repo.LookupBySourceIDs(ctx, decision.CRMID, decision.SiteID)
		if err != nil || !crm || !site {
			t.Fatalf("lookup: %v %v %v", crm, site, err)
		}
	}
	checkOD(t)
	for _, tc := range []struct {
		name                   string
		broken, healthy, admin *pgxpool.Pool
		peer                   *sshtest.Peer
		check                  func(*testing.T)
	}{
		{"SITE", site, od, siteAdmin, sitePeer, checkSite}, {"OD", od, site, odAdmin, odPeer, checkOD},
	} {
		t.Run("recovery_"+tc.name, func(t *testing.T) {
			idle, busy := acquire(t, ctx, tc.broken), acquire(t, ctx, tc.broken)
			defer busy.Release()
			oldIdle, oldBusy := idle.Conn().PgConn().PID(), busy.Conn().PgConn().PID()
			idle.Release()
			active := make(chan error, 1)
			go func() { _, err := busy.Exec(ctx, "SELECT pg_sleep(30)"); active <- err }()
			eventually(t, ctx, func() bool {
				var active bool
				err := tc.admin.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND state='active' AND query='SELECT pg_sleep(30)')", oldBusy).Scan(&active)
				return err == nil && active
			})
			tc.peer.Drop()
			select {
			case err := <-active:
				if err == nil {
					t.Fatal("interrupted SQL succeeded")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("SQL remained blocked")
			}
			if err := tc.healthy.Ping(ctx); err != nil {
				t.Fatalf("healthy pool unavailable: %v", err)
			}
			eventually(t, ctx, func() bool { return tc.broken.Ping(ctx) == nil })
			a, b := acquire(t, ctx, tc.broken), acquire(t, ctx, tc.broken)
			defer a.Release()
			defer b.Release()
			for _, c := range []*pgxpool.Conn{a, b} {
				got := c.Conn().PgConn().PID()
				if got == oldIdle || got == oldBusy {
					t.Fatalf("stale backend PID %d", got)
				}
				if tc.name == "OD" {
					requireValue[string](t, ctx, c, "SHOW search_path", "apps")
				}
			}
			tc.check(t)
		})
	}
	t.Run("late_CreateResource", func(t *testing.T) { testLateResource(t, ctx, sitePeer, hosts, od) })
	t.Run("partial_Up_and_session_lock_transfer", func(t *testing.T) {
		exec(odAdmin, "DROP SCHEMA apps CASCADE; CREATE SCHEMA apps AUTHORIZATION stage3_od")
		od.Reset()
		otherOD.Reset()
		direct := stdlib.OpenDBFromPool(od)
		defer direct.Close()
		provider, err := goose.NewProvider(goose.DialectPostgres, direct, migrations.Files, goose.WithTableName("apps.goose_db_version"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := provider.UpTo(ctx, 20261121180000); err != nil {
			t.Fatal(err)
		}
		requireValue[int](t, ctx, od, "SELECT count(*) FROM apps.goose_db_version WHERE is_applied AND version_id>0", 2)
		held := acquire(t, ctx, od)
		defer held.Release()
		tx, err := held.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, "LOCK TABLE apps.client_links IN ACCESS EXCLUSIVE MODE"); err != nil {
			t.Fatal(err)
		}
		result := make(chan error, 1)
		go func() { result <- MigrateOD(ctx, od) }()
		eventually(t, ctx, func() bool {
			var held bool
			err := odAdmin.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND granted)").Scan(&held)
			return err == nil && held
		})
		odPeer.Drop()
		select {
		case err := <-result:
			if err == nil {
				t.Fatal("interrupted Up succeeded")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Up hung after SSH loss")
		}
		eventually(t, ctx, func() bool { return otherOD.Ping(ctx) == nil })
		if err := MigrateOD(ctx, otherOD); err != nil {
			t.Fatal(err)
		}
		requireValue[int](t, ctx, otherOD, "SELECT count(*) FROM apps.goose_db_version WHERE is_applied AND version_id>0", 3)
	})
}

func testLateResource(t *testing.T, ctx context.Context, peer *sshtest.Peer, hosts string, healthy *pgxpool.Pool) {
	tunnel, err := sshtunnel.New(peer.Config, "localhost:5432", hosts)
	if err != nil {
		t.Fatal(err)
	}
	defer tunnel.Close()
	cfg, err := tunnelPoolConfig(testDBConfig("stage3_reader"), tunnel, false)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MinConns = 1
	cfg.HealthCheckPeriod = time.Hour
	cfg.ShouldPing = func(context.Context, pgxpool.ShouldPingParams) bool { return false }
	entered := make(chan uint32, 1)
	release := make(chan struct{})
	var attempts, rejected, resets atomic.Int32
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		if attempts.Add(1) == 1 {
			entered <- c.PgConn().PID()
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
	prepare := cfg.PrepareConn
	cfg.PrepareConn = func(ctx context.Context, c *pgx.Conn) (bool, error) {
		ok, err := prepare(ctx, c)
		if !ok && err == nil {
			rejected.Add(1)
		}
		return ok, err
	}
	startDial := make(chan struct{})
	dial := cfg.ConnConfig.DialFunc
	cfg.ConnConfig.DialFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
		select {
		case <-startDial:
			return dial(ctx, network, address)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { tunnel.Close(); pool.Close() }()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	if err := tunnel.Start(ctx, func() { pool.Reset(); resets.Add(1) }, func() {}); err != nil {
		t.Fatal(err)
	}
	if err := sshtunnel.EnableRecovery(tunnel); err != nil {
		t.Fatal(err)
	}
	close(startDial)
	var old uint32
	select {
	case old = <-entered:
	case <-ctx.Done():
		t.Fatal("background constructor did not reach AfterConnect")
	}
	repo := sitePG.NewSiteRepo(pool)
	peer.Drop()
	eventually(t, ctx, func() bool { return resets.Load() == 1 })
	eventually(t, ctx, func() bool {
		c, err := tunnel.DialContext(ctx, "tcp", tunnel.Address())
		if err != nil {
			return false
		}
		c.Close()
		return true
	})
	if err := healthy.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	close(release)
	eventually(t, ctx, func() bool { return pool.Stat().IdleConns() > 0 })
	conn := acquire(t, ctx, pool)
	got := conn.Conn().PgConn().PID()
	conn.Release()
	if got == old || rejected.Load() != 1 {
		t.Fatalf("PrepareConn rejected=%d; PIDs %d -> %d", rejected.Load(), old, got)
	}
	if got, err := repo.GetByID(ctx, 2); err != nil || got.ID != 2 {
		t.Fatalf("SiteReader after late constructor: %+v %v", got, err)
	}
}

func testDBConfig(user string) config.DBConfig {
	return config.DBConfig{Host: "localhost", Port: "5432", User: user, Password: "synthetic", Name: "postgres", SSLMode: "disable"}
}
func testAdmin(t *testing.T, ctx context.Context, port, version int) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, fmt.Sprintf("postgres://postgres:synthetic@127.0.0.1:%d/postgres?sslmode=disable", port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	requireValue[int](t, ctx, pool, "SELECT current_setting('server_version_num')::int", version)
	return pool
}
func requireValue[T comparable](t *testing.T, ctx context.Context, p interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, query string, want T) {
	t.Helper()
	var got T
	if err := p.QueryRow(ctx, query).Scan(&got); err != nil || got != want {
		t.Fatalf("%s: got %v, want %v: %v", query, got, want, err)
	}
}
func acquire(t *testing.T, ctx context.Context, p *pgxpool.Pool) *pgxpool.Conn {
	t.Helper()
	c, err := p.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func eventually(t *testing.T, ctx context.Context, ready func() bool) {
	t.Helper()
	limit, stop := context.WithTimeout(ctx, 8*time.Second)
	defer stop()
	for !ready() {
		select {
		case <-limit.Done():
			t.Fatal("condition did not become true")
		case <-time.After(20 * time.Millisecond):
		}
	}
}
