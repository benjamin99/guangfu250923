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
		`create table if not exists volunteer_organizations (
            id uuid primary key default gen_random_uuid(),
            last_updated timestamptz,
            registration_status text,
            organization_nature text,
            organization_name text,
            coordinator text,
            contact_info text,
            registration_method text,
            service_content text,
            meeting_info text,
            notes text,
            image_url text
        )`,
		`create index if not exists idx_vol_org_updated on volunteer_organizations(last_updated)`,
		`create table if not exists delivery_records (
            id uuid primary key default gen_random_uuid(),
            supply_item_id uuid not null references supply_items(id) on delete cascade,
            quantity int not null,
            notes text,
            created_at timestamptz not null default now()
        )`,
		`create index if not exists idx_delivery_records_supply_item on delivery_records(supply_item_id)`,
		`create or replace view supplies_overview as
        select 
            si.id as item_id,
            r.id as request_id,
            r.name as org,
            r.address,
            r.phone,
            r.status,
            (si.received_count >= si.total_count) as is_completed,
            exists(select 1 from supply_items si2 where si2.request_id = r.id and si2.tag ilike '%medical%' ) as has_medical,
            extract(epoch from r.created_at)::bigint as created_at,
            extract(epoch from greatest(r.created_at, si.created_at))::bigint as updated_at,
            si.id as item_id_dup,
            si.name as item_name,
            si.tag as item_type,
            si.total_count as item_need,
            si.received_count as item_got,
            si.unit as item_unit,
            case when si.received_count >= si.total_count then 'completed' when si.received_count = 0 then 'pending' else 'partial' end as item_status,
            dr.id as delivery_id,
            extract(epoch from dr.created_at)::bigint as delivery_timestamp,
            dr.quantity as delivery_quantity,
            dr.notes as delivery_notes,
            -- per-request aggregates
            ( select count(*) from supply_items x where x.request_id = r.id ) as total_items_in_request,
            ( select count(*) from supply_items x where x.request_id = r.id and x.received_count >= x.total_count ) as completed_items_in_request,
            ( select count(*) from supply_items x where x.request_id = r.id and x.received_count < x.total_count ) as pending_items_in_request,
            -- system aggregates
            ( select count(*) from requests ) as total_requests,
            ( select count(*) from requests where status='pending' or status='partial' ) as active_requests,
            ( select count(*) from requests where status='fulfilled' ) as completed_requests,
            ( select count(*) from requests where status='closed' ) as cancelled_requests,
            ( select count(*) from supply_items ) as total_items,
            ( select count(*) from supply_items where received_count >= total_count ) as completed_items,
            ( select count(*) from supply_items where received_count < total_count ) as pending_items,
            ( select count(*) from requests r2 where r2.status='pending' and exists (select 1 from supply_items si3 where si3.request_id=r2.id and si3.tag ilike '%medical%') ) as medical_requests,
            0 as urgent_requests -- placeholder (need business rule)
        from supply_items si
        join requests r on r.id = si.request_id
        left join lateral (
            select * from delivery_records d where d.supply_item_id = si.id order by d.created_at desc limit 1
        ) dr on true;
        `,
		`create table if not exists shelters (
            id uuid primary key default gen_random_uuid(),
            name text not null,
            location text not null,
            phone text not null,
            link text,
            status text not null,
            capacity int,
            current_occupancy int,
            available_spaces int,
            facilities text[],
            contact_person text,
            notes text,
            lat double precision,
            lng double precision,
            opening_hours text,
            created_at timestamptz not null default now(),
            updated_at timestamptz not null default now()
        )`,
		`create index if not exists idx_shelters_status on shelters(status)`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			return err
		}
	}
	return nil
}
