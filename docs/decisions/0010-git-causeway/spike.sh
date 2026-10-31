#!/bin/bash
#
# Spike for ADR 0010: git causeway via a remote helper over `container exec`.
#
# Verifies on macOS 26 / Apple Silicon:
#   1. A minimal remote helper (connect capability) pipes git through
#      `container exec`: fetch, push, binary data, exit codes.
#   2. Initial transfer: all local branches and tags as origin/* in the
#      crannog, the start branch checked out, nothing uncommitted or untracked.
#   3. Fetch-back with +refs/heads/* returns only the crannog's own branches,
#      including one the agent created in a worktree.
#   4. The repo on a named volume survives stop/start and delete + run with a
#      new image.
#   5. The data for the destroy summary can be computed, and removing the
#      remote removes its refs.
#
# Run directly on the Mac. Usage: ./spike.sh [--keep]

set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
P="crannog-spike3"
C="$P"            # crannog (container) name, also the remote name
VOL="$P-vol"
IMG="$P-agent"
BIN_MB=20

KEEP=0
[ "${1:-}" = "--keep" ] && KEEP=1

PASS=0
FAIL=0
WORK="$(mktemp -d -t crannog-spike3)"

say()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
info() { printf '   info: %s\n' "$*"; }

# check <description> <expect: ok|fail> <command...>
check() {
  desc="$1"; expect="$2"; shift 2
  if "$@" >"$WORK/last.log" 2>&1; then got=ok; else got=fail; fi
  if [ "$got" = "$expect" ]; then
    PASS=$((PASS + 1)); printf '   \033[32mPASS\033[0m %s\n' "$desc"
  else
    FAIL=$((FAIL + 1)); printf '   \033[31mFAIL\033[0m %s (expected %s, got %s)\n' "$desc" "$expect" "$got"
    sed 's/^/        /' "$WORK/last.log" | tail -5
  fi
}

# probe <description> <command...>: reports a finding, no expectation.
probe() {
  desc="$1"; shift
  if "$@" >/dev/null 2>&1; then
    printf '   \033[33mFIND\033[0m %s? yes\n' "$desc"
  else
    printf '   \033[33mFIND\033[0m %s? no\n' "$desc"
  fi
}

remove_all() {
  container delete --force "$C" >/dev/null 2>&1
  container volume delete "$VOL" >/dev/null 2>&1
}

cleanup() {
  if [ "$KEEP" = 1 ]; then
    say "Kept running. Clean up with:"
    echo "   container delete --force $C; container volume delete $VOL; rm -rf $WORK"
    return
  fi
  remove_all
  rm -rf "$WORK"
}
trap cleanup EXIT
# container catches SIGINT and exits normally, so bash would just continue.
trap 'exit 130' INT

# Commands in the crannog.
agent() { container exec --user agent --workdir /work/repo "$C" bash -c "$1"; }
root()  { container exec "$C" bash -c "$1"; }

# Host git: isolated from the user's git config (signing, hooks, ...).
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=User GIT_AUTHOR_EMAIL=user@crannog.invalid
export GIT_COMMITTER_NAME=User GIT_COMMITTER_EMAIL=user@crannog.invalid
REPO="$WORK/host-repo"
hg() { git -C "$REPO" "$@"; }

# The helper must be in PATH as git-remote-crannog.
mkdir -p "$WORK/bin"
cp "$HERE/git-remote-crannog" "$WORK/bin/"
chmod +x "$WORK/bin/git-remote-crannog"
export PATH="$WORK/bin:$PATH"
export CRANNOG_SPIKE_LOG="$WORK/helper.log"

now() { perl -MTime::HiRes=time -e 'printf "%.2f", time'; }
mbps() { perl -e "printf '%.1f MB/s (%.2fs)', $BIN_MB / ($2 - $1), $2 - $1"; }

start_crannog() {
  container run -d --init --name "$C" -v "$VOL:/work" "$IMG:$1"
}

# --- Preconditions ------------------------------------------------------------

say "Preconditions"
command -v container >/dev/null || { echo "container CLI not found"; exit 1; }
info "$(container --version 2>&1 | head -1)"
info "$(git --version)"
container system status >/dev/null 2>&1 || { echo "Run 'container system start' first"; exit 1; }
remove_all

# --- 1. Images and crannog ----------------------------------------------------

