package main

import (
	"context"
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

func payChannelCreateAndClaim() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "paychan_create_and_claim",
		Description: "Create a payment channel and claim from it.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			_, err := setup.SetupPaymentChannel(context.Background(), rpc,
				src.Address, src.Secret,
				dest.Address, "5000000000", 86400, // 5000 XRP, 1 day settle
			)
			if err != nil {
				t.Fatal("setup payment channel:", err)
			}

			// Find the channel from account_channels.
			raw, err := rpc.Call("account_channels", map[string]interface{}{
				"account":      src.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_channels:", err)
			}
			var resp struct {
				Channels []struct {
					ChannelID string `json:"channel_id"`
					Amount    string `json:"amount"`
					Balance   string `json:"balance"`
				} `json:"channels"`
			}
			json.Unmarshal(raw, &resp)
			if len(resp.Channels) == 0 {
				t.Fatal("no channels found")
			}
			channelID := resp.Channels[0].ChannelID
			t.Logf("channel %s: amount=%s balance=%s", channelID, resp.Channels[0].Amount, resp.Channels[0].Balance)

			// Claim from the channel (destination can claim without signature for close).
			claimResult, err := rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "PaymentChannelClaim",
				"Channel":         channelID,
				"Flags":           65536, // tfClose
			})
			if err != nil {
				t.Fatal("channel claim:", err)
			}
			t.Logf("channel claim: %s", claimResult.EngineResult)
		},
	}
}
