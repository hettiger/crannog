#!/bin/bash
#
# Spike for ADR 0011 and ADR 0028: four agents in one image, agent homes via env vars.
#
# Verifies on macOS 26 / Apple Silicon:
#   1. The default image builds with Claude Code, Codex, OpenCode, Pi and Node.
#   2. Two crannogs: volumes /work and /state, repo at /work/<host path>.
#   3. CA via mount into the trust store; agents' hosts reachable via the proxy.
#   4. Agent homes relocated via env vars into the shared mount; Codex SQLite
#      in /state; where OpenCode puts its DB; files written outside the
#      expected places.
#   5. (--manual) Logins in crannog a, reuse in crannog b, one agent calling
#      another.
#
# The proxy runs in warn mode: nothing is blocked, unlisted hosts are logged.
# The summary lists them as candidates for the default allowlist.
#
# Run directly on the Mac. Usage: ./spike.sh [--keep | --manual]

set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
P="crannog-spike4"
NET="$P-net"
SUBNET="192.168.236.0/24"
IMG="$P-agent:spike"
IRON="docker.io/ironsh/iron-proxy:0.54.0@sha256:6d074be2de9188f88d08013788d5500fb4de8557f16c2e5b27af4d8d2cb1e3fb"
CA_NAME="crannog spike4 CA"
MGMT_KEY="spike-management-key"
BASE_DOMAINS="*.anthropic.com *.claude.ai *.claude.com *.openai.com *.chatgpt.com
  registry.npmjs.org"
AGENTS_IN=/home/agent/.crannog/agents

KEEP=0
MANUAL=0
case "${1:-}" in
  --keep) KEEP=1 ;;
  --manual) KEEP=1; MANUAL=1 ;;
esac

PASS=0
FAIL=0
WORK="$(mktemp -d -t crannog-spike4)"
REPO="$WORK/host/code/myrepo"          # repo path on the "host"
REPO_IN="/work$REPO"                   # same path in the crannog

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
  container delete --force "$P-a" "$P-b" "$P-proxy" >/dev/null 2>&1
  for x in a b; do
    container volume delete "$P-$x-work" "$P-$x-state" >/dev/null 2>&1
  done
  container network delete "$NET" >/dev/null 2>&1
}

cleanup() {
  if [ "$KEEP" = 1 ]; then
    say "Kept running. Clean up with:"
    echo "   container delete --force $P-a $P-b $P-proxy"
    echo "   container volume delete $P-a-work $P-a-state $P-b-work $P-b-state"
    echo "   container network delete $NET; rm -rf $WORK"
    return
  fi
  remove_all
  rm -rf "$WORK"
}
trap cleanup EXIT
# container catches SIGINT and exits normally, so bash would just continue.
trap 'exit 130' INT

# Non-login shell: keeps the image's PATH (Debian's /etc/profile resets it).
in_a() { container exec --user agent --workdir "$REPO_IN" "$P-a" bash -c "$1"; }
in_b() { container exec --user agent --workdir "$REPO_IN" "$P-b" bash -c "$1"; }

wait_for() {
  i=0
  while ! "$@" >/dev/null 2>&1 && [ $i -lt 60 ]; do sleep 1; i=$((i + 1)); done
  "$@" >/dev/null 2>&1
}

write_proxy_config() {
  {
    cat <<EOF
dns:
  enabled: false
proxy:
  tunnel_listen: ":8080"
tls:
  mode: mitm
  ca_cert: /etc/iron-proxy/ca.crt
  ca_key: /etc/iron-proxy/ca.key
metrics:
  listen: "127.0.0.1:9090"
management:
  listen: "127.0.0.1:9092"
transforms:
  - name: allowlist
    config:
      warn: true
      domains:
EOF
    for d in $BASE_DOMAINS; do echo "        - \"$d\""; done
    echo "log:"
    echo "  level: info"
  } > "$WORK/proxy/proxy.yaml"
}

# Files written since the marker outside the shared agent homes, per crannog.
leaks() {
  container exec "$1" sh -c "find /home/agent /state -newer /tmp/marker -type f \
    -not -path '$AGENTS_IN/*' 2>/dev/null | sed 's|/[^/]*\$||' | sort | uniq -c | sort -rn | head -20"
}

# --- Preconditions ------------------------------------------------------------

say "Preconditions"
command -v container >/dev/null || { echo "container CLI not found"; exit 1; }
info "$(container --version 2>&1 | head -1)"
container system status >/dev/null 2>&1 || { echo "Run 'container system start' first"; exit 1; }
remove_all

mkdir -p "$WORK/ca" "$WORK/proxy" "$WORK/agents" "$REPO"
for x in a b; do mkdir -p "$WORK/crannog-$x/ca"; done
# crannog creates the agent homes on the host (CODEX_HOME must exist).
for a in claude codex opencode pi; do mkdir -p "$WORK/agents/$a"; done

