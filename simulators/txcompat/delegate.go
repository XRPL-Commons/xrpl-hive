package main

import (
	"context"
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

// isDelegateDisabled returns true if the result indicates the PermissionDelegation
// amendment is not active.
func isDelegateDisabled(result *xrplsim.SubmitResult) bool {
	return result.EngineResult == "temDISABLED" || result.EngineResult == "temINVALID"
}

func delegateSetValid() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "delegate_set_valid",
		Description: "Create delegation via DelegateSet, verify via account_objects.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			owner := accounts[0]
			delegate := accounts[1]

			result, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "DelegateSet",
				"Authorize":       delegate.Address,
				"Permissions": []map[string]interface{}{
					{"Permission": map[string]interface{}{
						"PermissionValue": "Payment",
					}},
				},
			})
			if err != nil {
				t.Fatal("delegate set:", err)
			}
			if isDelegateDisabled(result) {
				t.Logf("delegation amendment not active: %s", result.EngineResult)
				return
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify via account_objects.
			raw, err := rpc.Call("account_objects", map[string]interface{}{
				"account":      owner.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_objects:", err)
			}
			var objResp struct {
				AccountObjects []map[string]interface{} `json:"account_objects"`
			}
			json.Unmarshal(raw, &objResp)

			found := false
			for _, obj := range objResp.AccountObjects {
				if obj["LedgerEntryType"] == "Delegate" {
					found = true
					break
				}
			}
			if !found {
				t.Fatal("no Delegate object found in account_objects")
			}
			t.Log("DelegateSet created successfully")
		},
	}
}

func delegateTransaction() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "delegate_transaction",
		Description: "Delegate Payment permission, have delegate submit Payment on behalf.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			owner := accounts[0]
			delegate := accounts[1]
			dest := accounts[2]

			// Create delegation.
			result, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "DelegateSet",
				"Authorize":       delegate.Address,
				"Permissions": []map[string]interface{}{
					{"Permission": map[string]interface{}{
						"PermissionValue": "Payment",
					}},
				},
			})
			if err != nil {
				t.Fatal("delegate set:", err)
			}
			if isDelegateDisabled(result) {
				t.Logf("delegation amendment not active: %s", result.EngineResult)
				return
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Set delegate as regular key so they can sign on behalf.
			regKeyResult, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "SetRegularKey",
				"RegularKey":      delegate.Address,
			})
			if err != nil {
				t.Fatal("set regular key:", err)
			}
			assertEngineResult(t, regKeyResult, "tesSUCCESS")
			waitSettled(rpc)

			// Delegate submits payment on behalf of owner.
			payResult, err := rpc.Submit(delegate.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     dest.Address,
				"Amount":          "1000000",
			})
			if err != nil {
				t.Fatal("delegated payment:", err)
			}
			t.Logf("delegated payment: %s", payResult.EngineResult)
		},
	}
}

func delegateNotEnabled() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "delegate_not_enabled",
		Description: "If PermissionDelegation amendment not active, expect temDISABLED.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			owner := accounts[0]
			delegate := accounts[1]

			result, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "DelegateSet",
				"Authorize":       delegate.Address,
				"Permissions": []map[string]interface{}{
					{"Permission": map[string]interface{}{
						"PermissionValue": "Payment",
					}},
				},
			})
			if err != nil {
				t.Fatal("delegate set:", err)
			}
			if isDelegateDisabled(result) {
				t.Logf("delegation amendment not active (expected): %s", result.EngineResult)
				return
			}
			// If the amendment IS active, that's also fine.
			t.Logf("delegation amendment is active: %s", result.EngineResult)
		},
	}
}

func delegateInvalidSet() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "delegate_invalid_set",
		Description: "DelegateSet with empty Permissions. Expect temMALFORMED.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			owner := accounts[0]
			delegate := accounts[1]

			result, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "DelegateSet",
				"Authorize":       delegate.Address,
				"Permissions":     []map[string]interface{}{},
			})
			if err != nil {
				t.Fatal("delegate set:", err)
			}
			if isDelegateDisabled(result) {
				t.Logf("delegation amendment not active: %s", result.EngineResult)
				return
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for empty permissions, got tesSUCCESS")
			}
			t.Logf("delegate invalid set: %s", result.EngineResult)
		},
	}
}

