package database_test

import (
	"context"
	"testing"

	"github.com/bharatmfg/directory/internal/database"
	"github.com/bharatmfg/directory/internal/testutil"
)

func TestMigrationsUpDownUp(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewEmptyDB(t)

	if err := database.MigrateUp(ctx, pool); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := database.MigrateDownAll(ctx, pool); err != nil {
		t.Fatalf("down: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name <> 'goose_db_version'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected no tables after full down migration, found %d", n)
	}
	if err := database.MigrateUp(ctx, pool); err != nil {
		t.Fatalf("re-up: %v", err)
	}
}

func TestReferenceDataSeeded(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewDB(t)
	checks := map[string]int{
		"SELECT count(*) FROM indian_states":                      36,
		"SELECT count(*) FROM business_roles":                     15,
		"SELECT count(*) FROM categories WHERE parent_id IS NULL": 14,
	}
	for q, want := range checks {
		var got int
		if err := pool.QueryRow(ctx, q).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s = %d, want %d", q, got, want)
		}
	}
	var deepest int
	if err := pool.QueryRow(ctx, `WITH RECURSIVE t AS (
		SELECT id, 1 AS depth FROM categories WHERE parent_id IS NULL
		UNION ALL SELECT c.id, t.depth + 1 FROM categories c JOIN t ON c.parent_id = t.id)
		SELECT max(depth) FROM t`).Scan(&deepest); err != nil {
		t.Fatal(err)
	}
	if deepest < 3 {
		t.Errorf("expected at least 3 levels of categories, got %d", deepest)
	}
}
