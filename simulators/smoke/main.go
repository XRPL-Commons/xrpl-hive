package main

import (
	"context"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

func main() {
	suite := xrplsim.Suite{
		Name:        "smoke",
		Description: "Basic client startup and RPC health checks for XRPL nodes.",
	}

	// Test: server_info returns valid response for each client type.
	suite.Add(xrplsim.ClientTestSpec{
		Name:        "server_info (CLIENT)",
		Description: "Verify the client starts and responds to server_info.",
		Role:        "xrpl_validator",
		Run: func(t *xrplsim.T, c *xrplsim.Client) {
			rpc := xrplsim.NewRPCClient(c.RPCEndpoint())
			info, err := rpc.ServerInfo()
			if err != nil {
				t.Fatal("server_info failed:", err)
			}
			t.Logf("server_state: %s, build_version: %s", info.ServerState, info.BuildVersion)
		},
	})

	// Test: node reaches ledger 3 within 60 seconds.
	suite.Add(xrplsim.ClientTestSpec{
		Name:        "ledger_advance (CLIENT)",
		Description: "Verify the client advances past ledger 3.",
		Role:        "xrpl_validator",
		Parameters: xrplsim.Params{
			"XRPL_STANDALONE":   "1",
			"XRPL_NETWORK_ID":   "10000",
			"XRPL_PEER_PRIVATE": "1",
		},
		Run: func(t *xrplsim.T, c *xrplsim.Client) {
			rpc := xrplsim.NewRPCClient(c.RPCEndpoint())
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			// In standalone mode, ledgers must be closed manually.
			if err := setup.WaitSettled(ctx, rpc, 4); err != nil {
				t.Fatal("ledger_accept failed:", err)
			}

			if err := rpc.WaitForLedger(ctx, 3, 60*time.Second); err != nil {
				t.Fatal("node did not advance to ledger 3:", err)
			}
			t.Log("node reached ledger 3")
		},
	})

	// Test: wallet_propose returns valid account.
	suite.Add(xrplsim.ClientTestSpec{
		Name:        "wallet_propose (CLIENT)",
		Description: "Verify wallet_propose returns a valid account.",
		Role:        "xrpl_validator",
		Run: func(t *xrplsim.T, c *xrplsim.Client) {
			rpc := xrplsim.NewRPCClient(c.RPCEndpoint())
			result, err := rpc.WalletPropose()
			if err != nil {
				t.Fatal("wallet_propose failed:", err)
			}
			if result.AccountID == "" {
				t.Fatal("wallet_propose returned empty account_id")
			}
			t.Logf("proposed account: %s", result.AccountID)
		},
	})

	xrplsim.MustRun(xrplsim.New(), suite)
}
