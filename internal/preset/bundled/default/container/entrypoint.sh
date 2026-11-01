#!/bin/sh
# Prepares the crannog on every start, then idles until it is stopped.
# Everything here must be repeatable: the container's writable layer
# survives stop and start.
set -eu

# Trust the crannog's CA: the proxy intercepts TLS with it.
if [ -s /etc/crannog/ca.crt ]; then
  sudo cp /etc/crannog/ca.crt /usr/local/share/ca-certificates/crannog.crt
  sudo update-ca-certificates >/dev/null 2>&1
fi

# The volumes start empty and owned by root.
sudo chown agent:agent /work /state

# Per-crannog agent state, see CODEX_SQLITE_HOME and OPENCODE_DB in
# config.yml. OpenCode fails to start without its directory.
mkdir -p /state/codex /state/opencode

# Readiness marker: crannog waits until /tmp/ready holds the boot id of
# this start. A marker from an earlier start holds another boot id.
cat /proc/sys/kernel/random/boot_id > /tmp/ready

# Idle. As PID 1 without a handler, sleep would ignore SIGTERM and the
# crannog would only stop after the runtime's timeout.
trap 'exit 0' TERM INT
sleep infinity &
wait
