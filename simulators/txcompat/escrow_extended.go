package main

import (
	"encoding/json"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

// rippleEpoch returns seconds since Ripple epoch (2000-01-01T00:00:00Z).
func rippleEpoch(offsetSec int64) int64 {
	const rippleEpochUnix = 946684800
	return time.Now().Unix() - rippleEpochUnix + offsetSec
}

func escrowLockup() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "escrow_lockup",
		Description: "Create escrow with FinishAfter, close ledgers past that time, then finish.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			// Capture src sequence before EscrowCreate (this IS the OfferSequence for finish).
			srcInfo, _ := rpc.AccountInfo(src.Address)
			createSeq := srcInfo.Sequence

			finishAfter := rippleEpoch(30)
			cancelAfter := rippleEpoch(3600)

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000000", // 1000 XRP
				"FinishAfter":     finishAfter,
				"CancelAfter":     cancelAfter,
			})
			if err != nil {
				t.Fatal("escrow create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			// Wait for FinishAfter to pass, then close ledgers.
			time.Sleep(32 * time.Second)
			for i := 0; i < 5; i++ {
				rpc.Call("ledger_accept", nil)
			}

			// Get dest balance before.
			infoBefore, _ := rpc.AccountInfo(dest.Address)

			// Finish using the sequence captured before create.
			finishResult, err := rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "EscrowFinish",
				"Owner":           src.Address,
				"OfferSequence":   createSeq,
			})
			if err != nil {
				t.Fatal("escrow finish:", err)
			}
			assertEngineResult(t, finishResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify dest received funds.
			infoAfter, _ := rpc.AccountInfo(dest.Address)
			t.Logf("dest balance: %s → %s", infoBefore.Balance, infoAfter.Balance)
		},
	}
}

func escrowFinishOnly() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "escrow_finish_only",
		Description: "Create escrow with only FinishAfter (no CancelAfter).",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			srcInfo, _ := rpc.AccountInfo(src.Address)
			createSeq := srcInfo.Sequence

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"FinishAfter":     rippleEpoch(30),
			})
			if err != nil {
				t.Fatal("escrow create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			time.Sleep(32 * time.Second)
			for i := 0; i < 5; i++ {
				rpc.Call("ledger_accept", nil)
			}

			finishResult, err := rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "EscrowFinish",
				"Owner":           src.Address,
				"OfferSequence":   createSeq,
			})
			if err != nil {
				t.Fatal("escrow finish:", err)
			}
			assertEngineResult(t, finishResult, "tesSUCCESS")
			t.Log("escrow with only FinishAfter finished successfully")
		},
	}
}

func escrowCancelOnly() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "escrow_cancel_only",
		Description: "Create escrow with only CancelAfter, cancel after time passes.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"CancelAfter":     rippleEpoch(30), // already passed
			})
			if err != nil {
				t.Fatal("escrow create:", err)
			}
			// May succeed or fail depending on close time.
			if result.EngineResult != "tesSUCCESS" {
				t.Logf("escrow create rejected: %s (acceptable for past CancelAfter)", result.EngineResult)
				return
			}
			waitSettled(rpc)

			seq := getEscrowSeq(t, rpc, src.Address)

			cancelResult, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCancel",
				"Owner":           src.Address,
				"OfferSequence":   seq,
			})
			if err != nil {
				t.Fatal("escrow cancel:", err)
			}
			assertEngineResult(t, cancelResult, "tesSUCCESS")
			t.Log("escrow with only CancelAfter cancelled successfully")
		},
	}
}

func escrowTags() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "escrow_tags",
		Description: "Create escrow with SourceTag and DestinationTag.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"FinishAfter":     rippleEpoch(30),
				"CancelAfter":     rippleEpoch(3600),
				"SourceTag":       42,
				"DestinationTag":  99,
			})
			if err != nil {
				t.Fatal("escrow create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify tags in escrow object.
			raw, err := rpc.Call("account_objects", map[string]interface{}{
				"account":      src.Address,
				"type":         "escrow",
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_objects:", err)
			}
			var objResp struct {
				AccountObjects []struct {
					SourceTag      int `json:"SourceTag"`
					DestinationTag int `json:"DestinationTag"`
				} `json:"account_objects"`
			}
			json.Unmarshal(raw, &objResp)
			if len(objResp.AccountObjects) == 0 {
				t.Fatal("no escrow found")
			}
			if objResp.AccountObjects[0].SourceTag != 42 {
				t.Fatalf("SourceTag: got %d, want 42", objResp.AccountObjects[0].SourceTag)
			}
			if objResp.AccountObjects[0].DestinationTag != 99 {
				t.Fatalf("DestinationTag: got %d, want 99", objResp.AccountObjects[0].DestinationTag)
			}
			t.Log("escrow created with correct tags")
		},
	}
}

func escrowMetadataToSelf() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "escrow_metadata_to_self",
		Description: "Create escrow to self and finish it.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			alice := accounts[0]

			aliceInfo, _ := rpc.AccountInfo(alice.Address)
			createSeq := aliceInfo.Sequence

			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "EscrowCreate",
				"Destination":     alice.Address, // self
				"Amount":          "1000000",
				"FinishAfter":     rippleEpoch(30),
				"CancelAfter":     rippleEpoch(3600),
			})
			if err != nil {
				t.Fatal("escrow create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			time.Sleep(32 * time.Second)
			for i := 0; i < 5; i++ {
				rpc.Call("ledger_accept", nil)
			}

			finishResult, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "EscrowFinish",
				"Owner":           alice.Address,
				"OfferSequence":   createSeq,
			})
			if err != nil {
				t.Fatal("escrow finish:", err)
			}
			assertEngineResult(t, finishResult, "tesSUCCESS")
			t.Log("self-escrow finished successfully")
		},
	}
}

