-- +goose Up

-- The album opens with the couple's first photo together (the one they posted),
-- then the timeline follows. Only the order of the first two frames changes.

-- +goose StatementBegin
update sections
set payload = jsonb_set(
        payload,
        '{moments}',
        jsonb_build_array(
            payload -> 'moments' -> 1,
            payload -> 'moments' -> 0
        ) || (
            select coalesce(jsonb_agg(moment order by ordinality), '[]'::jsonb)
            from jsonb_array_elements(payload -> 'moments') with ordinality as t(moment, ordinality)
            where ordinality > 2
        )
    ),
    updated_at = now()
where slug = 'our_story'
  and jsonb_array_length(payload -> 'moments') >= 2;
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
update sections
set payload = jsonb_set(
        payload,
        '{moments}',
        jsonb_build_array(
            payload -> 'moments' -> 1,
            payload -> 'moments' -> 0
        ) || (
            select coalesce(jsonb_agg(moment order by ordinality), '[]'::jsonb)
            from jsonb_array_elements(payload -> 'moments') with ordinality as t(moment, ordinality)
            where ordinality > 2
        )
    ),
    updated_at = now()
where slug = 'our_story'
  and jsonb_array_length(payload -> 'moments') >= 2;
-- +goose StatementEnd
