-- +goose Up

-- The hero payload carried four fields the site never drew.
--
-- `title` and `body` came from SectionBase, which every other section uses for
-- a heading and a paragraph — but the hero has neither: the names are the
-- brand wordmark and the rest is the photograph. The sentence sitting in
-- `body` ("Estamos muito felizes em compartilhar mais este momento de nossas
-- vidas com vocês!") had never appeared anywhere.
--
-- `milestones` is worse than unused: it held three captions ("Nossa família",
-- "Nossa casa em Atibaia", "O grande dia") from an earlier hero design. The
-- current one reads the list only to borrow a photo when `hero_image_url` is
-- empty, and those entries carry no photo — so nothing of them reached a
-- screen. `images` was empty throughout.
--
-- Fields nobody can see are worse than missing ones: the admin panel offers
-- them for editing, and the couple spends care on text that goes nowhere.
update sections
set payload = payload - 'title' - 'body' - 'images' - 'milestones',
    updated_at = now()
where slug = 'hero';

-- +goose Down

-- The captions are restored as they were; the sentence is not, because it was
-- never displayed and would only reappear as a puzzle in the panel.
update sections
set payload = payload || jsonb_build_object(
        'title', 'Jade & João',
        'milestones', jsonb_build_array(
            jsonb_build_object('label', 'Nossa família'),
            jsonb_build_object('label', 'Nossa casa em Atibaia'),
            jsonb_build_object('label', 'O grande dia')
        )
    ),
    updated_at = now()
where slug = 'hero';
