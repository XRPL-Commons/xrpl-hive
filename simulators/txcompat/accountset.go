package main

import (
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func accountSetFlags() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_set_flags",
		Description: "Set and clear account flags via AccountSet.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Set asfRequireDest (1).
			result, err := rpc.SubmitAccountSet(acct.Secret, acct.Address, 1)
			if err != nil {
				t.Fatal("set flag:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify flag is set.
			raw, err := rpc.Call("account_info", map[string]interface{}{
				"account":      acct.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_info:", err)
			}
			var resp struct {
				AccountData struct {
					Flags json.Number `json:"Flags"`
				} `json:"account_data"`
			}
			json.Unmarshal(raw, &resp)
			t.Logf("flags after set: %s", resp.AccountData.Flags)

			// Clear the flag (ClearFlag).
			clearResult, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AccountSet",
				"ClearFlag":       1,
			})
			if err != nil {
				t.Fatal("clear flag:", err)
			}
			assertEngineResult(t, clearResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify flag is cleared.
			raw, err = rpc.Call("account_info", map[string]interface{}{
				"account":      acct.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_info after clear:", err)
			}
			json.Unmarshal(raw, &resp)
			t.Logf("flags after clear: %s", resp.AccountData.Flags)
		},
	}
}
