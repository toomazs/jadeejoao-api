-- name: ListGiftsWithProgress :many
select g.id, g.title, g.description, g.image_url, g.goal_centavos, g.quota_centavos,
       g.max_units, g.active, g.sort,
       coalesce(sum(c.amount_centavos) filter (where c.status = 'declared'), 0)::bigint  as declared_centavos,
       coalesce(sum(c.amount_centavos) filter (where c.status = 'confirmed'), 0)::bigint as confirmed_centavos
from gifts g
left join contributions c on c.gift_id = g.id
where g.active or not @only_active::boolean
group by g.id
order by g.sort, g.created_at;

-- name: GetGiftWithProgress :one
select g.id, g.title, g.description, g.image_url, g.goal_centavos, g.quota_centavos,
       g.max_units, g.active, g.sort,
       coalesce(sum(c.amount_centavos) filter (where c.status = 'declared'), 0)::bigint  as declared_centavos,
       coalesce(sum(c.amount_centavos) filter (where c.status = 'confirmed'), 0)::bigint as confirmed_centavos
from gifts g
left join contributions c on c.gift_id = g.id
where g.id = $1
group by g.id;

-- name: GetGiftForUpdate :one
select id, quota_centavos, max_units, active
from gifts
where id = $1
for update;

-- name: SumGiftNonCancelled :one
select coalesce(sum(amount_centavos), 0)::bigint
from contributions
where gift_id = $1 and status <> 'cancelled';

-- name: InsertContribution :one
insert into contributions (gift_id, group_id, contributor_name, amount_centavos)
values ($1, $2, $3, $4)
returning id, gift_id, group_id, contributor_name, amount_centavos, status, created_at, confirmed_at;
