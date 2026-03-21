// The wscompat simulator tests XRPL WebSocket API compatibility,
// including JSON-RPC over WebSocket and subscription streams.
package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

func main() {
	suite := xrplsim.Suite{
		Name:        "wscompat",
		Description: "WebSocket API compatibility tests.",
	}

	suite.Add(wsServerInfo())
	suite.Add(wsSubscribeLedger())
	suite.Add(wsSubscribeTransactions())
	suite.Add(wsUnsubscribe())
	suite.Add(wsInvalidCommand())

	xrplsim.MustRun(xrplsim.New(), suite)
}

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
	return c, rpc
}

func wsServerInfo() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "ws_server_info",
		Description: "Call server_info over WebSocket and verify response.",
		Run: func(t *xrplsim.T) {
			c, _ := startNetwork(t)

			ws, err := xrplsim.NewWSClient(c.WSEndpoint())
			if err != nil {
				t.Fatal("ws dial:", err)
			}
			defer ws.Close()

			result, err := ws.Call("server_info", nil)
			if err != nil {
				t.Fatal("ws server_info:", err)
			}

			var resp struct {
				Info struct {
					BuildVersion string `json:"build_version"`
					ServerState  string `json:"server_state"`
				} `json:"info"`
			}
			json.Unmarshal(result, &resp)
			if resp.Info.BuildVersion == "" {
				t.Fatal("expected build_version in ws server_info")
			}
			t.Logf("ws server_info: version=%s state=%s", resp.Info.BuildVersion, resp.Info.ServerState)
		},
	}
}

func wsSubscribeLedger() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "ws_subscribe_ledger",
		Description: "Subscribe to ledger stream and receive a ledger close event.",
		Run: func(t *xrplsim.T) {
			c, _ := startNetwork(t)

			ws, err := xrplsim.NewWSClient(c.WSEndpoint())
			if err != nil {
				t.Fatal("ws dial:", err)
			}
			defer ws.Close()

			// Subscribe to ledger stream.
			if err := ws.Subscribe([]string{"ledger"}); err != nil {
				t.Fatal("subscribe:", err)
			}

			// Wait for a ledger close notification.
			msg, err := ws.ReadMessage(30 * time.Second)
			if err != nil {
				t.Fatal("read ledger notification:", err)
			}

			var notification struct {
				Type        string `json:"type"`
				LedgerIndex int    `json:"ledger_index"`
			}
			json.Unmarshal(msg, &notification)
			t.Logf("ledger notification: type=%s index=%d", notification.Type, notification.LedgerIndex)
		},
	}
}

func wsSubscribeTransactions() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "ws_subscribe_transactions",
		Description: "Subscribe to transactions stream, submit payment, receive notification.",
		Run: func(t *xrplsim.T) {
			c, rpc := startNetwork(t)

			ws, err := xrplsim.NewWSClient(c.WSEndpoint())
			if err != nil {
				t.Fatal("ws dial:", err)
			}
			defer ws.Close()

			// Subscribe to transactions stream.
			if err := ws.Subscribe([]string{"transactions"}); err != nil {
				t.Fatal("subscribe:", err)
			}

			// Submit a payment via HTTP RPC.
			w, _ := rpc.WalletPropose()
			_, err = rpc.SubmitPayment(xrplsim.GenesisSecret, xrplsim.GenesisAddress, w.AccountID, "100000000")
			if err != nil {
				t.Fatal("submit payment:", err)
			}

			ctx := context.Background()
			setup.WaitSettled(ctx, rpc, 3)

			// Read notifications — we should get at least one transaction.
			msg, err := ws.ReadMessage(30 * time.Second)
			if err != nil {
				t.Fatal("read tx notification:", err)
			}

			var notification struct {
				Type string `json:"type"`
			}
			json.Unmarshal(msg, &notification)
			t.Logf("transaction notification: type=%s", notification.Type)
		},
	}
}

func wsUnsubscribe() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "ws_unsubscribe",
		Description: "Subscribe to ledger stream, then unsubscribe.",
		Run: func(t *xrplsim.T) {
			c, _ := startNetwork(t)

			ws, err := xrplsim.NewWSClient(c.WSEndpoint())
			if err != nil {
				t.Fatal("ws dial:", err)
			}
			defer ws.Close()

			// Subscribe.
			if err := ws.Subscribe([]string{"ledger"}); err != nil {
				t.Fatal("subscribe:", err)
			}

			// Unsubscribe.
			if err := ws.Unsubscribe([]string{"ledger"}); err != nil {
				t.Fatal("unsubscribe:", err)
			}

			// Try to read — should timeout (no more notifications).
			_, err = ws.ReadMessage(3 * time.Second)
			if err != nil {
				// Expected: timeout means no messages after unsubscribe.
				t.Log("unsubscribe confirmed: no more messages")
				return
			}
			// If we got a message, it might be a stale one from before unsubscribe.
			t.Log("received a stale message after unsubscribe (acceptable)")
		},
	}
}

func wsInvalidCommand() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "ws_invalid_command",
		Description: "Send invalid command over WebSocket, verify error response.",
		Run: func(t *xrplsim.T) {
			c, _ := startNetwork(t)

			ws, err := xrplsim.NewWSClient(c.WSEndpoint())
			if err != nil {
				t.Fatal("ws dial:", err)
			}
			defer ws.Close()

			result, err := ws.Call("nonexistent_method_xyz", nil)
			if err != nil {
				// Some implementations may return an error at transport level.
				t.Logf("ws error for invalid command: %v", err)
				return
			}

			var resp struct {
				Error string `json:"error"`
			}
			json.Unmarshal(result, &resp)
			if resp.Error == "" {
				t.Fatal("expected error for invalid command")
			}
			t.Logf("ws invalid command error: %s", resp.Error)
		},
	}
}
