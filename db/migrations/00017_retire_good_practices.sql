-- +goose Up

-- The couple dropped the house-rules section from the site. Disabled, not
-- deleted: the copy stays in the database and the admin can bring it back
-- with one toggle.

update sections
set enabled = false,
    updated_at = now()
where slug = 'good_practices';

-- +goose Down

update sections
set enabled = true,
    updated_at = now()
where slug = 'good_practices';
