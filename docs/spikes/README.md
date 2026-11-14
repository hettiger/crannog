# Spikes

Scripts and results that confirmed assumptions of ADRs on Apple Containers.
Rerun them after updating the runtime or a pinned image.

| Spike | Confirms |
| ----- | -------- |
| [apple-containers](apple-containers/README.md) | Egress only via the proxy, no LAN access, start time ([ADR 0008](../decisions/0008-apple-containers-runtime.md)) |
| [iron-proxy-and-agent-home](iron-proxy-and-agent-home/README.md) | iron-proxy features ([ADR 0009](../decisions/0009-iron-proxy-proxy-causeway.md)), shared agent logins ([ADR 0011](../decisions/0011-agent-home.md)) |
| [git-causeway](git-causeway/README.md) | The remote helper over `container exec` ([ADR 0015](../decisions/0015-git-causeway.md)) |
| [agents-default-image](agents-default-image/README.md) | Four agents in one image, agent homes via variables ([ADR 0011](../decisions/0011-agent-home.md), [ADR 0028](../decisions/0028-agents-in-the-default-image.md)) |
| [default-image-and-network](default-image-and-network/README.md) | Proxy address ([ADR 0010](../decisions/0010-proxy-address.md)), name limits ([ADR 0017](../decisions/0017-one-crannog-per-start-branch.md)), `sudo apt-get` via the proxy ([ADR 0028](../decisions/0028-agents-in-the-default-image.md)), proxy memory ([ADR 0009](../decisions/0009-iron-proxy-proxy-causeway.md)) |
