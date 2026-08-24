-- +goose Up

-- Catarina's announcement, the scene that closes the couple's story and hands
-- the page over to the day itself. The wink coordinates are data: a new photo
-- needs new numbers here, never a code change.

update sections
set payload = payload || jsonb_build_object(
        'announcement', jsonb_build_object(
            'label', 'Mamãe e papai vão se casar!',
            'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/catarina-anuncio.jpg',
            'eye_x', 60.6,
            'eye_y', 16.9
        )
    ),
    updated_at = now()
where slug = 'our_story';

-- +goose Down

update sections
set payload = payload - 'announcement',
    updated_at = now()
where slug = 'our_story';
