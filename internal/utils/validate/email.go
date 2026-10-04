package validate

import (
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/pkg/displayname"
)

func Email(email string) bool {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	return true
}

func EmailBulk(emails []string) bool {
	for i := range emails {
		if !Email(emails[i]) {
			return false
		}
	}
	return true
}

var nameRE = regexp.MustCompile(`^\p{L}[\p{L}\p{M}\p{N}'\-\.\s]{0,98}\p{L}$`)

func EmailName(name *string) bool {
	*name = strings.TrimSpace(*name)
	if *name == "" {
		return false
	}
	runes := []rune(*name)
	if len(runes) < 2 {
		return false
	}
	if len(runes) > 100 {
		return false
	}
	if !nameRE.MatchString(*name) {
		return false
	}
	return true
}

// MailboxNameShown refuses a mailbox display name under the person naming
// rules, since warmup shows it in other workspaces' inboxes. Length stays
// with the caller's own bound.
func MailboxNameShown(name string) *errx.Error {
	if _, r := displayname.Check(name, displayname.Person); r != displayname.OK && r != displayname.TooLong {
		return displayname.Error("Mailbox name", displayname.Person, r)
	}
	return nil
}

// ValidateTrackingDomain checks a mailbox's custom tracking domain against the
// same rule as the campaign override. Callers normalize the value first, so a
// pasted URL is reduced to its host rather than rejected.
func ValidateTrackingDomain(domain string) *errx.Error {
	if domain == "" {
		return errx.ErrEmailTrackingDomain
	}
	if len(domain) > 253 {
		return errx.ErrEmailTrackingDomainLength
	}
	if !TrackingHostname(strings.ToLower(domain)) {
		return errx.ErrEmailTrackingDomain
	}
	return nil
}

// EmailTimezone accepts an IANA zone name the runtime can actually load, or the
// empty string meaning "not configured". A zone that does not resolve would be
// silently coerced to UTC by the scheduler, which is how a mailbox ends up
// sending at the wrong local hour with nothing to point at.
func EmailTimezone(tz string) *errx.Error {
	if tz == "" {
		return nil
	}
	if len(tz) > 64 {
		return errx.ErrEmailTimezone
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return errx.ErrEmailTimezone
	}
	return nil
}

// WarmupFolder accepts the folder (Gmail label) name warmup mail is filed
// into, or the empty string meaning the instance default.
//
// The value is handed straight to a provider as a mailbox name, so it stays a
// single-level printable name: a separator would create a child of whatever it
// names (and "INBOX/Warmbly" on a server whose delimiter is "." is one folder
// literally called that), and IMAP refuses control characters outright.
func WarmupFolder(name string) *errx.Error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if len([]rune(name)) > config.WarmupFolderMaxLen {
		return errx.ErrEmailWarmupFolder
	}
	if strings.ContainsAny(name, `/\.%*"`) {
		return errx.ErrEmailWarmupFolder
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return errx.ErrEmailWarmupFolder
		}
	}
	return nil
}
