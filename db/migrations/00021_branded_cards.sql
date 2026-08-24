-- +goose Up

-- Navigation and search platforms get their own brand marks, so each one
-- reads as a card of its own rather than another button in a row. Also drops
-- the Airbnb neighbourhood chips (the platform card covers that ground) and
-- the big-day body that only repeated the venue note.

update sections
set payload = payload || jsonb_build_object(
        'maps_logo_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/apps/google-maps.svg',
        'waze_logo_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/apps/waze.png'
    ),
    updated_at = now()
where slug = 'getting_there';

-- +goose StatementBegin
update sections
set payload = jsonb_build_object(
        'airbnb_areas', '[]'::jsonb,
        'lodgings', jsonb_build_array(
            jsonb_build_object(
                'name', 'Faro Hotel Atibaia',
                'area', 'Centro',
                'link', 'https://www.farohotelatibaia.com/',
                'notes', 'No centro, com piscinas, academia e café da manhã incluso.',
                'shuttle_served', true
            ),
            jsonb_build_object(
                'name', 'Itapetinga Hotel',
                'area', 'A 800 m do centro',
                'link', 'https://www.itapetingahotel.com.br/',
                'notes', 'Piscina ao ar livre e café da manhã com vista para as montanhas.',
                'shuttle_served', true
            ),
            jsonb_build_object(
                'name', 'Pousada Vista da Pedra',
                'area', 'Centro',
                'link', 'https://www.booking.com/searchresults.pt-br.html?ss=Pousada+Vista+da+Pedra+Atibaia',
                'notes', 'Opção simples e bem avaliada, a poucos minutos do centro.',
                'shuttle_served', true
            ),
            jsonb_build_object(
                'name', 'Booking',
                'link', 'https://www.booking.com/city/br/atibaia.pt-br.html',
                'notes', 'Todos os hotéis da cidade, com preços.',
                'logo_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/apps/booking.svg',
                'platform', true,
                'shuttle_served', false
            ),
            jsonb_build_object(
                'name', 'Airbnb',
                'link', 'https://www.airbnb.com.br/s/Atibaia--SP/homes',
                'notes', 'Casas inteiras, boas para famílias e grupos.',
                'logo_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/apps/airbnb.svg',
                'platform', true,
                'shuttle_served', false
            )
        )
    ) || (payload - 'lodgings' - 'airbnb_areas'),
    updated_at = now()
where slug = 'stay';
-- +goose StatementEnd

update sections
set payload = payload - 'body',
    updated_at = now()
where slug = 'big_day';

update sections
set payload = payload || jsonb_build_object(
        'images', jsonb_build_array('https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/dresscode-referencia.jpg')
    ),
    updated_at = now()
where slug = 'dress_code';

-- +goose Down

update sections
set payload = payload - 'maps_logo_url' - 'waze_logo_url',
    updated_at = now()
where slug = 'getting_there';

update sections
set payload = payload || jsonb_build_object('images', '[]'::jsonb),
    updated_at = now()
where slug = 'dress_code';
