package main

import (
	"context"
	"fmt"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func main() {
	suite := xrplsim.Suite{
		Name:        "propagation",
		Description: "Transaction propagation across XRPL implementations.",
	}

	// Test: submit payment to one impl, verify on another.
	suite.Add(xrplsim.TestSpec{
		Name:        "cross-impl-payment",
		Description: "Submit payment to implementation A, verify account exists on implementation B.",
		Run: func(t *xrplsim.T) {
			clients, err := t.Sim.ClientTypes()
			if err != nil {
				t.Fatal("failed to get client types:", err)
			}
			if len(clients) < 2 {
				t.Fatal("need at least 2 client types for cross-impl propagation test")
			}

			topo := xrplsim.NewTopology(len(clients))

			// Create network.
			if err := t.Sim.CreateNetwork(t.SuiteID, "testnet"); err != nil {
				t.Fatal("failed to create network:", err)
			}

			// Start one node of each client type.
			var nodes []*xrplsim.Client
			var peerAddrs []string
			for i, cd := range clients {
				c := t.StartClient(cd.Name,
					xrplsim.WithValidatorConfig(topo, i, peerAddrs),
					xrplsim.WithInitialNetworks([]string{"testnet"}),
					// peer_private=1 (the client default) makes rippled drop
					// transactions relayed by non-cluster peers, so a payment
					// submitted to the rxrpl node never reaches rippled and the
					// propagation check times out. Same fix sync (#14) and soak
					// (#15) apply.
					xrplsim.Params{"XRPL_PEER_PRIVATE": "0"},
				)
				ip, err := t.Sim.ContainerNetworkIP(t.SuiteID, "testnet", c.Container)
				if err != nil {
					t.Fatal("failed to get container IP:", err)
				}
				peerAddrs = append(peerAddrs, fmt.Sprintf("%s:%d", ip, xrplsim.DefaultPeerPort))
				nodes = append(nodes, c)
			}

			// Connect all nodes to each other via admin RPC.
			for i, node := range nodes {
				rpc := xrplsim.NewRPCClient(node.RPCEndpoint())
				for j, peer := range nodes {
					if i == j {
						continue
					}
					peerIP, _ := t.Sim.ContainerNetworkIP(t.SuiteID, "testnet", peer.Container)
					if err := rpc.Connect(peerIP, xrplsim.DefaultPeerPort); err != nil {
						t.Logf("warning: connect %s -> %s failed: %v", clients[i].Name, clients[j].Name, err)
					}
				}
			}

			// Wait for all nodes to reach ledger 5.
			ctx := context.Background()
			for i, node := range nodes {
				rpc := xrplsim.NewRPCClient(node.RPCEndpoint())
				// rxrpl idle pace is ~30s/ledger + ~60s bootstrap, so even
				// ledger 5 can take ~210s. 120s timed out before warmup
				// finished. 600s scopes to "genuinely stuck".
				if err := rpc.WaitForLedger(ctx, 5, 600*time.Second); err != nil {
					t.Fatalf("node %s did not reach ledger 5: %v", clients[i].Name, err)
				}
			}

			// Submit payment via node 0.
			rpc0 := xrplsim.NewRPCClient(nodes[0].RPCEndpoint())
			result, err := rpc0.SubmitPayment(
				xrplsim.GenesisSecret,
				xrplsim.GenesisAddress,
				"ra8sezk7XT7JRgE1myhUBZJUDCUH3qrWMU",
				"100000000",
			)
			if err != nil {
				t.Fatal("submit payment failed:", err)
			}
			t.Logf("submitted via %s: %s = %s", clients[0].Name, result.TxHash, result.EngineResult)

			// Wait for propagation.
			rpc1 := xrplsim.NewRPCClient(nodes[1].RPCEndpoint())
			// The payment lands a few ledgers after submit; at ~30s/ledger
			// reaching seq 8 from seq 5 is ~90s+. 60s was too tight. 300s
			// gives the relayed tx time to propagate and a ledger to close.
			if err := rpc1.WaitForLedger(ctx, 8, 300*time.Second); err != nil {
				t.Fatalf("node %s did not advance: %v", clients[1].Name, err)
			}

			// Verify account exists on node 1.
			acct, err := rpc1.AccountInfo("ra8sezk7XT7JRgE1myhUBZJUDCUH3qrWMU")
			if err != nil {
				t.Fatalf("account not found on %s: %v", clients[1].Name, err)
			}
			t.Logf("account balance on %s: %s", clients[1].Name, acct.Balance)
		},
	})

	xrplsim.MustRun(xrplsim.New(), suite)
}
