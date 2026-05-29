# Custom base image for coder agent.
# Add extra tools on top of the channels-bridge runtime.
FROM channels-bridge AS base

RUN apt-get update && apt-get install -y --no-install-recommends \
    ripgrep \
    fd-find \
    jq \
    && rm -rf /var/lib/apt/lists/*