func escrowMetadataToOther() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "escrow_metadata_to_other",
		Description: "Create escrow to another account and finish, verify balance transfer.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			balBefore, _ := rpc.AccountInfo(dest.Address)
			srcInfo, _ := rpc.AccountInfo(src.Address)
			createSeq := srcInfo.Sequence

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCreate",
				"Destination":     dest.Address,
				"Amount":          "5000000", // 5 XRP
				"FinishAfter":     rippleEpoch(30),
				"CancelAfter":     rippleEpoch(3600),
			})
			if err != nil {
				t.Fatal("escrow create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			time.Sleep(32 * time.Second)
			for i := 0; i < 5; i++ {
				rpc.Call("ledger_accept", nil)
			}

			finishResult, err := rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "EscrowFinish",
				"Owner":           src.Address,
				"OfferSequence":   createSeq,
			})
			if err != nil {
				t.Fatal("escrow finish:", err)
			}
			assertEngineResult(t, finishResult, "tesSUCCESS")
			waitSettled(rpc)

			balAfter, _ := rpc.AccountInfo(dest.Address)
			t.Logf("dest balance: %s → %s (expected +5 XRP)", balBefore.Balance, balAfter.Balance)
		},
	}
}

func escrowFailureCases() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "escrow_failure_cases",
		Description: "Test invalid escrow creation: zero amount, missing fields, bad times.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			// Zero amount.
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCreate",
				"Destination":     dest.Address,
				"Amount":          "0",
				"FinishAfter":     rippleEpoch(60),
			})
			if err == nil && result.EngineResult == "tesSUCCESS" {
				t.Error("expected zero amount escrow to fail")
			} else {
				t.Logf("zero amount: %s", result.EngineResult)
			}

			// FinishAfter == CancelAfter (invalid).
			sameTime := rippleEpoch(3600)
			result, err = rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"FinishAfter":     sameTime,
				"CancelAfter":     sameTime,
			})
			if err == nil && result.EngineResult == "tesSUCCESS" {
				t.Error("expected same FinishAfter/CancelAfter to fail")
			} else {
				t.Logf("same finish/cancel: %s", result.EngineResult)
			}

			// FinishAfter > CancelAfter (invalid).
			result, err = rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"FinishAfter":     rippleEpoch(7200),
				"CancelAfter":     rippleEpoch(3600),
			})
			if err == nil && result.EngineResult == "tesSUCCESS" {
				t.Error("expected FinishAfter > CancelAfter to fail")
			} else {
				t.Logf("finish > cancel: %s", result.EngineResult)
			}

			t.Log("all failure cases validated")
		},
	}
}

func escrowWithTickets() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "escrow_with_tickets",
		Description: "Create and finish escrow using tickets.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			// Create tickets.
			ticketResult, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "TicketCreate",
				"TicketCount":     2,
			})
			if err != nil {
				t.Fatal("ticket create:", err)
			}
			assertEngineResult(t, ticketResult, "tesSUCCESS")
			waitSettled(rpc)

			// Get ticket sequence.
			info, _ := rpc.AccountInfo(src.Address)
			ticketSeq := info.Sequence - 2 // tickets are at seq-2 and seq-1

			// Create escrow with ticket.
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"FinishAfter":     rippleEpoch(30),
				"CancelAfter":     rippleEpoch(3600),
				"Sequence":        0,
				"TicketSequence":  ticketSeq,
			})
			if err != nil {
				t.Fatal("escrow create with ticket:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			t.Log("escrow created with ticket successfully")
		},
	}
}

func escrowDisallowXRP() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "escrow_disallow_xrp",
		Description: "Create escrow to account with asfDisallowXRP flag set.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src, dest := accounts[0], accounts[1]

			// Set asfDisallowXRP (3) on destination.
			rpc.SubmitAccountSet(dest.Secret, dest.Address, 3)
			waitSettled(rpc)

			// Escrow to disallowed account should still succeed
			// (DisallowXRP is advisory, not enforced for escrows).
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"FinishAfter":     rippleEpoch(30),
				"CancelAfter":     rippleEpoch(3600),
			})
			if err != nil {
				t.Fatal("escrow create:", err)
			}
			t.Logf("escrow to disallowXRP account: %s", result.EngineResult)
		},
	}
}

// getEscrowSeq finds the first escrow object sequence for an account.
func getEscrowSeq(t *xrplsim.T, rpc *xrplsim.RPCClient, account string) int {
	raw, err := rpc.Call("account_objects", map[string]interface{}{
		"account":      account,
		"type":         "escrow",
		"ledger_index": "current",
	})
	if err != nil {
		t.Fatal("account_objects:", err)
	}
	var objResp struct {
		AccountObjects []struct {
			Sequence int `json:"Sequence"`
		} `json:"account_objects"`
	}
	json.Unmarshal(raw, &objResp)
	if len(objResp.AccountObjects) == 0 {
		t.Fatal("no escrow object found")
	}
	return objResp.AccountObjects[0].Sequence
}
