-- name: ListSections :many
select slug, payload, enabled, updated_at
from sections
order by slug;

-- name: GetSection :one
select slug, payload, enabled, updated_at
from sections
where slug = $1;

-- name: UpdateSection :one
update sections
set payload = $2, enabled = $3, updated_at = now()
where slug = $1
returning slug, payload, enabled, updated_at;
