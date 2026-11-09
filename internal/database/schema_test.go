package database_test

import (
	"context"
	"testing"

	"github.com/bharatmfg/directory/internal/database"
	"github.com/bharatmfg/directory/internal/testutil"
)

// A shared database: another application already has public.goose_db_version at version 31.
// With a dedicated schema the app must create everything in that schema, keep its own migration
// history and leave the other application's objects untouched.
func TestDedicatedSchemaInSharedDatabase(t *testing.T) {
	ctx := context.Background()
	other := testutil.NewEmptyDB(t)
	if _, err := other.Exec(ctx, `CREATE TABLE public.goose_db_version (id serial PRIMARY KEY, version_id bigint NOT NULL,
		is_applied boolean NOT NULL, tstamp timestamp DEFAULT now());
		INSERT INTO public.goose_db_version (version_id, is_applied) VALUES (0, true), (31, true);
		CREATE EXTENSION IF NOT EXISTS citext;`); err != nil {
		t.Fatal(err)
	}

	pool, err := database.Connect(ctx, other.Config().ConnString(), "peoplvance_dir")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if got := database.Schema(pool); got != "peoplvance_dir" {
		t.Fatalf("Schema() = %q", got)
	}
	if err := database.MigrateUp(ctx, pool); err != nil {
		t.Fatalf("migrate up: %v", err)
	}

	var inSchema, inPublic, ourVersion, theirVersion int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM information_schema.tables WHERE table_schema = 'peoplvance_dir' AND table_name = 'companies'),
		(SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name <> 'goose_db_version'),
		(SELECT max(version_id) FROM peoplvance_dir.goose_db_version),
		(SELECT max(version_id) FROM public.goose_db_version)`).Scan(&inSchema, &inPublic, &ourVersion, &theirVersion); err != nil {
		t.Fatal(err)
	}
	if inSchema != 1 || inPublic != 0 {
		t.Fatalf("tables: schema=%d public=%d", inSchema, inPublic)
	}
	if ourVersion < 7 || theirVersion != 31 {
		t.Fatalf("versions: ours=%d theirs=%d", ourVersion, theirVersion)
	}
	// Reference data landed in the schema and unqualified queries resolve there.
	var states int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM indian_states`).Scan(&states); err != nil || states != 36 {
		t.Fatalf("states = %d, err = %v", states, err)
	}
	// Idempotent.
	if err := database.MigrateUp(ctx, pool); err != nil {
		t.Fatalf("second migrate up: %v", err)
	}
}
