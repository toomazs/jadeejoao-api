-- name: ListGroupsForImport :many
select id, label
from guest_groups;

-- name: ListGuestsForImport :many
select id, group_id, full_name, normalized_name, is_primary, category
from guests;

-- name: InsertGuestGroup :one
insert into guest_groups (label)
values ($1)
returning id;

-- name: UpdateGroupLabel :exec
update guest_groups
set label = $2
where id = $1;

-- name: InsertImportedGuest :one
insert into guests (group_id, full_name, normalized_name, is_primary, category)
values ($1, $2, $3, $4, $5)
returning id;

-- name: UpdateGuestIdentity :exec
update guests
set full_name = $2, category = $3, updated_at = now()
where id = $1;

-- name: SetGroupPrimary :exec
update guests
set is_primary = (id = @guest_id::uuid), updated_at = now()
where group_id = @group_id::uuid;

-- name: InsertImportReport :one
insert into imports (report)
values ($1)
returning id;
