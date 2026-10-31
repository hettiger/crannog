#!/bin/bash
#
# Spike for ADR 0006: Apple Containers as crannog runtime.
#
# Verifies on macOS 26 / Apple Silicon:
#   1. Image build from a Dockerfile outside any project dir, startup time.
#   2. Egress enforcement: agent on an --internal network, proxy sidecar on
#      internal + default network, domain allowlist, live allowlist change,
#      no bypass even as root.
#   3. Host exposure: what the agent can reach on the host.
#   4. Git causeway over SSH: push into the crannog, agent commits, fetch back.
#
# Run directly on the Mac (not inside a macOS VM: no nested virtualization).
# Usage: ./spike.sh [--keep]   (--keep leaves containers running for poking)

set -u

HERE="$(cd "$(dirname "$0")" && pwd)"
P="crannog-spike"
NET="$P-net"
SUBNET="192.168.234.0/24"
GW="192.168.234.1"
SSH_PORT=22234
HOST_PORT_ALL=18234
HOST_PORT_LO=18235
KEEP=0
[ "${1:-}" = "--keep" ] && KEEP=1

PASS=0
FAIL=0
LISTENER_PIDS=""
WORK="$(mktemp -d -t crannog-spike)"

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

cleanup() {
  for pid in $LISTENER_PIDS; do kill "$pid" 2>/dev/null; done
  if [ "$KEEP" = 1 ]; then
    say "Kept running. Clean up with:"
    echo "   container delete --force $P-agent $P-proxy; container network delete $NET"
    echo "   ssh key: $WORK/id_ed25519"
    return
  fi
  container delete --force "$P-agent" "$P-proxy" >/dev/null 2>&1
  container network delete "$NET" >/dev/null 2>&1
  rm -rf "$WORK"
}
trap cleanup EXIT

agent()      { container exec --user agent "$P-agent" "$@"; }
agent_root() { container exec "$P-agent" "$@"; }

# --- Preconditions ----------------------------------------------------------

say "Preconditions"
[ "$(uname -m)" = "arm64" ] || { echo "Apple Silicon required"; exit 1; }
major="$(sw_vers -productVersion | cut -d. -f1)"
[ "$major" -ge 26 ] || { echo "macOS 26 required (found $(sw_vers -productVersion))"; exit 1; }
command -v container >/dev/null || { echo "container CLI not found"; exit 1; }
info "$(container --version 2>&1 | head -1)"
container system status >/dev/null 2>&1 || { echo "Run 'container system start' first"; exit 1; }

# Leftovers from an aborted run.
container delete --force "$P-agent" "$P-proxy" >/dev/null 2>&1
container network delete "$NET" >/dev/null 2>&1

ssh-keygen -q -t ed25519 -N '' -f "$WORK/id_ed25519"
cp -R "$HERE/agent" "$WORK/agent"
cp "$WORK/id_ed25519.pub" "$WORK/agent/authorized_keys"

# --- 1. Build -----------------------------------------------------------------

say "1. Build images (context outside any project dir)"
SECONDS=0
check "build proxy image" ok container build -t "$P-proxy:spike" "$HERE/proxy"
check "build agent image" ok container build -t "$P-agent:spike" "$WORK/agent"
info "build took ${SECONDS}s"

# --- 2. Network + egress ------------------------------------------------------

say "2. Network and egress enforcement"
check "create internal network $SUBNET" ok \
  container network create --internal --subnet "$SUBNET" "$NET"

# Default network first: the first network provides the default route.
check "start proxy (default + internal network)" ok \
  container run -d --name "$P-proxy" --network default --network "$NET" "$P-proxy:spike"

PROXY_IP="$(container exec "$P-proxy" ip -4 -o addr | awk '{print $4}' | cut -d/ -f1 | grep '^192\.168\.234\.')"
info "proxy internal IP: ${PROXY_IP:-<unknown>}"
container exec "$P-proxy" ip -4 -o addr 2>/dev/null | awk '{print "   info: proxy if " $2 " " $4}'
container exec "$P-proxy" ip route 2>/dev/null | sed 's/^/   info: proxy route /'
PROXY="http://$PROXY_IP:8888"

SECONDS=0
check "start agent (internal network only, ssh on 127.0.0.1:$SSH_PORT)" ok \
  container run -d --name "$P-agent" --network "$NET" -p "127.0.0.1:$SSH_PORT:22" "$P-agent:spike"
i=0
while ! nc -z 127.0.0.1 "$SSH_PORT" >/dev/null 2>&1 && [ $i -lt 60 ]; do sleep 1; i=$((i + 1)); done
info "agent start until ssh port open: ${SECONDS}s"
agent_root ip -4 -o addr 2>/dev/null | awk '{print "   info: agent if " $2 " " $4}'
agent_root ip route 2>/dev/null | sed 's/^/   info: agent route /'

