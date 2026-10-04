package apikey

import (
	"net"
	"strings"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// ErrCodeKeyExceedsCaller is the response code for a key wider than the credential creating or editing it.
const ErrCodeKeyExceedsCaller = "api_key_permissions_exceed_caller"

func exceedsCaller(msg string) *errx.Error {
	return errx.NewWithIdentifier(errx.Forbidden, ErrCodeKeyExceedsCaller, msg)
}

// WithinKey refuses a key shape that reaches past the API key making the request:
// more permissions, more mailboxes, or more source addresses than it has itself.
func WithinKey(caller *models.APIKey, perms uint64, ips []string, accounts []uuid.UUID) *errx.Error {
	if caller == nil {
		return exceedsCaller("the key making this request could not be read")
	}
	if perms&^caller.Permissions != 0 {
		return exceedsCaller("an API key cannot hold permissions the key making this request does not have")
	}
	if len(caller.AllowedEmailAccounts) > 0 {
		if len(accounts) == 0 {
			return exceedsCaller("this key is limited to some mailboxes, so a key it manages must be limited to them too")
		}
		own := make(map[uuid.UUID]bool, len(caller.AllowedEmailAccounts))
		for _, id := range caller.AllowedEmailAccounts {
			own[id] = true
		}
		for _, id := range accounts {
			if !own[id] {
				return exceedsCaller("an API key cannot reach a mailbox the key making this request cannot")
			}
		}
	}
	if len(caller.AllowedIPs) > 0 {
		normalized, xerr := validateAllowedIPs(ips)
		if xerr != nil {
			return xerr
		}
		if len(normalized) == 0 {
			return exceedsCaller("this key is limited to some IP addresses, so a key it manages must be limited to them too")
		}
		for _, entry := range normalized {
			if !ipEntryWithin(entry, caller.AllowedIPs) {
				return exceedsCaller("an API key cannot allow an address the key making this request does not allow")
			}
		}
	}
	return nil
}

// ipEntryWithin reports whether an address or block is covered by one of the allowed entries.
func ipEntryWithin(entry string, allowed []string) bool {
	ip, block := parseEntry(entry)
	if ip == nil {
		return false
	}
	for _, a := range allowed {
		aip, ablock := parseEntry(a)
		if aip == nil {
			continue
		}
		switch {
		case ablock == nil && block == nil:
			if aip.Equal(ip) {
				return true
			}
		case ablock != nil && block == nil:
			if ablock.Contains(ip) {
				return true
			}
		case ablock != nil && block != nil:
			ones, bits := block.Mask.Size()
			aones, abits := ablock.Mask.Size()
			if bits == abits && ones >= aones && ablock.Contains(block.IP) {
				return true
			}
		}
	}
	return false
}

func parseEntry(s string) (net.IP, *net.IPNet) {
	if strings.Contains(s, "/") {
		_, block, err := net.ParseCIDR(s)
		if err != nil {
			return nil, nil
		}
		return block.IP, block
	}
	return net.ParseIP(s), nil
}
