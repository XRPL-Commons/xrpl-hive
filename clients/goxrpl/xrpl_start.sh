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
node_size = "${XRPL_NODE_SIZE:-tiny}"
validators_file = "$VALIDATORS"
database_path = "/tmp/goxrpl/db"
debug_logfile = "/tmp/goxrpl/db/debug.log"
relay_proposals = "trusted"
relay_validations = "all"
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

# With ips and ips_fixed both empty, goXRPL bootstraps from the public
# mainnet hubs like rippled; with peer_private=0 it then joins mainnet and
# follows its ledgers. Bootstrap from our own port instead (dropped as a
# self-connection) so the node only meets the test network's peers.
if [ -z "$XRPL_BOOTNODE" ] && [ "${XRPL_STANDALONE:-0}" != "1" ]; then
    echo "ips = [\"127.0.0.1 ${XRPL_PEER_PORT:-51235}\"]" >> $CONFIG
else
    echo 'ips = []' >> $CONFIG
fi

# XRPL_LOGLEVEL 0-5 (silent to trace) mapped onto goXRPL's levels; logs go to
# stdout so hive captures them with the container output.
case "${XRPL_LOGLEVEL:-3}" in
    0|1) LOGLEVEL=error ;;
    2) LOGLEVEL=warn ;;
    4) LOGLEVEL=debug ;;
    5) LOGLEVEL=trace ;;
    *) LOGLEVEL=info ;;
esac

# Port configuration.
cat >> $CONFIG <<CFGEOF

[logging]
level = "$LOGLEVEL"
output = "stdout"

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
delete_batch = 100
back_off_milliseconds = 100
age_threshold_seconds = 60
recovery_wait_seconds = 5

[resources]
memory_mb = ${XRPL_MEMORY_MB:-3072}

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
normal_consensus_increase_percent = 20
slow_consensus_decrease_percent = 50
maximum_txn_per_account = 10
minimum_last_ledger_buffer = 2
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
elif [ "${XRPL_STANDALONE:-0}" = "1" ]; then
    echo 'validators = []' > $VALIDATORS
    echo 'validator_list_sites = []' >> $VALIDATORS
    echo 'validator_list_keys = []' >> $VALIDATORS
else
    # goXRPL refuses to join a network without a trust configuration, while
    # rippled starts with an empty [validators] section. Trust a placeholder
    # list publisher key with no site, which leaves the UNL empty as rippled's
    # does.
    echo 'validators = []' > $VALIDATORS
    echo 'validator_list_sites = []' >> $VALIDATORS
    echo 'validator_list_keys = ["EDE52570D37DCCFA27EFB013FAF72F1A2EA6795DCF3D1A49296B00F8986FC169C1"]' >> $VALIDATORS
fi

# Enable amendments from genesis in standalone mode. XRPL_FEATURES can be
# "all" (every amendment this build supports) or a comma-separated list.
if [ -n "$XRPL_FEATURES" ]; then
    if [ "$XRPL_FEATURES" = "all" ]; then
        FEATS=$(cat /etc/goxrpl/all_amendments.txt)
    else
        FEATS=$(echo "$XRPL_FEATURES" | tr ',' '\n')
    fi
    echo -e "\n[amendments]\nupvote = [" >> $CONFIG
    for feat in $FEATS; do
        echo "    \"$feat\"," >> $CONFIG
    done
    echo ']' >> $CONFIG
fi

echo "=== goXRPL config ==="
cat $CONFIG
echo "=== validators ==="
cat $VALIDATORS
echo "==================="

if [ "${XRPL_STANDALONE:-0}" = "1" ]; then
    # --start builds a fresh genesis ledger, which is when goXRPL enables the
    # upvoted amendments; the container always starts from an empty database.
    exec goxrpl server --conf $CONFIG -a --start
else
    exec goxrpl server --conf $CONFIG
fi
