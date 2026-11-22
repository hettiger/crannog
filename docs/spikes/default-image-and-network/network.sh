#!/bin/bash
#
# Spike: how a crannog reaches its proxy, DNS in a crannog, app preview basics.
#
# The proxy gets a new IP on every start, so the crannog reaches it by the
# name crannog-proxy, which crannog writes to the crannog's /etc/hosts after
# each start. Verifies on macOS 26 / Apple Silicon:
#   1. /etc/hosts in a crannog: how it is provided, writable by root.
#   2. curl, Node, sudo apt-get via http://crannog-proxy:8080, with and
#      without --no-dns.
#   3. DNS: does a crannog resolve external names (DNS tunneling past the
#      proxy)? With and without --no-dns.
#   4. Stop/start: what happens to the /etc/hosts entry, rewriting it works.
#   5. Network aliases: full error message.
#   6. App preview basics: the Mac reaches a server in a crannog by IP;
#      container DNS domain and names (read only, nothing is changed).
#
# Reuses the image of spike.sh (builds it if missing).
#
# Run directly on the Mac. Usage: ./network.sh [--keep]

set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
P="crannog-spike6"
NET="$P-net"
SUBNET="192.168.238.0/24"
SUBNET_RE='^192\.168\.238\.'
IMG="crannog-spike5-agent:spike"
IRON="docker.io/ironsh/iron-proxy:0.54.0@sha256:6d074be2de9188f88d08013788d5500fb4de8557f16c2e5b27af4d8d2cb1e3fb"
CA_NAME="crannog spike6 CA"
MGMT_KEY="spike-management-key"
DOMAINS="*.anthropic.com *.claude.ai *.claude.com *.openai.com *.chatgpt.com
  opencode.ai models.opencode.ai pi.dev registry.npmjs.org deb.debian.org"
PROXY_URL="http://crannog-proxy:8080"

KEEP=0
[ "${1:-}" = --keep ] && KEEP=1

PASS=0
FAIL=0
WORK="$(mktemp -d -t crannog-spike6)"

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

