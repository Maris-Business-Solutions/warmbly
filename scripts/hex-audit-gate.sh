#!/bin/sh
# Runs `mix hex.audit` in the current Mix project and fails on a retired
# dependency, or on a flagged one with a newer release to move to. A flagged
# package already on its latest release is listed and left to review.
set -eu

main() {
	report=$(mktemp)
	trap 'rm -f "$report"' EXIT INT TERM

	if mix hex.audit >"$report" 2>&1; then
		cat "$report"
		return 0
	fi
	cat "$report"

	if grep -qi 'retired' "$report"; then
		echo "hex-audit-gate: a locked dependency is retired" >&2
		exit 1
	fi

	flagged=$(grep -E '^  [a-z0-9_]+ [0-9][^ ]* - ' "$report" | awk '{print $1, $2}' | sort -u)
	if [ -z "$flagged" ]; then
		echo "hex-audit-gate: the audit failed and its report could not be read" >&2
		exit 1
	fi

	stale=""
	while read -r name locked; do
		info=$(mix hex.info "$name" </dev/null)
		# The newest stable release: pre-releases carry a "-".
		latest=$(printf '%s\n' "$info" | awk '/^Recent releases:/ { found = 1; next } found && NF && $1 !~ /-/ { print $1; exit }')
		if [ -z "$latest" ]; then
			echo "hex-audit-gate: no release list for $name" >&2
			exit 1
		fi
		if [ "$latest" != "$locked" ]; then
			stale="$stale $name $locked->$latest"
		fi
	done <<EOF
$flagged
EOF

	if [ -n "$stale" ]; then
		echo "hex-audit-gate: flagged with a newer release available:$stale" >&2
		exit 1
	fi
	echo "hex-audit-gate: every flagged package is on its latest release"
	return 0
}

main "$@"
