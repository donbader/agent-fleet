# Codex runtime — minimal agent container
# Provides Node.js + git for running OpenAI Codex CLI
FROM node:22-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    iptables \
    && rm -rf /var/lib/apt/lists/*

# Install codex CLI globally
RUN npm install -g @openai/codex

# Create agent user
RUN useradd -m -s /bin/bash agent

# Entrypoint: set up iptables redirect then run agent
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["sleep", "infinity"]
