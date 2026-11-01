#!/bin/bash
#
# End-to-end smoke test of crannog on macOS 26 / Apple Silicon with Apple
# Containers. Builds real crannogs from a temporary repository with the
# default preset and checks checkout, sessions, the git causeway, the proxy
# causeway, apply (reload and rebuild), list, destroy and self-repair.
#
# Uses the real ~/.crannog: the instruction blocks land in your agent homes,
# as with any crannog. The temporary repository's state in
# ~/.crannog/repositories/ is removed afterwards; images are kept.
#
# Run it in a terminal: some steps ask for a setup approval (answer Yes).
#
# Usage: ./smoke.sh <dir with crannog and git-remote-crannog> [--keep]

set -u

BIN="${1:-}"
[ -n "$BIN" ] && [ -x "$BIN/crannog" ] && [ -x "$BIN/git-remote-crannog" ] || {
  echo "usage: $0 <dir with crannog and git-remote-crannog> [--keep]"; exit 1; }
BIN="$(cd "$BIN" && pwd)"
KEEP=0
[ "${2:-}" = --keep ] && KEEP=1
export PATH="$BIN:$PATH"

PASS=0
FAIL=0
WORK="$(cd "$(mktemp -d -t crannog-smoke)" && pwd -P)"
REPO="$WORK/code/smoke-app"

say()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
info() { printf '   info: %s\n' "$*"; }
find_() { printf '   \033[33mFIND\033[0m %s\n' "$*"; }

# check <description> <expect: ok|fail> <command...>
check() {
  desc="$1"; expect="$2"; shift 2
  if "$@" >"$WORK/last.log" 2>&1; then got=ok; else got=fail; fi
  if [ "$got" = "$expect" ]; then
    PASS=$((PASS + 1)); printf '   \033[32mPASS\033[0m %s\n' "$desc"
  else
    FAIL=$((FAIL + 1)); printf '   \033[31mFAIL\033[0m %s (expected %s, got %s)\n' "$desc" "$expect" "$got"
    sed 's/^/        /' "$WORK/last.log" | tail -8
  fi
}

# probe <description> <command...>: reports a finding, no expectation.
probe() {
  desc="$1"; shift
  if "$@" >/dev/null 2>&1; then find_ "$desc? yes"; else find_ "$desc? no"; fi
}

# crannog without a terminal on stdin: never asks.
cn() { (cd "$REPO" && crannog "$@" </dev/null); }
# A command in a session of the crannog of main, without a terminal.
in_main() { cn checkout main -- sh -c "$1"; }
g() { git -C "$REPO" "$@"; }
# has <pattern> <command...>: the command's output (stdout and stderr)
# contains pattern.
has() { pat="$1"; shift; "$@" 2>&1 | grep -q -- "$pat"; }

repo_id() { printf '%s' "$(g rev-parse --path-format=absolute --git-common-dir)" | shasum -a 256 | cut -c1-64; }
remotes() { g remote | grep '^crannog-' || true; }

# status_of <branch>: status and sessions from crannog list.
status_of() { cn list 2>/dev/null | awk -v b="$1" '$1 == b { print $2, $3 }'; }

wait_until() {
  i=0
  while ! "$@" >/dev/null 2>&1 && [ $i -lt 120 ]; do sleep 1; i=$((i + 1)); done
  "$@" >/dev/null 2>&1
}

# Background session in the crannog of a branch that runs until stopped.
start_bg() {
  (cd "$REPO" && crannog checkout "$1" -- sh -c \
    'touch /tmp/bg-started; while [ ! -e /tmp/bg-stop ]; do sleep 1; done; rm -f /tmp/bg-stop' \
    </dev/null >"$WORK/bg.log" 2>&1) &
  BG=$!
  wait_until sh -c "[ \"\$(cd '$REPO' && crannog list </dev/null 2>/dev/null | awk -v b='$1' '\$1 == b { print \$3 }')\" -ge 1 ]"
}
stop_bg() { cn checkout "$1" -- touch /tmp/bg-stop >/dev/null 2>&1; wait "$BG" 2>/dev/null; }

cleanup() {
  if [ "$KEEP" = 1 ]; then
    say "Kept. Clean up with:"
    echo "   cd $REPO && crannog destroy --yes main; crannog destroy --yes feature/smoke"
    echo "   rm -rf ~/.crannog/repositories/$ID $WORK"
    return
  fi
  for b in main feature/smoke; do cn destroy --yes "$b" >/dev/null 2>&1; done
  [ -n "${ID:-}" ] && rm -rf "$HOME/.crannog/repositories/$ID"
  rm -rf "$WORK"
}
trap cleanup EXIT
trap 'exit 130' INT