# --- 1. Image -----------------------------------------------------------------

say "1. Default image"
SECONDS=0
check "build image (Claude Code, Codex, OpenCode, Pi, Node)" ok container build -t "$IMG" "$HERE/container"
info "build took ${SECONDS}s"
check "pull iron-proxy pinned by digest" ok container image pull "$IRON"

# --- 2. Proxy and crannogs ----------------------------------------------------

say "2. Proxy and two crannogs"
check "generate CA" ok \
  container run --rm -v "$WORK/ca:/out" "$IRON" generate-ca -outdir /out -name "$CA_NAME"
cp "$WORK/ca/ca.crt" "$WORK/ca/ca.key" "$WORK/proxy/"
for x in a b; do cp "$WORK/ca/ca.crt" "$WORK/crannog-$x/ca/"; done
write_proxy_config
check "create internal network" ok container network create --internal --subnet "$SUBNET" "$NET"
check "start proxy" ok \
  container run -d --name "$P-proxy" --network default --network "$NET" \
    -v "$WORK/proxy:/etc/iron-proxy" -e "IRON_MANAGEMENT_API_KEY=$MGMT_KEY" \
    "$IRON" -config /etc/iron-proxy/proxy.yaml
check "proxy healthy" ok wait_for \
  container exec "$P-proxy" wget -q -O /dev/null http://127.0.0.1:9090/healthz
PROXY_IP="$(container exec "$P-proxy" ip -4 -o addr | awk '{print $4}' | cut -d/ -f1 | grep '^192\.168\.236\.')"
PROXY="http://$PROXY_IP:8080"
info "proxy: $PROXY"

for x in a b; do
  container volume create "$P-$x-work" >/dev/null
  container volume create "$P-$x-state" >/dev/null
  check "start crannog $x (volumes /work + /state, env relocation)" ok \
    container run -d --name "$P-$x" --network "$NET" \
      -v "$P-$x-work:/work" -v "$P-$x-state:/state" \
      -v "$WORK/agents:$AGENTS_IN" \
      -v "$WORK/crannog-$x/ca:/etc/crannog:ro" \
      -e "HTTPS_PROXY=$PROXY" -e "https_proxy=$PROXY" \
      -e "HTTP_PROXY=$PROXY" -e "http_proxy=$PROXY" \
      -e "NO_PROXY=localhost,127.0.0.1,::1" -e "no_proxy=localhost,127.0.0.1,::1" \
      -e "NODE_EXTRA_CA_CERTS=/etc/crannog/ca.crt" \
      -e "NODE_USE_ENV_PROXY=1" \
      -e "CODEX_CA_CERTIFICATE=/etc/crannog/ca.crt" \
      -e "CLAUDE_CONFIG_DIR=$AGENTS_IN/claude" \
      -e "CODEX_HOME=$AGENTS_IN/codex" \
      -e "CODEX_SQLITE_HOME=/state/codex" \
      -e "OPENCODE_CONFIG_DIR=$AGENTS_IN/opencode" \
      -e "OPENCODE_DB=/state/opencode/opencode.db" \
      -e "PI_CODING_AGENT_DIR=$AGENTS_IN/pi" \
      "$IMG"
  check "crannog $x ready (CA trusted, volumes owned by agent)" ok wait_for \
    container exec "$P-$x" test -f /tmp/ready
  container exec --user agent "$P-$x" sh -c \
    "mkdir -p '$REPO_IN' /state/codex /state/opencode && git init -q '$REPO_IN'"
done

# --- 3. Volumes, path, CA -----------------------------------------------------

say "3. Volumes, repo path, CA"
check "/work and /state are separate mounts" ok in_a \
  '[ "$(stat -c %d /work)" != "$(stat -c %d /state)" ]'
check "repo at /work/<host path>" ok in_a "git -C '$REPO_IN' rev-parse --git-dir"
check "agent writes /work and /state without sudo" ok in_a 'touch /work/w /state/s'
check "CA key not in the crannog" fail in_a 'test -e /etc/crannog/ca.key'
check "curl trusts the proxy CA (system store)" ok in_a 'curl -sS -m 20 -o /dev/null https://api.anthropic.com'
check "node fetch via proxy (NODE_USE_ENV_PROXY) trusts the CA (NODE_EXTRA_CA_CERTS)" ok in_a \
  'node -e "fetch(\"https://api.anthropic.com\").then(()=>process.exit(0),e=>{console.error(e);process.exit(1)})"'

# --- 4. Agents ----------------------------------------------------------------

say "4. Agents"
for a in claude codex opencode pi; do
  check "$a --version" ok in_a "$a --version"
  info "$a: $(in_a "command -v $a; $a --version 2>&1 | head -1" | tr '\n' ' ')"
