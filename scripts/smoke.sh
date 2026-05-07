#!/usr/bin/env bash
# smoke.sh — exercise the chatwoot CLI against a real, local Chatwoot.
#
# Refuses to run unless ~/.chatwoot/config.yaml's base_url is localhost.
# This is destructive: it changes status, assignee, labels, and priority
# on a real conversation, and posts a public reply and a private note.
# Only run against a dev/local instance.

set -uo pipefail

# ----------------------------------------------------------------------------
# Guards
# ----------------------------------------------------------------------------

config_file="${HOME}/.chatwoot/config.yaml"

if [[ ! -f "$config_file" ]]; then
  echo "smoke: no config at $config_file (run 'chatwoot auth login')" >&2
  exit 1
fi

base_url=$(sed -nE 's/^base_url:[[:space:]]*//p' "$config_file" | head -1 | tr -d '"' | tr -d "'")
case "$base_url" in
  http://localhost:*|http://localhost|http://127.0.0.1:*|http://127.0.0.1|http://0.0.0.0:*|http://0.0.0.0)
    ;;
  *)
    echo "smoke: refusing to run against non-localhost base_url: ${base_url}" >&2
    echo "       update ~/.chatwoot/config.yaml or run 'chatwoot auth login' against localhost." >&2
    exit 1
    ;;
esac

if ! command -v jq >/dev/null 2>&1; then
  echo "smoke: jq is required" >&2
  exit 1
fi

# ----------------------------------------------------------------------------
# Build the binary fresh
# ----------------------------------------------------------------------------

cd "$(dirname "$0")/.."
go build -o chatwoot ./cmd/chatwoot/
BIN="$(pwd)/chatwoot"

echo "smoke: localhost ok ($base_url) — running CLI surface"
echo

# ----------------------------------------------------------------------------
# Test harness
# ----------------------------------------------------------------------------

PASS=0
FAIL=0
FAILURES=()

# expect: name, regex pattern, command...
expect() {
  local name="$1"; local pattern="$2"; shift 2
  local out rc=0
  out=$("$@" 2>&1) || rc=$?
  if [[ $rc -ne 0 ]]; then
    printf '  FAIL  %-46s (exit %d)\n' "$name" "$rc"
    printf '%s\n' "$out" | sed 's/^/        /'
    FAIL=$((FAIL+1))
    FAILURES+=("$name (nonzero exit)")
    return
  fi
  if [[ -z "$pattern" ]] || printf '%s' "$out" | grep -Eq -- "$pattern"; then
    printf '  PASS  %s\n' "$name"
    PASS=$((PASS+1))
  else
    printf '  FAIL  %-46s (no match for /%s/)\n' "$name" "$pattern"
    printf '%s\n' "$out" | sed 's/^/        /'
    FAIL=$((FAIL+1))
    FAILURES+=("$name (pattern miss)")
  fi
}

# expect_fail: name, error-message regex, command...
expect_fail() {
  local name="$1"; local pattern="$2"; shift 2
  local out rc=0
  out=$("$@" 2>&1) || rc=$?
  if [[ $rc -eq 0 ]]; then
    printf '  FAIL  %-46s (expected nonzero exit)\n' "$name"
    FAIL=$((FAIL+1))
    FAILURES+=("$name (should have failed)")
    return
  fi
  if printf '%s' "$out" | grep -Eq -- "$pattern"; then
    printf '  PASS  %s\n' "$name"
    PASS=$((PASS+1))
  else
    printf '  FAIL  %-46s (failed but message missed /%s/)\n' "$name" "$pattern"
    printf '%s\n' "$out" | sed 's/^/        /'
    FAIL=$((FAIL+1))
    FAILURES+=("$name (error pattern miss)")
  fi
}

# ----------------------------------------------------------------------------
# Plurals (list)
# ----------------------------------------------------------------------------

echo "## plurals"
expect "convs"     "^ID|No conversations" "$BIN" convs --assignee all
expect "contacts"  "^ID|No contacts"      "$BIN" contacts
expect "inboxes"   "^ID|No inboxes"       "$BIN" inboxes
expect "agents"    "^ID|No agents"        "$BIN" agents
expect "labels"    "^ID|No labels"        "$BIN" labels
expect "teams"     "^ID|No teams"         "$BIN" teams
expect "me"        "^ID:"                  "$BIN" me

# ----------------------------------------------------------------------------
# Discover IDs we can poke at for singular tests
# ----------------------------------------------------------------------------

