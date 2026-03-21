package main

import (
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func didSetAndDelete() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "did_set_and_delete",
		Description: "Set DID data, verify via account_objects, then delete.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// DIDSet.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "DIDSet",
				"URI":             "68747470733A2F2F6578616D706C652E636F6D", // https://example.com
				"Data":            "48656C6C6F",                             // Hello
			})
			if err != nil {
				t.Fatal("did set:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify via account_objects.
			raw, err := rpc.Call("account_objects", map[string]interface{}{
				"account":      acct.Address,
				"type":         "did",
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
				t.Fatal("no DID object found")
			}

			// DIDDelete.
			delResult, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "DIDDelete",
			})
			if err != nil {
				t.Fatal("did delete:", err)
			}
			assertEngineResult(t, delResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify removed.
			raw, err = rpc.Call("account_objects", map[string]interface{}{
				"account":      acct.Address,
				"type":         "did",
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_objects after delete:", err)
			}
			json.Unmarshal(raw, &objResp)
			if len(objResp.AccountObjects) != 0 {
				t.Fatal("expected DID removed after delete")
			}
			t.Log("DID set and deleted successfully")
		},
	}
}
