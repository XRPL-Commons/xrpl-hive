#!/bin/bash
set -e

CONFIG=/etc/goxrpl/xrpld.toml
VALIDATORS=/etc/goxrpl/validators.toml

# Build TOML config from XRPL_* env vars.
cat > $CONFIG <<CFGEOF
compression = false
peer_private = ${XRPL_PEER_PRIVATE:-1}
peers_max = ${XRPL_PEERS_MAX:-21}
max_transactions = 250
network_id = ${XRPL_NETWORK_ID:-10000}
ledger_history = ${XRPL_LEDGER_HISTORY:-256}
fetch_depth = "full"
ssl_verify = 0
node_size = "${XRPL_NODE_SIZE:-tiny}"
signing_support = true
validators_file = "$VALIDATORS"
database_path = "/tmp/goxrpl/db"
debug_logfile = "/tmp/goxrpl/db/debug.log"
relay_proposals = "trusted"
relay_validations = "all"
CFGEOF

# Validator seed.
if [ -n "$XRPL_VALIDATOR_SEED" ]; then
    echo "validation_seed = \"$XRPL_VALIDATOR_SEED\"" >> $CONFIG
fi

# Peer discovery — build ips_fixed array.
if [ -n "$XRPL_BOOTNODE" ]; then
    echo 'ips_fixed = [' >> $CONFIG
    IFS=',' read -ra PEERS <<< "$XRPL_BOOTNODE"
    for peer in "${PEERS[@]}"; do
        echo "    \"${peer//:/ }\"," >> $CONFIG
    done
    echo ']' >> $CONFIG
else
    echo 'ips_fixed = []' >> $CONFIG
fi

# Port configuration.
cat >> $CONFIG <<CFGEOF

[server]
ports = ["port_rpc_admin_local", "port_ws_admin_local", "port_peer"]

[port_rpc_admin_local]
port = ${XRPL_RPC_PORT:-5005}
ip = "0.0.0.0"
admin = ["0.0.0.0"]
protocol = "http"

[port_ws_admin_local]
port = ${XRPL_WS_PORT:-6006}
ip = "0.0.0.0"
admin = ["0.0.0.0"]
protocol = "ws"

[port_peer]
port = ${XRPL_PEER_PORT:-51235}
ip = "0.0.0.0"
protocol = "peer"

[node_db]
type = "pebble"
path = "/tmp/goxrpl/db/pebble"
online_delete = ${XRPL_LEDGER_HISTORY:-256}

[transaction_queue]
ledgers_in_queue = 20
minimum_queue_size = 2000
CFGEOF

# Convert uploaded validators.json -> validators.toml.
if [ -f /xrpl/validators.json ]; then
    echo 'validators = [' > $VALIDATORS
    python3 -c "
import json
data = json.load(open('/xrpl/validators.json'))
for v in data.get('validators', []):
    print(f'    \"{v}\",')
" >> $VALIDATORS
    echo ']' >> $VALIDATORS
    echo 'validator_list_sites = []' >> $VALIDATORS
    echo 'validator_list_keys = []' >> $VALIDATORS
else
    echo 'validators = []' > $VALIDATORS
    echo 'validator_list_sites = []' >> $VALIDATORS
    echo 'validator_list_keys = []' >> $VALIDATORS
fi

echo "=== goXRPL config ==="
cat $CONFIG
echo "=== validators ==="
cat $VALIDATORS
echo "==================="

exec goxrpl server --conf $CONFIG
