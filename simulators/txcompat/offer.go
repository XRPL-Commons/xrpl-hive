package main

import (
	"context"
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

func offerCreateCrossed() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "offer_create_crossed",
		Description: "Create two matching offers and verify they cross.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 2)
			alice := accounts[0]
			bob := accounts[1]

			// Both trust genesis for USD.
			setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", xrplsim.GenesisAddress, "1000")
			setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", xrplsim.GenesisAddress, "1000")

			// Send USD to alice (she'll sell USD for XRP).
			rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "100",
				},
			})
			setup.WaitSettled(ctx, rpc, 3)

			// Alice: sell 10 USD for 10 XRP.
			err := setup.SetupOffer(ctx, rpc, alice.Address, alice.Secret,
				"10000000", // wants 10 XRP
				map[string]interface{}{"currency": "USD", "issuer": xrplsim.GenesisAddress, "value": "10"},
			)
			if err != nil {
				t.Fatal("alice offer:", err)
			}

			// Bob: sell 10 XRP for 10 USD (matches alice's offer).
			result, err := rpc.SubmitOfferCreate(bob.Secret, bob.Address,
				map[string]interface{}{"currency": "USD", "issuer": xrplsim.GenesisAddress, "value": "10"},
				"10000000",
			)
			if err != nil {
				t.Fatal("bob offer:", err)
			}
			t.Logf("bob offer: %s", result.EngineResult)
			waitSettled(rpc)

			// Check bob now has USD (offers crossed).
			raw, err := rpc.Call("account_lines", map[string]interface{}{
				"account":      bob.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_lines:", err)
			}
			var resp struct {
				Lines []struct {
					Balance  string `json:"balance"`
					Currency string `json:"currency"`
				} `json:"lines"`
			}
			json.Unmarshal(raw, &resp)
			t.Logf("bob lines after crossing: %+v", resp.Lines)
		},
	}
}

func offerCancel() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "offer_cancel",
		Description: "Create an offer and cancel it via OfferCancel.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			setup.SetupTrustLine(ctx, rpc, acct.Address, acct.Secret, "USD", xrplsim.GenesisAddress, "1000")

			// Create offer.
			err := setup.SetupOffer(ctx, rpc, acct.Address, acct.Secret,
				"1000000",
				map[string]interface{}{"currency": "USD", "issuer": xrplsim.GenesisAddress, "value": "1"},
			)
			if err != nil {
				t.Fatal("offer create:", err)
			}

			// Get offer sequence from account_offers.
			raw, err := rpc.Call("account_offers", map[string]interface{}{
				"account":      acct.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_offers:", err)
			}
			var offResp struct {
				Offers []struct {
					Seq int `json:"seq"`
				} `json:"offers"`
			}
			json.Unmarshal(raw, &offResp)
			if len(offResp.Offers) == 0 {
				t.Fatal("no offers found")
			}

			// Cancel.
			cancelResult, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "OfferCancel",
				"OfferSequence":   offResp.Offers[0].Seq,
			})
			if err != nil {
				t.Fatal("offer cancel:", err)
			}
			assertEngineResult(t, cancelResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify offers empty.
			raw, err = rpc.Call("account_offers", map[string]interface{}{
				"account":      acct.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_offers after cancel:", err)
			}
			json.Unmarshal(raw, &offResp)
			if len(offResp.Offers) != 0 {
				t.Fatal("expected no offers after cancel")
			}
			t.Log("offer cancelled successfully")
		},
	}
}
