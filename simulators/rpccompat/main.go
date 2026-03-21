// The rpccompat simulator tests XRPL JSON-RPC API compatibility using
// data-driven .io test files. Each .io file defines a sequence of
// request/response pairs that are validated against each client type.
//
// The simulator starts clients in standalone mode and uses ledger_accept
// to advance ledgers, giving the node real state to query — matching how
// Ethereum Hive bootstraps chain state before running rpc-compat tests.
package main

import (
	"fmt"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/iotest"
)

func main() {
	suite := xrplsim.Suite{
		Name:        "rpccompat",
		Description: "RPC API compatibility tests using .io file specifications.",
	}

	// Discover all .io test files.
	tests, err := iotest.Discover("tests")
	if err != nil {
		panic("failed to discover tests: " + err.Error())
	}

	// For each .io file, create a test that starts a client in standalone mode,
	// advances the ledger, then runs the .io exchanges.
	for _, test := range tests {
		test := test // capture for closure
		suite.Add(xrplsim.ClientTestSpec{
			Name:        test.Name + " (CLIENT)",
			Description: test.Comment,
			Category:    iotest.Category(test),
			Role:        "xrpl_validator",
			// Start in standalone mode so we can use ledger_accept.
			Parameters: xrplsim.Params{
				"XRPL_STANDALONE":  "1",
				"XRPL_NETWORK_ID":  "10000",
				"XRPL_LOGLEVEL":    "3",
				"XRPL_PEER_PRIVATE": "1",
			},
			Run: func(t *xrplsim.T, c *xrplsim.Client) {
				rpc := xrplsim.NewRPCClient(c.RPCEndpoint())

				// Wait for RPC to be responsive.
				ready := false
				for i := 0; i < 30; i++ {
					if _, err := rpc.ServerInfo(); err == nil {
						ready = true
						break
					}
					time.Sleep(time.Second)
				}
				if !ready {
					t.Fatal("node RPC not responsive after 30s")
				}

				// Advance a few ledgers so the node has closed state.
				for i := 0; i < 3; i++ {
					rpc.Call("ledger_accept", nil)
					time.Sleep(200 * time.Millisecond)
				}

				// Run the .io test exchanges.
				iotest.RunFile(t, rpc, test)
			},
		})
	}

	if len(suite.Tests) == 0 {
		panic("no .io test files found in tests/ directory")
	}

	fmt.Printf("rpccompat: loaded %d .io test files\n", len(suite.Tests))
	xrplsim.MustRun(xrplsim.New(), suite)
}
