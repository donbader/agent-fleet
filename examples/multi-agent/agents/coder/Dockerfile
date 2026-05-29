# Custom tools for coder agent.
# This Dockerfile adds extra dev tools on top of the base runtime.
# Referenced by user_base_image_stage in agent.yaml.

FROM node:22-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ripgrep \
    fd-find \
    jq \
    && rm -rf /var/lib/apt/lists/*
