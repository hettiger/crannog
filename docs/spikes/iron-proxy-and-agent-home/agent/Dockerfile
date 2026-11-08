FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      sudo curl ca-certificates git netcat-openbsd iproute2 nodejs npm procps \
 && rm -rf /var/lib/apt/lists/*

# Codex CLI (npm package ships a native binary).
RUN npm install -g @openai/codex && npm cache clean --force

# Non-root agent user with passwordless sudo. Also in group root, to test
# whether that grants write access to bind mounts shown as root:root.
RUN useradd -m -s /bin/bash -u 1000 agent \
 && usermod -aG root agent \
 && echo 'agent ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/agent \
 && chmod 0440 /etc/sudoers.d/agent

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod 0755 /usr/local/bin/entrypoint.sh

USER agent
WORKDIR /home/agent
ENV PATH="/home/agent/.local/bin:${PATH}"

# Claude Code, native install.
RUN curl -fsSL https://claude.ai/install.sh | bash

# Agent homes live in the shared mount; the image only links to them.
RUN mkdir -p /home/agent/.crannog/agents /home/agent/.crannog/ca \
 && rm -rf /home/agent/.claude /home/agent/.claude.json /home/agent/.codex \
 && ln -s .crannog/agents/claude/.claude /home/agent/.claude \
 && ln -s .crannog/agents/claude/.claude.json /home/agent/.claude.json \
 && ln -s .crannog/agents/codex /home/agent/.codex

CMD ["/usr/local/bin/entrypoint.sh"]
