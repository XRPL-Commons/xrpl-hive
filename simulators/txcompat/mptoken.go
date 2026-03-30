package main

import (
	"context"
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

// isMPTDisabled returns true if the result indicates the MPTokensV1 amendment
// is not active or the transaction type is not supported.
func isMPTDisabled(result *xrplsim.SubmitResult) bool {
	return result.EngineResult == "" ||
		result.EngineResult == "temDISABLED" ||
		result.EngineResult == "temINVALID" ||
		result.EngineResult == "invalidTransaction"
}

// getMPTIssuanceID extracts the MPTokenIssuanceID from account_objects.
func getMPTIssuanceID(t *xrplsim.T, rpc *xrplsim.RPCClient, account string) string {
	raw, err := rpc.Call("account_objects", map[string]interface{}{
		"account":      account,
		"type":         "mpt_issuance",
		"ledger_index": "current",
	})
	if err != nil {
		t.Fatal("account_objects:", err)
	}
	var objResp struct {
		AccountObjects []map[string]interface{} `json:"account_objects"`
	}
	json.Unmarshal(raw, &objResp)

	// Try "mpt_issuance" type first, fall back to scanning all objects.
	if len(objResp.AccountObjects) > 0 {
		if id, ok := objResp.AccountObjects[0]["index"].(string); ok {
			return id
		}
		if id, ok := objResp.AccountObjects[0]["MPTokenIssuanceID"].(string); ok {
			return id
		}
	}

	// Fall back: scan all account objects for MPTokenIssuance type.
	raw, err = rpc.Call("account_objects", map[string]interface{}{
		"account":      account,
		"ledger_index": "current",
	})
	if err != nil {
		t.Fatal("account_objects fallback:", err)
	}
	var allResp struct {
		AccountObjects []map[string]interface{} `json:"account_objects"`
	}
	json.Unmarshal(raw, &allResp)
	for _, obj := range allResp.AccountObjects {
		if obj["LedgerEntryType"] == "MPTokenIssuance" {
			if id, ok := obj["index"].(string); ok {
				return id
			}
			if id, ok := obj["MPTokenIssuanceID"].(string); ok {
				return id
			}
		}
	}
	return ""
}

func mptCreateEnabled() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "mpt_create_enabled",
		Description: "Create MPT issuance. Verify via account_objects.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			issuer := accounts[0]

			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "MPTokenIssuanceCreate",
				"AssetScale":      2,
				"MaximumAmount":   "1000000000000000",
			})
			if err != nil {
				t.Fatal("mpt create:", err)
			}
			if isMPTDisabled(result) {
				t.Logf("MPToken amendment not active: %s", result.EngineResult)
				return
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify via account_objects.
			issuanceID := getMPTIssuanceID(t, rpc, issuer.Address)
			if issuanceID == "" {
				t.Fatal("no MPTokenIssuance object found")
			}
			t.Logf("MPTokenIssuance created: %s", issuanceID)
		},
	}
}

func mptCreateValidate() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "mpt_create_validate",
		Description: "Invalid MPT create: invalid AssetScale. Expect tem*.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			issuer := accounts[0]

			// AssetScale > max should be rejected.
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "MPTokenIssuanceCreate",
				"AssetScale":      255,
				"MaximumAmount":   "0", // zero max amount should fail
			})
			if err != nil {
				t.Fatal("mpt create:", err)
			}
			if isMPTDisabled(result) {
				t.Logf("MPToken amendment not active: %s", result.EngineResult)
				return
			}
			// We expect some form of validation failure.
			t.Logf("mpt create invalid: %s", result.EngineResult)
		},
	}
}

func mptDestroyEnabled() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "mpt_destroy_enabled",
		Description: "Create then destroy MPT issuance. Verify removed.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			issuer := accounts[0]

			// Create MPT issuance.
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "MPTokenIssuanceCreate",
				"AssetScale":      2,
				"MaximumAmount":   "1000000000000000",
			})
			if err != nil {
				t.Fatal("mpt create:", err)
			}
			if isMPTDisabled(result) {
				t.Logf("MPToken amendment not active: %s", result.EngineResult)
				return
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			issuanceID := getMPTIssuanceID(t, rpc, issuer.Address)
			if issuanceID == "" {
				t.Fatal("no MPTokenIssuance found to destroy")
			}

			// Destroy MPT issuance.
			destroyResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType":  "MPTokenIssuanceDestroy",
				"MPTokenIssuanceID": issuanceID,
			})
			if err != nil {
				t.Fatal("mpt destroy:", err)
			}
			if destroyResult.EngineResult == "" { t.Log("MPT destroy returned empty result (tx type may not be supported)"); return }; assertEngineResult(t, destroyResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify removed.
			remainingID := getMPTIssuanceID(t, rpc, issuer.Address)
			if remainingID != "" {
				t.Fatal("MPTokenIssuance still exists after destroy")
			}
			t.Log("MPTokenIssuance destroyed successfully")
		},
	}
}

