-- +goose Up

-- Two reference looks for the dress code — one for her, one for him — each
-- with its own photograph. The couple can rewrite, reorder or add looks in
-- the admin without a deploy.

-- +goose StatementBegin
update sections
set payload = jsonb_build_object(
        'looks', jsonb_build_array(
            jsonb_build_object(
                'title', 'Para elas',
                'body', 'Vestido longo ou midi, tecidos fluidos e cores que combinem com o campo. A festa é ao ar livre, em grama — prefira salto bloco, anabela ou uma sapatilha bonita ao salto fino.',
                'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/dresscode-referencia.jpg'
            ),
            jsonb_build_object(
                'title', 'Para eles',
                'body', 'Camisa social, calça de alfaiataria e sapato confortável. Terno é bem-vindo, mas não obrigatório: a ideia é estar elegante sem passar calor no fim da tarde.',
                'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/dresscode-eles.jpg'
            )
        )
    ) || (payload - 'looks' - 'images' - 'body'),
    updated_at = now()
where slug = 'dress_code';
-- +goose StatementEnd

-- +goose Down

update sections
set payload = (payload - 'looks') || jsonb_build_object(
        'body', 'Traje esporte fino: elegância com conforto. Lembre que a festa é ao ar livre, em grama — escolha calçados que combinem com o gramado. Vista-se bonito e sinta-se à vontade para dançar!'
    ),
    updated_at = now()
where slug = 'dress_code';
