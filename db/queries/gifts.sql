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

-- name: InsertGift :one
insert into gifts (title, description, image_url, goal_centavos, quota_centavos, max_units, active, sort)
values ($1, $2, $3, $4, $5, $6, $7, $8)
returning id;

-- name: UpdateGift :execrows
update gifts
set title = $2, description = $3, image_url = $4, goal_centavos = $5,
    quota_centavos = $6, max_units = $7, active = $8, sort = $9
where id = $1;

-- name: CountGiftContributions :one
select count(*)
from contributions
where gift_id = $1;

-- name: DeleteGiftRow :execrows
delete from gifts
where id = $1;

-- name: DeactivateGift :execrows
update gifts
set active = false
where id = $1;

-- name: ListContributions :many
select c.id, c.gift_id, g.title as gift_title, c.group_id, c.contributor_name,
       c.amount_centavos, c.status, c.created_at, c.confirmed_at
from contributions c
join gifts g on g.id = c.gift_id
where (@status_filter::text = '' or c.status = @status_filter::text)
order by c.created_at desc;

-- name: UpdateContributionStatus :one
update contributions
set status = @status::text,
    confirmed_at = case when @status::text = 'confirmed' then now() else confirmed_at end
where id = @id
returning id, gift_id, group_id, contributor_name, amount_centavos, status, created_at, confirmed_at;
