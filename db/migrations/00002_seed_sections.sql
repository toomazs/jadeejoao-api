-- +goose Up

-- Seeds the closed slug set with real PT-BR defaults the couple can edit in
-- the admin. Payload shapes mirror the typed Go structs in internal/content.

insert into sections (slug, payload) values
('hero', $seed$
{
  "title": "Jade & João",
  "body": "Vamos nos casar! E queremos você com a gente nesse dia tão especial.",
  "images": [],
  "couple_names": "Jade & João",
  "event_datetime": "2027-08-07T15:00:00-03:00",
  "city_label": "Atibaia – SP"
}
$seed$::jsonb),
('our_story', $seed$
{
  "title": "Nossa História",
  "body": "Entre risadas, viagens e muitos cafés, construímos uma história que agora ganha o seu capítulo mais bonito. Em breve contamos tudo por aqui — com fotos e os melhores momentos.",
  "images": []
}
$seed$::jsonb),
('big_day', $seed$
{
  "title": "O Grande Dia",
  "body": "A cerimônia e a festa acontecem na casa dos noivos, em Atibaia. O espaço é ao ar livre, com grama — escolha um calçado confortável!",
  "images": [],
  "venue_notes": "Espaço externo com grama. Evite salto fino e prefira calçados confortáveis.",
  "programme": [
    {"time": "15:00", "label": "Recepção dos convidados"},
    {"time": "16:00", "label": "Entrada do noivo"},
    {"time": "16:10", "label": "Padrinhos e madrinhas"},
    {"time": "16:30", "label": "Entrada da noiva"},
    {"time": "16:35", "label": "Início da cerimônia"},
    {"time": "17:30", "label": "Encerramento e começo da festa"}
  ]
}
$seed$::jsonb),
('rsvp', $seed$
{
  "title": "Confirmação de Presença",
  "body": "Digite seu nome completo para encontrar seu convite e confirmar a presença de cada pessoa do seu grupo. Se não encontrar seu nome, fale com os noivos.",
  "images": [],
  "deadline": "2027-07-07"
}
$seed$::jsonb),
('getting_there', $seed$
{
  "title": "Como Chegar",
  "body": "A festa será na casa dos noivos, no Jardim Paulista, em Atibaia – SP.",
  "images": [],
  "address": "Rua Piraju, 306 – Jardim Paulista, Atibaia – SP, 12947-321",
  "map_embed_url": "https://www.google.com/maps?q=Rua+Piraju,+306,+Jardim+Paulista,+Atibaia+-+SP&output=embed",
  "parking_notes": "Não há estacionamento próximo ao local. Sugerimos dormir na cidade e usar Uber ou o translado que vamos oferecer."
}
$seed$::jsonb),
('stay', $seed$
{
  "title": "Onde Ficar",
  "body": "Separamos sugestões de hotéis e pousadas em Atibaia. Haverá van/translado entre as hospedagens sugeridas e o local da festa.",
  "images": [],
  "lodgings": [],
  "airbnb_areas": ["Jardim Paulista", "Centro"]
}
$seed$::jsonb),
('gifts_intro', $seed$
{
  "title": "Lista de Presentes",
  "body": "O maior presente é ter você com a gente! Mas, se quiser nos mimar, preparamos uma lista com metas e cotas — cada contribuição vira um pedacinho do nosso novo capítulo. O pagamento é por PIX, direto para os noivos.",
  "images": []
}
$seed$::jsonb),
('dress_code', $seed$
{
  "title": "Dress Code",
  "body": "Traje esporte fino: elegância com conforto. Lembre que a festa é ao ar livre, em grama — escolha calçados que combinem com o gramado. Vista-se bonito e sinta-se à vontade para dançar!",
  "images": [],
  "attire": "Sofisticado, confortável, vestido longo, esporte fino."
}
$seed$::jsonb),
('good_practices', $seed$
{
  "title": "Para Aproveitar Nosso Dia",
  "body": "Algumas combinações carinhosas para o nosso dia sair perfeito:",
  "images": [],
  "rules": [
    "Deixe as opiniões polêmicas em casa — dia de casamento não é dia de discutir",
    "O open bar é generoso, mas não vire decoração no chão",
    "Se beber, não dirija: haverá translado para os hotéis sugeridos",
    "Aproveite muito, dance e celebre com a gente!"
  ]
}
$seed$::jsonb),
('messages_intro', $seed$
{
  "title": "Recado aos Noivos",
  "body": "Deixe aqui seu carinho, um conselho ou aquela história que só você sabe. Os noivos vão ler cada recado com o coração quentinho.",
  "images": []
}
$seed$::jsonb);

-- +goose Down

delete from sections where slug in (
  'hero', 'our_story', 'big_day', 'rsvp', 'getting_there',
  'stay', 'gifts_intro', 'dress_code', 'good_practices', 'messages_intro'
);
