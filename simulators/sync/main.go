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

	clientList, err := xrplsim.New().ClientTypes()
	if err != nil || len(clientList) == 0 {
		// Fallback: register a single test that will fail at runtime with a
		// clear error if no clients are wired up. Otherwise we'd register
		// nothing and the run would silently report 0 tests.
		suite.Add(xrplsim.TestSpec{
			Name:        "late-join-sync",
			Description: "Start a network, advance ledgers, then add a new node that must sync.",
			Run: func(t *xrplsim.T) {
				t.Fatal("no client types available")
			},
		})
		xrplsim.MustRun(xrplsim.New(), suite)
		return
	}

	// Build the cross-product of (initialClient, lateClient) so each pair is
	// exercised. With one client this yields a single test; with two clients
	// (e.g. rxrpl + rippled) it yields four — including the cross-impl
	// catch-up cases (initialA + lateB and initialB + lateA), which are the
	// real interop signal.
	//
	// Why not mix client types within the *initial* network? Each impl
	// auto-advances ledger #1→#2 unilaterally at boot using its own clock;
	// the resulting #2 hashes diverge by close_time and consensus can never
	// align on a common LCL. A homogeneous initial network sidesteps this
	// bootstrap divergence so the late-join sync is what actually gets
	// tested.
	for _, initCli := range clientList {
		for _, lateCli := range clientList {
			initName := initCli.Name
			lateName := lateCli.Name
			testName := fmt.Sprintf("late-join-sync/initial=%s/late=%s", initName, lateName)
			suite.Add(xrplsim.TestSpec{
				Name:        testName,
				Description: fmt.Sprintf("Initial network of %s nodes; %s late joiner catches up.", initName, lateName),
				Run:         makeLateJoinTest(initName, lateName),
			})
		}
	}

	xrplsim.MustRun(xrplsim.New(), suite)
}

// makeLateJoinTest returns a TestSpec.Run closure that boots a homogeneous
// network of `initialClient` validators, advances them past ledger 10, then
// attaches a `lateClient` node and verifies it syncs the chain plus the
// committed payment state.
func makeLateJoinTest(initialClient, lateClient string) func(t *xrplsim.T) {
	return func(t *xrplsim.T) {
		const numInitial = 2
		topo := xrplsim.NewTopology(numInitial + 1) // +1 for the late joiner

		netName := fmt.Sprintf("sync-net-%s-%s", initialClient, lateClient)
		if err := t.Sim.CreateNetwork(t.SuiteID, netName); err != nil {
			t.Fatal("failed to create network:", err)
		}

		quorum := fmt.Sprintf("%d", numInitial)

		// Start homogeneous initial nodes.
		var nodes []*xrplsim.Client
		var peerAddrs []string
		for i := 0; i < numInitial; i++ {
			c := t.StartClient(initialClient,
				xrplsim.WithValidatorConfig(topo, i, peerAddrs),
				xrplsim.WithInitialNetworks([]string{netName}),
				xrplsim.Params{"XRPL_VALIDATION_QUORUM": quorum},
			)
			ip, _ := t.Sim.ContainerNetworkIP(t.SuiteID, netName, c.Container)
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
				peerIP, _ := t.Sim.ContainerNetworkIP(t.SuiteID, netName, peer.Container)
				rpc.Connect(peerIP, xrplsim.DefaultPeerPort)
			}
		}

		// Wait for the network to advance.
		ctx := context.Background()
		for i, node := range nodes {
			rpc := xrplsim.NewRPCClient(node.RPCEndpoint())
			if err := rpc.WaitForLedger(ctx, 10, 300*time.Second); err != nil {
				t.Fatalf("initial node %d did not reach ledger 10: %v", i, err)
			}
		}
		t.Logf("initial %s network reached ledger 10", initialClient)

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

		// Start the late-joining node.
		lateNode := t.StartClient(lateClient,
			xrplsim.WithValidatorConfig(topo, numInitial, peerAddrs),
			xrplsim.WithInitialNetworks([]string{netName}),
			xrplsim.Params{"XRPL_VALIDATION_QUORUM": quorum},
		)

		// Connect late joiner to all existing nodes.
		lateRPC := xrplsim.NewRPCClient(lateNode.RPCEndpoint())
		for _, peer := range nodes {
			peerIP, _ := t.Sim.ContainerNetworkIP(t.SuiteID, netName, peer.Container)
			lateRPC.Connect(peerIP, xrplsim.DefaultPeerPort)
		}

		// Wait for late joiner to sync.
		if err := lateRPC.WaitForLedger(ctx, 10, 180*time.Second); err != nil {
			t.Fatalf("late-join %s node did not sync to ledger 10: %v", lateClient, err)
		}

		// Verify the late joiner has the account we created.
		acct, err := lateRPC.AccountInfo("rPMh7Pi9ct699iZUTWz6CFkakUy5Ju9f9v")
		if err != nil {
			t.Fatal("late-join node doesn't have the account:", err)
		}
		t.Logf("late-join %s synced from %s network — account balance: %s", lateClient, initialClient, acct.Balance)
	}
}
