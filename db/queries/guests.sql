-- name: GetGuestByNormalizedName :one
select id, group_id, full_name, is_primary, category, attending
from guests
where normalized_name = $1;

-- name: GetGuestGroup :one
select id, label
from guest_groups
where id = $1;

-- name: ListGroupMembers :many
select id, full_name, is_primary, category, attending, added_by_guest
from guests
where group_id = $1
order by is_primary desc, full_name;

-- Adds a companion the guest brought along. The row lock is on the group, not
-- on the guests table: two people opening the invitation on two phones must
-- not slip past the per-group ceiling by racing each other's count.
-- name: LockGroupForCompanion :one
select id
from guest_groups
where id = $1
for update;

-- name: CountGroupCompanions :one
select count(*)
from guests
where group_id = $1 and added_by_guest;

-- name: InsertCompanion :one
insert into guests (group_id, full_name, normalized_name, category, attending,
                    responded_at, added_by_guest)
values ($1, $2, $3, 'adult', $4, now(), true)
returning id, full_name, is_primary, category, attending;

-- name: UpdateGuestAttendance :execrows
update guests
set attending = $3, responded_at = now(), updated_at = now()
where id = $1 and group_id = $2;

-- name: SuggestGuestNames :many
select full_name
from guests
where normalized_name like @prefix::text || '%' escape '\'
order by full_name
limit 8;

-- name: ListAllGroups :many
select id, label
from guest_groups
order by label, id;

-- name: ListAllGuests :many
select id, group_id, full_name, is_primary, category, attending
from guests
order by is_primary desc, full_name;
