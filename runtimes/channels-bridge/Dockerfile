# channels-bridge runtime (TypeScript)
# Connects Telegram to an agent process via ACP protocol.
#
# Self-contained: clone this folder → docker build works.

FROM node:22-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    iptables \
    && rm -rf /var/lib/apt/lists/*

# Install codex CLI (default agent)
RUN npm install -g @openai/codex

# Set up bridge
WORKDIR /bridge
COPY package.json ./
RUN npm install --omit=dev && npm install -g tsx
COPY tsconfig.json ./
COPY src/ ./src/

# Create agent user
RUN useradd -m -s /bin/bash agent

# Copy entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
