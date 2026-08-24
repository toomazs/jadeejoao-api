-- +goose Up

-- The couple's external gift lists. Placeholder URLs point at each store's
-- home page until the couple sends their real list links — the admin panel
-- edits these without a migration.

insert into gifts (title, description, kind, external_url, platform, image_url, sort, active)
values
    (
        'Lista na Camicado',
        'Casa, mesa e banho — o enxoval que a Jade escolheu loja a loja.',
        'link',
        'https://www.camicado.com.br/',
        'Camicado',
        'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/lojas/camicado.png',
        100,
        true
    ),
    (
        'Lista no Mercado Livre',
        'De eletroportáteis ao que faltava na cozinha, com entrega em Atibaia.',
        'link',
        'https://www.mercadolivre.com.br/',
        'Mercado Livre',
        'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/lojas/mercado-livre.svg',
        101,
        true
    );

-- +goose Down

delete from gifts where kind = 'link' and platform in ('Camicado', 'Mercado Livre');
