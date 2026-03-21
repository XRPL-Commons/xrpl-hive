package main

import (
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func signerListSetAndMultisign() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "signer_list_set_and_multisign",
		Description: "Set a signer list and submit a multi-signed transaction.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			multiAcct := accounts[0]
			signer := accounts[1]

			// Set signer list.
			result, err := rpc.Submit(multiAcct.Secret, multiAcct.Address, map[string]interface{}{
				"TransactionType": "SignerListSet",
				"SignerQuorum":    1,
				"SignerEntries": []map[string]interface{}{
					{"SignerEntry": map[string]interface{}{
						"Account":      signer.Address,
						"SignerWeight": 1,
					}},
				},
			})
			if err != nil {
				t.Fatal("signer list set:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify signer list in account_objects.
			raw, err := rpc.Call("account_objects", map[string]interface{}{
				"account":      multiAcct.Address,
				"type":         "signer_list",
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_objects:", err)
			}
			var objResp struct {
				AccountObjects []interface{} `json:"account_objects"`
			}
			json.Unmarshal(raw, &objResp)
			if len(objResp.AccountObjects) == 0 {
				t.Fatal("no signer list found")
			}
			t.Logf("signer list set with %d object(s)", len(objResp.AccountObjects))
		},
	}
}

func ticketCreateAndUse() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "ticket_create_and_use",
		Description: "Create tickets and use one for a payment.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Get current sequence.
			info, err := rpc.AccountInfo(acct.Address)
			if err != nil {
				t.Fatal("account_info:", err)
			}

			// Create 2 tickets.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "TicketCreate",
				"TicketCount":     2,
			})
			if err != nil {
				t.Fatal("ticket create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// The first ticket sequence is the current sequence.
			ticketSeq := info.Sequence + 1

			// Use a ticket for a payment.
			dest, _ := rpc.WalletPropose()
			payResult, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     dest.AccountID,
				"Amount":          "1000000",
				"Sequence":        0, // Must be 0 when using ticket.
				"TicketSequence":  ticketSeq,
			})
			if err != nil {
				t.Fatal("payment with ticket:", err)
			}
			t.Logf("payment with ticket %d: %s", ticketSeq, payResult.EngineResult)
		},
	}
}
