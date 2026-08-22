-- +goose Up

-- Hero milestone triptych (desktop hero redesign): three story arches beside
-- the couple's names. Labels are neutral placeholders the couple edits in the
-- admin; only the wedding date is asserted — the other dates and all photos
-- stay empty until they fill them in.

update sections
set payload = payload || jsonb_build_object(
        'milestones', jsonb_build_array(
            jsonb_build_object('label', 'Nosso começo'),
            jsonb_build_object('label', 'O pedido'),
            jsonb_build_object('label', 'O grande dia', 'date', '2027-08-07')
        )
    ),
    updated_at = now()
where slug = 'hero'
  and not payload ? 'milestones';

-- +goose Down

update sections
set payload = payload - 'milestones',
    updated_at = now()
where slug = 'hero';
