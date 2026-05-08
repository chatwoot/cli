#!/usr/bin/env bash
# smoke.sh — exercise the chatwoot CLI surface against a local Chatwoot.
#
# Refuses to run unless ~/.chatwoot/config.yaml's base_url is localhost.
# This is destructive: it changes status/assignee/labels/priority on a real
# conversation, and posts a public reply and a private note. Only run
# against a dev/local instance.
#
# Beyond exit-code checks, the writes are verified via state read-back so
# the script catches "API accepted but didn't apply" bug classes (e.g. the
# priority-null/none discrepancy we shipped against).

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

cd "$(dirname "$0")/.." || exit 1
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

pass() { printf '  PASS  %s\n' "$1"; PASS=$((PASS+1)); }
fail() { printf '  FAIL  %s\n' "$1"; FAIL=$((FAIL+1)); FAILURES+=("$1"); }

# expect <name> <pattern> <cmd...>
expect() {
  local name="$1"; local pattern="$2"; shift 2
  local out rc=0
  out=$("$@" 2>&1) || rc=$?
  if [[ $rc -ne 0 ]]; then
    fail "$name (exit $rc)"
    printf '%s\n' "$out" | sed 's/^/        /'
    return
  fi
  if [[ -z "$pattern" ]] || printf '%s' "$out" | grep -Eq -- "$pattern"; then
    pass "$name"
  else
    fail "$name (no match for /$pattern/)"
    printf '%s\n' "$out" | sed 's/^/        /'
  fi
}

# expect_fail <name> <error-pattern> <cmd...>
expect_fail() {
  local name="$1"; local pattern="$2"; shift 2
  local out rc=0
  out=$("$@" 2>&1) || rc=$?
  if [[ $rc -eq 0 ]]; then
    fail "$name (expected nonzero exit)"
    return
  fi
  if printf '%s' "$out" | grep -Eq -- "$pattern"; then
    pass "$name"
  else
    fail "$name (failed but message missed /$pattern/)"
    printf '%s\n' "$out" | sed 's/^/        /'
  fi
}

# state_eq <name> <jq-expr> <expected>
# Reads `conv $conv_id -o json` and asserts a jq path == expected (string compare).
state_eq() {
  local name="$1"; local expr="$2"; local expected="$3"
  local got
  got=$("$BIN" conv "$conv_id" -o json 2>/dev/null | jq -rc "$expr" 2>/dev/null) || got="<jq-err>"
  if [[ "$got" == "$expected" ]]; then
    pass "$name"
  else
    fail "$name (want=$expected got=$got)"
  fi
}

# ----------------------------------------------------------------------------
# Plurals (list)
# ----------------------------------------------------------------------------

echo "## plurals"
expect "convs"           "^ID|No conversations" "$BIN" convs --assignee all
expect "contacts"        "^ID|No contacts"      "$BIN" contacts
expect "inboxes"         "^ID|No inboxes"       "$BIN" inboxes
expect "agents"          "^ID|No agents"        "$BIN" agents
expect "labels"          "^ID|No labels"        "$BIN" labels
expect "teams"           "^ID|No teams"         "$BIN" teams
expect "me"              "^ID:"                  "$BIN" me

# JSON sanity: must parse.
expect "convs -o json (parse)"     ""  bash -c "'$BIN' convs --assignee all -o json | jq -e . >/dev/null"
expect "contacts -o json (parse)"  ""  bash -c "'$BIN' contacts -o json | jq -e . >/dev/null"
expect "agents -o json (parse)"    ""  bash -c "'$BIN' agents -o json | jq -e . >/dev/null"

# Quiet mode: convs -q is the README's headline scripting affordance — IDs
# only, one per line, all numeric.
quiet_out=$("$BIN" convs --assignee all -q 2>&1)
if [[ -z "$quiet_out" ]]; then
  pass "convs -q (no convs found)"
elif printf '%s' "$quiet_out" | awk 'NF && $0 !~ /^[0-9]+$/ { exit 1 }'; then
  pass "convs -q (numeric ids only)"
