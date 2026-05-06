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

# Validation quorum (for single-validator networks).
if [ -n "$XRPL_VALIDATION_QUORUM" ]; then
    echo -e "\n[validation_quorum]\n$XRPL_VALIDATION_QUORUM" >> $CONFIG
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
# Uses pure shell (no python3 dependency) to parse the simple JSON array.
if [ -f /xrpl/validators.json ]; then
    echo "[validators]" > $VALIDATORS
    tr ',' '\n' < /xrpl/validators.json | sed -n 's/.*"\(n[A-Za-z0-9]*\)".*/    \1/p' >> $VALIDATORS
else
    echo "[validators]" > $VALIDATORS
fi

echo "=== rippled config ==="
cat $CONFIG
echo "=== validators ==="
cat $VALIDATORS
echo "==================="

# Force-enable amendments in standalone mode.
# XRPL_FEATURES can be "all" (enable every supported amendment) or a
# comma-separated list of amendment names.
if [ -n "$XRPL_FEATURES" ]; then
    echo -e "\n[features]" >> $CONFIG
    if [ "$XRPL_FEATURES" = "all" ]; then
        # Use the pre-built amendment list extracted at Docker build time.
        cat /etc/rippled/all_amendments.txt >> $CONFIG
    else
        IFS=',' read -ra FEATS <<< "$XRPL_FEATURES"
        for feat in "${FEATS[@]}"; do
            echo "$feat" >> $CONFIG
        done
    fi
fi

# Standalone mode: -a flag makes rippled work without peers.
# In this mode, ledger_accept can be used to advance ledgers.
if [ "${XRPL_STANDALONE:-0}" = "1" ]; then
    exec rippled --conf $CONFIG -a
else
    exec rippled --conf $CONFIG
fi
