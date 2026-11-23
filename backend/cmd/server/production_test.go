package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/437i/client-matcher/backend/internal/platform/config"
	"github.com/437i/client-matcher/backend/internal/platform/db"
	"github.com/437i/client-matcher/backend/internal/platform/sshtunnel/sshtest"
)

func TestProductionProcess(t *testing.T) {
	if path := os.Getenv("CLIENT_MATCHER_STAGE3_PROCESS_CONFIG"); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var cfg config.Config
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatal(err)
		}
		ctx, stop := signalContext()
		defer stop()
		if err := runConfigured(ctx, cfg); err != nil {
			t.Fatal(err)
		}
		return
	}
	if os.Getenv("CLIENT_MATCHER_STAGE3_INTEGRATION") != "1" {
		t.Skip("requires disposable synthetic PostgreSQL servers")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	od, err := pgx.Connect(ctx, "postgres://postgres:synthetic@127.0.0.1:55416/postgres?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer od.Close(context.Background())
	siteDB, err := pgx.Connect(ctx, "postgres://postgres:synthetic@127.0.0.1:55410/postgres?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer siteDB.Close(context.Background())
	for conn, want := range map[*pgx.Conn]int{siteDB: 100023, od: 160015} {
		var version int
		if err := conn.QueryRow(ctx, "SELECT current_setting('server_version_num')::int").Scan(&version); err != nil || version != want {
			t.Fatalf("PostgreSQL version=%d, want %d: %v", version, want, err)
		}
	}
	execOD := func(t *testing.T, query string) {
		t.Helper()
		if _, err := od.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	execOD(t, "CREATE ROLE process_migrator LOGIN PASSWORD 'synthetic'; CREATE ROLE process_runtime LOGIN PASSWORD 'synthetic'; CREATE SCHEMA apps AUTHORIZATION process_migrator; GRANT USAGE, CREATE ON SCHEMA apps TO process_runtime")
	defer func() {
		if _, err := od.Exec(context.Background(), "DROP SCHEMA apps CASCADE; DROP ROLE process_runtime, process_migrator"); err != nil {
			t.Error(err)
		}
	}()
	site := sshtest.New(t, "127.0.0.1:0", "127.0.0.1:55410", false)
	own := sshtest.New(t, "127.0.0.1:0", "127.0.0.1:55416", true)
	dir := t.TempDir()
	hosts := filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(hosts, []byte(site.KnownHost+own.KnownHost), 0600); err != nil {
		t.Fatal(err)
	}
	crmPath, err := filepath.Abs("../../../testdata/crm.csv")
	if err != nil {
		t.Fatal(err)
	}
	listening, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listening.Addr().String()
	listening.Close()
	database := config.DBConfig{Host: "localhost", Port: "5432", User: "postgres", Password: "synthetic", Name: "postgres", SSLMode: "disable"}
	cfg := config.Config{DBTransport: config.TransportSSH, SiteSource: config.SitePostgres, SiteSSH: site.Config, ODSSH: own.Config, KnownHostsPath: hosts, SiteDB: database, DB: database, Server: config.ServerConfig{Addr: address}, CSV: config.CSVConfig{CRMPath: crmPath}}
	start := func(t *testing.T, cfg config.Config) (*exec.Cmd, <-chan error) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "synthetic.json")
		raw, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProductionProcess$")
		cmd.Env = []string{"GIN_MODE=release", "CLIENT_MATCHER_STAGE3_PROCESS_CONFIG=" + path}
		log, err := os.Create(filepath.Join(t.TempDir(), "process.log"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { log.Close() })
		cmd.Stdout, cmd.Stderr = log, log
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		result := make(chan error, 1)
		go func() { result <- cmd.Wait(); close(result) }()
		t.Cleanup(func() {
			_ = cmd.Process.Kill()
			<-result
			if t.Failed() {
				_, _ = log.Seek(0, 0)
				raw, _ := io.ReadAll(log)
				t.Log(string(raw))
			}
		})
		return cmd, result
	}
	await := func(t *testing.T, result <-chan error, limit time.Duration, success bool) {
		t.Helper()
		select {
		case err := <-result:
			if (err == nil) != success {
				t.Fatalf("exit success=%v: %v", success, err)
			}
		case <-time.After(limit):
			t.Fatal("process did not exit")
		}
	}
	waitDrain := func(t *testing.T) {
		t.Helper()
		poll(t, func() bool { return site.Connections() == 0 && own.Connections() == 0 })
	}
	noHTTP := func(t *testing.T) {
		t.Helper()
		if c, err := net.DialTimeout("tcp", address, 100*time.Millisecond); err == nil {
			c.Close()
			t.Fatal("HTTP listened during startup")
		}
	}
	blockMigration := func(t *testing.T) pgx.Tx {
		t.Helper()
		tx, err := od.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, "LOCK TABLE apps.goose_db_version IN ACCESS EXCLUSIVE MODE"); err != nil {
			tx.Rollback(ctx)
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(ctx) })
		return tx
	}
	waitBlocked := func(t *testing.T) {
		t.Helper()
		inspector, err := pgx.Connect(ctx, "postgres://postgres:synthetic@127.0.0.1:55416/postgres?sslmode=disable")
		if err != nil {
			t.Fatal(err)
		}
		defer inspector.Close(ctx)
		poll(t, func() bool {
			var waiting bool
			err := inspector.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%goose_db_version%')").Scan(&waiting)
			return err == nil && waiting
		})
	}
	signal := func(t *testing.T, cmd *exec.Cmd) {
		t.Helper()
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
	}
	// Create an empty Goose journal, then block Goose's first read from it.
	if _, err := od.Exec(ctx, "SET LOCAL ROLE process_migrator; CREATE TABLE apps.goose_db_version(id bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,version_id bigint NOT NULL,is_applied boolean NOT NULL,tstamp timestamp DEFAULT now()); INSERT INTO apps.goose_db_version(version_id,is_applied) VALUES (0,true)"); err != nil {
		t.Fatal(err)
	}
	t.Run("migration_before_HTTP", func(t *testing.T) {
		tx := blockMigration(t)
		migrationConfig := cfg
		migrationConfig.DB.User = "process_migrator"
		cmd, result := start(t, migrationConfig)
		waitBlocked(t)
		noHTTP(t)
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		poll(t, func() bool { return responseCode(address, "/healthz") == 200 })
		signal(t, cmd)
		await(t, result, 5*time.Second, true)
		waitDrain(t)
	})
	if t.Failed() {
		return
	}
	t.Run("OD readiness", func(t *testing.T) {
		execOD(t, "GRANT SELECT(id,version_id,is_applied) ON apps.goose_db_version TO process_runtime;"+
			"INSERT INTO apps.client_links(crm_id,site_id) VALUES ('synthetic-readiness',42);"+
			"CREATE TABLE public.client_links (LIKE apps.client_links INCLUDING ALL);"+
			"INSERT INTO public.client_links(crm_id,site_id) VALUES ('synthetic-public',43)")
		t.Cleanup(func() { execOD(t, "DROP TABLE public.client_links") })
		var isolated bool
		err := od.QueryRow(ctx, `SELECT NOT rolsuper AND NOT EXISTS (SELECT FROM pg_auth_members WHERE member = r.oid)
		    AND (SELECT count(*) FROM pg_class
		         WHERE oid IN ('apps.client_links'::regclass, 'apps.goose_db_version'::regclass)
		           AND relowner = 'process_migrator'::regrole) = 2
		    FROM pg_roles r WHERE rolname = 'process_runtime'`).Scan(&isolated)
		if err != nil || !isolated {
			t.Fatalf("runtime isolation or migration ownership: %v, %v", isolated, err)
		}
		reads := []string{"id", "crm_id", "site_id", "created_at"}
		writes := []string{"crm_id", "site_id", "manually_confirmed", "confirmation_codes", "confirmed_at", "assessment_rule_version", "confirmation_reason"}
		grant := func(t *testing.T, table, privilege string, columns []string) {
			t.Helper()
			if len(columns) > 0 {
				execOD(t, "GRANT "+privilege+"("+strings.Join(columns, ",")+") ON "+table+" TO process_runtime")
			}
		}
		grant(t, "public.client_links", "SELECT", reads)
		grant(t, "public.client_links", "INSERT", writes)
		snapshot := func(t *testing.T, table string) string {
			t.Helper()
			var rows string
			if err := od.QueryRow(ctx, "SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY id), '[]')::text FROM "+table+" r").Scan(&rows); err != nil {
				t.Fatal(err)
			}
			return rows
		}
		for _, tc := range []struct {
			name          string
			reads, writes []string
			rename, ready bool
		}{
			{name: "missing_INSERT", reads: reads},
			{name: "missing_SELECT", writes: writes},
			{name: "missing_confirmation_reason_INSERT", reads: reads, writes: writes[:len(writes)-1]},
			{name: "missing_apps_table", reads: reads, writes: writes, rename: true},
			{name: "all_column_privileges", reads: reads, writes: writes, ready: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				const table = "apps.client_links"
				grant(t, table, "SELECT", tc.reads)
				grant(t, table, "INSERT", tc.writes)
				t.Cleanup(func() {
					execOD(t, "REVOKE SELECT("+strings.Join(reads, ",")+"), INSERT("+strings.Join(writes, ",")+") ON "+table+" FROM process_runtime")
				})
				assertLinkPrivileges(t, ctx, od, table, tc.reads, tc.writes)
				links := table
				if tc.rename {
					assertLinkPrivileges(t, ctx, od, "public.client_links", reads, writes)
					execOD(t, "ALTER TABLE apps.client_links RENAME TO hidden_client_links")
					links = "apps.hidden_client_links"
					t.Cleanup(func() { execOD(t, "ALTER TABLE apps.hidden_client_links RENAME TO client_links") })
				}
				beforeLinks := snapshot(t, links)
				beforeGoose := snapshot(t, "apps.goose_db_version")
				beforePublic := snapshot(t, "public.client_links")
				runtime := cfg
				runtime.DB.User = "process_runtime"
				startup, stop := context.WithCancel(ctx)
				result := make(chan error, 1)
				go func() {
					result <- runConfigured(startup, runtime)
					close(result)
				}()
				t.Cleanup(func() { stop(); <-result })
				poll(t, func() bool {
					select {
					case err := <-result:
						if tc.ready || !errors.Is(err, db.ErrODLinkStorageNotReady) {
							t.Fatalf("unexpected startup result: %v", err)
						}
						return true
					default:
						if !tc.ready {
							noHTTP(t)
							return false
						}
						return responseCode(address, "/healthz") == 200
					}
				})
				if tc.ready {
					stop()
					await(t, result, 5*time.Second, true)
				}
				noHTTP(t)
				waitDrain(t)
				poll(t, func() bool {
					var drained bool
					err := od.QueryRow(ctx, "SELECT NOT EXISTS (SELECT FROM pg_stat_activity WHERE usename = 'process_runtime')").Scan(&drained)
					return err == nil && drained
				})
				if snapshot(t, links) != beforeLinks || snapshot(t, "apps.goose_db_version") != beforeGoose || snapshot(t, "public.client_links") != beforePublic {
					t.Fatal("startup changed link rows or Goose history")
				}
			})
		}
	})
	for _, failure := range []string{"SITE SSH", "OD SSH", "SITE PostgreSQL", "OD PostgreSQL", "OD schema", "HTTP bind"} {
		t.Run(failure, func(t *testing.T) {
			bad := cfg
			switch failure {
			case "SITE SSH":
				bad.SiteSSH.Password = "incorrect"
			case "OD SSH":
				bad.ODSSH.User = "incorrect"
			case "SITE PostgreSQL":
				bad.SiteDB.User = "missing_user"
			case "OD PostgreSQL":
				bad.DB.Password = "incorrect"
			case "OD schema":
				if _, err := od.Exec(ctx, "ALTER SCHEMA apps RENAME TO hidden_apps"); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if _, err := od.Exec(ctx, "ALTER SCHEMA hidden_apps RENAME TO apps"); err != nil {
						t.Error(err)
					}
				})
			case "HTTP bind":
				occupied, err := net.Listen("tcp", address)
				if err != nil {
					t.Fatal(err)
				}
				defer occupied.Close()
			}
			_, result := start(t, bad)
			await(t, result, 6*time.Second, false)
			waitDrain(t)
		})
	}
	for _, failure := range []string{"SITE loss before final check", "OD loss during migration", "SIGTERM during migration"} {
		t.Run(failure, func(t *testing.T) {
			tx := blockMigration(t)
			defer tx.Rollback(ctx)
			cmd, result := start(t, cfg)
			waitBlocked(t)
			noHTTP(t)
			switch failure {
			case "SITE loss before final check":
				site.Drop()
			case "OD loss during migration":
				own.Drop()
			default:
				signal(t, cmd)
			}
			await(t, result, 6*time.Second, false)
			waitDrain(t)
		})
	}
	t.Run("SIGTERM during SSH handshake", func(t *testing.T) {
		peer, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer peer.Close()
		stalled := cfg
		stalled.ODSSH.Address = peer.Addr().String()
		cmd, result := start(t, stalled)
		if err := peer.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		c, err := peer.Accept()
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		signal(t, cmd)
		await(t, result, 3*time.Second, false)
		waitDrain(t)
	})
	for _, force := range []bool{false, true} {
		t.Run(fmt.Sprintf("SIGTERM HTTP drain forced=%t", force), func(t *testing.T) {
			cmd, result := start(t, cfg)
			poll(t, func() bool { return responseCode(address, "/healthz") == 200 })
			tx, err := od.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, "LOCK TABLE apps.client_links IN ACCESS EXCLUSIVE MODE"); err != nil {
				t.Fatal(err)
			}
			request := make(chan int, 1)
			go func() {
				client := http.Client{Timeout: 15 * time.Second}
				res, err := client.Get("http://" + address + "/api/v1/links")
				if err != nil {
					request <- 0
					return
				}
				res.Body.Close()
				request <- res.StatusCode
			}()
			inspector, err := pgx.Connect(ctx, "postgres://postgres:synthetic@127.0.0.1:55416/postgres?sslmode=disable")
			if err != nil {
				t.Fatal(err)
			}
			defer inspector.Close(ctx)
			poll(t, func() bool {
				var waiting bool
				err := inspector.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE 'SELECT % FROM client_links%')").Scan(&waiting)
				return err == nil && waiting
			})
			started := time.Now()
			signal(t, cmd)
			poll(t, func() bool { return responseCode(address, "/healthz") == 0 })
			if !force {
				if err := tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
			}
			await(t, result, shutdownTimeout+3*time.Second, !force)
			if force && time.Since(started) < shutdownTimeout {
				t.Fatal("drain ended before timeout")
			}
			if code := <-request; (!force && code != 200) || (force && code != 0) {
				t.Fatalf("HTTP result %d", code)
			}
			waitDrain(t)
		})
	}

	t.Run("SIGTERM during reconnect backoff", func(t *testing.T) {
		cmd, result := start(t, cfg)
		poll(t, func() bool { return responseCode(address, "/healthz") == 200 })
		own.Drop()
		poll(t, func() bool { return own.Connections() == 0 && responseCode(address, "/healthz") != 200 })
		signal(t, cmd)
		await(t, result, 3*time.Second, true)
		waitDrain(t)
	})
}

