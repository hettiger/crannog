#!/bin/bash
#
# Spike: assumptions about the default image, names and the network.
#
# Verifies on macOS 26 / Apple Silicon:
#   1. Repository path: git's common dir after cd with another case, from a
#      worktree, and through a symlinked path.
#   2. Name limits of containers, networks and volumes.
#   3. Proxy with 256M memory, crannog with 4 CPUs and 4G (default preset).
#   4. Plain HTTP via the tunnel listener, sudo keeping the proxy variables,
#      sudo apt-get via the proxy.
#   5. container exec passes the command's exit code.
#   6. Codex and OpenCode state in the dirs the entrypoint creates in /state.
#   7. Proxy memory under parallel TLS load.
#   8. Proxy IP after stop/start.
#
# The proxy enforces the default allowlist plus deb.debian.org.
#
# Run directly on the Mac. Usage: ./spike.sh [--keep]

set -u
# Host git without the user's configuration.
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1

HERE="$(cd "$(dirname "$0")" && pwd)"
P="crannog-spike5"
NET="$P-net"
SUBNET="192.168.237.0/24"
SUBNET_RE='^192\.168\.237\.'
IMG="$P-agent:spike"
IRON="docker.io/ironsh/iron-proxy:0.54.0@sha256:6d074be2de9188f88d08013788d5500fb4de8557f16c2e5b27af4d8d2cb1e3fb"
CA_NAME="crannog spike5 CA"
MGMT_KEY="spike-management-key"
DOMAINS="*.anthropic.com *.claude.ai *.claude.com *.openai.com *.chatgpt.com
  opencode.ai models.opencode.ai pi.dev registry.npmjs.org deb.debian.org"
AGENTS_IN=/home/agent/.crannog/agents
REPO_IN=/work/Users/agent/code/app

KEEP=0
[ "${1:-}" = --keep ] && KEEP=1

PASS=0
FAIL=0
WORK="$(mktemp -d -t crannog-spike5)"

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
    sed 's/^/        /' "$WORK/last.log" | tail -5
  fi
}

# probe <description> <command...>: reports a finding, no expectation.
probe() {
  desc="$1"; shift
  if "$@" >/dev/null 2>&1; then find_ "$desc? yes"; else find_ "$desc? no"; fi
}

# Exit code of a command, output discarded.
code() { "$@" </dev/null >/dev/null 2>&1; echo $?; }

delete_name() {
  case "$1" in
    container) container delete --force "$2" ;;
    network)   container network delete "$2" ;;
    volume)    container volume delete "$2" ;;
  esac >/dev/null 2>&1
}

remove_all() {
  container delete --force "$P-a" "$P-proxy" >/dev/null 2>&1
  container volume delete "$P-work" "$P-state" >/dev/null 2>&1
  container network delete "$NET" >/dev/null 2>&1
  if [ -f "$WORK/names" ]; then
    while read -r kind name; do delete_name "$kind" "$name"; done <"$WORK/names"
  fi
}

