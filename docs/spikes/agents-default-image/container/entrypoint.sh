#!/bin/sh
# Trust the crannog's CA (mounted per crannog), own the volumes, then idle.
set -eu

if [ -s /etc/crannog/ca.crt ]; then
  sudo cp /etc/crannog/ca.crt /usr/local/share/ca-certificates/crannog.crt
  sudo update-ca-certificates >/dev/null 2>&1
fi
sudo chown agent:agent /work /state
touch /tmp/ready
exec sleep infinity
