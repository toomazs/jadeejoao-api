-- +goose Up

-- João's bio, in his own words — the counterpart to Jade's in 00014, so the
-- two chapters of the film finally carry the same weight.

-- +goose StatementBegin
update sections
set payload = jsonb_set(
        payload,
        '{groom,bio}',
        to_jsonb($bio$Sou um homem de 33 anos que ama cuidar da casa e das plantas. Nos projetos que construo no meu dia a dia, seja no cultivo da terra ou nas melhorias da casa, me realizo vendo a vida florescer.

Aprendi a aproveitar os pequenos rituais, com um bom café, um vinho ou uma receita improvisada. Compartilhar filmes, séries, músicas e jogos com as minhas meninas faz dos meus dias os melhores dias.

Amo a rotina, a estabilidade e a tranquilidade de morar no interior. E, como se a vida já não tivesse sido generosa demais comigo, vou casar com a minha parceira, que construiu tudo isso junto comigo. Parece um sonho.$bio$::text)
    ),
    updated_at = now()
where slug = 'our_story';
-- +goose StatementEnd

-- +goose Down

update sections
set payload = (payload #- '{groom,bio}'),
    updated_at = now()
where slug = 'our_story';
