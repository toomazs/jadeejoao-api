-- +goose Up

-- Example gifts showing the three pricing shapes (free amount, quota with
-- limited units, quota unlimited). Real gifts are managed in the admin.

insert into gifts (title, description, goal_centavos, quota_centavos, max_units, sort) values
(
  'Para o João fazer a barba',
  'A barba do noivo precisa estar impecável no grande dia. Contribua com qualquer valor e garanta que a Jade reconheça o rapaz no altar.',
  20000, null, null, 10
),
(
  'Jantar romântico na lua de mel',
  'Quatro cotas de um jantar à luz de velas. Cada cota paga um prato chique com nome francês que a gente não sabe pronunciar.',
  60000, 15000, 4, 20
),
(
  'Adega dos noivos',
  'Cotas de R$ 50 para encher nossa adega. Prometemos brindar por você a cada garrafa aberta.',
  100000, 5000, null, 30
);

-- +goose Down

delete from gifts where title in (
  'Para o João fazer a barba',
  'Jantar romântico na lua de mel',
  'Adega dos noivos'
);
