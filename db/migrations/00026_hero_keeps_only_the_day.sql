-- +goose Up

-- The hero drops `couple_names`, and `event_datetime` becomes `event_date`.
--
-- Neither field did what its name promised in the panel. The names at the top
-- of the site are the brand wordmark, an image — the text only reached the
-- footer signature, so editing it looked like nothing happened. And the
-- ceremony time is printed nowhere: it existed solely to aim the countdown,
-- which made it a field whose only possible use was to break something.
--
-- The hour moves into the code (content.CeremonyHour), where it is not a
-- question anyone has to answer. The names move into the footer, likewise.
update sections
set payload = (payload - 'couple_names' - 'event_datetime')
              || jsonb_build_object('event_date', left(payload->>'event_datetime', 10)),
    updated_at = now()
where slug = 'hero';

-- +goose Down

update sections
set payload = (payload - 'event_date') || jsonb_build_object(
        'couple_names', 'Jade & João',
        'event_datetime', (payload->>'event_date') || 'T15:00:00-03:00'
    ),
    updated_at = now()
where slug = 'hero';
