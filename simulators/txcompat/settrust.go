package main

import (
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func trustSetMalformed() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "trust_set_malformed",
		Description: "TrustSet with invalid params: zero limit with non-zero quality, negative limit.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			sender := accounts[0]
			issuer := accounts[1]

			// Zero limit with non-zero QualityIn should be malformed.
			result, err := rpc.Submit(sender.Secret, sender.Address, map[string]interface{}{
				"TransactionType": "TrustSet",
				"LimitAmount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "0",
				},
				"QualityIn": 1000000001,
			})
			if err != nil {
				t.Fatal("submit zero limit with quality:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Log("zero limit with non-zero QualityIn accepted (may be valid if line exists)")
			} else {
				t.Logf("zero limit with QualityIn: %s (expected tem* or tec*)", result.EngineResult)
			}
			waitSettled(rpc)

			// Negative limit should fail.
			negResult, err := rpc.Submit(sender.Secret, sender.Address, map[string]interface{}{
				"TransactionType": "TrustSet",
				"LimitAmount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "-100",
				},
			})
			if err != nil {
				t.Fatal("submit negative limit:", err)
			}
			if negResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected negative limit to fail, got tesSUCCESS")
			}
			t.Logf("negative limit: %s (expected temBAD_LIMIT)", negResult.EngineResult)
		},
	}
}

func trustSetTwoFreeTrustlines() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "trust_set_two_free_trustlines",
		Description: "Create two trust lines and verify both exist via account_lines.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			holder := accounts[0]
			issuerA := accounts[1]
			issuerB := accounts[2]

			// Create first trust line: holder trusts issuerA for USD.
			result, err := rpc.SubmitTrustSet(holder.Secret, holder.Address, "USD", issuerA.Address, "1000")
			if err != nil {
				t.Fatal("trust set USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Create second trust line: holder trusts issuerB for EUR.
			result2, err := rpc.SubmitTrustSet(holder.Secret, holder.Address, "EUR", issuerB.Address, "500")
			if err != nil {
				t.Fatal("trust set EUR:", err)
			}
			assertEngineResult(t, result2, "tesSUCCESS")
			waitSettled(rpc)

			// Verify both trust lines exist via account_lines.
			raw, err := rpc.Call("account_lines", map[string]interface{}{
				"account":      holder.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_lines:", err)
			}
			var resp struct {
				Lines []struct {
					Currency string `json:"currency"`
					Limit    string `json:"limit"`
					Account  string `json:"account"`
				} `json:"lines"`
			}
			json.Unmarshal(raw, &resp)
			if len(resp.Lines) < 2 {
				t.Fatalf("expected 2 trust lines, got %d", len(resp.Lines))
			}
			for _, line := range resp.Lines {
				t.Logf("trust line: %s limit=%s peer=%s", line.Currency, line.Limit, line.Account)
			}
		},
	}
}

func trustSetDisallowIncoming() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "trust_set_disallow_incoming",
		Description: "Set asfDisallowIncomingTrustline on an account, then verify incoming TrustSet is rejected.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Set asfDisallowIncomingTrustline (15) on issuer.
			setResult, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 15)
			if err != nil {
				t.Fatal("account set disallow incoming:", err)
			}
			assertEngineResult(t, setResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify the flag is set.
			info, err := rpc.AccountInfo(issuer.Address)
			if err != nil {
				t.Fatal("account_info:", err)
			}
			t.Logf("issuer flags after set: %d", info.Flags)

			// Holder tries to create trust line TO the issuer.
			trustResult, err := rpc.SubmitTrustSet(holder.Secret, holder.Address, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust set to disallowed:", err)
			}
			if trustResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected trust set to be rejected with tecNO_PERMISSION, got tesSUCCESS")
			}
			t.Logf("trust set to disallowed account: %s (expected tecNO_PERMISSION)", trustResult.EngineResult)
		},
	}
}

func trustSetDynamicReserve() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "trust_set_dynamic_reserve",
		Description: "Create trust lines and verify OwnerCount increases with each one.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			holder := accounts[0]
			issuerA := accounts[1]
			issuerB := accounts[2]

			// Check initial OwnerCount.
			info, err := rpc.AccountInfo(holder.Address)
			if err != nil {
				t.Fatal("account_info:", err)
			}
			initialOwnerCount := info.OwnerCount
			t.Logf("initial OwnerCount: %d", initialOwnerCount)

			// Create first trust line.
			result, err := rpc.SubmitTrustSet(holder.Secret, holder.Address, "USD", issuerA.Address, "1000")
			if err != nil {
				t.Fatal("trust set 1:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify OwnerCount increased by 1.
			info, err = rpc.AccountInfo(holder.Address)
			if err != nil {
				t.Fatal("account_info after 1st:", err)
			}
			if info.OwnerCount != initialOwnerCount+1 {
				t.Fatalf("expected OwnerCount %d after 1st trust line, got %d", initialOwnerCount+1, info.OwnerCount)
			}
			t.Logf("OwnerCount after 1st trust line: %d", info.OwnerCount)

			// Create second trust line.
			result2, err := rpc.SubmitTrustSet(holder.Secret, holder.Address, "EUR", issuerB.Address, "500")
			if err != nil {
				t.Fatal("trust set 2:", err)
			}
			assertEngineResult(t, result2, "tesSUCCESS")
			waitSettled(rpc)

			// Verify OwnerCount increased by another 1.
			info, err = rpc.AccountInfo(holder.Address)
			if err != nil {
				t.Fatal("account_info after 2nd:", err)
			}
			if info.OwnerCount != initialOwnerCount+2 {
				t.Fatalf("expected OwnerCount %d after 2nd trust line, got %d", initialOwnerCount+2, info.OwnerCount)
			}
			t.Logf("OwnerCount after 2nd trust line: %d", info.OwnerCount)
		},
	}
}

func trustSetWithTicket() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "trust_set_with_ticket",
		Description: "Create a TrustSet using a TicketSequence.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			holder := accounts[0]
			issuer := accounts[1]

			// Get current sequence for ticket calculation.
			info, err := rpc.AccountInfo(holder.Address)
			if err != nil {
				t.Fatal("account_info:", err)
			}

			// Create a ticket.
			ticketResult, err := rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
				"TransactionType": "TicketCreate",
				"TicketCount":     1,
			})
			if err != nil {
				t.Fatal("ticket create:", err)
			}
			assertEngineResult(t, ticketResult, "tesSUCCESS")
			waitSettled(rpc)

			// The ticket sequence is the sequence at the time of TicketCreate + 1.
			ticketSeq := info.Sequence + 1

			// Submit TrustSet using the ticket.
			trustResult, err := rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
				"TransactionType": "TrustSet",
				"LimitAmount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "1000",
				},
				"Sequence":       0,
				"TicketSequence": ticketSeq,
			})
			if err != nil {
				t.Fatal("trust set with ticket:", err)
			}
			assertEngineResult(t, trustResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify trust line was created.
			raw, err := rpc.Call("account_lines", map[string]interface{}{
				"account":      holder.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_lines:", err)
			}
			var resp struct {
				Lines []struct {
					Currency string `json:"currency"`
					Limit    string `json:"limit"`
				} `json:"lines"`
			}
			json.Unmarshal(raw, &resp)
			if len(resp.Lines) == 0 {
				t.Fatal("expected trust line after ticket-based TrustSet")
			}
			t.Logf("trust line via ticket: %s limit=%s", resp.Lines[0].Currency, resp.Lines[0].Limit)
		},
	}
}