echo
echo "## discovering test IDs"
conv_id=$("$BIN"    convs --assignee all -o json 2>/dev/null | jq -r '.data.payload[0].id // empty')
contact_id=$("$BIN" contacts                  -o json 2>/dev/null | jq -r '.payload[0].id // empty')
inbox_id=$("$BIN"   inboxes                   -o json 2>/dev/null | jq -r '.payload[0].id // empty')
agent_id=$("$BIN"   agents                    -o json 2>/dev/null | jq -r '.[0].id // empty')
team_id=$("$BIN"    teams                     -o json 2>/dev/null | jq -r '.[0].id // empty')
echo "  conv=$conv_id  contact=$contact_id  inbox=$inbox_id  agent=$agent_id  team=$team_id"

# ----------------------------------------------------------------------------
# Singular reads
# ----------------------------------------------------------------------------

echo
echo "## singular reads"
if [[ -n "$conv_id" ]]; then
  expect "conv $conv_id (default view)"      "^ID:"            "$BIN" conv "$conv_id"
  expect "conv $conv_id view (id-first)"     "^ID:"            "$BIN" conv "$conv_id" view
  expect "conv view $conv_id (verb-first)"   "^ID:"            "$BIN" conv view "$conv_id"
  expect "conv $conv_id messages"            "^ID|No messages" "$BIN" conv "$conv_id" messages
else
  echo "  SKIP  no conversations available — singular conv tests skipped"
fi

if [[ -n "$contact_id" ]]; then
  expect "contact $contact_id"                       "^ID:"                       "$BIN" contact "$contact_id"
  expect "contact $contact_id conversations"         "^ID|No conversations"       "$BIN" contact "$contact_id" conversations
fi

if [[ -n "$inbox_id" ]]; then
  expect "inbox $inbox_id"                           "^ID:"                       "$BIN" inbox "$inbox_id"
fi

# ----------------------------------------------------------------------------
# Writes (require a conversation)
# ----------------------------------------------------------------------------

if [[ -z "$conv_id" ]]; then
  echo
  echo "## writes — SKIPPED (no conversation to operate on)"
else
  echo
  echo "## writes (will actually mutate conv $conv_id)"

  expect "reply"               "Sent reply"  "$BIN" conv "$conv_id" reply "smoke: public reply"
  expect "reply --private"     "Sent note"   "$BIN" conv "$conv_id" reply "smoke: private note" --private

  expect "resolve"             "-> resolved" "$BIN" conv "$conv_id" resolve
  expect "open"                "-> open"     "$BIN" conv "$conv_id" open
  expect "pending"             "-> pending"  "$BIN" conv "$conv_id" pending
  expect "snooze"              "-> snoozed"  "$BIN" conv "$conv_id" snooze
  expect "snooze --until 24h"  "until"       "$BIN" conv "$conv_id" snooze --until 24h
  expect "open (cleanup)"      "-> open"     "$BIN" conv "$conv_id" open

  if [[ -n "$agent_id" ]]; then
    expect "assign --agent N"   "assigned"   "$BIN" conv "$conv_id" assign --agent "$agent_id"
    expect "assign --agent me"  "assigned"   "$BIN" conv "$conv_id" assign --agent me
  fi

  if [[ -n "$team_id" ]]; then
    # the team-only review-fix path: must succeed and report team
    expect "assign --team N (team-only)"     "team"   "$BIN" conv "$conv_id" assign --team "$team_id"
  else
    echo "  SKIP  assign --team N (no teams configured)"
  fi

  expect "unassign"            "unassigned"   "$BIN" conv "$conv_id" unassign

  expect "label foo,bar"       "labels:"      "$BIN" conv "$conv_id" label "smoke,test"

  expect "priority urgent"     "-> urgent"    "$BIN" conv "$conv_id" priority urgent
  # the priority-none review-fix: must succeed (server used to reject null)
  expect "priority none"       "-> none"      "$BIN" conv "$conv_id" priority none
fi

# ----------------------------------------------------------------------------
# Failure paths (validate clear error messages)
# ----------------------------------------------------------------------------

echo
echo "## expected failures"
if [[ -n "$conv_id" ]]; then
  expect_fail "assign with no flags" "--agent or --team required"  "$BIN" conv "$conv_id" assign
  expect_fail "priority bogus"        "must be one of|priority"     "$BIN" conv "$conv_id" priority bogus
fi
expect_fail "snooze --until garbage"  "invalid --until"             "$BIN" conv "${conv_id:-1}" snooze --until "not-a-time"

# ----------------------------------------------------------------------------
# Summary
# ----------------------------------------------------------------------------

echo
echo "## summary"
printf '  %d passed, %d failed\n' "$PASS" "$FAIL"
if [[ $FAIL -gt 0 ]]; then
  echo "  failures:"
  printf '    - %s\n' "${FAILURES[@]}"
  exit 1
fi
