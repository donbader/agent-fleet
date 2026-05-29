#!/bin/sh
set -e

GATEWAY_HOST="${GATEWAY_HOST:-gateway}"
GATEWAY_PORT="${GATEWAY_PORT:-8080}"

# Set up iptables redirect to gateway proxy
if command -v iptables >/dev/null 2>&1; then
    GATEWAY_IP=$(getent hosts "$GATEWAY_HOST" | awk '{print $1}' || echo "")
    if [ -n "$GATEWAY_IP" ]; then
        iptables -t nat -A OUTPUT -p tcp -d "$GATEWAY_IP" -j RETURN
        iptables -t nat -A OUTPUT -p tcp -j DNAT --to-destination "$GATEWAY_IP:$GATEWAY_PORT"
        echo "[codex] iptables redirect → $GATEWAY_IP:$GATEWAY_PORT"
    else
        echo "[codex] WARNING: could not resolve '$GATEWAY_HOST'"
    fi
fi

# Drop to agent user and exec
if [ $# -gt 0 ]; then
    exec su -c "$*" agent
else
    exec su -c "sleep infinity" agent
fi
