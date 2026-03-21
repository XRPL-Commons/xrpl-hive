# Testing Guide

This document explains how to add tests to xrpl-hive.

## Test Types

xrpl-hive has three test patterns:

| Pattern | Type | Use When |
|---------|------|----------|
| `.io` files | `ClientTestSpec` | Testing RPC responses against empty/genesis state (stateless) |
| Stateful tests | `TestSpec` | Testing RPC methods that require ledger state (funded accounts, objects) |
| Simulator | `TestSpec` / `NetworkTestSpec` | Testing network behavior (consensus, sync, propagation) |

## Adding a Stateless .io Test

The simplest way to add RPC coverage. No Go code required.

### 1. Create a `.io` file

```
simulators/rpccompat/tests/<category>/<test_name>.io
```

### 2. Write the test

```
// Description of what this test verifies
// speconly: true    (optional — validate structure only, not exact values)
>> {"method":"server_info","params":[{}]}
<< {"result":{"info":{"server_state":"...","build_version":"..."},"status":"success"}}
```

**Syntax:**
- `>>` — JSON-RPC request to send
- `<<` — Expected JSON-RPC response
- `"..."` — Wildcard, matches any value
- `// speconly: true` — Only validate response structure (keys and types), not values
- Extra fields in the actual response are allowed (implementations may add fields)

### 3. That's it

The `rpccompat` simulator auto-discovers all `.io` files. Run with:

```bash
make rpccompat
```

### Categories

Organize tests into directories by domain:
- `server/` — server_info, fee, ping, etc.
- `account/` — account_info, account_lines, etc.
- `ledger/` — ledger, ledger_entry, etc.
- `tx/` — submit, tx, sign, etc.
- `admin/` — feature, consensus_info, etc.
- `errors/` — error handling tests
- `orderbook/` — book_offers, book_changes
- `nft/` — NFT-related queries
- `path/` — pathfinding
- `utility/` — wallet_propose, etc.

## Adding a Stateful Test

For RPC methods that need ledger state (e.g., testing `account_lines` after creating a trust line).

### 1. Edit `simulators/rpccompat-stateful/main.go`

### 2. Add a test function

```go
func myNewTest() xrplsim.TestSpec {
    return xrplsim.TestSpec{
        Name: "my_test_name",
        Description: "What this test verifies",
        Run: func(t *xrplsim.T) {
            // Start a validator network
            ctx := context.Background()
            client, rpc := startNetwork(t)
            _ = client

            // Set up state
            accounts, err := setup.FundN(ctx, rpc, 2, "10000000000")
            if err != nil {
                t.Fatal("fund:", err)
            }
            setup.WaitSettled(ctx, rpc, 2)

            // Query and verify
            resp, err := rpc.Call("account_info", map[string]interface{}{
                "account": accounts[0].Address,
            })
            if err != nil {
                t.Fatal("account_info:", err)
            }
            // ... assert on resp fields
        },
    }
}
```

### 3. Wire it into the suite

Add to the `Tests` slice in `main()`.

### Available Setup Helpers (`xrplsim/setup/`)

- `FundAccount(ctx, rpc, address, amount)` — Fund one account from genesis
- `FundN(ctx, rpc, n, amount)` — Create and fund N accounts
- `SetupTrustLine(ctx, rpc, secret, account, currency, issuer, limit)` — Create trust line
- `SetupOffer(ctx, rpc, secret, account, takerPays, takerGets)` — Create DEX offer
- `WaitSettled(ctx, rpc, ledgersAhead)` — Wait for ledgers to close

### Available RPCClient Helpers (`xrplsim/rpcclient.go`)

- `rpc.Call(method, params)` — Generic RPC call
- `rpc.CallRaw(jsonBytes)` — Raw JSON-RPC
- `rpc.SubmitPayment(secret, from, to, amount)` — XRP payment
- `rpc.SubmitTrustSet(secret, account, currency, issuer, limit)` — Trust line
- `rpc.SubmitOfferCreate(secret, account, takerPays, takerGets)` — DEX offer
- `rpc.SubmitAccountSet(secret, account, setFlag)` — Account flags
- `rpc.Submit(secret, account, txJSON)` — Generic tx submission
- `rpc.AccountInfo(account)` — Account query
- `rpc.Tx(hash)` — Transaction lookup
- `rpc.WaitForLedger(ctx, seq, timeout)` — Wait for ledger sequence

## Adding a New Simulator

For tests with distinct infrastructure needs (e.g., multi-node consensus, WebSocket).

### 1. Create the directory

```
simulators/<name>/
├── main.go
├── Dockerfile
└── hive_context.txt    (contains: ../..)
```

### 2. Write the simulator

```go
package main

import "github.com/xrpl-commons/xrpl-hive/xrplsim"

func main() {
    suite := xrplsim.Suite{
        Name:        "my-simulator",
        Description: "What this simulator tests",
        Tests: []xrplsim.AnyTest{
            xrplsim.ClientTestSpec{
                Name: "my_test",
                Run: func(t *xrplsim.T, c *xrplsim.Client) {
                    rpc := xrplsim.NewRPCClient(c.RPCEndpoint())
                    // ... test logic
                },
            },
        },
    }
    xrplsim.MustRun(xrplsim.New(), suite)
}
```

### 3. Create the Dockerfile

Copy from `simulators/smoke/Dockerfile` and adjust the build path.

### 4. Add a Makefile target

```makefile
my-simulator: build
	./bin/xrpl-hive --sim my-simulator --client rippled,goxrpl
```

## Running Tests

```bash
make build                # Build xrpl-hive binary
make smoke                # Basic liveness tests
make rpccompat            # Stateless RPC tests
make rpccompat-stateful   # Stateful RPC tests
make txcompat             # Transaction type tests
make wscompat             # WebSocket tests
make consensus            # Multi-validator consensus
make propagation          # Cross-implementation propagation
make sync                 # Late-join sync
make soak                 # Traffic + hash oracle (10m timeout)
make full                 # All simulators, all clients

# Run specific test pattern
./bin/xrpl-hive --sim rpccompat --client rippled --sim.limit "server/.*"

# View results
go run ./cmd/hiveview -serve -results workspace/logs
```
