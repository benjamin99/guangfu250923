package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Simple idempotent migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	stmts := []string{
		`create table if not exists requests (
            id uuid primary key default gen_random_uuid(),
            code text,
            name text not null,
            address text,
            phone text,
            contact text,
            status text not null default 'pending',
            needed_people int,
            notes text,
            lng double precision,
            lat double precision,
            map_link text,
            created_at timestamptz not null default now()
        )`,
		`create table if not exists supply_items (
            id uuid primary key default gen_random_uuid(),
            request_id uuid not null references requests(id) on delete cascade,
            tag text,
            name text not null,
            total_count int not null,
            received_count int not null default 0,
            unit text not null,
            created_at timestamptz not null default now()
        )`,
		`create index if not exists idx_supply_items_request_id on supply_items(request_id)`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			return err
		}
	}
	return nil
}
