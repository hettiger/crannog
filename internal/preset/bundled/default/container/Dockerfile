# crannog's default image: Debian with Claude Code, Codex, OpenCode and Pi.
# Each agent has its own block below and can be removed on its own.
FROM debian:trixie-slim

# --- Base --------------------------------------------------------------------
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates curl git sudo zsh procps iproute2 netcat-openbsd \
      xz-utils unzip less bubblewrap ripgrep fd-find \
 && rm -rf /var/lib/apt/lists/* \
 && ln -s /usr/bin/fdfind /usr/local/bin/fd

# The agent user. The crannog is the security boundary, so the agent may use
# sudo, e.g. to install packages.
RUN useradd -m -s /bin/zsh -u 1000 agent \
 && echo 'agent ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/agent \
 && chmod 0440 /etc/sudoers.d/agent

# sudo resets the environment. Keep the proxy variables, otherwise commands
# run with sudo (e.g. apt-get) bypass the proxy and cannot reach anything.
RUN echo 'Defaults env_keep += "HTTP_PROXY HTTPS_PROXY NO_PROXY http_proxy https_proxy no_proxy"' \
      > /etc/sudoers.d/proxy \
 && chmod 0440 /etc/sudoers.d/proxy

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
# The installer puts the binary in ~/.opencode/bin, which a login shell does
# not have in PATH.
RUN curl -fsSL https://opencode.ai/v2/install | bash \
 && ln -s /home/agent/.opencode/bin/opencode /home/agent/.local/bin/opencode

# --- Pi ----------------------------------------------------------------------
RUN npm install -g --ignore-scripts @earendil-works/pi-coding-agent \
 && npm cache clean --force

CMD ["/usr/local/bin/entrypoint.sh"]
