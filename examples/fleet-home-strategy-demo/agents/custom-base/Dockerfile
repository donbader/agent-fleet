# Custom base Dockerfile template for home overriding strategy.
# This is a partial template — provider injects it into the runtime Dockerfile.
# Build context is agents/custom-base/ (so COPY paths are relative to there).

# Install extra tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    ripgrep \
    fd-find \
    jq \
    && rm -rf /var/lib/apt/lists/*

# Stage home-override files (applied on every start by init script)
COPY --chown=${AGENT_USER}:${AGENT_USER} home-override/ /home/agent-override/

# Copy init scripts
COPY scripts/ /scripts/
RUN chmod +x /scripts/*.sh
