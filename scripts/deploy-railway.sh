#!/usr/bin/env bash
#
# Rolls the hosted control plane's Railway services onto one release.
#
# The five services are pinned to explicit image tags rather than a moving
# `:prod`, so a release does not deploy itself and a rollback is naming the
# previous tag. This script is what moves the pins, in one order, with a gate
# between each step.
#
# Order is not cosmetic: the backend applies the embedded migrations on boot,
# so it goes first and nothing else moves until it answers. A consumer that
# boots against a schema its build has never seen is the failure this prevents.
#
# The Kafka suffix is decided by what the release publishes, not by preference:
# backend, consumer and tracking have `-kafka` variants linking librdkafka,
# realtime and forms do not. Sending a node or a service a tag that was never
# built is a boot loop, so the map lives here once.
#
# Usage:
#   scripts/deploy-railway.sh v0.4.13            # roll every service
#   scripts/deploy-railway.sh v0.4.12 --dry-run  # print the plan, touch nothing
#   scripts/deploy-railway.sh v0.4.13 --only realtime,forms
#
# Needs: railway CLI (>= 5.49), jq, curl, and either a linked project or
# RAILWAY_TOKEN scoped to the target environment.

set -euo pipefail

REGISTRY="ghcr.io/warmbly/warmbly"
ENVIRONMENT="${RAILWAY_ENVIRONMENT_NAME:-production}"
DRY_RUN=0
SKIP_HEALTH=0
ONLY=""
VERSION=""

# Service order, and the tag suffix each one's image is published under.
SERVICES="backend consumer tracking realtime forms"
suffix_for() {
  case "$1" in
    backend | consumer | tracking) printf -- '-kafka' ;;
    *) printf '' ;;
  esac
}

# How long one service may take to reach SUCCESS before we stop the roll.
WAIT_TIMEOUT_SECONDS=600
POLL_INTERVAL_SECONDS=5

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

log() { printf '\n==> %s\n' "$*"; }

usage() {
  sed -n '2,26p' "$0" | sed 's/^# \{0,1\}//'
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      -h | --help)
        usage
        exit 0
        ;;
      --dry-run) DRY_RUN=1 ;;
      --skip-health) SKIP_HEALTH=1 ;;
      --environment)
        ENVIRONMENT="${2:-}"
        shift
        ;;
      --only)
        ONLY="${2:-}"
        shift
        ;;
      v[0-9]*) VERSION="$1" ;;
      *) die "unknown argument: $1 (try --help)" ;;
    esac
    shift
  done

  [ -n "$VERSION" ] || die "no version given, e.g. $0 v0.4.13"
  case "$VERSION" in
    v[0-9]*.[0-9]*.[0-9]*) ;;
    *) die "version must look like v1.2.3, got '$VERSION'" ;;
  esac

  for tool in railway jq curl; do
    command -v "$tool" >/dev/null 2>&1 || die "$tool is not installed"
  done
}

selected() {
  [ -z "$ONLY" ] && return 0
  case ",$ONLY," in
    *",$1,"*) return 0 ;;
    *) return 1 ;;
  esac
}

deployments_json() {
  railway deployment list --service "$1" --environment "$ENVIRONMENT" --json 2>/dev/null
}

# The newest deployment's id, whatever its status. Captured before a redeploy so
# the wait can tell the new deployment from the one it replaced, which is more
# reliable than comparing timestamps across two clocks.
newest_deployment_id() {
  deployments_json "$1" | jq -r 'sort_by(.createdAt) | reverse | .[0].id // empty'
}

running_image() {
  deployments_json "$1" | jq -r '
    map(select(.status == "SUCCESS")) | sort_by(.createdAt) | reverse
    | .[0].meta.image // empty'
}

