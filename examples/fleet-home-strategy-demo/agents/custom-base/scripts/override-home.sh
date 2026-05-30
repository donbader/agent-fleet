#!/bin/sh
# Override home directory with tracked configs on every container start.
# This ensures git-tracked configs always win over runtime state.
cp -a /home/agent-override/. /home/agent/
chown -R agent:agent /home/agent
echo "[init] home-override applied"
