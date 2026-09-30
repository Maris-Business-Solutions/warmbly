-- The Reply-To a send carried, so a reply that lands in the mailbox it named
-- is credited to the campaign even though that mailbox never sent to the
-- contact. Empty for no header, like thread_id.
ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS reply_to text NOT NULL DEFAULT '';
