#!/bin/sh
# Sandbox entrypoint: set up iptables redirect then exec the agent process
set -e

GATEWAY_HOST="${GATEWAY_HOST:-gateway}"
GATEWAY_PORT="${GATEWAY_PORT:-8080}"

# Redirect all outbound TCP traffic through the gateway proxy
# Skip traffic to the gateway itself (internal network)
if command -v iptables >/dev/null 2>&1; then
    # Get gateway IP
    GATEWAY_IP=$(getent hosts "$GATEWAY_HOST" | awk '{print $1}' || echo "")

    if [ -n "$GATEWAY_IP" ]; then
        # Redirect all TCP traffic (except to gateway) to the proxy
        iptables -t nat -A OUTPUT -p tcp -d "$GATEWAY_IP" -j RETURN
        iptables -t nat -A OUTPUT -p tcp -j DNAT --to-destination "$GATEWAY_IP:$GATEWAY_PORT"
        echo "[sandbox] iptables redirect configured → $GATEWAY_IP:$GATEWAY_PORT"
    else
        echo "[sandbox] WARNING: could not resolve gateway host '$GATEWAY_HOST'"
    fi
else
    echo "[sandbox] WARNING: iptables not available, proxy redirect not configured"
fi

# Drop to agent user and exec the command
if [ $# -gt 0 ]; then
    exec su -c "$*" agent
else
    echo "[sandbox] No command specified, sleeping..."
    exec sleep infinity
fi