# The id of a deployment of image $2 that is still starting, or nothing.
deployment_in_flight() {
  deployments_json "$1" | jq -r --arg img "$2" '
    map(select((.meta.image // "") == $img
      and (.status | IN("QUEUED", "WAITING", "BUILDING", "DEPLOYING", "INITIALIZING"))))
    | sort_by(.createdAt) | reverse | .[0].id // empty'
}

# Succeeds once a deployment other than $2 carrying image $3 exists, within $4 seconds.
deployment_appeared() {
  service="$1"
  previous_id="$2"
  image="$3"
  limit="$4"
  waited=0
  while [ "$waited" -lt "$limit" ]; do
    found=$(deployments_json "$service" | jq -r --arg prev "$previous_id" --arg img "$image" '
      map(select(.id != $prev and (.meta.image // "") == $img)) | .[0].id // empty')
    [ -n "$found" ] && return 0
    sleep 3
    waited=$((waited + 3))
  done
  return 1
}

# Waits for a deployment that is NOT $2, carries image $3, and reached SUCCESS.
wait_for_deployment() {
  service="$1"
  previous_id="$2"
  image="$3"
  waited=0

  while [ "$waited" -lt "$WAIT_TIMEOUT_SECONDS" ]; do
    entry=$(deployments_json "$service" | jq -r --arg prev "$previous_id" --arg img "$image" '
      map(select(.id != $prev and (.meta.image // "") == $img))
      | sort_by(.createdAt) | reverse | .[0] | "\(.status // "")|\(.id // "")"')
    status="${entry%%|*}"

    case "$status" in
      SUCCESS)
        printf '    %s is live on %s\n' "$service" "$image"
        return 0
        ;;
      FAILED | CRASHED)
        die "$service deployment ${entry##*|} ended $status; nothing after it was rolled"
        ;;
      "")
        printf '    waiting for a deployment to appear (%ss)\n' "$waited"
        ;;
      *)
        printf '    %s (%ss)\n' "$status" "$waited"
        ;;
    esac

    sleep "$POLL_INTERVAL_SECONDS"
    waited=$((waited + POLL_INTERVAL_SECONDS))
  done

  die "$service did not reach SUCCESS within ${WAIT_TIMEOUT_SECONDS}s"
}

# The service's own public hostname, read from its Railway env rather than
# hardcoded, so renaming a domain does not leave a stale probe here.
public_domain() {
  railway variables --service "$1" --environment "$ENVIRONMENT" --kv 2>/dev/null |
    sed -n 's/^RAILWAY_PUBLIC_DOMAIN=//p' | head -1
}

# Backend health is a hard gate: it is what proves the migrations applied and
# the API answers before anything downstream moves. Every other probe is
# advisory, because those hostnames can be mid-DNS-change without the release
# being at fault.
check_health() {
  service="$1"
  fatal="$2"
  [ "$SKIP_HEALTH" = 1 ] && return 0

  domain=$(public_domain "$service")
  if [ -z "$domain" ]; then
    printf '    no public domain, skipping health probe\n'
    return 0
  fi

  attempt=1
  while [ "$attempt" -le 12 ]; do
    code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 10 "https://${domain}/health" || true)
    if [ "$code" = "200" ]; then
      printf '    https://%s/health -> 200\n' "$domain"
      return 0
    fi
    sleep 5
    attempt=$((attempt + 1))
  done

  if [ "$fatal" = 1 ]; then
    die "https://${domain}/health never returned 200 (last: ${code:-no answer}); stopping the roll"
  fi
  printf '    warning: https://%s/health returned %s\n' "$domain" "${code:-no answer}"
}

# Refuses a version whose images are not actually published, before any pin
# moves. `check-images-public.sh` already speaks GHCR's anonymous token flow, so
# it is called once per tag shape rather than reimplemented here: a service
# rolled onto a tag that was never built is a boot loop, and the -kafka variants
# exist for only three of the five.
preflight_images() {
  checker="$(dirname "$0")/check-images-public.sh"
  [ -x "$checker" ] || {
    printf '    warning: %s missing, skipping the pullability check\n' "$checker"
    return 0
  }

  plain=""
  kafka=""
  for service in $SERVICES; do
    selected "$service" || continue
    if [ -n "$(suffix_for "$service")" ]; then
      kafka="$kafka $service"
    else
      plain="$plain $service"
    fi
  done

  # shellcheck disable=SC2086 # deliberate word splitting: a list of services
  [ -n "$plain" ] && "$checker" --tag "$VERSION" $plain >/dev/null
  # shellcheck disable=SC2086
  [ -n "$kafka" ] && "$checker" --tag "${VERSION}-kafka" $kafka >/dev/null
  return 0
}

roll_service() {
  service="$1"
  image="${REGISTRY}/${service}:${VERSION}$(suffix_for "$service")"

  log "$service -> $image"

  current=$(running_image "$service" || true)
  if [ "$current" = "$image" ]; then
    printf '    already on this image, skipping\n'
    return 0
  fi
  [ -n "$current" ] && printf '    currently %s\n' "$current"

  if [ "$DRY_RUN" = 1 ]; then
    printf '    dry run: would connect the image and redeploy\n'
    return 0
  fi

  # One deployment per roll: a second one replaces the first mid-boot, and the
  # backend migrates on boot, so a migration killed halfway leaves the schema dirty.
  in_flight=$(deployment_in_flight "$service" "$image")
  if [ -n "$in_flight" ]; then
    printf '    deployment %s of this image is already starting; waiting for it\n' "$in_flight"
    wait_for_deployment "$service" "" "$image"
  else
    previous_id=$(newest_deployment_id "$service")
    railway service source connect --image "$image" --service "$service" --environment "$ENVIRONMENT" >/dev/null
    # Connecting an image deploys it; redeploy only when that did not happen.
    if ! deployment_appeared "$service" "$previous_id" "$image" 30; then
      railway deployment redeploy --service "$service" --environment "$ENVIRONMENT" --from-source --yes >/dev/null
    fi
    wait_for_deployment "$service" "$previous_id" "$image"
  fi

  if [ "$service" = "backend" ]; then
    check_health "$service" 1
  else
    check_health "$service" 0
  fi
}

main() {
  parse_args "$@"

  log "rolling $ENVIRONMENT onto $VERSION"
  preflight_images
  for service in $SERVICES; do
    selected "$service" || continue
    roll_service "$service"
  done

  log "done"
  for service in $SERVICES; do
    printf '  %-9s %s\n' "$service" "$(running_image "$service" || echo unknown)"
  done
}

main "$@"
