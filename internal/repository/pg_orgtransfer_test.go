package repository

import "testing"

func TestBindOrgParamRenumbersOnlyTheOrganizationPlaceholder(t *testing.T) {
	cases := map[string]string{
		`organization_id = $1`: `organization_id = $2`,
		`email_id IN (SELECT id FROM email_accounts WHERE organization_id = $1)`: `email_id IN (SELECT id FROM email_accounts WHERE organization_id = $2)`,
		`org_id = $1 AND x = $10`: `org_id = $2 AND x = $10`,
		`a = $1 OR b = $1`:        `a = $2 OR b = $2`,
	}
	for in, want := range cases {
		if got := bindOrgParam(in, 2); got != want {
			t.Errorf("bindOrgParam(%q) = %q, want %q", in, got, want)
		}
	}
}
