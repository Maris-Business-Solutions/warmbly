#!/bin/sh
# Runs govulncheck and fails only on a vulnerability the code calls that has a
# fixed version to move to. Everything else is listed and left to review.
# Usage: scripts/govulncheck-gate.sh [govulncheck flags] [packages]
set -eu

main() {
	command -v jq >/dev/null 2>&1 || { echo "govulncheck-gate: jq is required" >&2; exit 2; }
	govulncheck=${GOVULNCHECK:-"go run golang.org/x/vuln/cmd/govulncheck@latest"}
	report=$(mktemp)
	trap 'rm -f "$report"' EXIT INT TERM

	# shellcheck disable=SC2086 # GOVULNCHECK may carry its own arguments.
	$govulncheck -format json "$@" >"$report"

	called='select(.finding != null) | .finding | select((.trace[0].function // "") != "")'
	fixable=$(jq -r "$called | select((.fixed_version // \"\") != \"\") | \"\(.osv) \(.trace[0].module) fixed in \(.fixed_version)\"" "$report" | sort -u)
	unfixed=$(jq -r "$called | select((.fixed_version // \"\") == \"\") | \"\(.osv) \(.trace[0].module)\"" "$report" | sort -u)

	if [ -n "$unfixed" ]; then
		echo "Called, no fixed version yet (review, not a failure):"
		echo "$unfixed" | sed 's/^/  /'
	fi
	if [ -n "$fixable" ]; then
		echo "Called, with a fixed version available:"
		echo "$fixable" | sed 's/^/  /'
		exit 1
	fi
	echo "govulncheck-gate: nothing called that a version bump would fix"
	return 0
}

main "$@"
