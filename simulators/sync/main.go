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
		// The trusted UNL contains only the initial validators. rippled
		// applies the BFT 80% rule (quorum = ceil(0.8 * trusted_count)),
		// so adding the late joiner here would force quorum=3 and the
		// 2-node initial network would never validate. The late joiner
		// is a pure observer/follower; it doesn't need to be trusted to
		// catch up — only to receive validations from the trusted set.
		topo := xrplsim.NewTopology(numInitial)

		netName := fmt.Sprintf("sync-net-%s-%s", initialClient, lateClient)
		if err := t.Sim.CreateNetwork(t.SuiteID, netName); err != nil {
			t.Fatal("failed to create network:", err)
		}

		quorum := fmt.Sprintf("%d", numInitial)

		// peer_private=1 (the topology default) makes rippled reject any
		// inbound connection that isn't in [ips_fixed], replying with a
		// 503 redirect. The late joiner's IP isn't known to the initial
		// nodes at boot time, so it would be rejected forever. Override
		// to 0 for the sync suite — rxrpl ignores this flag entirely
		// (rxrpl-only runs were already passing because of that).
		commonParams := xrplsim.Params{
			"XRPL_VALIDATION_QUORUM": quorum,
			"XRPL_PEER_PRIVATE":      "0",
		}

		// Start homogeneous initial nodes.
		var nodes []*xrplsim.Client
		var peerAddrs []string
		for i := 0; i < numInitial; i++ {
			c := t.StartClient(initialClient,
				xrplsim.WithValidatorConfig(topo, i, peerAddrs),
				xrplsim.WithInitialNetworks([]string{netName}),
				commonParams,
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

		// Wait for state to be committed. rippled paces ledgers slowly when
		// idle (ledgerIDLE_INTERVAL=15s) so allow enough time for ~5 closes
		// after the payment was submitted.
		const targetLedger = 15
		for i, node := range nodes {
			rpc := xrplsim.NewRPCClient(node.RPCEndpoint())
			if err := rpc.WaitForLedger(ctx, targetLedger, 180*time.Second); err != nil {
				t.Fatalf("initial node %d did not commit payment by ledger %d: %v", i, targetLedger, err)
			}
		}

		// Start the late-joining node.
		lateNode := t.StartClient(lateClient,
			xrplsim.WithValidatorConfig(topo, numInitial, peerAddrs),
			xrplsim.WithInitialNetworks([]string{netName}),
			commonParams,
		)

		// Connect late joiner to all existing nodes.
		lateRPC := xrplsim.NewRPCClient(lateNode.RPCEndpoint())
		for _, peer := range nodes {
			peerIP, _ := t.Sim.ContainerNetworkIP(t.SuiteID, netName, peer.Container)
			lateRPC.Connect(peerIP, xrplsim.DefaultPeerPort)
		}

		// Wait for late joiner to sync past the ledger that contains the
		// payment. Using the same target the initial network reached
		// guarantees AccountInfo can see the funded account.
		if err := lateRPC.WaitForLedger(ctx, targetLedger, 240*time.Second); err != nil {
			t.Fatalf("late-join %s node did not sync to ledger %d: %v", lateClient, targetLedger, err)
		}

		// rippled returns `noNetwork` from account_info while server_state
		// is "connected" (validations seen but state not yet replayed).
		// Retry until rippled finishes catchup or we time out. rxrpl
		// answers immediately so the loop exits on first try.
		var acct *xrplsim.AccountInfoResult
		var lastErr error
		acctDeadline := time.Now().Add(120 * time.Second)
		for time.Now().Before(acctDeadline) {
			acct, lastErr = lateRPC.AccountInfo("rPMh7Pi9ct699iZUTWz6CFkakUy5Ju9f9v")
			if lastErr == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if lastErr != nil {
			t.Fatalf("late-join %s node didn't replay state for account in time: %v", lateClient, lastErr)
		}
		t.Logf("late-join %s synced from %s network — account balance: %s", lateClient, initialClient, acct.Balance)
	}
}
