package store_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bharatmfg/directory/internal/secure"
	"github.com/bharatmfg/directory/internal/store"
	"github.com/bharatmfg/directory/internal/testutil"
)

// TestPerformance100k loads 100,000 synthetic companies (half published) and checks that the
// main read paths stay index-backed and fast. Run with: PERF=1 go test -run Performance ./internal/store/
func TestPerformance100k(t *testing.T) {
	if os.Getenv("PERF") != "1" {
		t.Skip("set PERF=1 to run the 100k-record performance test")
	}
	ctx := context.Background()
	pool := testutil.NewDB(t)
	start := time.Now()
	_, err := pool.Exec(ctx, `
		SET statement_timeout = 0;
		INSERT INTO data_sources (id, source_type, provider, allowed_internal_use, allowed_public_display)
		VALUES ('00000000-0000-0000-0000-0000000000aa', 'manual_verification', 'perf-synthetic', true, true);
		INSERT INTO companies (id, display_name, slug, normalized_name, publication_status, published_at)
		SELECT gen_random_uuid(), initcap(substr(md5(g::text), 1, 7)) || ' ' || (ARRAY['Rugs','Tiles','Namkeen','Bricks','Machining'])[1 + g % 5] || ' ' || g,
		       'perf-synthetic-' || g, substr(md5(g::text), 1, 7) || ' ' || lower((ARRAY['Rugs','Tiles','Namkeen','Bricks','Machining'])[1 + g % 5]) || ' ' || g,
		       'approved_private', NULL
		FROM generate_series(1, 100000) g;
		INSERT INTO field_provenance (company_id, entity_type, entity_id, field_name, source_id, permission_status)
		SELECT id, 'company', id, 'display_name', '00000000-0000-0000-0000-0000000000aa', 'permitted' FROM companies;
		INSERT INTO facilities (company_id, city, state_code, is_primary, source_id)
		SELECT id, 'City' || (row_number() OVER () % 200), (ARRAY['MH','GJ','TN','UP','RJ'])[1 + (row_number() OVER ())::int % 5], true,
		       '00000000-0000-0000-0000-0000000000aa' FROM companies;
		INSERT INTO field_provenance (company_id, entity_type, entity_id, field_name, source_id, permission_status)
		SELECT company_id, 'facility', id, f, '00000000-0000-0000-0000-0000000000aa', 'permitted'
		FROM facilities, unnest(ARRAY['city','state_code']) f;
		INSERT INTO company_categories (company_id, category_id, source_id)
		SELECT c.id, (SELECT id FROM categories WHERE slug = (ARRAY['rugs-and-carpets','tiles','namkeen-and-snacks','bricks','cnc-machined-components'])[1 + abs(hashtext(c.id::text)) % 5]),
		       '00000000-0000-0000-0000-0000000000aa' FROM companies c;
		UPDATE companies SET publication_status = 'published', published_at = now() WHERE substring(slug from '[0-9]+$')::int % 2 = 0;
		ANALYZE;`)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("loaded 100k companies in %s", time.Since(start).Round(time.Millisecond))
	box, _ := secure.NewBox("dGVzdC1rZXktdGVzdC1rZXktdGVzdC1rZXktdGVzdC1rZXk=")
	_ = store.New(pool, box)

	timeIt := func(name string, budget time.Duration, fn func() error) {
		t.Helper()
		best := time.Hour
		for i := 0; i < 3; i++ { // best of 3 to reduce noise
			s := time.Now()
			if err := fn(); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			best = min(best, time.Since(s))
		}
		t.Logf("%-40s %8s (budget %s)", name, best.Round(time.Microsecond), budget)
		if best > budget {
			t.Errorf("%s took %s, budget %s", name, best, budget)
		}
	}
	timeIt("public search q (trigram)", 800*time.Millisecond, func() error {
		_, _, err := store.SearchPublicCompanies(ctx, pool, store.PublicFilter{Q: "ea5d2f1 tiles"}, 20, 0)
		return err
	})
	timeIt("public search category+state", 800*time.Millisecond, func() error {
		_, _, err := store.SearchPublicCompanies(ctx, pool, store.PublicFilter{Category: "building-and-construction-materials", State: "GJ"}, 20, 0)
		return err
	})
	timeIt("public profile by slug", 100*time.Millisecond, func() error {
		_, err := store.GetPublicCompany(ctx, pool, "perf-synthetic-4242")
		return err
	})
	timeIt("admin list q=", 800*time.Millisecond, func() error {
		_, _, err := store.ListAdminCompanies(ctx, pool, store.CompanyFilter{Q: "f1f836c"}, 25, 0)
		return err
	})
	var plan string
	rows, _ := pool.Query(ctx, `EXPLAIN SELECT id FROM companies WHERE normalized_name % 'ea5d2f1 tiles' AND deleted_at IS NULL`)
	for rows.Next() {
		var l string
		_ = rows.Scan(&l)
		plan += l + "\n"
	}
	rows.Close()
	if !strings.Contains(plan, "companies_normalized_name_trgm") {
		t.Errorf("trigram search is not index-backed:\n%s", plan)
	}
}
