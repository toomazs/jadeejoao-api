-- +goose Up

-- The story-film chapters: one per person, with the photos the couple chose
-- (uploaded to the public bucket) and their real Instagram handles. Bios are
-- intentionally absent until the couple writes them (admin panel).

update sections
set payload = payload || jsonb_build_object(
        'bride', jsonb_build_object(
            'name', 'Jade',
            'photo_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/couple/jade.jpg',
            'instagram', 'xadenascimento'
        ),
        'groom', jsonb_build_object(
            'name', 'João',
            'photo_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/couple/joao.jpg',
            'instagram', 'joaodiaspedro'
        )
    ),
    updated_at = now()
where slug = 'our_story';

-- +goose Down

update sections
set payload = payload - 'bride' - 'groom',
    updated_at = now()
where slug = 'our_story';
