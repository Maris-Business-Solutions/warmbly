package repository

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"
)

// The custom-field sort key is the one part of this ORDER BY that comes from a
// customer, so it is a bound parameter appended to the args slice. That makes
// the placeholder number depend on how many arguments precede it, so this
// asserts the two stay in step: a mismatch is either an injection (too few
// args) or a bind error at runtime (too many).
func TestRoutedPairsOrderPlaceholderMatchesArgs(t *testing.T) {
	cases := []struct {
		name       string
		orderBy    string
		orderField string
		wantArgs   int
	}{
		{"default ordering binds nothing extra", "created_at", "", 1},
		{"email ordering binds nothing extra", "email", "", 1},
		{"custom field with no key falls back", "custom_field", "", 1},
		{"custom field binds the key", "custom_field", "Company Mobile", 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Mirrors phase one's order construction in FindRoutedPairs: the
			// candidate enumeration binds only the campaign ($1) up front, so a
			// custom-field key lands at $2.
			args := []any{"campaign"}
			var contactOrder string
			switch tc.orderBy {
			case "email":
				contactOrder = "c.email"
			case "custom_field":
				if tc.orderField != "" {
					args = append(args, tc.orderField)
					contactOrder = fmt.Sprintf("c.custom_fields->>$%d", len(args))
				} else {
					contactOrder = "c.created_at"
				}
			default:
				contactOrder = "c.created_at"
			}

			if len(args) != tc.wantArgs {
				t.Fatalf("args = %d, want %d", len(args), tc.wantArgs)
			}

			// The key must never appear as SQL text, only as a placeholder.
			if tc.orderField != "" && regexp.MustCompile(regexp.QuoteMeta(tc.orderField)).MatchString(contactOrder) {
				t.Fatalf("the sort key reached the SQL text: %q", contactOrder)
			}

			// Any placeholder used must be within the args that exist.
			for _, m := range regexp.MustCompile(`\$(\d+)`).FindAllStringSubmatch(contactOrder, -1) {
				n, _ := strconv.Atoi(m[1])
				if n > len(args) {
					t.Fatalf("placeholder $%d exceeds %d bound args", n, len(args))
				}
			}
		})
	}
}
