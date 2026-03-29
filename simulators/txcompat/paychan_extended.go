package main

import (
	"context"
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

func paychanSimple() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "paychan_simple",
		Description: "Create payment channel, fund it, verify via account_channels.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType":    "PaymentChannelCreate",
				"Destination":        dest.Address,
				"Amount":             "1000000000", // 1000 XRP
				"SettleDelay":        86400,
				"PublicKey":          "0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
			})
			if err != nil {
				t.Fatal("paychan create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			channels := getChannels(t, rpc, src.Address)
			if len(channels) != 1 {
				t.Fatalf("expected 1 channel, got %d", len(channels))
			}
			if channels[0].Amount != "1000000000" {
				t.Fatalf("channel amount: got %s, want 1000000000", channels[0].Amount)
			}
			t.Logf("channel created: %s amount=%s", channels[0].ChannelID, channels[0].Amount)
		},
	}
}

func paychanSettleDelay() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "paychan_settle_delay",
		Description: "Create channel with settle delay, request close, verify not immediately closed.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			channelID := mustCreateChannel(t, rpc, src, dest, "1000000000", 86400)

			// Source requests close (tfClose).
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "PaymentChannelClaim",
				"Channel":         channelID,
				"Flags":           65536, // tfClose
			})
			if err != nil {
				t.Fatal("claim close:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Channel should still exist (settle delay not elapsed).
			channels := getChannels(t, rpc, src.Address)
			if len(channels) == 0 {
				t.Fatal("channel should still exist during settle delay")
			}
			t.Log("channel still open during settle delay as expected")
		},
	}
}

func paychanDstTag() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "paychan_dst_tag",
		Description: "Create payment channel with DestinationTag.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			// Set asfRequireDest on destination.
			rpc.SubmitAccountSet(dest.Secret, dest.Address, 1) // asfRequireDest
			waitSettled(rpc)

			// Channel without tag should fail.
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "PaymentChannelCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000000",
				"SettleDelay":     86400,
				"PublicKey":       "0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
			})
			if err == nil && result.EngineResult == "tesSUCCESS" {
				t.Error("expected failure without DestinationTag")
			} else {
				t.Logf("no tag: %s (expected failure)", result.EngineResult)
			}

			// Channel with tag should succeed.
			result, err = rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "PaymentChannelCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000000",
				"SettleDelay":     86400,
				"PublicKey":       "0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
				"DestinationTag":  42,
			})
			if err != nil {
				t.Fatal("paychan create with tag:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			t.Log("channel created with DestinationTag")
		},
	}
}

func paychanFund() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "paychan_fund",
		Description: "Create channel then fund it with additional XRP.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			channelID := mustCreateChannel(t, rpc, src, dest, "1000000000", 86400)

			// Fund the channel with more XRP.
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "PaymentChannelFund",
				"Channel":         channelID,
				"Amount":          "500000000", // +500 XRP
			})
			if err != nil {
				t.Fatal("paychan fund:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify increased amount.
			channels := getChannels(t, rpc, src.Address)
			if len(channels) == 0 {
				t.Fatal("no channels")
			}
			if channels[0].Amount != "1500000000" {
				t.Fatalf("channel amount: got %s, want 1500000000", channels[0].Amount)
			}
			t.Log("channel funded to 1500 XRP")
		},
	}
}

func paychanMultipleChannels() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "paychan_multiple_channels",
		Description: "Create multiple payment channels to the same destination.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			mustCreateChannel(t, rpc, src, dest, "100000000", 86400)
			mustCreateChannel(t, rpc, src, dest, "200000000", 86400)

			channels := getChannels(t, rpc, src.Address)
			if len(channels) != 2 {
				t.Fatalf("expected 2 channels, got %d", len(channels))
			}
			t.Log("2 channels created to same destination")
		},
	}
}