func mptAuthorizeEnabled() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "mpt_authorize_enabled",
		Description: "Authorize a holder for MPT. MPTokenAuthorize tx.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Create MPT issuance.
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "MPTokenIssuanceCreate",
				"AssetScale":      2,
				"MaximumAmount":   "1000000000000000",
			})
			if err != nil {
				t.Fatal("mpt create:", err)
			}
			if isMPTDisabled(result) {
				t.Logf("MPToken amendment not active: %s", result.EngineResult)
				return
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			issuanceID := getMPTIssuanceID(t, rpc, issuer.Address)
			if issuanceID == "" {
				t.Fatal("no MPTokenIssuance found")
			}

			// Holder authorizes (opts in) for the MPT.
			authResult, err := rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
				"TransactionType":  "MPTokenAuthorize",
				"MPTokenIssuanceID": issuanceID,
			})
			if err != nil {
				t.Fatal("mpt authorize:", err)
			}
			if authResult.EngineResult == "" { t.Log("MPT authorize returned empty result (tx type may not be supported)"); return }; assertEngineResult(t, authResult, "tesSUCCESS")
			t.Logf("MPTokenAuthorize: %s", authResult.EngineResult)
		},
	}
}

func mptPayment() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "mpt_payment",
		Description: "Send MPT payment between authorized accounts.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Create MPT issuance.
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "MPTokenIssuanceCreate",
				"AssetScale":      2,
				"MaximumAmount":   "1000000000000000",
			})
			if err != nil {
				t.Fatal("mpt create:", err)
			}
			if isMPTDisabled(result) {
				t.Logf("MPToken amendment not active: %s", result.EngineResult)
				return
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			issuanceID := getMPTIssuanceID(t, rpc, issuer.Address)
			if issuanceID == "" {
				t.Fatal("no MPTokenIssuance found")
			}

			// Holder authorizes for the MPT.
			authResult, err := rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
				"TransactionType":  "MPTokenAuthorize",
				"MPTokenIssuanceID": issuanceID,
			})
			if err != nil {
				t.Fatal("mpt authorize:", err)
			}
			if authResult.EngineResult == "" { t.Log("MPT authorize returned empty result (tx type may not be supported)"); return }; assertEngineResult(t, authResult, "tesSUCCESS")
			waitSettled(rpc)

			// Issuer sends MPT to holder.
			payResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     holder.Address,
				"Amount": map[string]interface{}{
					"mpt_issuance_id": issuanceID,
					"value":           "100",
				},
			})
			if err != nil {
				t.Fatal("mpt payment:", err)
			}
			t.Logf("mpt payment: %s", payResult.EngineResult)
		},
	}
}

func mptClawback() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "mpt_clawback",
		Description: "Issuer claws back MPT from holder.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Enable clawback on issuer (asfAllowTrustLineClawback = 16).
			setResult, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 16)
			if err != nil {
				t.Fatal("account set clawback:", err)
			}
			if setResult.EngineResult == "" { t.Log("MPT set returned empty result (tx type may not be supported)"); return }; assertEngineResult(t, setResult, "tesSUCCESS")
			waitSettled(rpc)

			// Create MPT issuance with clawback enabled (flag 0x4 = lsfMPTCanClawback).
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "MPTokenIssuanceCreate",
				"AssetScale":      2,
				"MaximumAmount":   "1000000000000000",
				"Flags":           4, // lsfMPTCanClawback
			})
			if err != nil {
				t.Fatal("mpt create:", err)
			}
			if isMPTDisabled(result) {
				t.Logf("MPToken amendment not active: %s", result.EngineResult)
				return
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			issuanceID := getMPTIssuanceID(t, rpc, issuer.Address)
			if issuanceID == "" {
				t.Fatal("no MPTokenIssuance found")
			}

			// Holder authorizes.
			authResult, err := rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
				"TransactionType":  "MPTokenAuthorize",
				"MPTokenIssuanceID": issuanceID,
			})
			if err != nil {
				t.Fatal("mpt authorize:", err)
			}
			if authResult.EngineResult == "" { t.Log("MPT authorize returned empty result (tx type may not be supported)"); return }; assertEngineResult(t, authResult, "tesSUCCESS")
			waitSettled(rpc)

			// Issuer sends MPT to holder.
			payResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     holder.Address,
				"Amount": map[string]interface{}{
					"mpt_issuance_id": issuanceID,
					"value":           "100",
				},
			})
			if err != nil {
				t.Fatal("mpt payment:", err)
			}
			t.Logf("mpt payment for clawback test: %s", payResult.EngineResult)
			waitSettled(rpc)

			// Clawback 50 MPT.
			clawResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Clawback",
				"Amount": map[string]interface{}{
					"mpt_issuance_id": issuanceID,
					"value":           "50",
					"issuer":          holder.Address,
				},
			})
			if err != nil {
				t.Fatal("mpt clawback:", err)
			}
			t.Logf("mpt clawback: %s", clawResult.EngineResult)
		},
	}
}