# --- Preconditions ------------------------------------------------------------

say "Preconditions"
[ -t 0 ] && [ -t 1 ] || { echo "Run this in a terminal: some steps ask for approval."; exit 1; }
command -v container >/dev/null || { echo "container CLI not found"; exit 1; }
container system status >/dev/null 2>&1 || { echo "Run 'container system start' first"; exit 1; }
info "$(container --version 2>&1 | head -1), macOS $(sw_vers -productVersion), $(git --version)"
info "crannog $(crannog version)"
check "crannog preset list shows default" ok has '^default *bundled' crannog preset list

mkdir -p "$REPO"
git init -q -b main "$REPO"
g config user.name "Smoke Test"
g config user.email "smoke@crannog.invalid"
echo "hello" >"$REPO/README.md"; g add README.md; g commit -qm "Initial commit"; g tag v1
g switch -qc feature/smoke; echo feature >"$REPO/feature.txt"; g add feature.txt; g commit -qm "Feature"
g switch -q main
echo "SECRET=1" >"$REPO/.env"
ID="$(repo_id)"
REPO_IN="/work$(g rev-parse --show-toplevel)"
info "repository: $REPO (id ${ID:0:8})"
check "crannog list without crannogs" ok has 'No crannogs' cn list

# --- 1. First checkout --------------------------------------------------------

say "1. First checkout (preset default, builds the image; may take minutes)"
SECONDS=0
(cd "$REPO" && crannog checkout -- sh -c 'echo hello-from-crannog; exit 7' </dev/null 2>&1) | tee "$WORK/first.log"
rc=${PIPESTATUS[0]}
info "first checkout took ${SECONDS}s"
check "exit code of the session passed through (7, got $rc)" ok [ "$rc" = 7 ]
check "session output" ok grep -q hello-from-crannog "$WORK/first.log"
check "source printed: preset default" ok grep -q 'Using .crannog/ from preset default' "$WORK/first.log"
check "no approval needed for the preset" fail grep -qi approv "$WORK/first.log"
check "crannog stopped after the session" ok [ "$(status_of main)" = "stopped 0" ]
check "no crannog remote while stopped" ok [ -z "$(remotes)" ]
check "approved copy of the preset stored" ok test -s "$HOME/.crannog/repositories/$ID/approved/latest"
check "instruction block in Claude's agent home" ok grep -q 'crannog:begin' "$HOME/.crannog/agents/claude/CLAUDE.md"
for f in codex/AGENTS.md opencode/AGENTS.md pi/AGENTS.md; do
  check "instruction block in agents/$f" ok grep -q 'crannog:begin' "$HOME/.crannog/agents/$f"
done

# --- 2. Inside the crannog ----------------------------------------------------

say "2. Inside the crannog"
SECONDS=0
check "warm start and session" ok in_main true
info "warm checkout took ${SECONDS}s"
check "session runs in /work/<host path>" ok in_main "[ \"\$(pwd)\" = '$REPO_IN' ]"
check "session runs as agent" ok in_main '[ "$(id -un)" = agent ]'
check "agent's login shell is zsh" ok in_main '[ "$(getent passwd agent | cut -d: -f7)" = /bin/zsh ]'
check "start branch main checked out" ok in_main '[ "$(git symbolic-ref --short HEAD)" = main ]'
check "other branches as origin/*, tag v1" ok in_main \
  'git rev-parse -q --verify origin/feature/smoke && git rev-parse -q --verify origin/main && git rev-parse -q --verify refs/tags/v1'
check "only main as local branch" ok in_main '[ "$(git for-each-ref --format="%(refname)" refs/heads)" = refs/heads/main ]'
check "untracked .env stayed on the host" fail in_main 'test -e .env'
check "git author mirrored from the host" ok in_main '[ "$(git config user.name)" = "Smoke Test" ] && [ "$(git config user.email)" = smoke@crannog.invalid ]'
check "crannog-proxy in /etc/hosts" ok in_main 'grep -q " crannog-proxy$" /etc/hosts'
check "HTTPS via the proxy to an allowed host" ok in_main 'curl -sS -m 30 -o /dev/null https://api.anthropic.com'
check "example.com blocked" fail in_main 'curl -fsS -m 30 -o /dev/null https://example.com'
check "Node fetch via the proxy" ok in_main \
  'node -e "fetch(\"https://registry.npmjs.org/\").then(()=>process.exit(0),e=>{console.error(e);process.exit(1)})"'
check "sudo apt-get update via the proxy" ok in_main 'sudo apt-get update -qq -o APT::Update::Error-Mode=any'
check "/state/codex and /state/opencode exist" ok in_main 'test -d /state/codex && test -d /state/opencode'
for a in claude codex opencode pi; do
  check "$a in PATH of the session" ok in_main "$a --version"
