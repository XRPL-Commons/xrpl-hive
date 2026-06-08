package main

import (
	"context"
	"fmt"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func main() {
	suite := xrplsim.Suite{
		Name:        "consensus",
		Description: "Consensus agreement across mixed XRPL validator sets.",
	}

	// Test: mixed validators reach consensus and agree on ledger hashes.
	suite.Add(xrplsim.TestSpec{
		Name:        "mixed-validator-hash-agreement",
		Description: "All nodes in a mixed validator set agree on ledger hashes.",
		Run: func(t *xrplsim.T) {
			clients, err := t.Sim.ClientTypes()
			if err != nil {
				t.Fatal("failed to get client types:", err)
			}
			if len(clients) < 2 {
				t.Fatal("need at least 2 client types for consensus test")
			}

			topo := xrplsim.NewTopology(len(clients))

			if err := t.Sim.CreateNetwork(t.SuiteID, "consensus-net"); err != nil {
				t.Fatal("failed to create network:", err)
			}

			// Start nodes.
			var nodes []*xrplsim.Client
			var peerAddrs []string
			for i, cd := range clients {
				c := t.StartClient(cd.Name,
					xrplsim.WithValidatorConfig(topo, i, peerAddrs),
					xrplsim.WithInitialNetworks([]string{"consensus-net"}),
					// peer_private=1 (the client default) makes rippled drop
					// transactions relayed by non-cluster peers, so a mixed
					// rxrpl+rippled network never converges on tx-bearing
					// ledgers. Same fix the sync (#14) and soak (#15) suites
					// apply.
					xrplsim.Params{"XRPL_PEER_PRIVATE": "0"},
				)
				ip, _ := t.Sim.ContainerNetworkIP(t.SuiteID, "consensus-net", c.Container)
				peerAddrs = append(peerAddrs, fmt.Sprintf("%s:%d", ip, xrplsim.DefaultPeerPort))
				nodes = append(nodes, c)
			}

			// Peer all nodes.
			for i, node := range nodes {
				rpc := xrplsim.NewRPCClient(node.RPCEndpoint())
				for j, peer := range nodes {
					if i == j {
						continue
					}
					peerIP, _ := t.Sim.ContainerNetworkIP(t.SuiteID, "consensus-net", peer.Container)
					rpc.Connect(peerIP, xrplsim.DefaultPeerPort)
				}
			}

			// Wait for all nodes to advance to ledger 10.
			ctx := context.Background()
			targetSeq := 10
			for i, node := range nodes {
				rpc := xrplsim.NewRPCClient(node.RPCEndpoint())
				// An idle 2-validator rxrpl network paces at ~30s/ledger plus
				// a ~60s bootstrap, so reaching ledger 10 takes ~340s. 120s
				// timed out at seq ~5. 600s scopes this to "consensus stuck"
				// rather than "still warming up". rippled-only runs are
				// unaffected (they reach 10 in well under a minute).
				if err := rpc.WaitForLedger(ctx, targetSeq, 600*time.Second); err != nil {
					t.Fatalf("node %s did not reach ledger %d: %v", clients[i].Name, targetSeq, err)
				}
			}

			// Compare ledger hashes using oracle.
			var oracleNodes []xrplsim.OracleNode
			for i, node := range nodes {
				oracleNodes = append(oracleNodes, xrplsim.OracleNode{
					Name:   clients[i].Name,
					Client: xrplsim.NewRPCClient(node.RPCEndpoint()),
				})
			}

			oracle := xrplsim.NewOracle(oracleNodes)
			checkSeq := targetSeq - 2 // Check a safely finalized ledger.
			comp, err := oracle.CompareAtSequence(ctx, checkSeq)
			if err != nil {
				t.Fatal("oracle comparison failed:", err)
			}

			if !comp.Agreed {
				t.Logf("DIVERGENCE at ledger %d:", checkSeq)
				for _, d := range comp.Divergences {
					t.Log("  ", d)
				}
				t.Fatal("nodes disagree on ledger hash")
			}

			t.Logf("all %d nodes agree on ledger %d hash", len(nodes), checkSeq)
		},
	})

	xrplsim.MustRun(xrplsim.New(), suite)
}
