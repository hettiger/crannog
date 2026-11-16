FROM debian:trixie-slim

# --- Base --------------------------------------------------------------------
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates curl git sudo zsh procps iproute2 netcat-openbsd \
      xz-utils unzip less bubblewrap ripgrep fd-find \
 && rm -rf /var/lib/apt/lists/* \
 && ln -s /usr/bin/fdfind /usr/local/bin/fd

RUN useradd -m -s /bin/zsh -u 1000 agent \
 && echo 'agent ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/agent \
 && chmod 0440 /etc/sudoers.d/agent

# Node.js (needed by Pi, useful for most projects). Debian's is too old.
RUN set -eu; \
    base=https://nodejs.org/dist/latest-v24.x; \
    file="$(curl -fsSL "$base/SHASUMS256.txt" | awk '/linux-arm64\.tar\.xz$/ {print $2}')"; \
    curl -fsSL "$base/$file" -o /tmp/node.tar.xz; \
    curl -fsSL "$base/SHASUMS256.txt" | grep " $file\$" | sed "s| $file\$| /tmp/node.tar.xz|" | sha256sum -c -; \
    tar -xJf /tmp/node.tar.xz -C /usr/local --strip-components=1 \
      --exclude CHANGELOG.md --exclude README.md --exclude LICENSE; \
    rm /tmp/node.tar.xz

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod 0755 /usr/local/bin/entrypoint.sh

USER agent
WORKDIR /home/agent
# Agents install into the agent user's home so they can update themselves.
ENV PATH="/home/agent/.local/bin:${PATH}"
RUN npm config set prefix /home/agent/.local

# --- Claude Code -------------------------------------------------------------
RUN curl -fsSL https://claude.ai/install.sh | bash

# --- Codex -------------------------------------------------------------------
RUN curl -fsSL https://chatgpt.com/codex/install.sh | CODEX_NON_INTERACTIVE=1 sh

# --- OpenCode ----------------------------------------------------------------
RUN curl -fsSL https://opencode.ai/v2/install | bash \
 && ln -s /home/agent/.opencode/bin/opencode /home/agent/.local/bin/opencode

# --- Pi ----------------------------------------------------------------------
RUN npm install -g --ignore-scripts @earendil-works/pi-coding-agent \
 && npm cache clean --force

CMD ["/usr/local/bin/entrypoint.sh"]
