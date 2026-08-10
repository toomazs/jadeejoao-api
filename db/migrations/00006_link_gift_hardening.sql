-- +goose Up

-- Review hardening for dual-mode gifts and the typeahead:
-- 1. The https-only and platform-required rules for link gifts existed only
--    in Go; mirror them as CHECKs so out-of-band writes cannot store cards
--    with junk or missing links.
-- 2. A pattern-ops index so the suggest LIKE prefix scan does not depend on
--    the database collation.

alter table gifts
    add constraint gifts_link_url_https check (
        kind <> 'link' or external_url like 'https://%'
    );

alter table gifts
    add constraint gifts_link_platform check (
        kind <> 'link' or (platform is not null and platform <> '')
    );

create index guests_normalized_name_pattern_idx
    on guests (normalized_name text_pattern_ops);

-- +goose Down

drop index guests_normalized_name_pattern_idx;
alter table gifts drop constraint gifts_link_platform;
alter table gifts drop constraint gifts_link_url_https;
