// Package database owns the PostgreSQL connection pool and schema migrations.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/bharatmfg/directory/db"
)

// Connect opens a pgx pool and verifies connectivity. If schema is non-empty, every connection uses
// search_path "<schema>, public" (public only so shared extensions such as citext resolve), the schema
// is created if missing, and migration history is kept in <schema>.goose_db_version. That lets the
// app live safely inside a database shared with other applications.
func Connect(ctx context.Context, url string, schema ...string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	if len(schema) > 0 && schema[0] != "" {
		if !validSchema.MatchString(schema[0]) {
			return nil, fmt.Errorf("invalid schema name %q", schema[0])
		}
		cfg.ConnConfig.RuntimeParams["search_path"] = schema[0] + ", public"
	}
	cfg.MaxConns = 20
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 10 * time.Minute
	cfg.ConnConfig.RuntimeParams["application_name"] = "bharatmfg-api"
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "15000"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if s := Schema(pool); s != "" {
		if _, err := pool.Exec(pingCtx, `CREATE SCHEMA IF NOT EXISTS `+s); err != nil {
			pool.Close()
			return nil, fmt.Errorf("create schema %s: %w", s, err)
		}
	}
	return pool, nil
}

var validSchema = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// Schema returns the dedicated schema the pool was opened with, or "" for the default (public).
func Schema(pool *pgxpool.Pool) string {
	sp := pool.Config().ConnConfig.RuntimeParams["search_path"]
	if sp == "" {
		return ""
	}
	first, _, _ := strings.Cut(sp, ",")
	if first = strings.TrimSpace(first); validSchema.MatchString(first) && first != "public" {
		return first
	}
	return ""
}

func provider(pool *pgxpool.Pool) (*goose.Provider, *sql.DB, error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	migrations, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		return nil, nil, err
	}
	var opts []goose.ProviderOption
	if s := Schema(pool); s != "" {
		// Never share a version table with another application in the same database.
		opts = append(opts, goose.WithTableName(s+".goose_db_version"))
	}
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations, opts...)
	if err != nil {
		sqlDB.Close()
		return nil, nil, err
	}
	return p, sqlDB, nil
}

// MigrateUp applies all pending migrations.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool) error {
	p, sqlDB, err := provider(pool)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	_, err = p.Up(ctx)
	return err
}

// MigrateDownAll rolls back every migration (used by tests and `migrate reset`).
func MigrateDownAll(ctx context.Context, pool *pgxpool.Pool) error {
	p, sqlDB, err := provider(pool)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	_, err = p.DownTo(ctx, 0)
	return err
}

// MigrateDown rolls back the most recent migration.
func MigrateDown(ctx context.Context, pool *pgxpool.Pool) error {
	p, sqlDB, err := provider(pool)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	_, err = p.Down(ctx)
	return err
}

// MigrationStatus returns a human-readable list of migration states.
func MigrationStatus(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	p, sqlDB, err := provider(pool)
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()
	st, err := p.Status(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(st))
	for _, s := range st {
		out = append(out, fmt.Sprintf("%-6s %s", s.State, s.Source.Path))
	}
	return out, nil
}