say "1. Images and crannog"
for v in 1 2; do
  cp -R "$HERE/agent" "$WORK/agent-v$v"
  sed -i '' "s/IMAGE_VERSION/$v/" "$WORK/agent-v$v/Dockerfile"
  check "build agent image v$v" ok container build -t "$IMG:v$v" "$WORK/agent-v$v"
done
check "create volume" ok container volume create "$VOL"
check "start crannog with image v1, volume at /work" ok start_crannog v1
check "agent owns /work" ok root "chown agent:agent /work"
check "init empty repo on start branch feature-x" ok container exec --user agent "$C" bash -c \
  "git init -q -b feature-x /work/repo && git -C /work/repo config receive.denyCurrentBranch updateInstead"

# --- 2. Initial transfer ------------------------------------------------------

say "2. Initial transfer"
git init -q -b main "$REPO"
echo "hello" > "$REPO/README.md"; hg add README.md; hg commit -qm "Initial commit"
hg tag v1
hg switch -qc other-branch; echo other > "$REPO/other.txt"; hg add other.txt; hg commit -qm "Other work"
hg switch -q main
hg switch -qc feature-x; echo feature > "$REPO/feature.txt"; hg add feature.txt; hg commit -qm "Feature work"
echo "SECRET=1" > "$REPO/.env"
echo "uncommitted" >> "$REPO/README.md"
hg remote add "$C" "crannog::$C"
# Newer git creates refs/remotes/<remote>/HEAD on fetch; not wanted here.
hg config "remote.$C.followRemoteHEAD" never

check "push branches as origin/*, tags, start branch" ok \
  hg push -q "$C" 'refs/heads/*:refs/remotes/origin/*' 'refs/tags/*:refs/tags/*' 'feature-x:refs/heads/feature-x'
info "helper log: $(sort -u "$CRANNOG_SPIKE_LOG" 2>/dev/null | tr '\n' ' ')"
check "start branch feature-x checked out with its files" ok agent \
  '[ "$(git symbolic-ref --short HEAD)" = feature-x ] && test -f feature.txt'
check "origin/main, origin/other-branch, origin/feature-x exist" ok agent \
  'git rev-parse -q --verify origin/main && git rev-parse -q --verify origin/other-branch && git rev-parse -q --verify origin/feature-x'
check "tag v1 exists" ok agent 'git rev-parse -q --verify refs/tags/v1'
check "only feature-x as local branch" ok agent \
  '[ "$(git for-each-ref --format="%(refname)" refs/heads)" = refs/heads/feature-x ]'
check "untracked .env not transferred" fail agent 'test -e .env'
check "uncommitted change not transferred, tree clean" ok agent \
  '! grep -q uncommitted README.md && [ -z "$(git status --porcelain)" ]'
hg checkout -q -- README.md

# --- 3. Round trip ------------------------------------------------------------

say "3. Round trip"
check "agent commits on feature-x" ok agent \
  'echo agent > agent.txt && git add agent.txt && git commit -qm "Agent change"'
check "agent creates worktree with new branch agent-exp" ok agent \
  'git worktree add -q -b agent-exp /work/worktrees/agent-exp origin/main &&
   cd /work/worktrees/agent-exp && echo exp > exp.txt && git add exp.txt && git commit -qm "Experiment"'
check "user fetches" ok hg fetch -q "$C"
check "fetched refs are exactly feature-x and agent-exp" ok sh -c \
  "[ \"\$(git -C '$REPO' for-each-ref --format='%(refname:short)' 'refs/remotes/$C/' | sort | tr '\n' ' ')\" = '$C/agent-exp $C/feature-x ' ]"
check "agent's commit arrived" ok sh -c "git -C '$REPO' log --oneline '$C/feature-x' | grep -q 'Agent change'"

hg merge -q --ff-only "$C/feature-x"
echo user > "$REPO/user.txt"; hg add user.txt; hg commit -qm "User change"
check "user pushes follow-up (updateInstead)" ok hg push -q "$C" feature-x
check "follow-up checked out in crannog" ok agent 'test -f user.txt'

agent 'echo dirty >> user.txt'
echo more >> "$REPO/user.txt"; hg commit -qam "User change 2"
check "push into dirty tree is rejected (exit code propagated)" fail hg push -q "$C" feature-x
agent 'git checkout -q -- user.txt'
check "push succeeds once the tree is clean" ok hg push -q "$C" feature-x

