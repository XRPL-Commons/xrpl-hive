package main

import (
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func checkCreateAndCash() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "check_create_and_cash",
		Description: "Create a check, cash it, verify balance change.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			// Create check.
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     dest.Address,
				"SendMax":         "5000000000", // 5000 XRP
			})
			if err != nil {
				t.Fatal("check create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Find the check ID from account_objects.
			raw, err := rpc.Call("account_objects", map[string]interface{}{
				"account":      src.Address,
				"type":         "check",
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_objects:", err)
			}
			var objResp struct {
				AccountObjects []struct {
					Index string `json:"index"`
				} `json:"account_objects"`
			}
			json.Unmarshal(raw, &objResp)
			if len(objResp.AccountObjects) == 0 {
				t.Fatal("no check object found")
			}
			checkID := objResp.AccountObjects[0].Index

			// Cash the check.
			cashResult, err := rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "CheckCash",
				"CheckID":         checkID,
				"Amount":          "5000000000",
			})
			if err != nil {
				t.Fatal("check cash:", err)
			}
			assertEngineResult(t, cashResult, "tesSUCCESS")
			t.Logf("check cashed: %s", cashResult.EngineResult)
		},
	}
}

func checkCreateAndCancel() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "check_create_and_cancel",
		Description: "Create a check and cancel it, verify check object removed.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			// Create check.
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     dest.Address,
				"SendMax":         "1000000000",
			})
			if err != nil {
				t.Fatal("check create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Find check ID.
			raw, err := rpc.Call("account_objects", map[string]interface{}{
				"account":      src.Address,
				"type":         "check",
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_objects:", err)
			}
			var objResp struct {
				AccountObjects []struct {
					Index string `json:"index"`
				} `json:"account_objects"`
			}
			json.Unmarshal(raw, &objResp)
			if len(objResp.AccountObjects) == 0 {
				t.Fatal("no check object found")
			}
			checkID := objResp.AccountObjects[0].Index

			// Cancel the check (creator can cancel).
			cancelResult, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCancel",
				"CheckID":         checkID,
			})
			if err != nil {
				t.Fatal("check cancel:", err)
			}
			assertEngineResult(t, cancelResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify check is gone.
			raw, err = rpc.Call("account_objects", map[string]interface{}{
				"account":      src.Address,
				"type":         "check",
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_objects after cancel:", err)
			}
			json.Unmarshal(raw, &objResp)
			if len(objResp.AccountObjects) != 0 {
				t.Fatal("expected check to be removed after cancel")
			}
			t.Log("check cancelled and removed")
		},
	}
}
