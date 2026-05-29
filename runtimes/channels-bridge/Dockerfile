# channels-bridge runtime
# Multi-stage build: compile bridge binary, then runtime with agent tools.
#
# Self-contained: clone this folder → docker build works.

FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 go build -o /bridge ./cmd/bridge

# Runtime stage
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

# Copy bridge binary
COPY --from=builder /bridge /usr/local/bin/bridge

# Create agent user
RUN useradd -m -s /bin/bash agent

# Copy entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
