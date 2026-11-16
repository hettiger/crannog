#!/bin/bash
#
# Spike for ADR 0030: crannog runs itself again through
# /usr/bin/sandbox-exec with a profile that allows writes only in the
# anchors (ADR 0013), the git directory, the builder directory of the
# runtime and a few devices.
#
# Verifies on macOS 26 / Apple Silicon:
#   1. The profile: reads everywhere, writes only where allowed; parameters,
#      regex-quote and symlinked paths behave as the profile needs them.
#   2. The self-test: creating a file in the home fails with EPERM.
#   3. Nesting: git-remote-crannog can run inside the sandbox of crannog;
#      crannog cannot start inside another sandbox.
#   4. The setup directory can be replaced through its temporary sibling.
#   5. git works with only its git directory writable, in a linked worktree
#      too.
#   6. The container CLI works in the sandbox: build, volume, network,
#      create, exec with a pipe and with a terminal, delete.
#   7. With --go-test: go test runs with only the temporary directory and
#      the build cache writable. This runs crannog's tests on the host, so
#      it is skipped unless every check before it passed. It needs the
#      script in its place in the crannog repository and the Go modules
#      downloaded.
#
# Run directly on the Mac, in a terminal.
# Usage: ./spike.sh [--keep] [--go-test]

set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$HERE/../../.." && pwd -P)"
P="crannog-spike6"
C="$P"
VOL="$P-vol"
NET="$P-net"
IMG="$P-image"
BASE="debian:trixie-slim"

KEEP=0
GO_TEST=0
for arg in "$@"; do
  case "$arg" in
    --keep) KEEP=1 ;;
    --go-test) GO_TEST=1 ;;
    *) echo "usage: $0 [--keep] [--go-test]"; exit 1 ;;
  esac
done

PASS=0
FAIL=0
START="$(date '+%Y-%m-%d %H:%M:%S')"
# The cleanup deletes $WORK: it must be the directory mktemp created. It
# is in /tmp, not in the user's temporary directory: the profile allows
# writes in all of that, and most of $WORK has to be outside of it.
WORK="$(mktemp -d /tmp/seatbelt-spike6.XXXXXX)" || { echo "error: mktemp failed" >&2; exit 1; }
[ -n "$WORK" ] && [ -d "$WORK" ] || { echo "error: mktemp returned '$WORK'" >&2; exit 1; }
UNRESOLVED="$WORK"
WORK="$(cd -P -- "$WORK" && pwd -P)" || exit 1
case "$WORK" in
  /private/tmp/seatbelt-spike6.*) ;;
  *) echo "error: unexpected temporary directory '$WORK'" >&2; exit 1 ;;
esac
[ -n "${TMPDIR:-}" ] || { echo "error: TMPDIR is not set" >&2; exit 1; }
TMP="$(cd -P -- "$TMPDIR" && pwd -P)" || exit 1
# The self-test file of the spike, not the one crannog will use.
PROBE="$HOME/.crannog-spike6-check"
# The container CLI itself writes the build context here (container build).
BUILDER="$(cd -P -- "$HOME" && pwd -P)/Library/Application Support/com.apple.container/builder"

say()  { printf '\n\033[1m== %s\033[0m\n' "$*"; }
info() { printf '   info: %s\n' "$*"; }

# check <description> <expect: ok|fail> <command...>
# The command gets no terminal on stdin, so it cannot wait for an answer.
check() {
  desc="$1"; expect="$2"; shift 2
  if "$@" >"$WORK/last.log" 2>&1 </dev/null; then got=ok; else got=fail; fi
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
  if "$@" >"$WORK/probe.log" 2>&1 </dev/null; then
    printf '   \033[33mFIND\033[0m %s? yes\n' "$desc"
  else
    printf '   \033[33mFIND\033[0m %s? no\n' "$desc"
    sed 's/^/        /' "$WORK/probe.log" | tail -3
  fi
}

remove_all() {
  container delete --force "$C" >/dev/null 2>&1
  container volume delete "$VOL" >/dev/null 2>&1
  container network delete "$NET" >/dev/null 2>&1
}

