-- name: ListGroupsForImport :many
select id, label
from guest_groups;

-- name: ListGuestsForImport :many
select id, group_id, full_name, normalized_name, is_primary, category
from guests;

-- Wipes the guest list so an import can rebuild it from scratch. Only reached
-- through the importer's replace mode, which the operator asks for explicitly:
-- the normal import upserts and never deletes (AD-10). Groups cascade to their
-- guests; contributions keep their history with group_id nulled.
-- name: DeleteAllGuestGroups :exec
delete from guest_groups;

-- name: InsertGuestGroup :one
insert into guest_groups (label)
values ($1)
returning id;

-- name: UpdateGroupLabel :exec
update guest_groups
set label = $2
where id = $1;

-- name: InsertImportedGuest :one
insert into guests (group_id, full_name, normalized_name, is_primary, category,
                    gender, side, circle, ceremony_role, notes)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
returning id;

-- name: UpdateGuestIdentity :exec
update guests
set full_name = $2, category = $3, gender = $4, side = $5,
    circle = $6, ceremony_role = $7, notes = $8, updated_at = now()
where id = $1;

-- name: SetGroupPrimary :exec
update guests
set is_primary = (id = @guest_id::uuid), updated_at = now()
where group_id = @group_id::uuid;

-- name: InsertImportReport :one
insert into imports (report)
values ($1)
returning id;
