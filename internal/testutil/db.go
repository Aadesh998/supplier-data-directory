// Package testutil provides an isolated, migrated PostgreSQL database per test.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io/fs"
	"net/url"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bharatmfg/directory/db"
	"github.com/bharatmfg/directory/internal/database"
)

// DefaultTestURL points at the database created by infra/postgres/init-test-db.sql.
const DefaultTestURL = "postgres://bharatmfg:devpassword@localhost:5434/bharatmfg_test?sslmode=disable"

// AdminURL returns the URL of the maintenance database used to create per-test databases.
func AdminURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	return DefaultTestURL
}

// NewDB creates a fresh database, applies migrations, and drops it when the test ends.
// Tests are skipped when PostgreSQL is unreachable unless REQUIRE_DB=1.
func NewDB(t testing.TB) *pgxpool.Pool {
	t.Helper()
	return newDB(t, true)
}

// NewEmptyDB creates a fresh database without running migrations.
func NewEmptyDB(t testing.TB) *pgxpool.Pool {
	t.Helper()
	return newDB(t, false)
}

func newDB(t testing.TB, migrate bool) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	admin, err := pgxpool.New(ctx, AdminURL())
	if err == nil {
		err = admin.Ping(ctx)
	}
	if err != nil {
		if os.Getenv("REQUIRE_DB") == "1" {
			t.Fatalf("test database unavailable: %v", err)
		}
		t.Skipf("skipping: test database unavailable (%v); run `make db-up`", err)
	}

	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := "t_" + hex.EncodeToString(b)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close()
		t.Fatalf("create test database: %v", err)
	}

	u, _ := url.Parse(AdminURL())
	u.Path = "/" + name
	pool, err := database.Connect(ctx, u.String())
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = admin.Exec(c, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		admin.Close()
	})
	if migrate {
		if err := database.MigrateUp(ctx, pool); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	return pool
}

// Seed applies all seed files except the development-only demo publish seed.
func Seed(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	entries, err := fs.Glob(db.Seeds, "seeds/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(entries)
	for _, e := range entries {
		if e == "seeds/002_demo_publish.sql" {
			continue
		}
		b, err := fs.ReadFile(db.Seeds, e)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), string(b)); err != nil {
			t.Fatalf("seed %s: %v", e, err)
		}
	}
}
