-- name: GetGuestByNormalizedName :one
select id, group_id, full_name, is_primary, category, attending
from guests
where normalized_name = $1;

-- name: GetGuestGroup :one
select id, label
from guest_groups
where id = $1;

-- name: ListGroupMembers :many
select id, full_name, is_primary, category, attending
from guests
where group_id = $1
order by is_primary desc, full_name;

-- name: UpdateGuestAttendance :execrows
update guests
set attending = $3, responded_at = now(), updated_at = now()
where id = $1 and group_id = $2;

-- name: ListAllGroups :many
select id, label
from guest_groups
order by label, id;

-- name: ListAllGuests :many
select id, group_id, full_name, is_primary, category, attending
from guests
order by is_primary desc, full_name;