cleanup() {
  if [ "$KEEP" = 1 ]; then
    say "Kept running. Clean up with:"
    echo "   container delete --force $P-a $P-proxy"
    echo "   container volume delete $P-work $P-state"
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

wait_for() {
  i=0
  while ! "$@" >/dev/null 2>&1 && [ $i -lt 60 ]; do sleep 1; i=$((i + 1)); done
  "$@" >/dev/null 2>&1
}

proxy_healthy() { container exec "$P-proxy" wget -q -O /dev/null http://127.0.0.1:9090/healthz; }

# IPv4 address of a container on the internal network.
ip_of() {
  container exec "$1" ip -4 -o addr 2>/dev/null | awk '{print $4}' | cut -d/ -f1 | grep "$SUBNET_RE"
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
      warn: false
      domains:
EOF
    for d in $DOMAINS; do echo "        - \"$d\""; done
    echo "log:"
    echo "  level: info"
  } >"$WORK/proxy/proxy.yaml"
}

# --- Preconditions ------------------------------------------------------------

say "Preconditions"
command -v container >/dev/null || { echo "container CLI not found"; exit 1; }
info "$(container --version 2>&1 | head -1)"
info "$(sw_vers -productVersion 2>/dev/null), $(git --version)"
container system status >/dev/null 2>&1 || { echo "Run 'container system start' first"; exit 1; }
remove_all

SECONDS=0
check "build image (default image + sudo env_keep)" ok container build -t "$IMG" "$HERE/container"
info "build took ${SECONDS}s"
check "pull iron-proxy pinned by digest" ok container image pull "$IRON"

# --- 1. Repository path -------------------------------------------------------

say "1. Repository path (host)"
common() { (cd "$1" && git rev-parse --path-format=absolute --git-common-dir); }
toplevel() { (cd "$1" && git rev-parse --show-toplevel); }

REPO="$WORK/Case/Code/App"
mkdir -p "$REPO/sub"
git init -q "$REPO"
git -C "$REPO" -c user.name=spike -c user.email=spike@example.com commit -q --allow-empty -m init
ORIG="$(common "$REPO")"
info "\$WORK:                $WORK"
info "\$WORK, pwd -P:        $(cd "$WORK" && pwd -P)"
info "common dir:           $ORIG"
info "toplevel:             $(toplevel "$REPO")"

LOWER="$WORK/case/code/app"
if [ -d "$LOWER" ]; then
  info "common dir, cd lower: $(common "$LOWER")"
  info "toplevel, cd lower:   $(toplevel "$LOWER")"
  info "from a subdir, lower: $(common "$LOWER/sub")"
  info "pwd -P, lower:        $(cd "$LOWER" && pwd -P)"
  info "realpath, lower:      $(realpath "$LOWER" 2>&1)"
  check "same common dir after cd with another case" ok [ "$(common "$LOWER")" = "$ORIG" ]
  check "same common dir from a subdir with another case" ok [ "$(common "$LOWER/sub")" = "$ORIG" ]
else
  info "file system is case-sensitive, case not checked"
fi
git -C "$REPO" worktree add -q "$WORK/wt" -b wt
info "common dir, worktree: $(common "$WORK/wt")"
check "worktree reports the main worktree's common dir" ok [ "$(common "$WORK/wt")" = "$ORIG" ]
PHYS="$(cd "$WORK" && pwd -P)/Case/Code/App"
check "same common dir via the physical path" ok [ "$(common "$PHYS")" = "$ORIG" ]

# --- 2. Name limits -----------------------------------------------------------

say "2. Name limits (lowercase letters, digits, dashes)"

name_of() {
  s="$P-"
  while [ ${#s} -lt "$1" ]; do s="${s}abcdefghi0"; done
  printf '%s' "${s:0:$1}"
}

# try_name <kind> <length>: creates (containers: runs) and deletes a name.
try_name() {
  nm="$(name_of "$2")"
  echo "$1 $nm" >>"$WORK/names"
  case "$1" in
    container) container run --name "$nm" "$IMG" true ;;
    network)   container network create "$nm" ;;
    volume)    container volume create "$nm" ;;
  esac >"$WORK/name.log" 2>&1
  rc=$?
  delete_name "$1" "$nm"
  return $rc
}

limit() {
  kind="$1"; ok=0; bad=""; err=""
  for n in 32 63 64 128 255 256 512; do
    if try_name "$kind" "$n"; then ok=$n; else bad=$n; err="$(head -1 "$WORK/name.log")"; break; fi
  done
  if [ -z "$bad" ]; then
    find_ "$kind names: up to 512 chars work"
    return
  fi
  lo=$ok; hi=$bad
  while [ $((hi - lo)) -gt 1 ]; do
    mid=$(((lo + hi) / 2))
    if try_name "$kind" "$mid"; then lo=$mid; else hi=$mid; err="$(head -1 "$WORK/name.log")"; fi
  done
  find_ "$kind names: max $lo chars; $hi fails: $err"
}

limit container
limit network
limit volume
check "a name with digits and dashes works (container)" ok try_name container 40

# --- 3. Proxy and crannog -----------------------------------------------------

say "3. Proxy (256M) and crannog (4 CPUs, 4G)"
mkdir -p "$WORK/ca" "$WORK/proxy" "$WORK/crannog-ca"
# crannog creates the agent homes on the host (CODEX_HOME must exist).
for a in claude codex opencode pi; do mkdir -p "$WORK/agents/$a"; done
check "generate CA" ok \
  container run --rm -v "$WORK/ca:/out" "$IRON" generate-ca -outdir /out -name "$CA_NAME"