done
check "CA key not in the crannog" ok in_main '! test -e /etc/crannog/ca.key && test -s /etc/crannog/ca.crt'
check "readiness marker holds the boot id" ok in_main '[ "$(cat /tmp/ready)" = "$(cat /proc/sys/kernel/random/boot_id)" ]'
check "checkout -- passes arguments unchanged" ok has "say hi|it's|\$HOME|" cn checkout -- printf '%s|' 'say hi' "it's" '$HOME'

# --- 3. Git causeway ----------------------------------------------------------

say "3. Git causeway"
start_bg main
REMOTE="$(remotes)"
info "remote while running: ${REMOTE:-none}"
check "remote crannog-<slug> exists while a session runs" ok [ -n "$REMOTE" ]
check "crannog list: running, 1 session" ok [ "$(status_of main)" = "running 1" ]
check "second session without a terminal: warning, no question" ok has 'already running' cn checkout -- true
check "agent commits on main and on a new branch in a worktree" ok in_main \
  'echo agent >agent.txt && git add agent.txt && git commit -qm "Agent change" &&
   git worktree add -q -b agent-exp ../agent-exp origin/main &&
   cd ../agent-exp && echo exp >exp.txt && git add exp.txt && git commit -qm Experiment'
check "git fetch from the crannog" ok g fetch -q "$REMOTE"
check "fetched main and agent-exp" ok sh -c \
  "[ \"\$(git -C '$REPO' for-each-ref --format='%(refname:short)' 'refs/remotes/$REMOTE/' | sort | tr '\n' ' ')\" = '$REMOTE/agent-exp $REMOTE/main ' ]"
check "no remote HEAD tracked" fail g rev-parse -q --verify "refs/remotes/$REMOTE/HEAD"
g merge -q --ff-only "$REMOTE/main"
echo user >"$REPO/user.txt"; g add user.txt; g commit -qm "User change"
check "push from the host into the checked-out branch" ok g push -q "$REMOTE" main
check "user's commit checked out in the crannog" ok in_main 'test -f user.txt'
stop_bg main
check "crannog stopped after the last session" ok [ "$(status_of main)" = "stopped 0" ]
check "remote removed when stopped" ok [ -z "$(remotes)" ]
check "fetching a stopped crannog fails (helper never starts it)" fail g fetch -q "crannog::${REMOTE#crannog-}"

# --- 4. Self-repair -----------------------------------------------------------

say "4. Killed crannog process and self-repair"
start_bg main
CRANNOG_PID="$(pgrep -f 'crannog checkout main -- sh -c touch /tmp/bg-starte[d]' | head -1)"
info "killing crannog (pid ${CRANNOG_PID:-none})"
[ -n "$CRANNOG_PID" ] && kill -9 "$CRANNOG_PID"
sleep 1
check "the session keeps running without the crannog process" ok [ "$(status_of main)" = "running 1" ]
cn checkout main -- touch /tmp/bg-stop >/dev/null 2>&1
wait_until sh -c "! pgrep -f 'bg-starte[d]' >/dev/null"
check "next crannog call stops the crannog without sessions" ok wait_until sh -c "[ \"\$(cd '$REPO' && crannog list </dev/null 2>/dev/null | awk '\$1 == \"main\" {print \$2}')\" = stopped ]"
check "and removes its remote" ok [ -z "$(remotes)" ]
g remote add "$REMOTE" "crannog::${REMOTE#crannog-}"
g remote add crannog-mine https://example.com/mine.git
cn list >/dev/null 2>&1
check "stale crannog remote removed" ok sh -c "! git -C '$REPO' remote | grep -qx '$REMOTE'"
check "user's crannog-* remote with another URL kept" ok sh -c "git -C '$REPO' remote | grep -qx crannog-mine"
g remote remove crannog-mine
check "checkout refuses a foreign remote of the crannog's name" fail sh -c \
  "git -C '$REPO' remote add '$REMOTE' https://example.com/x.git && cd '$REPO' && crannog checkout -- true </dev/null"
g remote remove "$REMOTE"

# --- 5. Setup changes ---------------------------------------------------------

say "5. preset publish, approvals, apply"
check "preset publish default writes .crannog/" ok cn preset publish default
check "preset publish asks before replacing (no terminal: error)" fail cn preset publish default
check "content equal to the preset needs no approval" ok has 'up to date' cn apply
perl -i -pe 's/^(        - "deb.debian.org")$/$1\n        - "example.com"/' "$REPO/.crannog/iron-proxy.yaml"
check "changed setup without a terminal: approval error" fail cn apply
check "--yes does not approve" fail cn apply --yes