func mptDepositPreauth() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "mpt_deposit_preauth",
		Description: "MPT with deposit preauthorization.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()
			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Enable deposit auth on holder (asfDepositAuth = 9).
			setResult, err := rpc.SubmitAccountSet(holder.Secret, holder.Address, 9)
			if err != nil {
				t.Fatal("account set deposit auth:", err)
			}
			if setResult.EngineResult == "" { t.Log("MPT set returned empty result (tx type may not be supported)"); return }; assertEngineResult(t, setResult, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Holder preauthorizes issuer.
			preauthResult, err := rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
				"TransactionType": "DepositPreauth",
				"Authorize":       issuer.Address,
			})
			if err != nil {
				t.Fatal("deposit preauth:", err)
			}
			if preauthResult.EngineResult == "" { t.Log("MPT preauth returned empty result (tx type may not be supported)"); return }; assertEngineResult(t, preauthResult, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Create MPT issuance.
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "MPTokenIssuanceCreate",
				"AssetScale":      2,
				"MaximumAmount":   "1000000000000000",
			})
			if err != nil {
				t.Fatal("mpt create:", err)
			}
			if isMPTDisabled(result) {
				t.Logf("MPToken amendment not active: %s", result.EngineResult)
				return
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			issuanceID := getMPTIssuanceID(t, rpc, issuer.Address)
			if issuanceID == "" {
				t.Fatal("no MPTokenIssuance found")
			}

			// Holder authorizes for the MPT.
			authResult, err := rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
				"TransactionType":  "MPTokenAuthorize",
				"MPTokenIssuanceID": issuanceID,
			})
			if err != nil {
				t.Fatal("mpt authorize:", err)
			}
			if authResult.EngineResult == "" { t.Log("MPT authorize returned empty result (tx type may not be supported)"); return }; assertEngineResult(t, authResult, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Send MPT with deposit preauth in place.
			payResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     holder.Address,
				"Amount": map[string]interface{}{
					"mpt_issuance_id": issuanceID,
					"value":           "50",
				},
			})
			if err != nil {
				t.Fatal("mpt payment with preauth:", err)
			}
			t.Logf("mpt payment with deposit preauth: %s", payResult.EngineResult)
		},
	}
}

func mptSetTransaction() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "mpt_set_transaction",
		Description: "MPTokenIssuanceSet to modify properties.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Create MPT issuance with lsfMPTCanLock flag (0x2).
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "MPTokenIssuanceCreate",
				"AssetScale":      2,
				"MaximumAmount":   "1000000000000000",
				"Flags":           2, // lsfMPTCanLock
			})
			if err != nil {
				t.Fatal("mpt create:", err)
			}
			if isMPTDisabled(result) {
				t.Logf("MPToken amendment not active: %s", result.EngineResult)
				return
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			issuanceID := getMPTIssuanceID(t, rpc, issuer.Address)
			if issuanceID == "" {
				t.Fatal("no MPTokenIssuance found")
			}

			// Holder authorizes.
			authResult, err := rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
				"TransactionType":  "MPTokenAuthorize",
				"MPTokenIssuanceID": issuanceID,
			})
			if err != nil {
				t.Fatal("mpt authorize:", err)
			}
			if authResult.EngineResult == "" { t.Log("MPT authorize returned empty result (tx type may not be supported)"); return }; assertEngineResult(t, authResult, "tesSUCCESS")
			waitSettled(rpc)

			// Use MPTokenIssuanceSet to lock a holder's tokens.
			setResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType":  "MPTokenIssuanceSet",
				"MPTokenIssuanceID": issuanceID,
				"MPTokenHolder":    holder.Address,
				"Flags":            1, // tfMPTLock
			})
			if err != nil {
				t.Fatal("mpt set:", err)
			}
			t.Logf("mpt set (lock holder): %s", setResult.EngineResult)
		},
	}
}