cp "$WORK/ca/ca.crt" "$WORK/ca/ca.key" "$WORK/proxy/"
cp "$WORK/ca/ca.crt" "$WORK/crannog-ca/"
write_proxy_config
check "create internal network" ok container network create --internal --subnet "$SUBNET" "$NET"
check "start proxy with --memory 256M" ok \
  container run -d --name "$P-proxy" --memory 256M --network default --network "$NET" \
    -v "$WORK/proxy:/etc/iron-proxy" -e "IRON_MANAGEMENT_API_KEY=$MGMT_KEY" \
    "$IRON" -config /etc/iron-proxy/proxy.yaml
check "proxy healthy" ok wait_for proxy_healthy
PROXY_IP="$(ip_of "$P-proxy")"
PROXY="http://$PROXY_IP:8080"
info "proxy: $PROXY"

container volume create "$P-work" >/dev/null
container volume create "$P-state" >/dev/null
check "start crannog" ok \
  container run -d --name "$P-a" --cpus 4 --memory 4G --network "$NET" \
    -v "$P-work:/work" -v "$P-state:/state" \
    -v "$WORK/agents:$AGENTS_IN" \
    -v "$WORK/crannog-ca:/etc/crannog:ro" \
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
check "crannog ready" ok wait_for container exec "$P-a" test -f /tmp/ready
container exec --user agent "$P-a" sh -c "mkdir -p '$REPO_IN' && git init -q '$REPO_IN'"
info "crannog: $(in_a 'nproc') CPUs, $(in_a "free -m | awk '/^Mem:/ {print \$2}'") MiB"

# --- 4. Plain HTTP and apt ----------------------------------------------------

say "4. Plain HTTP via the proxy, sudo apt-get"
check "plain HTTP to an allowed host (deb.debian.org)" ok \
  in_a 'curl -fsS -m 30 -o /dev/null http://deb.debian.org/debian/dists/trixie/Release'
check "plain HTTP to another host is blocked (example.com)" fail \
  in_a 'curl -fsS -m 30 -o /dev/null http://example.com/'
check "HTTPS to another host is blocked (example.com)" fail \
  in_a 'curl -fsS -m 30 -o /dev/null https://example.com/'
check "sudoers valid" ok in_a 'sudo visudo -c'
check "sudo keeps all six proxy variables" ok in_a '[ "$(sudo env | grep -ci "_proxy=")" -eq 6 ]'
SECONDS=0
check "sudo apt-get update (http sources)" ok \
  in_a 'sudo apt-get update -o APT::Update::Error-Mode=any'
info "apt-get update took ${SECONDS}s"
check "sudo apt-get install jq" ok \
  in_a 'sudo apt-get install -y --no-install-recommends jq && jq --version'
check "without the proxy variables apt-get fails (no direct egress)" fail \
  in_a 'sudo timeout 60 env -u http_proxy -u HTTP_PROXY apt-get update -o APT::Update::Error-Mode=any'
in_a "sudo sed -i 's|http://deb.debian.org|https://deb.debian.org|' /etc/apt/sources.list.d/debian.sources"
probe "sudo apt-get update with https sources" \
  in_a 'sudo apt-get update -o APT::Update::Error-Mode=any'

# --- 5. Exit codes ------------------------------------------------------------

say "5. container exec exit codes"
c="$(code container exec "$P-a" sh -c 'exit 42')"
check "exit code of the command (42, got $c)" ok [ "$c" = 42 ]
c="$(code container exec --user agent --workdir "$REPO_IN" "$P-a" zsh -lc 'exit 7')"
check "exit code through a zsh login shell (7, got $c)" ok [ "$c" = 7 ]
info "with a tty (-t, exit 3): $(code container exec -t "$P-a" sh -c 'exit 3')"
info "killed by SIGTERM: $(code container exec "$P-a" sh -c 'kill -TERM $$')"
info "command not found: $(code container exec "$P-a" no-such-command)"

# --- 6. Agent dirs in /state --------------------------------------------------

say "6. Codex and OpenCode dirs in /state"
check "entrypoint created /state/codex and /state/opencode" ok \
  in_a 'test -d /state/codex && test -d /state/opencode'
