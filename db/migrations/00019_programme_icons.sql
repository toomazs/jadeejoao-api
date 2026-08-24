-- +goose Up

-- The programme's emblems become data: the couple picks which moments anchor
-- the day, and the site draws the icon. Also drops the venue note that only
-- repeated what the section's own body already says, and gives Catarina's
-- line its emphasis.

-- +goose StatementBegin
update sections
set payload = jsonb_build_object(
        'programme', jsonb_build_array(
            jsonb_build_object('time', '15:00', 'label', 'Recepção dos convidados', 'icon', 'guests'),
            jsonb_build_object('time', '16:00', 'label', 'Entrada do noivo'),
            jsonb_build_object('time', '16:10', 'label', 'Padrinhos e madrinhas'),
            jsonb_build_object('time', '16:20', 'label', 'Entrada de Linda e Marcos'),
            jsonb_build_object('time', '16:30', 'label', 'Floristas e entrada da noiva', 'icon', 'flowers'),
            jsonb_build_object('time', '16:35', 'label', 'Início da cerimônia', 'icon', 'ceremony'),
            jsonb_build_object('time', '16:50', 'label', 'Texto da Kyhsa'),
            jsonb_build_object('time', '17:00', 'label', 'Votos dos noivos', 'icon', 'vows'),
            jsonb_build_object('time', '17:05', 'label', 'Troca de alianças', 'icon', 'rings'),
            jsonb_build_object('time', '17:10', 'label', 'Dama de honra'),
            jsonb_build_object('time', '17:30', 'label', 'Encerramento e começo da festa', 'icon', 'party')
        )
    ) || (payload - 'programme' - 'venue_notes'),
    updated_at = now()
where slug = 'big_day';
-- +goose StatementEnd

update sections
set payload = jsonb_set(
        payload,
        '{announcement,label}',
        to_jsonb('Mamãe e papai **vão se casar!**'::text)
    ),
    updated_at = now()
where slug = 'our_story' and payload ? 'announcement';

-- +goose Down

update sections
set payload = payload || jsonb_build_object(
        'venue_notes', 'Espaço externo com grama. Evite salto fino e prefira calçados confortáveis.'
    ),
    updated_at = now()
where slug = 'big_day';
