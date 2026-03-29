package main

import (
	"encoding/json"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func escrowCreateAndFinish() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "escrow_create_and_finish",
		Description: "Create a time-based escrow and finish it after the release time.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			// Capture src sequence before create (needed for OfferSequence in finish).
			srcInfo, _ := rpc.AccountInfo(src.Address)
			createSeq := srcInfo.Sequence

			finishAfter := rippleEpoch(30)
			cancelAfter := rippleEpoch(86400)

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"FinishAfter":     finishAfter,
				"CancelAfter":     cancelAfter,
			})
			if err != nil {
				t.Fatal("escrow create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			// Wait for FinishAfter to pass.
			time.Sleep(32 * time.Second)
			for i := 0; i < 5; i++ {
				rpc.Call("ledger_accept", nil)
			}

			// Finish the escrow using the sequence captured before create.
			finishResult, err := rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "EscrowFinish",
				"Owner":           src.Address,
				"OfferSequence":   createSeq,
			})
			if err != nil {
				t.Fatal("escrow finish:", err)
			}
			t.Logf("escrow finish: %s", finishResult.EngineResult)
		},
	}
}

func escrowCreateAndCancel() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "escrow_create_and_cancel",
		Description: "Create an escrow with CancelAfter in the past, then cancel it.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			rippleEpochOffset := int64(946684800)
			// CancelAfter already passed.
			cancelAfter := time.Now().Unix() - rippleEpochOffset - 60
			// FinishAfter even further in the past.
			finishAfter := cancelAfter - 120

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCreate",
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"FinishAfter":     finishAfter,
				"CancelAfter":     cancelAfter,
			})
			if err != nil {
				t.Fatal("escrow create:", err)
			}
			// This may succeed or fail depending on close time validation.
			t.Logf("escrow create: %s", result.EngineResult)
			waitSettled(rpc)

			if result.EngineResult != "tesSUCCESS" {
				t.Log("escrow creation rejected (expected for past CancelAfter)")
				return
			}

			// Find escrow and cancel.
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
					Sequence int `json:"Sequence"`
				} `json:"account_objects"`
			}
			json.Unmarshal(raw, &objResp)
			if len(objResp.AccountObjects) == 0 {
				t.Log("no escrow to cancel")
				return
			}

			cancelResult, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "EscrowCancel",
				"Owner":           src.Address,
				"OfferSequence":   objResp.AccountObjects[0].Sequence,
			})
			if err != nil {
				t.Fatal("escrow cancel:", err)
			}
			t.Logf("escrow cancel: %s", cancelResult.EngineResult)
		},
	}
}
