FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      git openssh-server curl ca-certificates netcat-openbsd iproute2 \
 && rm -rf /var/lib/apt/lists/*

# Non-root agent user, key-only SSH login.
RUN useradd -m -s /bin/bash -u 1000 agent \
 && usermod -p '*' agent \
 && mkdir -p /run/sshd /home/agent/.ssh /work \
 && chown agent:agent /work \
 && sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config

COPY authorized_keys /home/agent/.ssh/authorized_keys
RUN chown -R agent:agent /home/agent/.ssh \
 && chmod 700 /home/agent/.ssh \
 && chmod 600 /home/agent/.ssh/authorized_keys

# Non-bare repo that accepts pushes to the checked-out branch.
RUN su agent -c 'git config --global user.name Agent \
 && git config --global user.email agent@crannog.invalid \
 && git init -q -b main /work/repo \
 && git -C /work/repo config receive.denyCurrentBranch updateInstead'

EXPOSE 22
CMD ["/usr/sbin/sshd", "-D", "-e"]
