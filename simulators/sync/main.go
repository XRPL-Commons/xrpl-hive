package main

import (
	"context"
	"fmt"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func main() {
	suite := xrplsim.Suite{
		Name:        "sync",
		Description: "Ledger sync tests — late-joining nodes catch up with existing networks.",
	}

	// Test: a node joins late and syncs correctly.
	suite.Add(xrplsim.TestSpec{
		Name:        "late-join-sync",
		Description: "Start a network, advance ledgers, then add a new node that must sync.",
		Run: func(t *xrplsim.T) {
			clients, err := t.Sim.ClientTypes()
			if err != nil || len(clients) == 0 {
				t.Fatal("no client types available")
			}

			// Use up to 3 validators for the initial network.
			numInitial := min(len(clients), 3)
			topo := xrplsim.NewTopology(numInitial + 1) // +1 for the late joiner

			if err := t.Sim.CreateNetwork(t.SuiteID, "sync-net"); err != nil {
				t.Fatal("failed to create network:", err)
			}

			// Set quorum to numInitial so the initial network can advance
			// before the late joiner is online.
			quorum := fmt.Sprintf("%d", numInitial)

			// Start initial nodes.
			var nodes []*xrplsim.Client
			var peerAddrs []string
			for i := 0; i < numInitial; i++ {
				cd := clients[i%len(clients)]
				c := t.StartClient(cd.Name,
					xrplsim.WithValidatorConfig(topo, i, peerAddrs),
					xrplsim.WithInitialNetworks([]string{"sync-net"}),
					xrplsim.Params{"XRPL_VALIDATION_QUORUM": quorum},
				)
				ip, _ := t.Sim.ContainerNetworkIP(t.SuiteID, "sync-net", c.Container)
				peerAddrs = append(peerAddrs, fmt.Sprintf("%s:%d", ip, xrplsim.DefaultPeerPort))
				nodes = append(nodes, c)
			}

			// Peer initial nodes.
			for i, node := range nodes {
				rpc := xrplsim.NewRPCClient(node.RPCEndpoint())
				for j, peer := range nodes {
					if i == j {
						continue
					}
					peerIP, _ := t.Sim.ContainerNetworkIP(t.SuiteID, "sync-net", peer.Container)
					rpc.Connect(peerIP, xrplsim.DefaultPeerPort)
				}
			}

			// Wait for the network to advance.
			ctx := context.Background()
			for i, node := range nodes {
				rpc := xrplsim.NewRPCClient(node.RPCEndpoint())
				if err := rpc.WaitForLedger(ctx, 10, 120*time.Second); err != nil {
					t.Fatalf("initial node %d did not reach ledger 10: %v", i, err)
				}
			}
			t.Log("initial network reached ledger 10")

			// Submit a payment to create some state.
			rpc0 := xrplsim.NewRPCClient(nodes[0].RPCEndpoint())
			rpc0.SubmitPayment(
				xrplsim.GenesisSecret, xrplsim.GenesisAddress,
				"rPMh7Pi9ct699iZUTWz6CFkakUy5Ju9f9v", "50000000",
			)

			// Wait for state to be committed.
			for _, node := range nodes {
				rpc := xrplsim.NewRPCClient(node.RPCEndpoint())
				rpc.WaitForLedger(ctx, 15, 60*time.Second)
			}

			// Start a late-joining node.
			lateClient := clients[0]
			lateNode := t.StartClient(lateClient.Name,
				xrplsim.WithValidatorConfig(topo, numInitial, peerAddrs),
				xrplsim.WithInitialNetworks([]string{"sync-net"}),
				xrplsim.Params{"XRPL_VALIDATION_QUORUM": quorum},
			)

			// Connect late joiner to all existing nodes.
			lateRPC := xrplsim.NewRPCClient(lateNode.RPCEndpoint())
			for _, peer := range nodes {
				peerIP, _ := t.Sim.ContainerNetworkIP(t.SuiteID, "sync-net", peer.Container)
				lateRPC.Connect(peerIP, xrplsim.DefaultPeerPort)
			}

			// Wait for late joiner to sync.
			if err := lateRPC.WaitForLedger(ctx, 10, 180*time.Second); err != nil {
				t.Fatal("late-join node did not sync to ledger 10:", err)
			}

			// Verify the late joiner has the account we created.
			acct, err := lateRPC.AccountInfo("rPMh7Pi9ct699iZUTWz6CFkakUy5Ju9f9v")
			if err != nil {
				t.Fatal("late-join node doesn't have the account:", err)
			}
			t.Logf("late-join node synced — account balance: %s", acct.Balance)
		},
	})

	xrplsim.MustRun(xrplsim.New(), suite)
}
