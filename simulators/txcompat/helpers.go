package main

import (
	"context"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

func startNetwork(t *xrplsim.T) (*xrplsim.Client, *xrplsim.RPCClient) {
	clients, err := t.Sim.ClientTypes()
	if err != nil || len(clients) == 0 {
		t.Fatal("no client types available")
	}

	c := t.StartClient(clients[0].Name, xrplsim.Params{
		"XRPL_STANDALONE":   "1",
		"XRPL_NETWORK_ID":   "10000",
		"XRPL_LOGLEVEL":     "3",
		"XRPL_PEER_PRIVATE": "1",
	})
	rpc := xrplsim.NewRPCClient(c.RPCEndpoint())

	for i := 0; i < 30; i++ {
		if _, err := rpc.ServerInfo(); err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	for i := 0; i < 3; i++ {
		rpc.Call("ledger_accept", nil)
		time.Sleep(200 * time.Millisecond)
	}
	return c, rpc
}

func mustFund(t *xrplsim.T, rpc *xrplsim.RPCClient, n int) []setup.Account {
	ctx := context.Background()
	accounts, err := setup.FundN(ctx, rpc, n, "10000000000")
	if err != nil {
		t.Fatal("fund accounts:", err)
	}
	return accounts
}

func assertEngineResult(t *xrplsim.T, result *xrplsim.SubmitResult, expected string) {
	if result.EngineResult != expected {
		t.Fatalf("expected %s, got %s: %s", expected, result.EngineResult, result.EngineResultMessage)
	}
}

func waitSettled(rpc *xrplsim.RPCClient) {
	ctx := context.Background()
	setup.WaitSettled(ctx, rpc, 3)
}
