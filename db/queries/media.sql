-- name: InsertMedia :one
insert into media (bucket_path, public_url, alt)
values ($1, $2, $3)
returning id, bucket_path, public_url, alt, created_at;

-- name: ListMedia :many
select id, bucket_path, public_url, alt, created_at
from media
order by created_at desc;

-- name: GetMedia :one
select id, bucket_path, public_url, alt, created_at
from media
where id = $1;

-- name: DeleteMedia :execrows
delete from media
where id = $1;
