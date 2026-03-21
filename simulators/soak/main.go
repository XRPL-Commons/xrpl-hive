package main

import (
	"context"
	"fmt"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func main() {
	suite := xrplsim.Suite{
		Name:        "soak",
		Description: "Soak test — generate diverse transaction traffic and verify hash agreement.",
	}

	suite.Add(xrplsim.TestSpec{
		Name:        "traffic-and-oracle",
		Description: "Generate mixed transactions, then verify all nodes agree on ledger hashes.",
		Run: func(t *xrplsim.T) {
			clients, err := t.Sim.ClientTypes()
			if err != nil || len(clients) == 0 {
				t.Fatal("no client types available")
			}

			numNodes := min(len(clients), 3)
			topo := xrplsim.NewTopology(numNodes)

			if err := t.Sim.CreateNetwork(t.SuiteID, "soak-net"); err != nil {
				t.Fatal("failed to create network:", err)
			}

			// Start nodes.
			var nodes []*xrplsim.Client
			var peerAddrs []string
			for i := 0; i < numNodes; i++ {
				cd := clients[i%len(clients)]
				c := t.StartClient(cd.Name,
					xrplsim.WithValidatorConfig(topo, i, peerAddrs),
					xrplsim.WithInitialNetworks([]string{"soak-net"}),
				)
				ip, _ := t.Sim.ContainerNetworkIP(t.SuiteID, "soak-net", c.Container)
				peerAddrs = append(peerAddrs, fmt.Sprintf("%s:%d", ip, xrplsim.DefaultPeerPort))
				nodes = append(nodes, c)
			}

			// Peer nodes.
			for i, node := range nodes {
				rpc := xrplsim.NewRPCClient(node.RPCEndpoint())
				for j, peer := range nodes {
					if i == j {
						continue
					}
					peerIP, _ := t.Sim.ContainerNetworkIP(t.SuiteID, "soak-net", peer.Container)
					rpc.Connect(peerIP, xrplsim.DefaultPeerPort)
				}
			}

			// Wait for network to be ready.
			ctx := context.Background()
			for i, node := range nodes {
				rpc := xrplsim.NewRPCClient(node.RPCEndpoint())
				if err := rpc.WaitForLedger(ctx, 5, 120*time.Second); err != nil {
					t.Fatalf("node %d did not reach ledger 5: %v", i, err)
				}
			}

			// Generate traffic: fund accounts and submit payments.
			submitRPC := xrplsim.NewRPCClient(nodes[0].RPCEndpoint())
			const numAccounts = 5
			const txPerAccount = 3
			var accounts []struct{ address, secret string }

			for i := 0; i < numAccounts; i++ {
				w, err := submitRPC.WalletPropose()
				if err != nil {
					t.Fatalf("wallet_propose failed: %v", err)
				}
				// Fund the account.
				_, err = submitRPC.SubmitPayment(
					xrplsim.GenesisSecret, xrplsim.GenesisAddress,
					w.AccountID, "1000000000",
				)
				if err != nil {
					t.Logf("warning: fund account %d failed: %v", i, err)
				}
				accounts = append(accounts, struct{ address, secret string }{w.AccountID, w.MasterSeed})
			}

			// Wait for funding to settle.
			submitRPC.WaitForLedger(ctx, 10, 60*time.Second)

			// Submit cross-account payments.
			var submitted, succeeded int
			for i, acct := range accounts {
				for j := 0; j < txPerAccount; j++ {
					dest := accounts[(i+j+1)%len(accounts)]
					result, err := submitRPC.SubmitPayment(acct.secret, acct.address, dest.address, "100000")
					if err != nil {
						t.Logf("tx %d/%d failed: %v", i, j, err)
						continue
					}
					submitted++
					if result.EngineResult == "tesSUCCESS" {
						succeeded++
					}
				}
			}
			t.Logf("submitted %d transactions, %d succeeded", submitted, succeeded)

			// Wait for all transactions to settle.
			for _, node := range nodes {
				rpc := xrplsim.NewRPCClient(node.RPCEndpoint())
				rpc.WaitForLedger(ctx, 20, 120*time.Second)
			}

			// Oracle comparison.
			var oracleNodes []xrplsim.OracleNode
			for i, node := range nodes {
				oracleNodes = append(oracleNodes, xrplsim.OracleNode{
					Name:   fmt.Sprintf("%s-%d", clients[i%len(clients)].Name, i),
					Client: xrplsim.NewRPCClient(node.RPCEndpoint()),
				})
			}

			oracle := xrplsim.NewOracle(oracleNodes)
			// Check multiple ledgers for agreement.
			for seq := 5; seq <= 18; seq++ {
				comp, err := oracle.CompareAtSequence(ctx, seq)
				if err != nil {
					t.Logf("oracle check at ledger %d failed: %v", seq, err)
					continue
				}
				if !comp.Agreed {
					t.Logf("DIVERGENCE at ledger %d:", seq)
					for _, d := range comp.Divergences {
						t.Log("  ", d)
					}
					t.Fatal("nodes disagree on ledger hash")
				}
			}

			t.Logf("all nodes agree on ledgers 5-18 after %d transactions", submitted)
		},
	})

	xrplsim.MustRun(xrplsim.New(), suite)
}