info "codex $(in_a 'codex --version 2>&1 | head -1'), opencode $(in_a 'opencode --version 2>&1 | head -1')"
# Without login: each run fails at the model call, but initializes its state.
in_a 'timeout 60 codex exec hi </dev/null'   >"$WORK/codex.log" 2>&1
in_a 'timeout 60 opencode run hi </dev/null' >"$WORK/opencode.log" 2>&1
for a in codex opencode; do
  info "$a without login: $(head -c 200 "$WORK/$a.log" | tr '\n' ' ')"
done
probe "Codex SQLite files in /state/codex" in_a 'ls /state/codex/*.sqlite'
probe "OpenCode DB in /state/opencode" in_a 'test -e /state/opencode/opencode.db'
info "/state:"
in_a 'cd /state && find . -maxdepth 2 2>/dev/null | sort' | sed 's/^/        /'

# --- 7. Proxy memory ----------------------------------------------------------

say "7. Proxy memory under load"
proxy_mem() {
  container exec "$P-proxy" sh -c 'for p in /proc/[0-9]*; do
      [ "$(cat "$p/comm" 2>/dev/null)" = iron-proxy ] && grep -E "VmRSS|VmHWM" "$p/status"
    done; grep MemTotal /proc/meminfo' 2>&1 | tr -s ' \n' ' '
}
info "proxy before: $(proxy_mem)"
TGZ="https://registry.npmjs.org/typescript/-/typescript-5.9.2.tgz"
SECONDS=0
in_a "seq 1 40 | xargs -P 20 -I{} curl -sS -m 300 -o /dev/null -w '%{http_code}\n' $TGZ" >"$WORK/load.log" 2>&1
OK200="$(grep -c '^200$' "$WORK/load.log")"
info "40 downloads, 20 in parallel, took ${SECONDS}s"
check "all 40 large HTTPS downloads via MITM succeed (got $OK200)" ok [ "$OK200" -eq 40 ]
SECONDS=0
check "npm install with many packages via the proxy" ok \
  in_a 'mkdir -p /tmp/npm && cd /tmp/npm && npm install --no-audit --no-fund eslint typescript vite @types/node'
info "npm install took ${SECONDS}s"
check "proxy still healthy" ok proxy_healthy
info "proxy after: $(proxy_mem)"
info "container stats: $(container stats --no-stream "$P-proxy" 2>&1 | tr -s ' \n' ' ')"

# --- 8. Proxy IP -------------------------------------------------------------

# The crannog reaches the proxy by IP here; the IP is not stable, which is why
# crannogs reach it by name (network.sh).
say "8. Proxy IP after stop/start"
A_IP="$(ip_of "$P-a")"
info "before: proxy $PROXY_IP, crannog $A_IP"

# Order of auto-stop: crannog, then proxy. Start: proxy first.
container stop "$P-a" >/dev/null 2>&1
container stop "$P-proxy" >/dev/null 2>&1
check "start proxy again" ok container start "$P-proxy"
check "proxy healthy again" ok wait_for proxy_healthy
IP1="$(ip_of "$P-proxy")"
probe "proxy keeps its IP when started first (got $IP1)" [ "$IP1" = "$PROXY_IP" ]
check "start crannog again" ok container start "$P-a"
check "crannog running" ok wait_for in_a true
info "crannog after restart: $(ip_of "$P-a")"
probe "crannog reaches the proxy with its old proxy URL" \
  in_a 'curl -sS -m 30 -o /dev/null https://api.anthropic.com'

# Reverse order, e.g. after a crash between the two starts.
container stop "$P-a" >/dev/null 2>&1
container stop "$P-proxy" >/dev/null 2>&1
container start "$P-a" >/dev/null 2>&1
wait_for in_a true
container start "$P-proxy" >/dev/null 2>&1
wait_for proxy_healthy
IP2="$(ip_of "$P-proxy")"
info "crannog started first: proxy $IP2, crannog $(ip_of "$P-a")"
probe "proxy keeps its IP when the crannog starts first" [ "$IP2" = "$PROXY_IP" ]

# --- Summary ------------------------------------------------------------------

say "Summary: $PASS passed, $FAIL failed"
echo "   Images kept for faster reruns. Remove with:"
echo "   container image delete $IMG $IRON"

[ "$FAIL" -eq 0 ]
