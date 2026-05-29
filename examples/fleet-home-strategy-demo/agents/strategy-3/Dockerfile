# Partial Dockerfile template — injected into the runtime's Dockerfile by the provider.
# Magic variables available:
#   ${AGENT_HOME}  - agent's home directory (e.g., /home/agent)
#   ${AGENT_USER}  - agent's OS username (e.g., agent)

# Install extra tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    ripgrep \
    fd-find \
    jq \
    && rm -rf /var/lib/apt/lists/*

# Pre-configure home directory (Docker populates volume from image on first run)
COPY home-override/.gitconfig ${AGENT_HOME}/.gitconfig
