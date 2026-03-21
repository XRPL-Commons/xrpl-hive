package main

import (
	"encoding/json"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func oracleSetAndDelete() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "oracle_set_and_delete",
		Description: "Set oracle price data and delete the oracle.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// OracleSet.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType":  "OracleSet",
				"OracleDocumentID": 1,
				"Provider":         "70726F7669646572", // "provider" hex
				"AssetClass":       "63757272656E6379", // "currency" hex
				"LastUpdateTime":   time.Now().Unix() - 10,
				"PriceDataSeries": []map[string]interface{}{
					{
						"PriceData": map[string]interface{}{
							"BaseAsset":  "XRP",
							"QuoteAsset": "USD",
							"AssetPrice": 740,
							"Scale":      3,
						},
					},
				},
			})
			if err != nil {
				t.Fatal("oracle set:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify via account_objects.
			raw, err := rpc.Call("account_objects", map[string]interface{}{
				"account":      acct.Address,
				"type":         "oracle",
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
				t.Fatal("no oracle object found")
			}

			// OracleDelete.
			delResult, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType":  "OracleDelete",
				"OracleDocumentID": 1,
			})
			if err != nil {
				t.Fatal("oracle delete:", err)
			}
			assertEngineResult(t, delResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify removed.
			raw, err = rpc.Call("account_objects", map[string]interface{}{
				"account":      acct.Address,
				"type":         "oracle",
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_objects after delete:", err)
			}
			json.Unmarshal(raw, &objResp)
			if len(objResp.AccountObjects) != 0 {
				t.Fatal("expected oracle removed after delete")
			}
			t.Log("oracle set and deleted successfully")
		},
	}
}
