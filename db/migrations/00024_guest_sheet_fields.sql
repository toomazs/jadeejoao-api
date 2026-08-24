-- +goose Up

-- The couple's spreadsheet carries more about each guest than the schema did:
-- which of them invited the person, gender, the circle they belong to, the
-- part they play in the ceremony, and a free note (mostly kinship: "Marido
-- Renata Gonçalves", "Filho Francisca de Freitas").
--
-- None of it is shown to guests. It exists so the couple can organise their
-- panel and so an import round-trips their sheet instead of dropping columns.
alter table guests
    add column gender        text check (gender in ('female', 'male')),
    add column side          text check (side in ('bride', 'groom', 'both')),
    add column circle        text not null default '',
    add column ceremony_role text not null default '',
    add column notes         text not null default '',
    -- Companions the guests add themselves through the site, so the couple can
    -- tell them apart from the names they typed into the spreadsheet.
    add column added_by_guest boolean not null default false;

-- "Adolescente" is a real line in the sheet and fits none of the four existing
-- buckets: a teenager eats an adult meal but is not counted as an adult.
alter table guests drop constraint guests_category_check;
alter table guests add constraint guests_category_check
    check (category in ('adult', 'teen', 'child', 'baby', 'elderly'));

-- +goose Down

alter table guests drop constraint guests_category_check;
update guests set category = 'adult' where category = 'teen';
alter table guests add constraint guests_category_check
    check (category in ('adult', 'child', 'baby', 'elderly'));

alter table guests
    drop column gender,
    drop column side,
    drop column circle,
    drop column ceremony_role,
    drop column notes,
    drop column added_by_guest;