cleanup() {
  rm -f -- "$PROBE"
  rm -rf -- "$TMP/crannog-spike6-build.d" "$TMP/crannog-spike6-diff.d"
  if [ "$KEEP" = 1 ]; then
    say "Kept. Clean up with:"
    echo "   container delete --force $C; container volume delete $VOL; container network delete $NET"
    printf '   rm -rf -- %q\n' "$WORK"
    return
  fi
  remove_all
  rm -rf -- "$WORK"
}
trap cleanup EXIT
# container catches SIGINT and exits normally, so bash would just continue.
trap 'exit 130' INT

# The profile under test. Later rules win. Paths are matched with symlinks
# resolved, so every parameter must be a resolved path.
#   ALLOW   a directory writable as a whole (stands for ~/.crannog)
#   TMP     the user's temporary directory, writable as a whole:
#           container build writes an archive of the build context
#           directly into it and ignores $TMPDIR
#   WT      a worktree: only .crannog and .crannog.tmp-* in it are writable
#   GITDIR  the common git directory of the repository
#   BUILDER the builder directory of the container runtime
PROFILE='(version 1)
(allow default)
(deny file-write*)
(allow file-write*
  (subpath (param "ALLOW"))
  (subpath (param "GITDIR"))
  (subpath (param "BUILDER"))
  (subpath (param "TMP"))
  (regex (string-append "^" (regex-quote (param "WT")) "/\\.crannog(\\.tmp-[0-9a-f]+)?(/|$)"))
  (literal "/dev/null")
  (literal "/dev/tty")
  (literal "/dev/dtracehelper")
  (regex #"^/dev/ttys[0-9]+$"))'

ALLOW="$WORK/allowed"
OUTSIDE="$WORK/outside"
WT="$WORK/repo"
LINKED="$WORK/linked"
mkdir -p "$ALLOW" "$OUTSIDE"
git init -q -b main "$WT"
git -C "$WT" -c user.name=Spike -c user.email=spike@crannog.invalid commit -q --allow-empty -m init
git -C "$WT" worktree add -q -b other "$LINKED"
GITDIR="$(git -C "$WT" rev-parse --path-format=absolute --git-common-dir)"

# sb <command...>: runs the command in the sandbox. SB_WT selects another
# worktree. The script makes it usable from sh -c as well.
SB="$WORK/sb"
export PROFILE ALLOW TMP WT GITDIR BUILDER
cat > "$SB" <<'SH'
#!/bin/sh
exec /usr/bin/sandbox-exec -p "$PROFILE" \
  -D "ALLOW=$ALLOW" -D "TMP=$TMP" -D "WT=${SB_WT:-$WT}" -D "GITDIR=$GITDIR" \
  -D "BUILDER=$BUILDER" "$@"
SH
chmod +x "$SB"
sb() { "$SB" "$@"; }
# sb_wt <worktree> <command...>: the same with another worktree.
sb_wt() { wt="$1"; shift; SB_WT="$wt" "$SB" "$@"; }

# --- 1. The profile -----------------------------------------------------------

say "1. The profile"
[ -x /usr/bin/sandbox-exec ] || { echo "/usr/bin/sandbox-exec not found"; exit 1; }
info "macOS $(sw_vers -productVersion), $(git --version)"
check "the profile compiles and a command runs" ok sb true
check "the exit code passes through" ok sh -c "'/usr/bin/sandbox-exec' -p '(version 1)(allow default)' sh -c 'exit 7'; [ \$? = 7 ]"
check "read outside the allowed paths" ok sb cat /etc/hosts
check "write in an allowed directory" ok sb touch "$ALLOW/a"
check "mkdir, rename and delete in it" ok sb sh -c "mkdir -p '$ALLOW/d/e' && mv '$ALLOW/a' '$ALLOW/d/e/b' && rm -r '$ALLOW/d'"
check "write next to the allowed directory" fail sb touch "$OUTSIDE/a"
check "write in the temporary directory" ok sb sh -c "mkdir '$TMP/crannog-spike6-x.d' && touch '$TMP/crannog-spike6-x.d/f' && rm -r '$TMP/crannog-spike6-x.d'"
check "write in /tmp, which is not the user's temporary directory" fail sb touch "/tmp/seatbelt-spike6-not-allowed"
touch "$OUTSIDE/victim"
check "delete a file outside" fail sb rm -f "$OUTSIDE/victim"
check "move a file from outside into an allowed directory" fail sb mv "$OUTSIDE/victim" "$ALLOW/victim"
check "chmod a file outside" fail sb chmod 600 "$OUTSIDE/victim"
check "append to a file outside" fail sb sh -c "echo x >> '$OUTSIDE/victim'"
ln -s "$OUTSIDE" "$ALLOW/link"
check "write through a symlink that leaves the allowed directory" fail sb touch "$ALLOW/link/a"
check "a child of a child is still in the sandbox" fail sb sh -c "sh -c \"sh -c 'touch $OUTSIDE/b'\""
check "write to /dev/null" ok sb sh -c 'echo x > /dev/null'
if [ -t 0 ]; then
  check "write to /dev/tty" ok sb sh -c 'printf "" > /dev/tty'
else
  info "no terminal: /dev/tty not checked"
fi
if [ "$UNRESOLVED" != "$WORK" ]; then
  info "the working directory is behind a symlink: $UNRESOLVED"
  probe "an unresolved path as parameter allows the write (expected: no)" \
    /usr/bin/sandbox-exec -p '(version 1)(allow default)(deny file-write*)(allow file-write* (subpath (param "A")))' \
    -D "A=$UNRESOLVED/allowed" touch "$ALLOW/unresolved"
else
  info "the working directory is not behind a symlink: unresolved parameters not checked"
fi

# --- 2. The self-test -----------------------------------------------------------

say "2. The self-test"
rm -f -- "$PROBE"
sb sh -c "set -C; : > '$PROBE'" 2>"$WORK/selftest.log"
check "creating a file in the home fails in the sandbox" fail test -e "$PROBE"
check "the error is 'Operation not permitted' (EPERM)" ok grep -qi 'operation not permitted' "$WORK/selftest.log"
check "without the sandbox the file is created" ok sh -c "set -C; : > '$PROBE'"
check "an existing file gives another error than EPERM outside the sandbox" fail sh -c "set -C; : > '$PROBE'"
rm -f -- "$PROBE"

# --- 3. Nesting -------------------------------------------------------------------

say "3. Nesting"
# git-remote-crannog runs inside the sandbox of crannog (crannog runs git
# push, git runs the helper): the same profile with the same parameters.
probe "same profile, same parameters: applies" sb "$SB" true
probe "same profile: write in an allowed directory" sb "$SB" touch "$ALLOW/nested-same"
probe "same profile: write outside (expected: no)" sb "$SB" touch "$OUTSIDE/nested-same"
# Anything else is refused, so the helper must not enter a sandbox again
# and crannog cannot start inside the sandbox of another program.
probe "same profile, another worktree parameter: applies (expected: no)" sb env "SB_WT=$LINKED" "$SB" true
probe "profile inside another sandbox: applies (expected: no)" \
  /usr/bin/sandbox-exec -p '(version 1)(allow default)' "$SB" true
check "nothing was written outside by nesting" fail sh -c "ls '$OUTSIDE' | grep -q nested"

# --- 4. Replacing the setup directory ------------------------------------------------

say "4. Replacing the setup directory"
mkdir "$WT/.crannog"; echo old > "$WT/.crannog/config"
T=".crannog.tmp-0123456789abcdef"
check "write in .crannog" ok sb sh -c "echo new > '$WT/.crannog/config'"
check "create the temporary sibling" ok sb sh -c "mkdir '$WT/$T' && mkdir '$WT/$T/new' && echo next > '$WT/$T/new/config'"
check "stash .crannog in the sibling" ok sb mv "$WT/.crannog" "$WT/$T/old"
check "install the new one as .crannog" ok sb mv "$WT/$T/new" "$WT/.crannog"
check "delete the sibling" ok sb rm -r "$WT/$T"
check "the new setup directory is in place" ok grep -q next "$WT/.crannog/config"
check "create .crannog where it is missing" ok sb_wt "$LINKED" mkdir "$LINKED/.crannog"
check "write another file in the worktree" fail sb touch "$WT/README"
check "a sibling with another name" fail sb mkdir "$WT/.crannog.tmp-evil"
check "a name that only starts with .crannog" fail sb mkdir "$WT/.crannogX"
check ".crannog of a worktree that is not a parameter" fail sb touch "$LINKED/.crannog/a"

# --- 5. git ---------------------------------------------------------------------------

say "5. git"
check "git status" ok sb git -C "$WT" status --porcelain
check "git remote add" ok sb git -C "$WT" remote add spike "crannog::spike"
check "git update-ref creates a ref" ok sb git -C "$WT" update-ref refs/remotes/spike/main HEAD
check "git update-ref --stdin deletes it" ok sb sh -c "echo 'delete refs/remotes/spike/main' | git -C '$WT' update-ref --no-deref --stdin"
check "git remote remove" ok sb git -C "$WT" remote remove spike
check "git status in the linked worktree" ok sb git -C "$LINKED" status --porcelain
check "git update-ref in the linked worktree" ok sb git -C "$LINKED" update-ref refs/remotes/spike/x HEAD
check "git ls-files with untracked files" ok sb git -C "$WT" ls-files -z --cached --others --exclude-standard -- .crannog
mkdir "$TMP/crannog-spike6-diff.d"
mkdir "$TMP/crannog-spike6-diff.d/a" "$TMP/crannog-spike6-diff.d/b"
echo 1 > "$TMP/crannog-spike6-diff.d/a/f"; echo 2 > "$TMP/crannog-spike6-diff.d/b/f"
check "git diff --no-index in a temporary directory (exit 1: differences)" ok \
  sh -c "cd '$TMP/crannog-spike6-diff.d' && '$SB' git --no-pager diff --no-index --no-prefix -- a b >/dev/null; [ \$? = 1 ]"
if [ -t 1 ]; then
  info "git diff with the pager: press q if it opens"
  (cd "$TMP/crannog-spike6-diff.d" && sb git -c core.pager='less' diff --no-index -- a b)
  check "git diff with the pager exits with 1" ok [ $? = 1 ]
fi
rm -rf -- "$TMP/crannog-spike6-diff.d"
check "git commit (writes objects) with the git directory writable" ok \
  sb git -C "$WT" -c user.name=Spike -c user.email=spike@crannog.invalid commit -q --allow-empty -m second

# --- 6. The container CLI ---------------------------------------------------------------

say "6. The container CLI"
if ! command -v container >/dev/null; then
  info "container CLI not found: skipped"
elif ! container system status >/dev/null 2>&1; then
  info "Run 'container system start' first: skipped"
else
  info "$(container --version 2>&1 | head -1)"
  remove_all
  CTX="$TMP/crannog-spike6-build.d"
  rm -rf -- "$CTX"; mkdir "$CTX"
  printf 'FROM %s\nCOPY marker /marker\n' "$BASE" > "$CTX/Dockerfile"
  echo built > "$CTX/marker"
  mkdir "$ALLOW/mount" "$OUTSIDE/mount"

  check "container list" ok sb container list --all --quiet
  check "container build" ok sb container build --tag "$IMG" "$CTX"
  probe "the build left its archive in the temporary directory" \
    sh -c "ls '$TMP' | grep -Eq '^[0-9A-F]{8}(-[0-9A-F]{4}){3}-[0-9A-F]{12}\.tar\$'"
  IMAGE="$IMG"
  if sb container image inspect "$IMG" >/dev/null 2>&1; then
    check "container image inspect" ok sb container image inspect "$IMG"
  else
    # Without the built image, the other commands are still checked.
    IMAGE="$BASE"
    info "no built image: continuing with $BASE"
  fi
  check "container volume create" ok sb container volume create "$VOL"
  check "container network create --internal" ok sb container network create --internal "$NET"
  check "container create with volume, network and bind mounts" ok sb container create --name "$C" \
    --network "$NET" --volume "$VOL:/work" \
    --volume "$ALLOW/mount:/mnt/allowed" --volume "$OUTSIDE/mount:/mnt/outside" \
    "$IMAGE" sleep infinity
  check "container start" ok sb container start "$C"
  check "container exec" ok sb container exec "$C" cat /etc/os-release
  [ "$IMAGE" = "$IMG" ] && check "the image is the built one" ok sb container exec "$C" cat /marker
  check "container exec with a pipe on stdin and stdout" ok \
    sh -c "[ \"\$(echo ping | '$SB' container exec --interactive '$C' cat)\" = ping ]"
  check "container exec passes the exit code" ok sh -c \
    "'$SB' container exec '$C' sh -c 'exit 7'; [ \$? = 7 ]"
  if [ -t 0 ] && [ -t 1 ]; then
    sb container exec --interactive --tty "$C" sh -c 'test -t 0 && test -t 1 && stty size >/dev/null'
    check "container exec with a terminal" ok [ $? = 0 ]
  else
    info "no terminal: container exec --tty not checked"
  fi
  check "the guest writes into a mount inside the allowed paths" ok sb container exec "$C" touch /mnt/allowed/from-guest
  # The runtime's daemon is not in the sandbox: mounts stay a matter of
  # the arguments crannog passes.
  probe "the guest writes into a mount outside the allowed paths (expected: yes)" \
    sb container exec "$C" touch /mnt/outside/from-guest
  check "container stop" ok sb container stop "$C"
  check "container delete" ok sb container delete "$C"
  check "container volume delete" ok sb container volume delete "$VOL"
  check "container network delete" ok sb container network delete "$NET"
  rm -rf -- "$CTX"
  [ "$KEEP" = 1 ] || container image delete "$IMG" >/dev/null 2>&1
fi

# --- 7. go test -----------------------------------------------------------------------------

say "7. go test"
if [ "$GO_TEST" = 0 ]; then
  info "not requested (--go-test): skipped"
elif [ "$FAIL" -ne 0 ]; then
  info "checks failed before: crannog's tests are not run on the host"
elif ! command -v go >/dev/null; then
  info "go not found: skipped"
elif ! grep -qx 'module github.com/hettiger/crannog' "$REPO_ROOT/go.mod" 2>/dev/null; then
  info "$REPO_ROOT is not the crannog repository: skipped"
else
  GOCACHE_DIR="$(cd -P -- "$(go env GOCACHE)" && pwd -P)"
  TEST_PROFILE='(version 1)
(allow default)
(deny file-write*)
(allow file-write*
  (subpath (param "TMP"))
  (subpath (param "GOCACHE"))
  (literal "/dev/null")
  (literal "/dev/tty")
  (literal "/dev/dtracehelper")
  (regex #"^/dev/ttys[0-9]+$"))'
  info "$(go version), GOCACHE=$GOCACHE_DIR"
  (cd "$REPO_ROOT" && /usr/bin/sandbox-exec -p "$TEST_PROFILE" -D "TMP=$TMP" -D "GOCACHE=$GOCACHE_DIR" \
    go test -count=1 ./... >"$WORK/gotest.log" 2>&1)
  check "go test ./... with only the temporary directory and the build cache writable" ok [ $? = 0 ]
  tail -25 "$WORK/gotest.log" | sed 's/^/        /'
  (cd "$REPO_ROOT" && /usr/bin/sandbox-exec -p "$TEST_PROFILE" -D "TMP=$TMP" -D "GOCACHE=$GOCACHE_DIR" \
    go vet ./... >"$WORK/govet.log" 2>&1)
  check "go vet ./..." ok [ $? = 0 ]
  (cd "$REPO_ROOT" && /usr/bin/sandbox-exec -p "$TEST_PROFILE" -D "TMP=$TMP" -D "GOCACHE=$GOCACHE_DIR" \
    sh -c 'test -z "$(gofmt -l .)"')
  check "gofmt -l ." ok [ $? = 0 ]
fi

# --- Denials ----------------------------------------------------------------------------------

say "Denied writes the system logged (expected ones from the checks above included)"
log show --start "$START" --style compact \
  --predicate 'sender == "Sandbox" AND eventMessage CONTAINS "deny" AND eventMessage CONTAINS "file-write"' 2>/dev/null |
  sed -E 's/^.*deny\([0-9]+\) //' | sort | uniq -c | sort -rn | head -40 | sed 's/^/   /'

# --- Summary ----------------------------------------------------------------------------------

say "Summary: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
