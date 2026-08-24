-- +goose Up

-- The invitation interlude: our_story's body is now the sentence the film
-- reveals between the couple's chapters and the big day, as dictated by the
-- couple's team.

update sections
set payload = jsonb_set(
        payload,
        '{body}',
        to_jsonb('Estamos muito felizes de te convidar para o nosso casamento!'::text)
    ),
    updated_at = now()
where slug = 'our_story';

-- +goose Down

update sections
set payload = jsonb_set(
        payload,
        '{body}',
        to_jsonb('Entre risadas, viagens e muitos cafés, construímos uma história que agora ganha o seu capítulo mais bonito. Em breve contamos tudo por aqui — com fotos e os melhores momentos.'::text)
    ),
    updated_at = now()
where slug = 'our_story';
