-- +goose Up

-- Two navigation deep links (Google Maps and Waze) pointed at the house, and
-- real places to sleep in Atibaia — researched from public listings, with the
-- couple's own shuttle note. All editable in the admin.

update sections
set payload = payload || jsonb_build_object(
        'maps_url', 'https://www.google.com/maps/search/?api=1&query=Rua+Piraju%2C+306+-+Jardim+Paulista%2C+Atibaia+-+SP%2C+12947-321',
        'waze_url', 'https://www.waze.com/ul?q=Rua%20Piraju%2C%20306%20-%20Jardim%20Paulista%2C%20Atibaia%20-%20SP&navigate=yes'
    ),
    updated_at = now()
where slug = 'getting_there';

-- +goose StatementBegin
update sections
set payload = payload || jsonb_build_object(
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
                'name', 'Mais opções no Booking',
                'area', 'Atibaia',
                'link', 'https://www.booking.com/city/br/atibaia.pt-br.html',
                'notes', 'Compare preços e disponibilidade de todos os hotéis da cidade.',
                'shuttle_served', false
            ),
            jsonb_build_object(
                'name', 'Casas no Airbnb',
                'area', 'Atibaia',
                'link', 'https://www.airbnb.com.br/s/Atibaia--SP/homes',
                'notes', 'Boa escolha para famílias e grupos que preferem uma casa inteira.',
                'shuttle_served', false
            )
        )
    ),
    updated_at = now()
where slug = 'stay';
-- +goose StatementEnd

-- +goose Down

update sections
set payload = payload - 'maps_url' - 'waze_url',
    updated_at = now()
where slug = 'getting_there';

update sections
set payload = jsonb_set(payload, '{lodgings}', '[]'::jsonb),
    updated_at = now()
where slug = 'stay';
