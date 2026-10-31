#!/bin/bash
#
# Spike for ADR 0007 and ADR 0008: iron-proxy and the agent home on Apple Containers.
#
# Verifies on macOS 26 / Apple Silicon:
#   1. Build the agent image, pull iron-proxy pinned by digest.
#   2. CA per crannog generated on the host; the host does not trust it.
#   3. iron-proxy as explicit proxy: MITM with that CA, domain allowlist, live
#      reload, warn mode for opening the proxy, management/metrics not exposed.
#   4. Credential injection: inject mode, replace mode (placeholder swap) with
#      require, secret rotation via file + reload.
#   5. Agent user and agent home: non-root user with passwordless sudo, bind
#      mount ownership and write access, sharing between two crannogs,
#      symlinks into the agent home, Claude Code and Codex installed.
#   6. (--manual) Real logins with Claude and ChatGPT subscriptions.
#
# Run directly on the Mac (not inside a macOS VM: no nested virtualization).
# Usage: ./spike.sh [--keep | --manual]
#   --keep    leave containers running for poking
#   --manual  like --keep, then print the manual login steps

set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
P="crannog-spike2"
NET="$P-net"
SUBNET="192.168.235.0/24"
IRON="docker.io/ironsh/iron-proxy:0.54.0@sha256:6d074be2de9188f88d08013788d5500fb4de8557f16c2e5b27af4d8d2cb1e3fb"
CA_NAME="crannog spike CA"
MGMT_KEY="spike-management-key"
PLACEHOLDER="crannog-placeholder-token"
SECRET_INJECTED="real-injected-$RANDOM$RANDOM"
SECRET_SWAPPED="real-swapped-$RANDOM$RANDOM"
SECRET_ROTATED="real-rotated-$RANDOM$RANDOM"
BASE_DOMAINS="api.github.com *.anthropic.com *.claude.ai *.claude.com
  *.claudeusercontent.com *.openai.com *.chatgpt.com registry.npmjs.org
  postman-echo.com"

KEEP=0
MANUAL=0
case "${1:-}" in
  --keep) KEEP=1 ;;
  --manual) KEEP=1; MANUAL=1 ;;
esac

PASS=0
FAIL=0
WORK="$(mktemp -d -t crannog-spike2)"

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
  container delete --force "$P-agent-a" "$P-agent-b" "$P-proxy" >/dev/null 2>&1
  container network delete "$NET" >/dev/null 2>&1
}

cleanup() {
  if [ "$KEEP" = 1 ]; then
    say "Kept running. Clean up with:"
    echo "   container delete --force $P-agent-a $P-agent-b $P-proxy; container network delete $NET; rm -rf $WORK"
    return
  fi
  remove_all
  rm -rf "$WORK"
}
trap cleanup EXIT

# Commands in an agent run in a login shell, so /etc/profile.d applies.
in_a() { container exec --user agent "$P-agent-a" bash -lc "$1"; }
in_b() { container exec --user agent "$P-agent-b" bash -lc "$1"; }

proxy_reload() {
  container exec "$P-proxy" wget -q -O - --post-data='' \
    --header "Authorization: Bearer $MGMT_KEY" http://127.0.0.1:9092/v1/reload
}

# write_config <extra domains> <warn: true|false>
write_config() {
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
      warn: $2
      domains:
EOF
    for d in $BASE_DOMAINS $1; do echo "        - \"$d\""; done
    cat <<EOF
  - name: secrets
    config:
      secrets:
        - source: { type: file, path: /etc/iron-proxy/secrets/injected }
          inject: { header: X-Crannog-Injected }
          rules: [{ host: postman-echo.com, paths: ["/get"] }]
        - source: { type: file, path: /etc/iron-proxy/secrets/swapped }
          replace: { proxy_value: "$PLACEHOLDER", match_headers: ["Authorization"], require: true }
          rules: [{ host: postman-echo.com, paths: ["/headers"] }]
log:
  level: info
EOF
  } > "$WORK/proxy/proxy.yaml.tmp"
  mv "$WORK/proxy/proxy.yaml.tmp" "$WORK/proxy/proxy.yaml"
}

