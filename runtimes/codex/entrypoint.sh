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

    # Redirect inbound auth port traffic to localhost.
    # Codex binds its OAuth callback server to 127.0.0.1, but Docker
    # delivers published-port traffic to the container's external interface.
    # route_localnet is set via Docker sysctls in the compose service.
    AUTH_PORT="${AUTH_PORT:-1455}"
    iptables -t nat -A PREROUTING -p tcp --dport "$AUTH_PORT" -j DNAT --to-destination 127.0.0.1:"$AUTH_PORT"
    echo "[codex] auth port $AUTH_PORT → localhost"
fi

# Drop to agent user and exec
if [ $# -gt 0 ]; then
    exec su -c "$*" agent
else
    exec su -c "sleep infinity" agent
fi
