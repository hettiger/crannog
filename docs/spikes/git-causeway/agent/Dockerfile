FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends git ca-certificates \
 && rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash -u 1000 agent \
 && su agent -c 'git config --global user.name Agent \
 && git config --global user.email agent@crannog.invalid'

# Replaced by spike.sh to tell image versions apart.
RUN echo IMAGE_VERSION > /image-version

CMD ["sleep", "infinity"]