# write_secret <name> <value>: atomic, no trailing newline.
write_secret() {
  printf '%s' "$2" > "$WORK/proxy/secrets/$1.tmp"
  mv "$WORK/proxy/secrets/$1.tmp" "$WORK/proxy/secrets/$1"
}

wait_for() {
  i=0
  while ! "$@" >/dev/null 2>&1 && [ $i -lt 60 ]; do sleep 1; i=$((i + 1)); done
  "$@" >/dev/null 2>&1
}

# --- Preconditions ----------------------------------------------------------

say "Preconditions"
[ "$(uname -m)" = "arm64" ] || { echo "Apple Silicon required"; exit 1; }
major="$(sw_vers -productVersion | cut -d. -f1)"
[ "$major" -ge 26 ] || { echo "macOS 26 required (found $(sw_vers -productVersion))"; exit 1; }
command -v container >/dev/null || { echo "container CLI not found"; exit 1; }
info "$(container --version 2>&1 | head -1)"
container system status >/dev/null 2>&1 || { echo "Run 'container system start' first"; exit 1; }
remove_all

mkdir -p "$WORK/ca" "$WORK/proxy/secrets" "$WORK/crannog-a" "$WORK/crannog-b" "$WORK/agents"

# --- 1. Images ----------------------------------------------------------------

say "1. Images"
SECONDS=0
check "build agent image (Claude Code, Codex)" ok container build -t "$P-agent:spike" "$HERE/agent"
info "build took ${SECONDS}s"
check "pull iron-proxy pinned by digest" ok container image pull "$IRON"

# --- 2. CA per crannog --------------------------------------------------------

say "2. CA per crannog, generated on the host"
check "generate CA via iron-proxy generate-ca into a host dir" ok \
  container run --rm -v "$WORK/ca:/out" "$IRON" generate-ca -outdir /out -name "$CA_NAME"
check "ca.crt and ca.key exist on the host" ok test -s "$WORK/ca/ca.crt" -a -s "$WORK/ca/ca.key"
info "CA files on host: $(stat -f '%Su:%Sg %Lp' "$WORK/ca/ca.key" 2>/dev/null) (written by root in a container)"
cp "$WORK/ca/ca.crt" "$WORK/ca/ca.key" "$WORK/proxy/"
cp "$WORK/ca/ca.crt" "$WORK/crannog-a/"
cp "$WORK/ca/ca.crt" "$WORK/crannog-b/"
check "host keychains do not contain the CA" fail security find-certificate -c "$CA_NAME"

# --- 3. Proxy -----------------------------------------------------------------

say "3. iron-proxy"
write_config "" false
write_secret injected "$SECRET_INJECTED"
write_secret swapped "$SECRET_SWAPPED"
check "create internal network $SUBNET" ok \
  container network create --internal --subnet "$SUBNET" "$NET"
check "start proxy (default + internal network)" ok \
  container run -d --name "$P-proxy" --network default --network "$NET" \
    -v "$WORK/proxy:/etc/iron-proxy" -e "IRON_MANAGEMENT_API_KEY=$MGMT_KEY" \
    "$IRON" -config /etc/iron-proxy/proxy.yaml
check "proxy healthy" ok wait_for \
  container exec "$P-proxy" wget -q -O /dev/null http://127.0.0.1:9090/healthz
PROXY_IP="$(container exec "$P-proxy" ip -4 -o addr | awk '{print $4}' | cut -d/ -f1 | grep '^192\.168\.235\.')"
info "proxy internal IP: ${PROXY_IP:-<unknown>}"
PROXY="http://$PROXY_IP:8080"

# --- 4. Agents ----------------------------------------------------------------

