#!/bin/bash
set -e

CONFIG=/etc/rxrpl/node.toml

# Build TOML config from XRPL_* env vars.
# rxrpl uses a different TOML schema than goXRPL.
cat > $CONFIG <<CFGEOF
[server]
bind = "0.0.0.0:${XRPL_RPC_PORT:-5005}"
admin_ips = ["0.0.0.0"]

[peer]
port = ${XRPL_PEER_PORT:-51235}
max_peers = ${XRPL_PEERS_MAX:-21}
tls_enabled = false
CFGEOF

# Peer discovery.
if [ -n "$XRPL_BOOTNODE" ]; then
    echo 'fixed_peers = [' >> $CONFIG
    IFS=',' read -ra PEERS <<< "$XRPL_BOOTNODE"
    for peer in "${PEERS[@]}"; do
        echo "    \"$peer\"," >> $CONFIG
    done
    echo ']' >> $CONFIG
else
    echo 'fixed_peers = []' >> $CONFIG
fi

# Validator seed as node_seed.
if [ -n "$XRPL_VALIDATOR_SEED" ]; then
    echo "node_seed = \"$XRPL_VALIDATOR_SEED\"" >> $CONFIG
fi

cat >> $CONFIG <<CFGEOF

[database]
path = "/var/lib/rxrpl/data"
backend = "memory"

[network]
network_id = ${XRPL_NETWORK_ID:-10000}
CFGEOF

# Convert uploaded validators.json -> trusted list in config.
if [ -f /xrpl/validators.json ]; then
    echo '[validators]' >> $CONFIG
    echo 'enabled = true' >> $CONFIG
    echo 'trusted = [' >> $CONFIG
    python3 -c "
import json
data = json.load(open('/xrpl/validators.json'))
for v in data.get('validators', []):
    print(f'    \"{v}\",')
" >> $CONFIG
    echo ']' >> $CONFIG
else
    echo '[validators]' >> $CONFIG
    echo 'enabled = false' >> $CONFIG
    echo 'trusted = []' >> $CONFIG
fi

# Log level mapping.
case "${XRPL_LOGLEVEL:-3}" in
    0|1) LEVEL="error" ;;
    2)   LEVEL="warn" ;;
    3)   LEVEL="info" ;;
    4|5) LEVEL="debug" ;;
esac

echo "=== rxrpl config ==="
cat $CONFIG
echo "==================="

# Standalone unless we're part of a multi-node network. The seed-anchor
# (first node) has no bootnodes but still needs P2P listening so peers
# and late joiners can reach it; XRPL_VALIDATOR_SEED is the right signal.
if [ -n "$XRPL_BOOTNODE" ] || [ -n "$XRPL_VALIDATOR_SEED" ]; then
    MODE="network"
else
    MODE="standalone"
fi

# rxrpl serves HTTP JSON-RPC and WebSocket on the same port; xrpl-hive expects
# WS on a separate port. Forward WS_PORT -> RPC_PORT in the background.
RPC_PORT="${XRPL_RPC_PORT:-5005}"
WS_PORT="${XRPL_WS_PORT:-6006}"
if [ "$RPC_PORT" != "$WS_PORT" ]; then
    (socat TCP-LISTEN:$WS_PORT,reuseaddr,fork TCP:127.0.0.1:$RPC_PORT &) 2>/dev/null
fi

exec rxrpl --config $CONFIG --log-level $LEVEL run --mode $MODE --bind "0.0.0.0:$RPC_PORT" --close-interval 3