func paychanDisallowIncoming() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "paychan_disallow_incoming",
		Description: "Cannot create channel to account with asfDisallowIncomingPayChan.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			// Set asfDisallowIncomingPayChan (14) on destination.
			rpc.SubmitAccountSet(dest.Secret, dest.Address, 14)
			waitSettled(rpc)

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "PaymentChannelCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000000",
				"SettleDelay":     86400,
				"PublicKey":       "0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
			})
			if err != nil {
				t.Fatal("paychan create:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Error("expected failure with disallow incoming paychan flag")
			} else {
				t.Logf("disallow incoming: %s (expected failure)", result.EngineResult)
			}
		},
	}
}

func paychanWithTickets() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "paychan_with_tickets",
		Description: "Create payment channel using a ticket.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			// Create ticket.
			ticketResult, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "TicketCreate",
				"TicketCount":     1,
			})
			if err != nil {
				t.Fatal("ticket create:", err)
			}
			assertEngineResult(t, ticketResult, "tesSUCCESS")
			waitSettled(rpc)

			info, _ := rpc.AccountInfo(src.Address)
			ticketSeq := info.Sequence - 1

			// Create channel with ticket.
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "PaymentChannelCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000000",
				"SettleDelay":     86400,
				"PublicKey":       "0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
				"Sequence":        0,
				"TicketSequence":  ticketSeq,
			})
			if err != nil {
				t.Fatal("paychan with ticket:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			t.Log("channel created with ticket")
		},
	}
}

func paychanDepositAuth() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "paychan_deposit_auth",
		Description: "Payment channel with deposit authorization on destination.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			channelID := mustCreateChannel(t, rpc, src, dest, "1000000000", 86400)

			// Enable deposit auth on destination.
			rpc.SubmitAccountSet(dest.Secret, dest.Address, 9) // asfDepositAuth
			waitSettled(rpc)

			// Claim from source should still work (source owns the channel).
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "PaymentChannelClaim",
				"Channel":         channelID,
				"Flags":           65536, // tfClose
			})
			if err != nil {
				t.Fatal("claim:", err)
			}
			t.Logf("claim with deposit auth: %s", result.EngineResult)
		},
	}
}

func paychanOptionalFields() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "paychan_optional_fields",
		Description: "Create channel with CancelAfter and DestinationTag.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "PaymentChannelCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000000",
				"SettleDelay":     3600,
				"PublicKey":       "0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
				"CancelAfter":     rippleEpoch(86400),
				"DestinationTag":  12345,
			})
			if err != nil {
				t.Fatal("paychan create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			channels := getChannels(t, rpc, src.Address)
			if len(channels) == 0 {
				t.Fatal("no channels")
			}
			t.Logf("channel with optional fields: id=%s", channels[0].ChannelID)
		},
	}
}

// --- helpers ---

type channelInfo struct {
	ChannelID string `json:"channel_id"`
	Amount    string `json:"amount"`
	Balance   string `json:"balance"`
}

func getChannels(t *xrplsim.T, rpc *xrplsim.RPCClient, account string) []channelInfo {
	raw, err := rpc.Call("account_channels", map[string]interface{}{
		"account":      account,
		"ledger_index": "current",
	})
	if err != nil {
		t.Fatal("account_channels:", err)
	}
	var resp struct {
		Channels []channelInfo `json:"channels"`
	}
	json.Unmarshal(raw, &resp)
	return resp.Channels
}

func mustCreateChannel(t *xrplsim.T, rpc *xrplsim.RPCClient, src, dest setup.Account, amount string, settleDelay int) string {
	_, err := setup.SetupPaymentChannel(context.Background(), rpc,
		src.Address, src.Secret, dest.Address, amount, settleDelay,
	)
	if err != nil {
		t.Fatal("create channel:", err)
	}

	channels := getChannels(t, rpc, src.Address)
	if len(channels) == 0 {
		t.Fatal("no channel created")
	}
	return channels[len(channels)-1].ChannelID
}
