#!/bin/sh
# Trust the crannog's CA (mounted per crannog), then idle.
set -eu

if [ -s /home/agent/.crannog/ca/ca.crt ]; then
  sudo cp /home/agent/.crannog/ca/ca.crt /usr/local/share/ca-certificates/crannog.crt
  sudo update-ca-certificates >/dev/null 2>&1
fi
# Make proxy and CA settings available to login shells, in case
# `container exec` does not inherit the environment of `container run`.
env | grep -E '^(HTTPS?_PROXY|https?_proxy|NO_PROXY|no_proxy|NODE_EXTRA_CA_CERTS|SSL_CERT_FILE|CODEX_CA_CERTIFICATE)=' \
  | sed 's/^/export /' | sudo tee /etc/profile.d/crannog.sh >/dev/null
touch /tmp/ready
exec sleep infinity
