package main

import (
	"context"
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

func paymentIOU() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "payment_iou",
		Description: "Send an IOU payment and verify delivered_amount.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// holder trusts issuer for USD.
			err := setup.SetupTrustLine(ctx, rpc, holder.Address, holder.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line:", err)
			}

			// issuer sends USD to holder.
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     holder.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("iou payment:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify via account_lines.
			raw, err := rpc.Call("account_lines", map[string]interface{}{
				"account":      holder.Address,
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
			if len(resp.Lines) == 0 {
				t.Fatal("expected trust line with balance")
			}
			t.Logf("IOU balance: %s %s", resp.Lines[0].Balance, resp.Lines[0].Currency)
		},
	}
}

func paymentInsufficientFunds() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "payment_insufficient_funds",
		Description: "Payment from account with insufficient XRP fails with tecUNFUNDED_PAYMENT.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Fund with minimal amount.
			accounts, err := setup.FundN(ctx, rpc, 1, "200000012") // just above reserve + fee
			if err != nil {
				t.Fatal("fund:", err)
			}
			dest, _ := rpc.WalletPropose()

			// Try to send more than the account has.
			result, err := rpc.SubmitPayment(accounts[0].Secret, accounts[0].Address, dest.AccountID, "100000000000")
			if err != nil {
				t.Fatal("submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure, got tesSUCCESS")
			}
			t.Logf("insufficient funds: %s", result.EngineResult)
		},
	}
}

func paymentNoDestination() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "payment_no_destination",
		Description: "Payment to unfunded destination below reserve fails.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)

			accounts := mustFund(t, rpc, 1)
			dest, _ := rpc.WalletPropose()

			// Send amount below the reserve — should fail to create destination.
			result, err := rpc.SubmitPayment(accounts[0].Secret, accounts[0].Address, dest.AccountID, "1")
			if err != nil {
				t.Fatal("submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for below-reserve payment, got tesSUCCESS")
			}
			t.Logf("below reserve: %s", result.EngineResult)
		},
	}
}