func assertLinkPrivileges(t *testing.T, ctx context.Context, od *pgx.Conn, table string, reads, writes []string) {
	t.Helper()
	var tableSelect, tableInsert bool
	err := od.QueryRow(ctx, "SELECT has_table_privilege('process_runtime',$1,'SELECT'), has_table_privilege('process_runtime',$1,'INSERT')", table).Scan(&tableSelect, &tableInsert)
	if err != nil || tableSelect || tableInsert {
		t.Fatalf("%s table privileges: SELECT=%v INSERT=%v: %v", table, tableSelect, tableInsert, err)
	}
	rows, err := od.Query(ctx, `SELECT attname::text,
	    has_column_privilege('process_runtime', attrelid, attnum, 'SELECT'),
	    has_column_privilege('process_runtime', attrelid, attnum, 'INSERT')
	    FROM pg_attribute WHERE attrelid = $1::text::regclass AND attnum > 0 AND NOT attisdropped
	    ORDER BY attname::text COLLATE "C"`, table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var gotReads, gotWrites []string
	for rows.Next() {
		var col string
		var read, write bool
		if err := rows.Scan(&col, &read, &write); err != nil {
			t.Fatal(err)
		}
		if read {
			gotReads = append(gotReads, col)
		}
		if write {
			gotWrites = append(gotWrites, col)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gotReads, slices.Sorted(slices.Values(reads))) || !slices.Equal(gotWrites, slices.Sorted(slices.Values(writes))) {
		t.Fatalf("%s effective SELECT=%v INSERT=%v, want SELECT=%v INSERT=%v", table, gotReads, gotWrites, reads, writes)
	}
}

func poll(t *testing.T, f func() bool) {
	t.Helper()
	until := time.Now().Add(6 * time.Second)
	for !f() {
		if time.Now().After(until) {
			t.Fatal("condition did not become true")
		}
		time.Sleep(15 * time.Millisecond)
	}
}

func responseCode(address, path string) int {
	client := http.Client{Timeout: 400 * time.Millisecond}
	res, err := client.Get(fmt.Sprintf("http://%s%s", address, path))
	if err != nil {
		return 0
	}
	defer res.Body.Close()
	return res.StatusCode
}
