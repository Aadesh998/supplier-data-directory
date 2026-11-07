// Package database owns the PostgreSQL connection pool and schema migrations.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/bharatmfg/directory/db"
)

// Connect opens a pgx pool and verifies connectivity.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
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
	return pool, nil
}

func provider(pool *pgxpool.Pool) (*goose.Provider, *sql.DB, error) {
	sqlDB := stdlib.OpenDBFromPool(pool)
	migrations, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		return nil, nil, err
	}
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations)
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