else
  fail "convs -q (non-numeric output: $(printf '%s' "$quiet_out" | head -1))"
fi

# Aliases.
expect "conversations (plural alias)"  "^ID|No conversations" "$BIN" conversations --assignee all

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
# Singular reads (id-first, verb-first, alias)
# ----------------------------------------------------------------------------

echo
echo "## singular reads"
if [[ -n "$conv_id" ]]; then
  expect "conv $conv_id (default view)"       "^ID:"            "$BIN" conv "$conv_id"
  expect "conv $conv_id view (id-first)"      "^ID:"            "$BIN" conv "$conv_id" view
  expect "conv view $conv_id (verb-first)"    "^ID:"            "$BIN" conv view "$conv_id"
  expect "conversation $conv_id view (alias)" "^ID:"            "$BIN" conversation "$conv_id" view
  expect "conv $conv_id messages"             "^ID|No messages" "$BIN" conv "$conv_id" messages
else
  echo "  SKIP  no conversations available — singular conv tests skipped"
fi

if [[ -n "$contact_id" ]]; then
  expect "contact $contact_id"                "^ID:"                 "$BIN" contact "$contact_id"
  expect "contact $contact_id conversations"  "^ID|No conversations" "$BIN" contact "$contact_id" conversations
fi

if [[ -n "$inbox_id" ]]; then
  expect "inbox $inbox_id"                    "^ID:"                 "$BIN" inbox "$inbox_id"
fi

# ----------------------------------------------------------------------------
# Writes — every write is followed by a state read-back so we catch the
# "API accepted but didn't apply" bug class.
# ----------------------------------------------------------------------------

if [[ -z "$conv_id" ]]; then
  echo
  echo "## writes — SKIPPED (no conversation to operate on)"