say "4. Two crannogs sharing the agent home"
for x in a b; do
  check "start agent $x (internal network only)" ok \
    container run -d --name "$P-agent-$x" --network "$NET" \
      -v "$WORK/agents:/home/agent/.crannog/agents" \
      -v "$WORK/crannog-$x:/home/agent/.crannog/ca" \
      -e "HTTPS_PROXY=$PROXY" -e "https_proxy=$PROXY" \
      -e "HTTP_PROXY=$PROXY" -e "http_proxy=$PROXY" \
      -e "NO_PROXY=localhost,127.0.0.1,::1" -e "no_proxy=localhost,127.0.0.1,::1" \
      -e "NODE_EXTRA_CA_CERTS=/home/agent/.crannog/ca/ca.crt" \
      -e "SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt" \
      -e "CODEX_CA_CERTIFICATE=/etc/ssl/certs/ca-certificates.crt" \
      "$P-agent:spike"
  check "agent $x ready (CA installed)" ok wait_for \
    container exec "$P-agent-$x" test -f /tmp/ready
done
probe "env of container run visible in container exec (without login shell)" \
  container exec --user agent "$P-agent-a" sh -c 'test "$HTTPS_PROXY" = "'"$PROXY"'"'

# --- 5. Egress via iron-proxy -------------------------------------------------

say "5. Egress via iron-proxy"
check "direct HTTPS (no proxy) is blocked" fail \
  in_a 'curl -sS -m 8 --noproxy "*" -o /dev/null https://example.com'
check "api.github.com allowed, MITM cert trusted via system store" ok \
  in_a 'curl -sS -m 20 -o /dev/null https://api.github.com'
check "leaf cert issued by the crannog CA" ok \
  in_a 'curl -sv -m 20 -o /dev/null https://api.github.com 2>&1 | grep -qi "issuer:.*'"$CA_NAME"'"'
check "example.com denied with 403" ok \
  in_a 'curl -s -m 20 -o /dev/null -w "%{http_connect}" https://example.com | grep -qx 403'
check "agent cannot reach proxy metrics (9090)" fail in_a "nc -z -w 3 $PROXY_IP 9090"
check "agent cannot reach proxy management (9092)" fail in_a "nc -z -w 3 $PROXY_IP 9092"

write_config "example.com" false
check "live reload via management API" ok proxy_reload
check "example.com allowed after reload, no restart" ok \
  in_a 'curl -sS -m 20 -o /dev/null https://example.com'

write_config "" true
proxy_reload >/dev/null 2>&1
check "open proxy (warn: true): example.org allowed" ok \
  in_a 'curl -sS -m 20 -o /dev/null https://example.org'
check "warned request for example.org in proxy log" ok \
  sh -c "container logs $P-proxy 2>&1 | grep 'example.org' | grep -qi warn"

write_config "" false
proxy_reload >/dev/null 2>&1
check "closed again: example.org denied" ok \
  in_a 'curl -s -m 20 -o /dev/null -w "%{http_connect}" https://example.org | grep -qx 403'

# --- 6. Credential injection --------------------------------------------------

say "6. Credential injection (secrets never in the crannog)"
check "inject: header set on matching host + path" ok \
  in_a "curl -sS -m 20 https://postman-echo.com/get | grep -q $SECRET_INJECTED"
check "replace: placeholder swapped for the real secret" ok \
  in_a "curl -sS -m 20 -H 'Authorization: Bearer $PLACEHOLDER' https://postman-echo.com/headers | grep -q $SECRET_SWAPPED"
check "require: request without placeholder rejected with 403" ok \
  in_a 'curl -s -m 20 -o /dev/null -w "%{http_code}" https://postman-echo.com/headers | grep -qx 403'
check "inject scoped by path: not added to /headers" fail \
  in_a "curl -sS -m 20 -H 'Authorization: Bearer $PLACEHOLDER' https://postman-echo.com/headers | grep -q $SECRET_INJECTED"
check "secret values not present in the agent's files or env" fail \
  in_a "{ env; cat /home/agent/.crannog/ca/* 2>/dev/null; } | grep -q 'real-'"
write_secret swapped "$SECRET_ROTATED"
proxy_reload >/dev/null 2>&1
check "rotation: rewritten secret file used after reload" ok \
  in_a "curl -sS -m 20 -H 'Authorization: Bearer $PLACEHOLDER' https://postman-echo.com/headers | grep -q $SECRET_ROTATED"

# --- 7. Agent user and agent home ---------------------------------------------

