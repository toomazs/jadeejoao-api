-- name: InsertMessage :one
insert into messages (group_id, author_name, body)
values ($1, $2, $3)
returning id, group_id, author_name, body, created_at;

-- name: ListMessages :many
select id, group_id, author_name, body, created_at
from messages
order by created_at desc;
