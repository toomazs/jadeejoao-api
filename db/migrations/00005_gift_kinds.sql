-- +goose Up

-- Dual-mode gifts (user decision 2026-08-10): the admin picks, per gift,
-- between PIX metas/cotas (kind = 'pix', unchanged behavior) and an external
-- store registry card (kind = 'link': platform + URL, the SPAs redirect with
-- target=_blank and no money ever flows through the API).

alter table gifts
    add column kind text not null default 'pix' check (kind in ('pix', 'link'));

alter table gifts
    add column platform text;

alter table gifts
    add column external_url text;

-- Shape guard: link gifts carry a URL and never money fields; pix gifts
-- never carry link fields. Mirrored in the service layer with PT-BR errors.
alter table gifts
    add constraint gifts_kind_shape check (
        (kind = 'link'
            and external_url is not null
            and goal_centavos is null
            and quota_centavos is null
            and max_units is null)
        or
        (kind = 'pix'
            and external_url is null
            and platform is null)
    );

-- +goose Down

alter table gifts drop constraint gifts_kind_shape;
alter table gifts drop column external_url;
alter table gifts drop column platform;
alter table gifts drop column kind;
