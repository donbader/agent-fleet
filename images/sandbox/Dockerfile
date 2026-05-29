# Agent sandbox base image
# Provides iptables for transparent proxy redirect + basic tools
FROM ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    iptables \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Create agent user
RUN useradd -m -s /bin/bash agent

# Copy iptables setup script
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# The entrypoint sets up iptables redirect then execs the agent process
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
