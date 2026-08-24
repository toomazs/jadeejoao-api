-- +goose Up

-- Jade's letter, as she broke it into paragraphs — plus the closing lines
-- about sharing his days.

-- +goose StatementBegin
update sections
set payload = jsonb_set(
        payload,
        '{letter_from_bride}',
        to_jsonb($bride$Eu tirei cartas no tarô e João tava lá. Veio rápido, do jeito que eu sempre quis. Fizemos nossa arte que carrega nosso nome e nossa cara. Sabemos o que queremos e queremos as mesmas coisas. João me apresentou minha banda favorita, me levou pra fazer tatuagens, me traz tudo quanto é arte pra apreciar, e tem a façanha de ser mais crítico que eu.

E tá aí nosso ponto. Somos críticos demais, com o mundo e as vezes e quase sempre um com o outro. Virginianos. Sempre tentando nos chamar atenção, e sempre nos acolhendo dentro do nosso abraço. João é nosso cara da casa, só tem mulher aqui e eu acho que ele tem é sorte. Mas vai ter o azar de que possivelmente eu e Catarina teremos o mesmo gênio e sincronizaremos o nosso ciclo, mas ele aguenta! Tem a arte da resiliência e de ver as coisas com um pouco mais de calma. E gosta de cuidar de plantas. Então confio. Me fez sua par e me deu sua cor, tem dividido força e energia comigo, pra construir nosso paraíso para amar, quando a vida tem sabor.

Amo participar dos seus dias, comer suas comidinhas incríveis, seu jeito carinhoso de cuidar das coisas e da nossa vida.$bride$::text)
    ),
    updated_at = now()
where slug = 'our_story';
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
update sections
set payload = jsonb_set(
        payload,
        '{letter_from_bride}',
        to_jsonb($bride$Eu tirei cartas no tarô e João tava lá. Veio rápido, do jeito que eu sempre quis. Fizemos nossa arte que carrega nosso nome e nossa cara. Sabemos o que queremos e queremos as mesmas coisas. João me apresentou minha banda favorita, me levou pra fazer tatuagens, me traz tudo quanto é arte pra apreciar, e tem a façanha de ser mais crítico que eu. E tá aí nosso ponto. Somos críticos demais, com o mundo e as vezes e quase sempre um com o outro. Virginianos. Sempre tentando nos chamar atenção, e sempre nos acolhendo dentro do nosso abraço. João é nosso cara da casa, só tem mulher aqui e eu acho que ele tem é sorte. Mas vai ter o azar de que possivelmente eu e Catarina teremos o mesmo gênio e sincronizaremos o nosso ciclo, mas ele aguenta! Tem a arte da resiliência e de ver as coisas com um pouco mais de calma. E gosta de cuidar de plantas. Então confio. Me fez sua par e me deu sua cor, tem dividido força e energia comigo, pra construir nosso paraíso para amar, quando a vida tem sabor.$bride$::text)
    ),
    updated_at = now()
where slug = 'our_story';
-- +goose StatementEnd
