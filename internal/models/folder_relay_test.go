package models

import "testing"

func TestRelaysFolderMove(t *testing.T) {
	cases := []struct {
		name               string
		provider, from, to string
		moved, want        bool
	}{
		{"archive an inbox message", "gmail", FolderInbox, FolderArchive, true, true},
		{"provider already agrees", "outlook", FolderArchive, FolderArchive, false, false},
		{"undo before the archive's answer", "outlook", FolderInbox, FolderInbox, true, true},
		{"sent copy stays in IMAP Sent", "smtp_imap", FolderSent, FolderTrash, true, false},
		{"sent copy stays in Outlook Sent Items", "outlook", FolderSent, FolderArchive, true, false},
		{"Gmail trashes the whole conversation", "gmail", FolderSent, FolderTrash, true, true},
		{"archiving a Gmail sent copy is a no-op", "gmail", FolderSent, FolderArchive, true, false},
		{"drafts never move", "gmail", FolderDrafts, FolderTrash, true, false},
		{"out of spam", "smtp_imap", FolderSpam, FolderInbox, true, true},
		{"spam is not a filing", "gmail", FolderInbox, FolderSpam, true, false},
		{"row from before provider_folder", "smtp_imap", "", FolderArchive, true, true},
	}
	for _, c := range cases {
		if got := RelaysFolderMove(c.provider, c.from, c.to, c.moved); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
