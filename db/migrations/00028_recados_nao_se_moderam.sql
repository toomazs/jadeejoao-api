-- +goose Up

-- Messages lose their moderation status.
--
-- The guestbook is write-only in public: guests leave a message and the site
-- never shows one back (AD-14). So approving or rejecting decided what appears
-- nowhere — the couple was being asked to judge messages written for them, in
-- order to change nothing. Five had arrived and none had ever been touched.
--
-- What remains is what was always the point: they read them.

alter table messages drop column status;

-- +goose Down

-- Everything comes back as pending. Which were approved is not recoverable and
-- was never observable either.
alter table messages add column status text not null default 'pending'
  check (status in ('pending', 'approved', 'rejected'));
