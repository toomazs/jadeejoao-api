-- name: InsertMessage :one
insert into messages (group_id, author_name, body)
values ($1, $2, $3)
returning id, group_id, author_name, body, status, created_at;

-- name: ListMessages :many
select id, group_id, author_name, body, status, created_at
from messages
where (@status_filter::text = '' or status = @status_filter::text)
order by created_at desc;

-- name: UpdateMessageStatus :one
update messages
set status = $2
where id = $1
returning id, group_id, author_name, body, status, created_at;
