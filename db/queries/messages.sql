-- name: InsertMessage :one
insert into messages (group_id, author_name, body)
values ($1, $2, $3)
returning id, group_id, author_name, body, created_at;

-- name: ListMessages :many
select id, group_id, author_name, body, created_at
from messages
order by created_at desc;

-- The wall on the site. Capped rather than complete: the page draws them all
-- at once and scrolls them past, so an unbounded list would grow into a
-- heavier page every month without anyone deciding to.
-- name: ListPublicMessages :many
select id, author_name, body, created_at
from messages
order by created_at desc
limit $1;

-- name: DeleteMessage :execrows
delete from messages
where id = $1;
