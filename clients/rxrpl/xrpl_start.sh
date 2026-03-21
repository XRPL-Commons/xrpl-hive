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

exec rxrpl --config $CONFIG --log_level $LEVEL
