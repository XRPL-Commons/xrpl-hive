// The rpccompat simulator tests XRPL JSON-RPC API compatibility using
// data-driven .io test files. Each .io file defines a sequence of
// request/response pairs that are validated against each client type.
package main

import (
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

	// Create a ClientTestSpec for each .io file, running it against every client.
	for _, test := range tests {
		test := test // capture for closure
		suite.Add(xrplsim.ClientTestSpec{
			Name:        test.Name + " (CLIENT)",
			Description: test.Comment,
			Category:    iotest.Category(test),
			Role:        "xrpl_validator",
			Run: func(t *xrplsim.T, c *xrplsim.Client) {
				rpc := xrplsim.NewRPCClient(c.RPCEndpoint())

				// Wait for node RPC to be responsive (not for ledger validation —
				// a single standalone node may not close ledgers without a quorum).
				ready := false
				for attempts := 0; attempts < 30; attempts++ {
					if _, err := rpc.ServerInfo(); err == nil {
						ready = true
						break
					}
					time.Sleep(time.Second)
				}
				if !ready {
					t.Fatal("node RPC not responsive after 30s")
				}

				// Run the .io test exchanges.
				iotest.RunFile(t, rpc, test)
			},
		})
	}

	if len(suite.Tests) == 0 {
		panic("no .io test files found in tests/ directory")
	}

	xrplsim.MustRun(xrplsim.New(), suite)
}
