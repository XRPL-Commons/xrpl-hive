#!/bin/bash
set -e

CONFIG=/etc/goxrpl/xrpld.toml
VALIDATORS=/etc/goxrpl/validators.toml

# Build complete TOML config from XRPL_* env vars.
cat > $CONFIG <<CFGEOF
compression = false
peer_private = ${XRPL_PEER_PRIVATE:-1}
peers_max = ${XRPL_PEERS_MAX:-50}
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
path_search = 2
path_search_fast = 2
path_search_max = 3
path_search_old = 2
workers = 0
io_workers = 0
prefetch_workers = 0
ledger_replay = 0
beta_rpc_api = 0
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

echo 'ips = []' >> $CONFIG

# Port configuration.
cat >> $CONFIG <<CFGEOF

[server]
ports = ["port_rpc_admin_local", "port_ws_admin_local", "port_peer"]

[port_rpc_admin_local]
port = ${XRPL_RPC_PORT:-5005}
ip = "0.0.0.0"
admin = ["0.0.0.0/0"]
protocol = "http"

[port_ws_admin_local]
port = ${XRPL_WS_PORT:-6006}
ip = "0.0.0.0"
admin = ["0.0.0.0/0"]
protocol = "ws"
send_queue_limit = 500

[port_peer]
port = ${XRPL_PEER_PORT:-51235}
ip = "0.0.0.0"
protocol = "peer"

[node_db]
type = "pebble"
path = "/tmp/goxrpl/db/pebble"
online_delete = ${XRPL_LEDGER_HISTORY:-256}
advisory_delete = 0
cache_size = 16384
cache_age = 5
fast_load = false
earliest_seq = 32570
delete_batch = 100
back_off_milliseconds = 100
age_threshold_seconds = 60
recovery_wait_seconds = 5

[sqlite]
journal_mode = "wal"
synchronous = "normal"
temp_store = "file"
page_size = 4096
journal_size_limit = 1582080

[overlay]
max_unknown_time = 600
max_diverged_time = 300

[transaction_queue]
ledgers_in_queue = 20
minimum_queue_size = 2000
retry_sequence_percent = 25
minimum_escalation_multiplier = 500
minimum_txn_in_ledger = 5
minimum_txn_in_ledger_standalone = 1000
target_txn_in_ledger = 50
maximum_txn_in_ledger = 0
normal_consensus_increase_percent = 20
slow_consensus_decrease_percent = 50
maximum_txn_per_account = 10
minimum_last_ledger_buffer = 2
zero_basefee_transaction_feelevel = 256000
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

if [ "${XRPL_STANDALONE:-0}" = "1" ]; then
    exec goxrpl server --conf $CONFIG -a
else
    exec goxrpl server --conf $CONFIG
fi