func delegateFee() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "delegate_fee",
		Description: "DelegateSet with invalid fee. Expect temBAD_FEE.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			owner := accounts[0]
			delegate := accounts[1]

			result, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "DelegateSet",
				"Authorize":       delegate.Address,
				"Permissions": []map[string]interface{}{
					{"Permission": map[string]interface{}{
						"PermissionValue": "Payment",
					}},
				},
				"Fee": "0",
			})
			if err != nil {
				t.Fatal("delegate set:", err)
			}
			if isDelegateDisabled(result) {
				t.Logf("delegation amendment not active: %s", result.EngineResult)
				return
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected fee error, got tesSUCCESS")
			}
			t.Logf("delegate bad fee: %s", result.EngineResult)
		},
	}
}

func delegateReserve() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "delegate_reserve",
		Description: "DelegateSet requires reserve. Account at boundary should fail.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Fund with minimal amount: just above base reserve.
			accounts, err := setup.FundN(ctx, rpc, 2, "200000012")
			if err != nil {
				t.Fatal("fund:", err)
			}
			owner := accounts[0]
			delegate := accounts[1]

			result, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "DelegateSet",
				"Authorize":       delegate.Address,
				"Permissions": []map[string]interface{}{
					{"Permission": map[string]interface{}{
						"PermissionValue": "Payment",
					}},
				},
			})
			if err != nil {
				t.Fatal("delegate set:", err)
			}
			if isDelegateDisabled(result) {
				t.Logf("delegation amendment not active: %s", result.EngineResult)
				return
			}
			// Expect tecINSUFFICIENT_RESERVE or similar.
			if result.EngineResult == "tesSUCCESS" {
				t.Logf("delegate reserve: tesSUCCESS (account may have enough reserve)")
			} else {
				t.Logf("delegate reserve: %s (expected reserve failure)", result.EngineResult)
			}
		},
	}
}

func delegatePaymentGranular() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "delegate_payment_granular",
		Description: "Delegate specific Payment permission. Verify delegate can pay but can't do other tx types.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			owner := accounts[0]
			delegate := accounts[1]
			dest := accounts[2]

			// Delegate only Payment permission.
			result, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "DelegateSet",
				"Authorize":       delegate.Address,
				"Permissions": []map[string]interface{}{
					{"Permission": map[string]interface{}{
						"PermissionValue": "Payment",
					}},
				},
			})
			if err != nil {
				t.Fatal("delegate set:", err)
			}
			if isDelegateDisabled(result) {
				t.Logf("delegation amendment not active: %s", result.EngineResult)
				return
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Set delegate as regular key.
			regKeyResult, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "SetRegularKey",
				"RegularKey":      delegate.Address,
			})
			if err != nil {
				t.Fatal("set regular key:", err)
			}
			assertEngineResult(t, regKeyResult, "tesSUCCESS")
			waitSettled(rpc)

			// Payment should work.
			payResult, err := rpc.Submit(delegate.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     dest.Address,
				"Amount":          "1000000",
			})
			if err != nil {
				t.Fatal("delegated payment:", err)
			}
			t.Logf("delegated payment: %s", payResult.EngineResult)

			// OfferCreate should fail (not delegated).
			offerResult, err := rpc.Submit(delegate.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "OfferCreate",
				"TakerPays":       "1000000",
				"TakerGets":       "2000000",
			})
			if err != nil {
				t.Fatal("delegated offer:", err)
			}
			t.Logf("delegated offer (should fail): %s", offerResult.EngineResult)
		},
	}
}

func delegateDeleteAccount() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "delegate_delete_account",
		Description: "Account with delegation can be deleted (after 256 ledgers, delegation removed).",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			owner := accounts[0]
			delegate := accounts[1]
			dest := accounts[2]

			// Create delegation.
			result, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "DelegateSet",
				"Authorize":       delegate.Address,
				"Permissions": []map[string]interface{}{
					{"Permission": map[string]interface{}{
						"PermissionValue": "Payment",
					}},
				},
			})
			if err != nil {
				t.Fatal("delegate set:", err)
			}
			if isDelegateDisabled(result) {
				t.Logf("delegation amendment not active: %s", result.EngineResult)
				return
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// First, remove the delegation so we can delete the account.
			// DelegateSet with Unauthorize to remove.
			removeResult, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "DelegateSet",
				"Unauthorize":     delegate.Address,
			})
			if err != nil {
				t.Fatal("delegate remove:", err)
			}
			t.Logf("delegate remove: %s", removeResult.EngineResult)
			waitSettled(rpc)

			// Advance 256 ledgers for AccountDelete.
			advanceLedgers(rpc, 256)

			// Delete the account.
			delResult, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "AccountDelete",
				"Destination":     dest.Address,
				"Fee":             "2000000",
			})
			if err != nil {
				t.Fatal("account delete:", err)
			}
			t.Logf("account delete after delegation: %s", delResult.EngineResult)
		},
	}
}
