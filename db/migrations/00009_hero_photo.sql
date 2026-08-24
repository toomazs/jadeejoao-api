-- +goose Up

-- The couple's chosen hero photograph, uploaded to the public storage bucket
-- (hero/jade-e-joao.jpg). The URL is data, not config: the admin panel will
-- overwrite it through the content endpoints when they swap the photo.

update sections
set payload = payload || jsonb_build_object(
        'hero_image_url',
        'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/hero/jade-e-joao.jpg'
    ),
    updated_at = now()
where slug = 'hero';

-- +goose Down

update sections
set payload = payload - 'hero_image_url',
    updated_at = now()
where slug = 'hero';
