# Example Dockerfile for coder agent
# This is used as user_base_image_stage — provider layers on top
FROM ubuntu:24.04

# Install custom tools
RUN apt-get update && apt-get install -y \
    ripgrep \
    fd-find \
    && rm -rf /var/lib/apt/lists/*

# Custom home directory (configs, scripts, etc.)
COPY home/ /home/agent/
