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

-- Who a guest may gather into their invitation: someone the couple already
-- invited AND who is still alone in their own invitation. Anyone heading an
-- invitation that holds other people is excluded — pulling them across would
-- orphan the rest of their family.
-- name: SuggestAvailableCompanions :many
select g.id, g.full_name
from guests g
where g.normalized_name like @prefix::text || '%' escape ''
  and g.group_id <> @group_id::uuid
  and (select count(*) from guests o where o.group_id = g.group_id) = 1
order by g.full_name
limit 8;

-- name: GetGuestWithGroupSize :one
select g.id, g.group_id, g.full_name, g.is_primary, g.attending,
       (select count(*) from guests o where o.group_id = g.group_id) as group_size
from guests g
where g.id = $1;

-- name: InsertGuestGroup :one
insert into guest_groups (label)
values ($1)
returning id;

-- name: MoveGuestToGroup :execrows
update guests
set group_id = $2, is_primary = false, added_by_guest = true, updated_at = now()
where id = $1;

-- name: DeleteGroupIfEmpty :exec
delete from guest_groups
where id = $1 and not exists (select 1 from guests where group_id = $1);

-- Sends someone back to the invitation they arrived with. `added_by_guest` in
-- the WHERE is the whole safety property: without it, anyone who can open an
-- invitation could move away the names the couple typed into their spreadsheet.
-- name: RestoreCompanionToOwnGroup :execrows
update guests
set group_id = $3, is_primary = true, added_by_guest = false, updated_at = now()
where id = $1 and group_id = $2 and added_by_guest;

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