say "   APPROVE: crannog apply shows the diff (example.com added; press q if it opens in a pager). Answer Yes."
(cd "$REPO" && crannog apply)
check "approved: proxy config written (crannog stopped, no reload)" ok \
  grep -q example.com "$HOME/.crannog/repositories/$ID/crannogs/"*/proxy/iron-proxy.yaml
check "example.com reachable after the next start" ok in_main 'curl -fsS -m 30 -o /dev/null https://example.com'

start_bg main
perl -i -ne 'print unless /^        - "example.com"$/' "$REPO/.crannog/iron-proxy.yaml"
check "removing it again is a known content: no question" ok cn apply
check "running session continued (reload, no rebuild)" ok [ "$(status_of main)" = "running 1" ]
check "example.com blocked again without restart" fail in_main 'curl -fsS -m 30 -o /dev/null https://example.com'

perl -i -pe 's/^env:$/env:\n  SMOKE_VAR: "from-config"/' "$REPO/.crannog/config.yml"
check "rebuild with a running session needs a confirmation (no terminal: error)" fail cn apply
say "   APPROVE: crannog apply shows the diff (SMOKE_VAR; press q if it opens in a pager) and says the running session ends. Answer Yes."
(cd "$REPO" && crannog apply)
wait "$BG" 2>/dev/null
check "rebuilt crannog stays stopped" ok [ "$(status_of main)" = "stopped 0" ]
check "new env after the rebuild" ok in_main '[ "$SMOKE_VAR" = from-config ]'
check "repository survived the rebuild" ok in_main 'test -f agent.txt && git rev-parse -q --verify agent-exp'
check "git causeway works after the rebuild" ok in_main 'git log --oneline -1 | grep -q "User change"'

g add .crannog; g commit -qm "Add crannog setup"
check "committed setup: same content, no question" ok has 'up to date' cn apply
g switch -q feature/smoke
check "setup of a branch that is not checked out is read from its commit" ok has 'Using .crannog/ from main (commit' cn apply main
g switch -q main

# --- 6. Second crannog, list --------------------------------------------------

say "6. A second crannog"
SECONDS=0
check "checkout feature/smoke (preset, image cached)" ok cn checkout feature/smoke -- sh -c '[ "$(git symbolic-ref --short HEAD)" = feature/smoke ]'
info "second crannog took ${SECONDS}s"
check "crannog list shows feature/smoke" ok has '^feature/smoke' cn list
check "crannog list shows main" ok has '^main' cn list

# --- 7. Destroy ---------------------------------------------------------------

say "7. Destroy"
check "agent leaves an unmerged branch and a dirty worktree" ok in_main \
  'git switch -qc lost && echo x >lost.txt && git add lost.txt && git commit -qm Lost && echo wip >>lost.txt'
check "destroy without a terminal and without --yes fails" fail cn destroy main
check "crannog stopped again after the check" ok [ "$(status_of main)" = "stopped 0" ]
cn destroy --yes main >"$WORK/destroy.log" 2>&1
check "destroy reports the unmerged branch" ok grep -q 'lost (1 commit)' "$WORK/destroy.log"
check "destroy reports the dirty worktree" ok grep -q "$REPO_IN" "$WORK/destroy.log"
NAME="crannog-${ID:0:8}-${REMOTE#crannog-}"
check "containers deleted" fail sh -c "container list --all --quiet | grep -q '^$NAME'"
check "volumes deleted" fail sh -c "container volume list --quiet | grep -q '^$NAME'"
check "network deleted" fail sh -c "container network list --quiet | grep -q '^$NAME'"
check "host directory deleted" fail test -d "$HOME/.crannog/repositories/$ID/crannogs/${REMOTE#crannog-}"
check "no temporary refs left" ok [ -z "$(g for-each-ref refs/crannog/)" ]
check "destroy of a missing crannog fails" fail cn destroy --yes main
check "destroy feature/smoke" ok cn destroy --yes feature/smoke
check "crannog list empty" ok has 'No crannogs' cn list

# --- Manual -------------------------------------------------------------------

say "Optional manual checks (not automated)"
cat <<EOF
   Interactive session with a TTY (Claude Code by default):
     cd $REPO && crannog checkout
   Shell:  crannog checkout -- zsh
   OpenCode reads AGENTS.md from OPENCODE_CONFIG_DIR? Ask it in a session:
     crannog checkout -- opencode run "Quote the crannog section of your instructions"
   Run with --keep to try these before the cleanup.
EOF

say "Summary: $PASS passed, $FAIL failed"
echo "   Images are kept. List them with: container image list | grep crannog-"
[ "$FAIL" -eq 0 ]