remove_all() {
  container delete --force "$P-a" "$P-b" "$P-proxy" "$P-alias" >/dev/null 2>&1
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

# run_in <a|b> <script>: as the agent user, non-login shell.
run_in() { container exec --user agent "$P-$1" bash -c "$2"; }
# as_root <a|b> <script>
as_root() { container exec --user root "$P-$1" sh -c "$2"; }

wait_for() {
  i=0
  while ! "$@" >/dev/null 2>&1 && [ $i -lt 60 ]; do sleep 1; i=$((i + 1)); done
  "$@" >/dev/null 2>&1
}

proxy_healthy() { container exec "$P-proxy" wget -q -O /dev/null http://127.0.0.1:9090/healthz; }

ip_of() {
  container exec "$1" ip -4 -o addr 2>/dev/null | awk '{print $4}' | cut -d/ -f1 | grep "$SUBNET_RE"
}

# set_hosts <a|b>: what crannog would do after each start. Rewrites the file
# in place (keeps the inode, in case it is a bind mount).
set_hosts() {
  ip="$(ip_of "$P-proxy")"
  as_root "$1" "grep -v ' crannog-proxy\$' /etc/hosts >/tmp/hosts \
    && echo '$ip crannog-proxy' >>/tmp/hosts && cat /tmp/hosts >/etc/hosts"
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
container system status >/dev/null 2>&1 || { echo "Run 'container system start' first"; exit 1; }
remove_all
if ! container image inspect "$IMG" >/dev/null 2>&1; then
  check "build image" ok container build -t "$IMG" "$HERE/container"
fi
check "pull iron-proxy pinned by digest" ok container image pull "$IRON"

mkdir -p "$WORK/ca" "$WORK/proxy" "$WORK/crannog-ca"
check "generate CA" ok \
  container run --rm -v "$WORK/ca:/out" "$IRON" generate-ca -outdir /out -name "$CA_NAME"
cp "$WORK/ca/ca.crt" "$WORK/ca/ca.key" "$WORK/proxy/"
cp "$WORK/ca/ca.crt" "$WORK/crannog-ca/"
write_proxy_config
check "create internal network" ok container network create --internal --subnet "$SUBNET" "$NET"
check "start proxy" ok \
  container run -d --name "$P-proxy" --memory 256M --network default --network "$NET" \
    -v "$WORK/proxy:/etc/iron-proxy" -e "IRON_MANAGEMENT_API_KEY=$MGMT_KEY" \
    "$IRON" -config /etc/iron-proxy/proxy.yaml
check "proxy healthy" ok wait_for proxy_healthy

# a: default DNS, b: --no-dns.
for x in a b; do
  dns=""
  [ "$x" = b ] && dns="--no-dns"
  container volume create "$P-$x-work" >/dev/null
  container volume create "$P-$x-state" >/dev/null
  check "start crannog $x ${dns:-(default DNS)}" ok \
    container run -d --name "$P-$x" $dns --network "$NET" \
      -v "$P-$x-work:/work" -v "$P-$x-state:/state" \
      -v "$WORK/crannog-ca:/etc/crannog:ro" \
      -e "HTTPS_PROXY=$PROXY_URL" -e "https_proxy=$PROXY_URL" \
      -e "HTTP_PROXY=$PROXY_URL" -e "http_proxy=$PROXY_URL" \
      -e "NO_PROXY=localhost,127.0.0.1,::1" -e "no_proxy=localhost,127.0.0.1,::1" \
      -e "NODE_EXTRA_CA_CERTS=/etc/crannog/ca.crt" \
      -e "NODE_USE_ENV_PROXY=1" \
      "$IMG"
  check "crannog $x ready" ok wait_for container exec "$P-$x" test -f /tmp/ready
done

# --- 1. /etc/hosts ------------------------------------------------------------

say "1. /etc/hosts in a crannog"
for x in a b; do
  info "crannog $x /etc/hosts: $(as_root "$x" 'cat /etc/hosts' | tr '\n' '|')"
  info "crannog $x mount: $(as_root "$x" 'grep -E " /etc/(hosts|resolv.conf) " /proc/mounts' | tr '\n' '|')"
  info "crannog $x resolv.conf: $(as_root "$x" 'grep -v "^#" /etc/resolv.conf 2>&1' | tr '\n' '|')"
  check "crannog $x: root writes crannog-proxy to /etc/hosts" ok set_hosts "$x"
done
check "agent cannot write /etc/hosts without sudo" fail run_in a 'echo x >>/etc/hosts'

# --- 2. Proxy by name ---------------------------------------------------------

say "2. Proxy via $PROXY_URL"
for x in a b; do
  check "crannog $x: crannog-proxy resolves (getent)" ok run_in "$x" 'getent hosts crannog-proxy'
  check "crannog $x: curl HTTPS via the proxy" ok \
    run_in "$x" 'curl -sS -m 30 -o /dev/null https://api.anthropic.com'
  check "crannog $x: Node fetch via the proxy" ok run_in "$x" \
    'node -e "fetch(\"https://api.anthropic.com\").then(()=>process.exit(0),e=>{console.error(e);process.exit(1)})"'
  check "crannog $x: sudo apt-get update via the proxy" ok \
    run_in "$x" 'sudo apt-get update -o APT::Update::Error-Mode=any'
done

# --- 3. DNS -------------------------------------------------------------------

say "3. DNS in a crannog (queries past the proxy)"
for x in a b; do
  for h in example.com deb.debian.org; do
    probe "crannog $x resolves $h" run_in "$x" "getent hosts $h"
  done
  probe "crannog $x resolves the proxy's container name" run_in "$x" "getent hosts $P-proxy"
done
check "crannog b (--no-dns) resolves no external name" fail run_in b 'getent hosts example.com'

# --- 4. Stop/start ------------------------------------------------------------

say "4. /etc/hosts after stop/start"
IP0="$(ip_of "$P-proxy")"
container stop "$P-a" >/dev/null 2>&1
container stop "$P-proxy" >/dev/null 2>&1
check "start proxy" ok container start "$P-proxy"
check "proxy healthy" ok wait_for proxy_healthy
check "start crannog a" ok container start "$P-a"
check "crannog a running" ok wait_for run_in a true
info "proxy IP before $IP0, after $(ip_of "$P-proxy")"
info "crannog a /etc/hosts after start: $(as_root a 'cat /etc/hosts' | tr '\n' '|')"
probe "crannog-proxy entry survived the restart" as_root a 'grep -q " crannog-proxy$" /etc/hosts'
check "rewrite /etc/hosts" ok set_hosts a
check "curl via the proxy after rewriting" ok \
  run_in a 'curl -sS -m 30 -o /dev/null https://api.anthropic.com'

# --- 5. Aliases ---------------------------------------------------------------

say "5. Network aliases"
if container run -d --name "$P-alias" --network "$NET,alias=spike-alias" "$IMG" sleep infinity \
    >"$WORK/alias.log" 2>&1; then
  find_ "--network <name>,alias=<alias> accepted"
  probe "alias resolves in crannog a" run_in a 'getent hosts spike-alias'
else
  find_ "--network <name>,alias=<alias> not accepted:"
  # Drop the progress lines ("[0/6] [0s]").
  grep -v '^\[' "$WORK/alias.log" | tail -5 | sed 's/^/        /'
fi
container delete --force "$P-alias" >/dev/null 2>&1

# --- 6. App preview basics ----------------------------------------------------

say "6. App preview basics (read only)"
CFG="$HOME/.config/container/config.toml"
if [ -f "$CFG" ]; then
  info "$CFG:"
  sed 's/^/        /' "$CFG"
else
  info "no $CFG"
fi
info "container system dns list: $(container system dns list 2>&1 | tr '\n' '|')"
DOMAIN="$(sed -n 's/^[[:space:]]*domain[[:space:]]*=[[:space:]]*"\(.*\)".*/\1/p' "$CFG" 2>/dev/null | head -1)"

for x in a b; do
  run_in "$x" "setsid -f node -e 'require(\"http\").createServer((q,r)=>r.end(\"ok\")).listen(8000,\"0.0.0.0\")' \
    >/dev/null 2>&1"
  check "crannog $x: server listening on 8000" ok wait_for run_in "$x" 'curl -sS -m 2 --noproxy "*" http://127.0.0.1:8000'
  ipx="$(ip_of "$P-$x")"
  probe "Mac reaches crannog $x by IP ($ipx:8000)" curl -sS -m 5 --noproxy "*" "http://$ipx:8000"
done
if [ -n "$DOMAIN" ]; then
  for x in a b; do
    probe "Mac resolves $P-$x.$DOMAIN" curl -sS -m 5 --noproxy "*" "http://$P-$x.$DOMAIN:8000"
    probe "crannog a resolves $P-$x.$DOMAIN" run_in a "getent hosts $P-$x.$DOMAIN"
  done
else
  info "no DNS domain configured, name checks skipped"
fi

# --- Summary ------------------------------------------------------------------

say "Summary: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