check "direct HTTPS to example.com is blocked"   fail agent curl -sS -m 8 -o /dev/null https://example.com
check "direct TCP to 1.1.1.1:443 is blocked"      fail agent nc -z -w 5 1.1.1.1 443
check "via proxy: api.github.com allowed"         ok   agent curl -sS -m 15 -o /dev/null -x "$PROXY" https://api.github.com
check "via proxy: example.com denied with 403"    ok   agent sh -c \
  "curl -s -m 15 -o /dev/null -w '%{http_connect}' -x '$PROXY' https://example.com | grep -qx 403"
probe "root can change routes (NET_ADMIN)" agent_root ip route replace default via "$PROXY_IP"
check "root + default route via proxy: still blocked" fail \
  agent_root curl -sS -m 8 -o /dev/null https://example.com

container exec "$P-proxy" sh -c \
  "echo '^example\\.com\$' >> /etc/tinyproxy/filter && pkill -USR1 tinyproxy" >/dev/null 2>&1
sleep 2
check "live allowlist change: example.com allowed without restart" ok \
  agent curl -sS -m 15 -o /dev/null -x "$PROXY" https://example.com

# --- 3. Host exposure ---------------------------------------------------------

say "3. Host exposure (macOS may ask to allow incoming connections for nc)"
nc -lk "$HOST_PORT_ALL" >/dev/null 2>&1 & LISTENER_PIDS="$LISTENER_PIDS $!"; disown
nc -lk 127.0.0.1 "$HOST_PORT_LO" >/dev/null 2>&1 & LISTENER_PIDS="$LISTENER_PIDS $!"; disown
sleep 1
probe "reach host service on 0.0.0.0 via gateway $GW"        agent nc -z -w 3 "$GW" "$HOST_PORT_ALL"
probe "reach host service on 127.0.0.1 via gateway $GW"      agent nc -z -w 3 "$GW" "$HOST_PORT_LO"
LAN_IP="$(ipconfig getifaddr en0 2>/dev/null || true)"
if [ -n "$LAN_IP" ]; then
  probe "reach host service on 0.0.0.0 via LAN IP $LAN_IP"  agent nc -z -w 3 "$LAN_IP" "$HOST_PORT_ALL"
fi
ROUTER_IP="$(route -n get default 2>/dev/null | awk '/gateway:/{print $2}')"
if [ -n "$ROUTER_IP" ]; then
  for port in 80 53; do
    probe "control: host reaches LAN router $ROUTER_IP:$port" nc -z -w 3 "$ROUTER_IP" "$port"
    probe "reach LAN router $ROUTER_IP:$port"                agent nc -z -w 3 "$ROUTER_IP" "$port"
  done
fi
probe "reach proxy container on port 22"             agent nc -z -w 3 "$PROXY_IP" 22

# --- 4. Git causeway over SSH -------------------------------------------------

say "4. Git causeway over SSH"
export GIT_SSH_COMMAND="ssh -F /dev/null -i $WORK/id_ed25519 -o IdentitiesOnly=yes -o IdentityAgent=none -o ForwardAgent=no -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR"
REPO="$WORK/host-repo"
git init -q -b main "$REPO"
git -C "$REPO" -c user.name=User -c user.email=user@crannog.invalid commit -q --allow-empty -m "Initial commit"
echo "hello" > "$REPO/README.md"
git -C "$REPO" add README.md
git -C "$REPO" -c user.name=User -c user.email=user@crannog.invalid commit -q -m "Add README"
git -C "$REPO" remote add crannog "ssh://agent@127.0.0.1:$SSH_PORT/work/repo"

check "ssh login as agent" ok $GIT_SSH_COMMAND -p "$SSH_PORT" agent@127.0.0.1 true
check "push main into crannog" ok git -C "$REPO" push -q crannog main
check "worktree in crannog updated (updateInstead)" ok agent test -f /work/repo/README.md
check "agent commits inside crannog" ok container exec --user agent --workdir /work/repo "$P-agent" \
  sh -c "echo agent > agent.txt && git add agent.txt && git commit -qm 'Agent change'"
check "user fetches agent's commit" ok sh -c \
  "git -C '$REPO' fetch -q crannog && git -C '$REPO' log --oneline crannog/main | grep -q 'Agent change'"
git -C "$REPO" merge -q --ff-only crannog/main >/dev/null 2>&1
echo "more" >> "$REPO/README.md"
git -C "$REPO" -c user.name=User -c user.email=user@crannog.invalid commit -qam "User change"
check "push follow-up while agent tree is clean" ok git -C "$REPO" push -q crannog main
check "follow-up visible in crannog worktree" ok agent grep -q more /work/repo/README.md

# --- Summary ------------------------------------------------------------------

say "Summary: $PASS passed, $FAIL failed"
echo "   Images kept for faster reruns. Remove with:"
echo "   container image delete $P-agent:spike $P-proxy:spike"
[ "$FAIL" -eq 0 ]
