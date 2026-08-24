-- +goose Up

-- Jade's bio, in her own words, and the couple's final letters (the versions
-- they approved — the groom's letter gained its second half, and the bride's
-- dropped the birthday closing).

-- +goose StatementBegin
update sections
set payload = jsonb_set(
        payload,
        '{bride,bio}',
        to_jsonb($bio$Sou uma mulher de 30 anos que ama criar momentos com música, luzes quentes e velas, para tomar um vinho gostoso e comer alguma coisinha também, enquanto minhas gatas passeiam pelo espaço.

Sou movida a propósito. Vivo sempre pensando no que me faz feliz e no que faz sentido, buscando cuidar da minha mente. Amo escrever — talvez exista uma escritora do futuro — e faço faculdade de Psicologia.

Tenho dois projetos lindos que movimentam meus dias. Gosto de cuidar do meu corpo, de criar projetos e de idealizar coisas. Amo morar no interior e pausar alguns momentos para olhar para o céu, para as montanhas e ouvir os passarinhos ao acordar.

Amo a minha vida, principalmente com o meu presentão, que vai casar comigo.$bio$::text)
    ) || jsonb_build_object(
        'letter_from_groom', $groom$Em um dia comum de trabalho, eu olho pro lado e posso ver a Jade, existindo sendo tudo que eu jamais imaginei que poderia ter ao meu lado, uma grande parceira, amiga, namorada e mãe da minha filha. Tem noção? Dia comum, com essa visão? Meu maior propósito é tentar fazer ela se sentir tão feliz todos os dias quanto eu me sinto nos meus dias comuns, e que a gente possa compartilhar uma vida cheia de alegria, de amor e de cumplicidade. Não existe ninguém como ela, tão esclarecida e matura com essa idade, parece que trouxe conhecimento de outras vidas. A Jade é forte e dedicada, estuda tanto a maternidade porque sempre soube que nada poderia nos antecipar do que seria o desafio de ter Catarina. Ela consegue, ela vence esse desafio diariamente, então eu consigo imaginar no futuro a Catarina olhando pra ela e dizendo com toda a sinceridade e nenhum comodismo que ela é a melhor mãe do mundo, porque essa é a verdade.

Já tivemos momentos muito marcantes na nossa linda história até aqui. Em cada um desses momentos eu percebi em você os traços que as pessoas destacam na sua personalidade quando te descrevem, e é fácil se apaixonar por você observando esses traços, isso sem nem contar o fato de que você é absurdamente linda e estilosa, até quando acorda e põe uma roupa confortável pra trabalhar de casa. É muito fácil ser feliz com a Jade.$groom$,
        'letter_from_bride', $bride$Eu tirei cartas no tarô e João tava lá. Veio rápido, do jeito que eu sempre quis. Fizemos nossa arte que carrega nosso nome e nossa cara. Sabemos o que queremos e queremos as mesmas coisas. João me apresentou minha banda favorita, me levou pra fazer tatuagens, me traz tudo quanto é arte pra apreciar, e tem a façanha de ser mais crítico que eu. E tá aí nosso ponto. Somos críticos demais, com o mundo e as vezes e quase sempre um com o outro. Virginianos. Sempre tentando nos chamar atenção, e sempre nos acolhendo dentro do nosso abraço. João é nosso cara da casa, só tem mulher aqui e eu acho que ele tem é sorte. Mas vai ter o azar de que possivelmente eu e Catarina teremos o mesmo gênio e sincronizaremos o nosso ciclo, mas ele aguenta! Tem a arte da resiliência e de ver as coisas com um pouco mais de calma. E gosta de cuidar de plantas. Então confio. Me fez sua par e me deu sua cor, tem dividido força e energia comigo, pra construir nosso paraíso para amar, quando a vida tem sabor.$bride$
    ),
    updated_at = now()
where slug = 'our_story';
-- +goose StatementEnd

-- +goose Down

update sections
set payload = (payload #- '{bride,bio}'),
    updated_at = now()
where slug = 'our_story';