say "7. Agent user and agent home (bind mount)"
check "agent user is not root" ok in_a 'test "$(id -u)" != 0'
check "passwordless sudo" ok in_a 'sudo -n true'
info "agent groups: $(in_a 'id -Gn' 2>/dev/null)"
info "mount as seen in crannog: $(in_a 'stat -c "%U:%G %a" ~/.crannog/agents' 2>/dev/null)"
info "mount on host: $(stat -f '%Su:%Sg %Lp' "$WORK/agents")"

for mode in 755 775 777; do
  mkdir -p "$WORK/agents/probe-$mode" && chmod "$mode" "$WORK/agents/probe-$mode"
  probe "agent user can write a host dir with mode $mode" \
    in_a "touch ~/.crannog/agents/probe-$mode/file"
done
probe "agent can write via sudo" in_a 'sudo touch ~/.crannog/agents/probe-755/sudo-file'
for f in probe-777/file probe-755/sudo-file; do
  [ -e "$WORK/agents/$f" ] && info "file created in crannog ($f), owner on host: $(stat -f '%Su:%Sg %Lp' "$WORK/agents/$f")"
done

# Prepare agent homes with the least permissive mode that works.
AGENT_MODE=""
for mode in 755 775 777; do
  if [ -e "$WORK/agents/probe-$mode/file" ]; then AGENT_MODE="$mode"; break; fi
done
info "agent home mode used for the following checks: ${AGENT_MODE:-none works}"
mkdir -p "$WORK/agents/claude/.claude" "$WORK/agents/codex"
[ -n "$AGENT_MODE" ] && chmod -R "$AGENT_MODE" "$WORK/agents/claude" "$WORK/agents/codex"

share_a_to_b() { in_a 'echo from-a > ~/.claude/shared-test' && in_b 'grep -q from-a ~/.claude/shared-test'; }
share_b_to_a() { in_b 'echo from-b >> ~/.claude/shared-test' && in_a 'grep -q from-b ~/.claude/shared-test'; }
check "write in crannog a, visible in crannog b" ok share_a_to_b
check "write in crannog b, visible in crannog a" ok share_b_to_a
check "~/.claude resolves into the agent home" ok in_a 'test -d ~/.claude/'
check "claude --version" ok in_a 'claude --version'
check "codex --version" ok in_a 'codex --version'
in_a 'timeout 60 claude -p "say hi" </dev/null' >"$WORK/claude.log" 2>&1
info "claude -p without login: $(head -c 160 "$WORK/claude.log" | tr '\n' ' ')"
probe "claude wrote .claude.json into the agent home" test -s "$WORK/agents/claude/.claude.json"
check "~/.claude.json is still a symlink after claude ran" ok in_a 'test -L ~/.claude.json'
in_a 'ln -sfn /etc/passwd ~/.crannog/agents/claude/evil-link' >/dev/null 2>&1
probe "symlink created in crannog shows up on host as symlink to a host path" \
  test -L "$WORK/agents/claude/evil-link"
rm -f "$WORK/agents/claude/evil-link"

# --- Summary ------------------------------------------------------------------

say "Summary: $PASS passed, $FAIL failed"
echo "   Images kept for faster reruns. Remove with:"
echo "   container image delete $P-agent:spike $IRON"

if [ "$MANUAL" = 1 ]; then
  say "Manual steps: subscription logins"
  cat <<EOF
   Agent home on the host: $WORK/agents

   Claude (Pro/Max):
     1. container exec -it --user agent $P-agent-a bash -l
        claude            # /login, choose subscription, paste the code
        claude -p "say hi"
        exit
     2. ls -la $WORK/agents/claude $WORK/agents/claude/.claude
        (expect .credentials.json and .claude.json on the host)
     3. container exec -it --user agent $P-agent-b bash -l
        claude -p "say hi"   # expect: works without login
        test -L ~/.claude.json && echo still-symlink

   Codex (ChatGPT Plus/Pro):
     4. In agent a: codex login --device-auth, then codex exec "say hi"
     5. In agent b: codex exec "say hi"   # expect: works without login
     6. ls -la $WORK/agents/codex

   Proxy log (allowed/denied hosts during login):
     container logs $P-proxy | tail -50
EOF
fi

[ "$FAIL" -eq 0 ]
