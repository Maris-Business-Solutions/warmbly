-- Whether Archive, Delete and Move to inbox in the unibox also move the message
-- in the mailbox itself. On by default: every move it makes can be undone there.
ALTER TABLE email_accounts
    ADD COLUMN IF NOT EXISTS relay_folder_moves boolean NOT NULL DEFAULT true;