dd if=/dev/urandom of="$REPO/user.bin" bs=1m count="$BIN_MB" 2>/dev/null
hg add user.bin; hg commit -qm "User binary"
t0="$(now)"; check "push ${BIN_MB} MB binary" ok hg push -q "$C" feature-x; t1="$(now)"
info "push: $(mbps "$t0" "$t1")"
host_sum="$(shasum -a 256 "$REPO/user.bin" | cut -d' ' -f1)"
check "binary identical in crannog" ok agent "[ \"\$(sha256sum user.bin | cut -d' ' -f1)\" = $host_sum ]"

agent "head -c ${BIN_MB}M /dev/urandom > agent.bin && git add agent.bin && git commit -qm 'Agent binary'"
crannog_sum="$(agent 'sha256sum agent.bin' | cut -d' ' -f1)"
t0="$(now)"; check "fetch ${BIN_MB} MB binary" ok hg fetch -q "$C"; t1="$(now)"
info "fetch: $(mbps "$t0" "$t1")"
check "binary identical on host" ok sh -c \
  "[ \"\$(git -C '$REPO' show '$C/feature-x:agent.bin' | shasum -a 256 | cut -d' ' -f1)\" = '$crannog_sum' ]"

# --- 4. Persistence -----------------------------------------------------------

say "4. Persistence"
head_before="$(agent 'git rev-parse HEAD')"
root 'touch /layer-marker'

check "stop crannog" ok container stop "$C"
check "start crannog" ok container start "$C"
check "repo HEAD unchanged after stop/start" ok agent "[ \"\$(git rev-parse HEAD)\" = $head_before ]"
probe "writable layer survives stop/start" root 'test -e /layer-marker'

check "delete container (volume stays)" ok container delete --force "$C"
check "volume still exists" ok sh -c "container volume list -q | grep -qx '$VOL'"
check "run again with image v2 on the same volume" ok start_crannog v2
check "image v2 in use" ok root '[ "$(cat /image-version)" = 2 ]'
check "repo HEAD unchanged after recreate" ok agent "[ \"\$(git rev-parse HEAD)\" = $head_before ]"
check "worktree agent-exp still works" ok agent \
  'git -C /work/worktrees/agent-exp log --oneline -1 | grep -q Experiment'
check "repo intact (git fsck)" ok agent 'git fsck --no-progress'
check "helper works after recreate" ok hg fetch -q "$C"
probe "writable layer survives recreate" root 'test -e /layer-marker'

# --- 5. Destroy summary -------------------------------------------------------

say "5. Destroy summary"
# Commits of each crannog branch not reachable from any other ref in the repo.
lost() {
  for ref in $(hg for-each-ref --format='%(refname)' "refs/remotes/$C/" | grep -v '/HEAD$'); do
    n="$(hg rev-list --count "$ref" --not --exclude="refs/remotes/$C/*" --all)"
    [ "$n" -gt 0 ] && echo "${ref#refs/remotes/$C/} $n"
  done
}
dirty() {
  agent 'git worktree list --porcelain | sed -n "s/^worktree //p" |
    while read -r w; do [ -n "$(git -C "$w" status --porcelain)" ] && echo "$w"; done'
}

check "fetch before summary" ok hg fetch -q "$C"
info "branches not in user's repo (commits): $(lost | tr '\n' ',')"
check "agent-exp and feature-x reported" ok \
  [ "$(lost | cut -d' ' -f1 | sort | tr '\n' ' ')" = "agent-exp feature-x " ]
hg merge -q --ff-only "$C/feature-x"
check "feature-x no longer reported after user merges it" ok \
  [ "$(lost | cut -d' ' -f1 | tr '\n' ' ')" = "agent-exp " ]

check "clean worktrees reported clean" ok [ -z "$(dirty)" ]
agent 'echo wip >> /work/worktrees/agent-exp/exp.txt'
check "dirty worktree reported" ok [ "$(dirty)" = /work/worktrees/agent-exp ]

check "remove remote" ok hg remote remove "$C"
check "remote-tracking refs removed with it" ok sh -c \
  "[ -z \"\$(git -C '$REPO' for-each-ref 'refs/remotes/$C/')\" ]"
check "merged work kept in user's branch" ok sh -c "git -C '$REPO' log --oneline feature-x | grep -q 'Agent binary'"
check "destroy: delete container" ok container delete --force "$C"
check "destroy: delete volume" ok container volume delete "$VOL"

# --- Summary ------------------------------------------------------------------

say "Summary: $PASS passed, $FAIL failed"
echo "   Images kept for faster reruns. Remove with:"
echo "   container image delete $IMG:v1 $IMG:v2"
[ "$FAIL" -eq 0 ]
