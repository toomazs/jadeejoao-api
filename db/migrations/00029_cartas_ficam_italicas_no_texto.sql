-- +goose Up

-- The letters carry their own italics now.
--
-- They were slanted by the stylesheet, which meant the panel could offer
-- italic on that field and changing it did nothing — and the couple could
-- never write one plain sentence inside a leaning letter. Taking the slant out
-- of the CSS without putting it in the text would have unslanted their
-- letters, so it moves here: every line wrapped, which is what the panel now
-- shows them and what they can undo a word at a time.
--
-- Line by line rather than the whole thing at once: the site splits paragraphs
-- on the blank line between them and renders the inline marks within each, so
-- a single pair of asterisks around everything would leave the marks showing.

update sections
set payload = jsonb_set(
      jsonb_set(
        payload,
        '{letter_from_bride}',
        to_jsonb(regexp_replace(payload->>'letter_from_bride', '([^\n]+)', '*\1*', 'g'))
      ),
      '{letter_from_groom}',
      to_jsonb(regexp_replace(payload->>'letter_from_groom', '([^\n]+)', '*\1*', 'g'))
    ),
    updated_at = now()
where slug = 'our_story'
  -- Never twice. Running this again would give the couple **bold** letters.
  and payload->>'letter_from_bride' not like '%*%'
  and payload->>'letter_from_groom' not like '%*%';

-- +goose Down

update sections
set payload = jsonb_set(
      jsonb_set(
        payload,
        '{letter_from_bride}',
        to_jsonb(regexp_replace(payload->>'letter_from_bride', '^\*(.*)\*$', '\1', 'gm'))
      ),
      '{letter_from_groom}',
      to_jsonb(regexp_replace(payload->>'letter_from_groom', '^\*(.*)\*$', '\1', 'gm'))
    ),
    updated_at = now()
where slug = 'our_story';