else
  echo
  echo "## writes (will mutate conv $conv_id; verifying state after each)"

  # ---- reply: capture message id via -q, verify content round-trip ----
  unique="smoke reply $(date +%s)-$RANDOM"
  reply_id=$("$BIN" -q conv "$conv_id" reply "$unique" 2>/dev/null || true)
  if [[ "$reply_id" =~ ^[0-9]+$ ]]; then
    pass "reply (msg id captured: $reply_id)"
    got=$("$BIN" conv "$conv_id" messages -o json 2>/dev/null \
      | jq -r --arg id "$reply_id" '.payload[] | select(.id == ($id|tonumber)) | .content')
    if [[ "$got" == "$unique" ]]; then
      pass "reply (content round-trip)"
    else
      fail "reply content mismatch (got=$got)"
    fi
  else
    fail "reply (-q did not return numeric id: '$reply_id')"
  fi

  # ---- private note: same approach, also verify .private == true ----
  unique_note="smoke note $(date +%s)-$RANDOM"
  note_id=$("$BIN" -q conv "$conv_id" reply "$unique_note" --private 2>/dev/null || true)
  if [[ "$note_id" =~ ^[0-9]+$ ]]; then
    pass "reply --private (msg id captured: $note_id)"
    got=$("$BIN" conv "$conv_id" messages -o json 2>/dev/null \
      | jq -rc --arg id "$note_id" '.payload[] | select(.id == ($id|tonumber)) | [.content, .private]')
    if [[ "$got" == "[\"$unique_note\",true]" ]]; then
      pass "reply --private (content+private flag round-trip)"
    else
      fail "reply --private mismatch (got=$got)"
    fi
  else
    fail "reply --private (-q did not return numeric id: '$note_id')"
  fi

  # ---- status verbs: each followed by state check ----
  expect    "resolve"               "-> resolved" "$BIN" conv "$conv_id" resolve
  state_eq  "resolve (state)"        '.status'    "resolved"

  expect    "open"                  "-> open"     "$BIN" conv "$conv_id" open
  state_eq  "open (state)"           '.status'    "open"

  expect    "pending"               "-> pending"  "$BIN" conv "$conv_id" pending
  state_eq  "pending (state)"        '.status'    "pending"

  expect    "snooze"                "-> snoozed"  "$BIN" conv "$conv_id" snooze
  state_eq  "snooze (state)"         '.status'    "snoozed"

  expect    "snooze --until 24h"    "until"       "$BIN" conv "$conv_id" snooze --until 24h
  state_eq  "snooze --until 24h (state)"  '.status'  "snoozed"

  expect    "snooze --until 7d"     "until"       "$BIN" conv "$conv_id" snooze --until 7d
  state_eq  "snooze --until 7d (state)"   '.status'  "snoozed"

  expect    "snooze --until date"   "until"       "$BIN" conv "$conv_id" snooze --until 2030-01-01
  state_eq  "snooze --until date (state)" '.status' "snoozed"

  expect    "open (cleanup)"        "-> open"     "$BIN" conv "$conv_id" open
  state_eq  "open cleanup (state)"   '.status'    "open"

  # ---- verb-first regression for a write verb ----
  expect    "conv resolve N (verb-first)"   "-> resolved"  "$BIN" conv resolve "$conv_id"
  state_eq  "verb-first resolve (state)"     '.status'     "resolved"
  "$BIN" conv "$conv_id" open >/dev/null  # restore

  # ---- assign --agent N: verify meta.assignee.id ----
  if [[ -n "$agent_id" ]]; then
    expect    "assign --agent N"          "assigned"           "$BIN" conv "$conv_id" assign --agent "$agent_id"
    state_eq  "assign --agent N (state)"  '.meta.assignee.id'   "$agent_id"

    # 'me' resolves via cached or lazy-fetched user_id (commit c786562 + later fix).
    expect "assign --agent me"            "assigned"            "$BIN" conv "$conv_id" assign --agent me
    me_id=$("$BIN" me -o json 2>/dev/null | jq -r '.id')
    state_eq "assign --agent me (state)"  '.meta.assignee.id'   "$me_id"
  fi

  # ---- assign --team N (the team-only review-fix path): omit assignee_id ----
  if [[ -n "$team_id" ]]; then
    # First unassign agent so we can verify team-only assign without prior agent override.
    "$BIN" conv "$conv_id" unassign >/dev/null
    expect    "assign --team N (team-only)" "team"             "$BIN" conv "$conv_id" assign --team "$team_id"
    state_eq  "assign --team N (state)"     '.meta.team.id'    "$team_id"
  fi

  # ---- unassign: verify meta.assignee == null ----
  expect    "unassign"             "unassigned"        "$BIN" conv "$conv_id" unassign
  state_eq  "unassign (state)"      '.meta.assignee'   "null"

  # ---- label: verify both labels appear in .labels ----
  l1="smoke-$RANDOM"; l2="test-$RANDOM"
  expect    "label $l1,$l2"            "labels:"     "$BIN" conv "$conv_id" label "$l1,$l2"
  state_eq  "label set (state)" \
    "(.labels | (index(\"$l1\") != null and index(\"$l2\") != null))"  "true"

  # ---- priority urgent ----
  expect    "priority urgent"          "-> urgent"   "$BIN" conv "$conv_id" priority urgent
  state_eq  "priority urgent (state)"   '.priority'  "urgent"

  # ---- priority none (review-fix: server returned 500 on literal "none";
  # we now send null which is the supported clear). State must be null/empty.
  expect    "priority none"            "-> none"     "$BIN" conv "$conv_id" priority none
  state_eq  "priority none (state)"     '.priority'  "null"
fi

# ----------------------------------------------------------------------------
# Failure paths — assert the *error message*, not just the exit code.
# ----------------------------------------------------------------------------

echo
echo "## expected failures"
if [[ -n "$conv_id" ]]; then
  expect_fail "assign with no flags"   "--agent or --team required"  "$BIN" conv "$conv_id" assign
  expect_fail "priority bogus"          "must be one of|priority"    "$BIN" conv "$conv_id" priority bogus
fi
expect_fail "snooze --until garbage"    "invalid --until"             "$BIN" conv "${conv_id:-1}" snooze --until "not-a-time"
expect_fail "conv 999999999 (404)"      "404|not found|Resource"      "$BIN" conv 999999999

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
