-- +goose Up

-- Real copy from the couple's support material (brand PDF + planning PPTX,
-- distilled in planning-artifacts): the official welcome sentence, the full
-- ceremony programme, and real-fact milestone labels (their daughter, their
-- home, the day). Replaces the earlier neutral placeholders.

update sections
set payload = payload || jsonb_build_object(
        'body', 'Estamos muito felizes em compartilhar mais este momento de nossas vidas com vocês!',
        'milestones', jsonb_build_array(
            jsonb_build_object('label', 'Nossa família'),
            jsonb_build_object('label', 'Nossa casa em Atibaia'),
            jsonb_build_object('label', 'O grande dia', 'date', '2027-08-07')
        )
    ),
    updated_at = now()
where slug = 'hero';

update sections
set payload = payload || jsonb_build_object(
        'programme', jsonb_build_array(
            jsonb_build_object('time', '15:00', 'label', 'Recepção dos convidados'),
            jsonb_build_object('time', '16:00', 'label', 'Entrada do noivo'),
            jsonb_build_object('time', '16:10', 'label', 'Padrinhos e madrinhas'),
            jsonb_build_object('time', '16:20', 'label', 'Entrada de Linda e Marcos'),
            jsonb_build_object('time', '16:30', 'label', 'Floristas e entrada da noiva'),
            jsonb_build_object('time', '16:35', 'label', 'Início da cerimônia'),
            jsonb_build_object('time', '16:50', 'label', 'Texto da Kyhsa'),
            jsonb_build_object('time', '17:00', 'label', 'Votos dos noivos'),
            jsonb_build_object('time', '17:05', 'label', 'Troca de alianças'),
            jsonb_build_object('time', '17:10', 'label', 'Dama de honra'),
            jsonb_build_object('time', '17:30', 'label', 'Encerramento e começo da festa')
        )
    ),
    updated_at = now()
where slug = 'big_day';

-- +goose Down

update sections
set payload = payload || jsonb_build_object(
        'body', 'Vamos nos casar! E queremos você com a gente nesse dia tão especial.',
        'milestones', jsonb_build_array(
            jsonb_build_object('label', 'Nosso começo'),
            jsonb_build_object('label', 'O pedido'),
            jsonb_build_object('label', 'O grande dia', 'date', '2027-08-07')
        )
    ),
    updated_at = now()
where slug = 'hero';

update sections
set payload = payload || jsonb_build_object(
        'programme', jsonb_build_array(
            jsonb_build_object('time', '15:00', 'label', 'Recepção dos convidados'),
            jsonb_build_object('time', '16:00', 'label', 'Entrada do noivo'),
            jsonb_build_object('time', '16:10', 'label', 'Padrinhos e madrinhas'),
            jsonb_build_object('time', '16:30', 'label', 'Entrada da noiva'),
            jsonb_build_object('time', '16:35', 'label', 'Início da cerimônia'),
            jsonb_build_object('time', '17:30', 'label', 'Encerramento e começo da festa')
        )
    ),
    updated_at = now()
where slug = 'big_day';
