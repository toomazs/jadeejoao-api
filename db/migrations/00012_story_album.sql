-- +goose Up

-- The instax story album: the couple's photo timeline (labels and dates come
-- from the couple's own captions), the letters they wrote each other (from
-- their Instagram), and the section title in their voice.

-- +goose StatementBegin
update sections
set payload = payload || jsonb_build_object(
        'title', 'Vamos te mostrar um pouco da nossa história!',
        'letter_from_groom', $groom$Em um dia comum de trabalho, eu olho pro lado e posso ver Jade assim, existindo sendo tudo que eu jamais imaginei que poderia ter ao meu lado, uma grande parceira, amiga, namorada e mãe da minha filha. Tem noção? Dia comum, com essa visão? Meu maior propósito é tentar fazer ela se sentir tão feliz todos os dias quanto eu me sinto nos meus dias comuns, e que a gente possa compartilhar uma vida cheia de alegria, de amor e de cumplicidade. To escrevendo o texto de aniversário dela enquanto ouço ela cantar no banho com a Cata. Daqui a pouco vamos cantar parabéns com um bolo lindo que ela ganhou, comer hamburger e talvez ver uma série se a neném pegar no sono. É muito fácil ser feliz com a Jade.

Comecei a escrever esse texto em algum desses dias, como um relato da vida que passei a ter desde que comecei a me relacionar com vc. Não sei se tenho um momento preferido de nós dois, talvez seja o de agora, enquanto estou deitado na nossa cama te ouvindo cantar Dorival no chuveiro com a sua voz maravilhosa, e tem grandes chances de ser o de agora a pouco quando vc me entregou o teste de gravidez e me fez perceber só depois de uns dez minutos que eu sou o cara mais sortudo do mundo, ou agora, enquanto vc organiza nossas caixas de mudança, com nossos dois filhos entrando em cada uma das caixas. Mesmo estando exaustos depois de finalmente estarmos nos sentindo definitivamente em casa depois de duas mudanças, talvez seja esse o momento preferido, com vc de barrigão podendo arrumar o quarto da Cata, ou enquanto estávamos a caminho da maternidade, e do nada, choveu por um minuto, como um anúncio da chegada dela. Mas acho que me decidi, meu momento preferido com vc é agora, enquanto te vejo ninando nossa filha, de pé, (e meu deus, como vc é linda, vc é uma mulher absurda) com Francisca e Vitório deitados do meu lado, sem nos importarmos tanto com a chegada do nosso primeiro aniversário de namoro, afinal o que é e o que foi esse ano? Perto da eternidade de amor que vc me fez sentir e do que ainda vamos viver, é pouco, mas pra mim, foi tudo.$groom$,
        'letter_from_bride', $bride$Eu tirei cartas no tarô e João tava lá.
Veio rápido, do jeito que eu sempre quis.
Fizemos nossa arte que carrega nosso nome e nossa cara. Sabemos o que queremos e queremos as mesmas coisas.
João me apresentou minha banda favorita, me levou pra fazer tatuagens, me traz tudo quanto é arte pra apreciar, e tem a façanha de ser mais crítico que eu. E tá aí nosso ponto. Somos críticos demais, com o mundo e as vezes e quase sempre um com o outro. Virginianos. Sempre tentando nos chamar atenção, e sempre nos acolhendo dentro do nosso abraço.
João é nosso cara da casa, só tem mulher aqui e eu acho que ele tem é sorte. Mas vai ter o azar de que possivelmente eu e Catarina teremos o mesmo gênio e sincronizaremos o nosso ciclo, mas ele aguenta! Tem a arte da resiliência e de ver as coisas com um pouco mais de calma. E gosta de cuidar de plantas. Então confio.
Me fez sua par e me deu sua cor, tem dividido força e energia comigo, pra construir nosso paraíso para amar, quando a vida tem sabor.
Feliz 33 anos pro nosso gato que ilumina a nossa vida 🤍$bride$,
        'moments', jsonb_build_array(
            jsonb_build_object('label', 'Jade e Francisca', 'date', '12 de maio de 2020', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/01-jade-e-francisca.jpg'),
            jsonb_build_object('label', 'Jade e João', 'date', '12 de junho de 2020', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/02-jade-e-joao.jpg'),
            jsonb_build_object('label', 'O anúncio da Catarina', 'date', '1º de agosto de 2020', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/03-anuncio-catarina.jpg'),
            jsonb_build_object('label', 'Sobrevivendo uma pandemia', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/04-sobrevivendo-uma-pandemia.jpg'),
            jsonb_build_object('label', 'João, Jade, Francisca e Vitório', 'date', '24 de dezembro de 2020', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/05-natal-2020.jpg'),
            jsonb_build_object('label', 'O fruto', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/06-o-fruto.jpg'),
            jsonb_build_object('label', 'Família linda', 'date', '2021', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/07-familia-linda-2021.jpg'),
            jsonb_build_object('label', 'João e Catarina', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/08-joao-e-catarina.jpg'),
            jsonb_build_object('label', 'Arte dos dois', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/09-arte-dos-dois.jpg'),
            jsonb_build_object('label', 'Catarina tocando o quadro', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/10-catarina-tocando-o-quadro.jpg'),
            jsonb_build_object('label', 'Família linda', 'date', '2022', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/11-familia-linda-2022.jpg'),
            jsonb_build_object('label', 'Os adolescentes da casa', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/12-adolescentes-da-casa.jpg'),
            jsonb_build_object('label', 'Família linda', 'date', '2023', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/13-familia-linda-2023.jpg'),
            jsonb_build_object('label', 'Vendo o pôr do sol', 'date', '2024', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/14-por-do-sol-2024.jpg'),
            jsonb_build_object('label', 'Vitório', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/15-vitorio.jpg'),
            jsonb_build_object('label', 'Família linda', 'date', '2025', 'image_url', 'https://ykvsimxpchaqbsqsqsfs.supabase.co/storage/v1/object/public/jadeejoao-bucket/story/16-familia-linda-2025.jpg')
        )
    ),
    updated_at = now()
where slug = 'our_story';
-- +goose StatementEnd

-- +goose Down

update sections
set payload = (payload - 'moments' - 'letter_from_groom' - 'letter_from_bride')
        || jsonb_build_object('title', 'Nossa História'),
    updated_at = now()
where slug = 'our_story';
