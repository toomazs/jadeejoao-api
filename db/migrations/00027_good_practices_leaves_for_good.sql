-- +goose Up

-- The house-rules section goes, and this time it goes for real.
--
-- 00017 only disabled it, on the theory that one toggle could bring it back.
-- That theory was wrong: the site has no renderer for this slug at all. The
-- App never matched on it, so turning it on would have shown nothing while
-- the panel and the nav both claimed it was there — a switch wired to a lamp
-- that was never installed.
--
-- The copy is kept in the Down, so the decision is reversible even though the
-- section is not coming back.

delete from sections where slug = 'good_practices';

-- +goose Down

insert into sections (slug, enabled, payload)
values ('good_practices', false, $seed$
{
  "title": "Para Aproveitar Nosso Dia",
  "body": "Algumas combinações carinhosas para o nosso dia sair perfeito:",
  "images": [],
  "rules": [
    "Deixe as opiniões polêmicas em casa — dia de casamento não é dia de discutir",
    "O open bar é generoso, mas não vire decoração no chão",
    "Se beber, não dirija: haverá translado para os hotéis sugeridos",
    "Aproveite muito, dance e celebre com a gente!"
  ]
}
$seed$::jsonb)
on conflict (slug) do nothing;
