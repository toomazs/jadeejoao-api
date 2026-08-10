-- +goose Up

-- Upper bounds on money columns (R$ 100.000.000,00 in centavos): without a
-- ceiling, two absurd public contributions could overflow the bigint SUM that
-- computes gift progress and 500 the whole gifts surface. The API enforces
-- the same maximum at the schema layer; this is defense-in-depth.

alter table contributions
    add constraint contributions_amount_max check (amount_centavos <= 10000000000);

alter table gifts
    add constraint gifts_goal_max check (goal_centavos is null or goal_centavos <= 10000000000);

alter table gifts
    add constraint gifts_quota_max check (quota_centavos is null or quota_centavos <= 10000000000);

-- +goose Down

alter table contributions drop constraint contributions_amount_max;
alter table gifts drop constraint gifts_goal_max;
alter table gifts drop constraint gifts_quota_max;
