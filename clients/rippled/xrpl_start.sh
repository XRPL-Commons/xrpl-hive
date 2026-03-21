#!/bin/bash
set -e

CONFIG=/etc/rippled/rippled.cfg
VALIDATORS=/etc/rippled/validators.txt

# Build rippled.cfg from XRPL_* environment variables.
cat > $CONFIG <<CFGEOF
[server]
port_peer
port_rpc
port_ws

[port_peer]
port=${XRPL_PEER_PORT:-51235}
ip=0.0.0.0
protocol=peer

[port_rpc]
port=${XRPL_RPC_PORT:-5005}
ip=0.0.0.0
admin=0.0.0.0
protocol=http

[port_ws]
port=${XRPL_WS_PORT:-6006}
ip=0.0.0.0
admin=0.0.0.0
protocol=ws

[node_db]
type=NuDB
path=/var/lib/rippled/db/nudb
online_delete=${XRPL_LEDGER_HISTORY:-256}
advisory_delete=0

[database_path]
/var/lib/rippled/db

[debug_logfile]
/var/lib/rippled/db/debug.log

[node_size]
${XRPL_NODE_SIZE:-tiny}

[peers_max]
${XRPL_PEERS_MAX:-21}

[network_id]
${XRPL_NETWORK_ID:-10000}

[validators_file]
$VALIDATORS

[ssl_verify]
0

[ledger_history]
${XRPL_LEDGER_HISTORY:-256}
CFGEOF

# Validator seed.
if [ -n "$XRPL_VALIDATOR_SEED" ]; then
    echo -e "\n[validation_seed]\n$XRPL_VALIDATOR_SEED" >> $CONFIG
fi

# Validator token (alternative to seed).
if [ -n "$XRPL_VALIDATOR_TOKEN" ]; then
    echo -e "\n[validator_token]\n$XRPL_VALIDATOR_TOKEN" >> $CONFIG
fi

# Peer discovery.
if [ -n "$XRPL_BOOTNODE" ]; then
    echo -e "\n[ips_fixed]" >> $CONFIG
    IFS=',' read -ra PEERS <<< "$XRPL_BOOTNODE"
    for peer in "${PEERS[@]}"; do
        echo "${peer//:/ }" >> $CONFIG
    done
fi

# Peer private mode.
if [ "${XRPL_PEER_PRIVATE:-1}" = "1" ]; then
    echo -e "\n[peer_private]\n1" >> $CONFIG
fi

# Log level mapping: 0-5 -> rippled severity.
case "${XRPL_LOGLEVEL:-3}" in
    0|1) SEVERITY="fatal" ;;
    2)   SEVERITY="error" ;;
    3)   SEVERITY="warning" ;;
    4)   SEVERITY="info" ;;
    5)   SEVERITY="trace" ;;
esac
echo -e "\n[rpc_startup]\n{\"command\": \"log_level\", \"severity\": \"$SEVERITY\"}" >> $CONFIG

# Convert uploaded validators.json -> validators.txt.
if [ -f /xrpl/validators.json ]; then
    echo "[validators]" > $VALIDATORS
    python3 -c "
import json
data = json.load(open('/xrpl/validators.json'))
for v in data.get('validators', []):
    print('    ' + v)
" >> $VALIDATORS
else
    echo "[validators]" > $VALIDATORS
fi

echo "=== rippled config ==="
cat $CONFIG
echo "=== validators ==="
cat $VALIDATORS
echo "==================="

exec rippled --conf $CONFIG
