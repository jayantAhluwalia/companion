// Package store is the only code that talks to Postgres. Queries are
// hand-written against pgx; the schema lives in embedded goose migrations
// that run at startup, so a deploy is also a migrate.
package store

import (
	"context"
	"embed"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Store struct {
	pool *pgxpool.Pool
}

// schemaName accepts only plain identifiers, because the name is spliced
// into SQL that cannot take a parameter.
var schemaName = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,40}$`)

func setSchema(schema string) (string, error) {
	if schema == "" || schema == "public" {
		return "", nil
	}
	if !schemaName.MatchString(schema) {
		return "", fmt.Errorf("DB_SCHEMA %q is not a plain lower-case identifier", schema)
	}
	return fmt.Sprintf(`set search_path to "%s"`, schema), nil
}

// Open connects to the database. A non-empty schema points every connection
// at that schema instead of "public": development and tests use one so they
// never touch real data in the same database.
func Open(ctx context.Context, databaseURL, schema string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	set, err := setSchema(schema)
	if err != nil {
		return nil, err
	}
	if set != "" {
		cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			_, err := conn.Exec(ctx, set)
			return err
		}
	}
	cfg.MaxConns = 6 // a handful of users; the Supabase pooler has its own limits
	cfg.MaxConnLifetime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Ping reports whether the database answers.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Migrate applies pending migrations. goose wants database/sql, so it gets
// its own short-lived connection via the pgx stdlib adapter.
func Migrate(ctx context.Context, databaseURL, schema string) error {
	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	set, err := setSchema(schema)
	if err != nil {
		return err
	}
	var opts []stdlib.OptionOpenDB
	if set != "" {
		opts = append(opts, stdlib.OptionAfterConnect(func(ctx context.Context, conn *pgx.Conn) error {
			if _, err := conn.Exec(ctx, fmt.Sprintf(`create schema if not exists "%s"`, schema)); err != nil {
				return err
			}
			_, err := conn.Exec(ctx, set)
			return err
		}))
	}
	db := stdlib.OpenDB(*cfg, opts...)
	defer db.Close()
	db.SetMaxOpenConns(1) // one connection, so the schema setting holds for every statement
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.UpContext(ctx, db, "migrations")
}

// DropSchema removes a development or test schema. It refuses "public".
func DropSchema(ctx context.Context, databaseURL, schema string) error {
	if set, err := setSchema(schema); err != nil || set == "" {
		return fmt.Errorf("refusing to drop schema %q", schema)
	}
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, fmt.Sprintf(`drop schema if exists "%s" cascade`, schema))
	return err
}

// Pool exposes the connection pool to the backup tool, which works on
// tables generically and has no business methods of its own.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }
