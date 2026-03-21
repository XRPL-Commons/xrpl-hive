# xrpl-hive

Client-agnostic integration testing framework for XRPL node implementations.

Forked from [Ethereum Hive](https://github.com/ethereum/hive) and adapted for the XRP Ledger protocol. xrpl-hive tests interoperability between any XRPL node implementation by orchestrating Docker containers with a standardized client contract.

## License

This project is licensed under the **GNU General Public License v3.0** — see the [COPYING](COPYING) file.

Based on [Ethereum Hive](https://github.com/ethereum/hive), Copyright (C) 2016 The Hive Authors.

## Supported Clients

| Client | Language | Config Format |
|--------|----------|---------------|
| [rippled](https://github.com/XRPLF/rippled) | C++ | INI (.cfg) |
| [goXRPL](https://github.com/LeJamon/goXRPLd) | Go | TOML |
| [rxrpl](https://github.com/peerparty/rxrpl) | Rust | TOML |

## Quick Start

```bash
# Build
make build

# Run smoke tests
./bin/xrpl-hive --sim smoke --client rippled,goxrpl

# Run all simulators
./bin/xrpl-hive --sim ".*" --client rippled,goxrpl,rxrpl

# Dev mode (API on localhost:3000, no simulators)
./bin/xrpl-hive --dev --client rippled,goxrpl
```

## Architecture

```
xrpl-hive/
├── clients/          # One dir per XRPL implementation (Dockerfile + entry script)
├── simulators/       # Test suites (standalone Go programs using xrplsim)
├── xrplsim/          # Public Go library for writing simulators
├── internal/         # Core orchestration (forked from Ethereum Hive)
└── hiveproxy/        # API proxy for Docker networking
```

### Client Contract

Each client is a Docker image that translates `XRPL_*` environment variables into its native config:

| Variable | Description | Default |
|---|---|---|
| `XRPL_NETWORK_ID` | Private network ID | `10000` |
| `XRPL_VALIDATOR_SEED` | Base58 validator seed | — |
| `XRPL_BOOTNODE` | Comma-separated `ip:port` peers | — |
| `XRPL_RPC_PORT` | JSON-RPC HTTP port | `5005` |
| `XRPL_WS_PORT` | WebSocket port | `6006` |
| `XRPL_PEER_PORT` | P2P protocol port | `51235` |
| `XRPL_LOGLEVEL` | 0-5 (silent to trace) | `3` |

Validators are uploaded as `/xrpl/validators.json`:
```json
{"validators": ["n9LXM...", "n9KTo..."]}
```

### Adding a New Client

1. Create `clients/yourclient/Dockerfile` — builds the node binary, writes `/version.txt`
2. Create `clients/yourclient/xrpl_start.sh` — translates `XRPL_*` env vars to native config
3. Create `clients/yourclient/hive.yaml` — declares roles: `["xrpl_validator"]`

### Simulators

Each simulator is a standalone program that uses the `xrplsim` library:

- **smoke** — Single-node liveness (server_info, ledger advance, wallet_propose)
- **propagation** — Cross-implementation transaction propagation
- **consensus** — Mixed-validator hash agreement
- **sync** — Late-join node synchronization
- **soak** — Traffic generation with ledger hash oracle

### Writing a Simulator

```go
package main

import "github.com/xrpl-commons/xrpl-hive/xrplsim"

func main() {
    suite := xrplsim.Suite{
        Name:        "my-test",
        Description: "My XRPL interop test.",
    }
    suite.Add(xrplsim.ClientTestSpec{
        Name: "basic-check (CLIENT)",
        Role: "xrpl_validator",
        Run: func(t *xrplsim.T, c *xrplsim.Client) {
            rpc := xrplsim.NewRPCClient(c.RPCEndpoint())
            info, _ := rpc.ServerInfo()
            t.Logf("state: %s", info.ServerState)
        },
    })
    xrplsim.MustRun(xrplsim.New(), suite)
}
```