done
for a in claude codex opencode pi; do
  check "$a in PATH of a zsh login shell (session)" ok \
    container exec --user agent "$P-a" zsh -lc "command -v $a"
done

container exec "$P-a" touch /tmp/marker
# Without login: each run fails at the model call, but initializes its state.
in_a 'timeout 60 claude -p hi </dev/null'       >"$WORK/claude.log" 2>&1
in_a 'timeout 60 codex exec hi </dev/null'      >"$WORK/codex.log" 2>&1
in_a 'timeout 60 opencode run hi </dev/null'    >"$WORK/opencode.log" 2>&1
in_a 'timeout 60 pi -p hi </dev/null'           >"$WORK/pi.log" 2>&1
for a in claude codex opencode pi; do
  info "$a without login: $(head -c 160 "$WORK/$a.log" | tr '\n' ' ')"
done

check "Claude: .claude.json inside CLAUDE_CONFIG_DIR" ok test -e "$WORK/agents/claude/.claude.json"
check "Claude: nothing written to ~/.claude or ~/.claude.json" fail in_a \
  'find ~/.claude ~/.claude.json -newer /tmp/marker 2>/dev/null | grep -q .'
probe "Codex: SQLite files in /state/codex" in_a 'ls /state/codex/*.sqlite'
check "Codex: no SQLite in the shared CODEX_HOME" fail sh -c "ls '$WORK/agents/codex'/*.sqlite*"
probe "OpenCode: DB in /state/opencode (OPENCODE_DB honored)" in_a 'ls /state/opencode/*.db'
probe "OpenCode: DB in ~/.local/share/opencode" in_a 'ls ~/.local/share/opencode/*.db'
check "OpenCode: no DB in the shared agent home" fail sh -c "find '$WORK/agents/opencode' -name '*.db*' | grep -q ."
probe "Pi: files in PI_CODING_AGENT_DIR" sh -c "find '$WORK/agents/pi' -type f | grep -q ."
info "files written outside the agent homes (count dir):"
leaks "$P-a" | sed 's/^/        /'
info "OpenCode service.json in the shared config dir: $(tr -d '\n' < "$WORK/agents/opencode/service.json" 2>/dev/null | cut -c1-200)"
info "agent home on the host:"
(cd "$WORK/agents" && find . -maxdepth 2 | sort | sed 's/^/        /')

# --- 5. Updates via the proxy -------------------------------------------------

say "5. Updates via the proxy"
probe "claude update works via the proxy" in_a 'timeout 120 claude update'
probe "npm registry reachable (Pi updates)" in_a 'npm view @earendil-works/pi-coding-agent version'

# --- Summary ------------------------------------------------------------------

say "Hosts not in the base allowlist (warned by the proxy)"
container logs "$P-proxy" 2>&1 | grep -i warn >"$WORK/warned.log"
hosts="$(grep -oE '"host":"[^"]+"|host=[^ ]+' "$WORK/warned.log" \
  | sed -E 's/"host":"|host=|"//g' | sort | uniq -c | sort -rn)"
if [ -n "$hosts" ]; then
  echo "$hosts" | sed 's/^/   /'
else
  info "could not parse hosts, raw warn lines:"
  tail -20 "$WORK/warned.log" | cut -c1-200 | sed 's/^/        /'
fi

say "Summary: $PASS passed, $FAIL failed"
echo "   Images kept for faster reruns. Remove with:"
echo "   container image delete $IMG $IRON"

if [ "$MANUAL" = 1 ]; then
  say "Manual steps"
  cat <<EOF
   Shell in a crannog (zsh, as in a session):
     container exec -it --user agent --workdir $REPO_IN $P-a zsh
     container exec -it --user agent --workdir $REPO_IN $P-b zsh

   1. Logins in crannog a:
        claude                      # /login, subscription, paste the code
        codex login --device-auth
        opencode                    # connect OpenAI: "Codex device code (legacy)"
        pi                          # /login (Anthropic or OpenAI)
   2. In crannog b, expect no login needed:
        claude -p "say hi"
        codex exec "say hi"
        pi -p "say hi"
        opencode run "say hi"       # with an Anthropic model via Claude Code: no login?
                                    # with OpenAI: expect a new login
   3. One agent calling another, in crannog a:
        echo 'def add(a, b): return a - b' > calc.py && git add . && git commit -qm wip
        claude --dangerously-skip-permissions \\
          -p "Ask opencode (opencode run ...) to review calc.py and report its answer"
   4. OpenCode in both crannogs at the same time (shared service.json):
        run "opencode" in a and b in parallel, both should work
        cat $WORK/agents/opencode/service.json
   5. Where things ended up:
        ls -la $WORK/agents/*
        container exec $P-a sh -c 'ls -la /state/* ~/.local/share ~/.config'
   6. Hosts the proxy saw:
        container logs $P-proxy | grep -i warn | tail -50
EOF
fi

[ "$FAIL" -eq 0 ]
