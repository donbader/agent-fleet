# Partial Dockerfile template — injected into the runtime's Dockerfile by the provider.
# Magic variables available:
#   ${AGENT_HOME}  - agent's home directory (e.g., /home/agent)
#   ${AGENT_USER}  - agent's OS username (e.g., agent)
#
# Purpose: track fixed configs (.gitconfig, .bashrc, tool settings) in git.
# Docker populates the named volume from the image on first run.

# Install extra tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    ripgrep \
    fd-find \
    jq \
    && rm -rf /var/lib/apt/lists/*

# Copy all tracked configs into the home directory
COPY home-override/ ${AGENT_HOME}/
