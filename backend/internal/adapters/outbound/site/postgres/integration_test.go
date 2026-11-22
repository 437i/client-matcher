package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/437i/client-matcher/backend/internal/domain/client"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSiteReaderPostgresIntegration(t *testing.T) {
	dsn := os.Getenv("CLIENT_MATCHER_SITE_TEST_DSN")
	if dsn == "" {
		t.Skip("set CLIENT_MATCHER_SITE_TEST_DSN to a disposable synthetic site database")
	}
	if appDSN := os.Getenv("CLIENT_MATCHER_TEST_DSN"); appDSN != "" && appDSN == dsn {
		t.Fatal("site and application integration tests must use separate databases")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("connect site test database")
	}
	t.Cleanup(pool.Close)
	var existing bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.children') IS NOT NULL").Scan(&existing); err != nil {
		t.Fatal(err)
	}
	if existing {
		t.Fatal("site test database must not already contain public.children")
	}
	_, err = pool.Exec(ctx, "CREATE TABLE public.children (id BIGINT PRIMARY KEY, title VARCHAR NULL, birthday TIMESTAMP NULL, published BOOLEAN NULL, type VARCHAR NULL, old BOOLEAN NULL, transfered BOOLEAN NULL, death_date TIMESTAMP NULL, id_child_fond VARCHAR NULL)")
	if err != nil {
		t.Fatal("create synthetic public.children")
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP TABLE IF EXISTS public.children"); err != nil {
			t.Errorf("drop synthetic public.children: %v", err)
		}
	})
	_, err = pool.Exec(ctx, "INSERT INTO public.children(id,title,birthday,published,type,old,transfered,death_date,id_child_fond) VALUES (9007199254740993,'  Иван   Петрович Сидоров  ','2020-01-01 22:30:00',false,'Other',true,true,'2021-01-01','999'), (2,NULL,NULL,NULL,NULL,NULL,NULL,NULL,NULL), (3,'Однасловное',NULL,NULL,NULL,NULL,NULL,NULL,'3'), (4,'Бесконечная Дата','infinity',NULL,NULL,NULL,NULL,NULL,NULL), (5,'Слишком Далёкая Дата','12000-01-01 00:00:00',NULL,NULL,NULL,NULL,NULL,NULL)")
	if err != nil {
		t.Fatal("insert synthetic site records")
	}
	execSQL := func(query string) {
		t.Helper()
		if _, err := pool.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	execSQL("INSERT INTO public.children(id,title,birthday) VALUES (6,'','-infinity'), (7,'','0001-01-01 BC')")
	execSQL("CREATE ROLE stage2_site_test_reader NOLOGIN")
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanup, "DROP OWNED BY stage2_site_test_reader; DROP ROLE stage2_site_test_reader"); err != nil {
			t.Error(err)
		}
	})
	execSQL("GRANT USAGE ON SCHEMA public TO stage2_site_test_reader; GRANT SELECT (id,title,birthday) ON public.children TO stage2_site_test_reader")
	openReader := func(timezone string) *pgxpool.Pool {
		t.Helper()
		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			t.Fatal("parse site test configuration")
		}
		cfg.ConnConfig.RuntimeParams["timezone"] = timezone
		cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, "SET ROLE stage2_site_test_reader")
			return err
		}
		reader, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal("open restricted site pool")
		}
		t.Cleanup(reader.Close)
		return reader
	}

	for _, timezone := range []string{"UTC", "America/Los_Angeles"} {
		zonePool := openReader(timezone)
		got, err := NewSiteRepo(zonePool).GetByID(ctx, 9007199254740993)
		if err != nil || got.ID != 9007199254740993 || got.ShortName != "Иван" || got.LastName != "Петрович Сидоров" || got.Birthday == nil || got.Birthday.String() != "2020-01-02" {
			t.Fatalf("read valid record at session timezone %s: %+v, %v", timezone, got, err)
		}
	}
	repo := NewSiteRepo(openReader("UTC"))
	for _, id := range []int64{2, 3} {
		got, err := repo.GetByID(ctx, id)
		if err != nil || got.Birthday != nil {
			t.Fatalf("NULL fields id %d: %+v, %v", id, got, err)
		}
	}
	got, err := repo.GetByID(ctx, 2)
	if err != nil || got.ShortName != "" || got.LastName != "" || got.UnstructuredName != "" {
		t.Fatalf("NULL title: %+v, %v", got, err)
	}
	got, err = repo.GetByID(ctx, 3)
	if err != nil || got.UnstructuredName != "Однасловное" {
		t.Fatalf("single word title: %+v, %v", got, err)
	}
	if _, err := repo.GetByID(ctx, 999); !errors.Is(err, client.ErrSiteNotFound) {
		t.Fatalf("missing record: %v", err)
	}
	for _, id := range []int64{4, 5, 6, 7} {
		if _, err := repo.GetByID(ctx, id); !errors.Is(err, client.ErrInvalidSourceData) {
			t.Fatalf("unsupported birthday id %d: %v", id, err)
		}
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := repo.GetByID(canceled, 2); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read: %v", err)
	}
	deadline, stopDeadline := context.WithDeadline(ctx, time.Now().Add(-time.Second))
	defer stopDeadline()
	if _, err := repo.GetByID(deadline, 2); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired read: %v", err)
	}
}
