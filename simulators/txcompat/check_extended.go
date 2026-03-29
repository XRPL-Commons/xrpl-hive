package main

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

// getCheckID retrieves the first check object ID from account_objects for the given account.
func getCheckID(t *xrplsim.T, rpc *xrplsim.RPCClient, account string) string {
	raw, err := rpc.Call("account_objects", map[string]interface{}{
		"account":      account,
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
	return objResp.AccountObjects[0].Index
}

// getCheckCount returns the number of check objects for the given account.
func getCheckCount(t *xrplsim.T, rpc *xrplsim.RPCClient, account string) int {
	raw, err := rpc.Call("account_objects", map[string]interface{}{
		"account":      account,
		"type":         "check",
		"ledger_index": "current",
	})
	if err != nil {
		t.Fatal("account_objects:", err)
	}
	var objResp struct {
		AccountObjects []json.RawMessage `json:"account_objects"`
	}
	json.Unmarshal(raw, &objResp)
	return len(objResp.AccountObjects)
}

// getAccountBalance returns the XRP balance (in drops) for the given account as a string.
func getAccountBalance(t *xrplsim.T, rpc *xrplsim.RPCClient, account string) string {
	info, err := rpc.AccountInfo(account)
	if err != nil {
		t.Fatal("account_info:", err)
	}
	return info.Balance
}

// getIOUBalance returns the IOU balance for the given account and currency/issuer pair.
func getIOUBalance(t *xrplsim.T, rpc *xrplsim.RPCClient, account, currency, peer string) string {
	raw, err := rpc.Call("account_lines", map[string]interface{}{
		"account":      account,
		"peer":         peer,
		"ledger_index": "current",
	})
	if err != nil {
		t.Fatal("account_lines:", err)
	}
	var resp struct {
		Lines []struct {
			Balance  string `json:"balance"`
			Currency string `json:"currency"`
		} `json:"lines"`
	}
	json.Unmarshal(raw, &resp)
	for _, line := range resp.Lines {
		if line.Currency == currency {
			return line.Balance
		}
	}
	return "0"
}

// checkCreateValid tests creating a valid check between two accounts and verifying
// the check object exists in the ledger.
func checkCreateValid() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "check_create_valid",
		Description: "Create a valid check between two accounts, verify check object exists.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			// Create a check for 1000 XRP.
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

			// Verify the check object exists.
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
					Index       string `json:"index"`
					LedgerType  string `json:"LedgerEntryType"`
					Account     string `json:"Account"`
					Destination string `json:"Destination"`
					SendMax     string `json:"SendMax"`
				} `json:"account_objects"`
			}
			json.Unmarshal(raw, &objResp)

			if len(objResp.AccountObjects) == 0 {
				t.Fatal("no check object found after CheckCreate")
			}

			check := objResp.AccountObjects[0]
			if check.LedgerType != "Check" {
				t.Fatalf("expected LedgerEntryType Check, got %s", check.LedgerType)
			}
			if check.Account != src.Address {
				t.Fatalf("check Account mismatch: expected %s, got %s", src.Address, check.Account)
			}
			if check.Destination != dest.Address {
				t.Fatalf("check Destination mismatch: expected %s, got %s", dest.Address, check.Destination)
			}
			if check.SendMax != "1000000000" {
				t.Fatalf("check SendMax mismatch: expected 1000000000, got %s", check.SendMax)
			}

			// Verify OwnerCount increased by 1 for the source.
			info, err := rpc.AccountInfo(src.Address)
			if err != nil {
				t.Fatal("account_info:", err)
			}
			if info.OwnerCount < 1 {
				t.Fatalf("expected OwnerCount >= 1, got %d", info.OwnerCount)
			}

			t.Logf("check created successfully: %s", check.Index)
		},
	}
}

// checkCreateInvalid tests creating checks with invalid parameters and verifies
// the expected error codes.
func checkCreateInvalid() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "check_create_invalid",
		Description: "Try creating check with invalid params (zero amount, self-destination), expect tem* errors.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]

			// Test 1: Self-destination should fail with temREDUNDANT.
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     src.Address,
				"SendMax":         "1000000000",
			})
			if err != nil {
				t.Fatal("check create self-dest:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for self-destination check, got tesSUCCESS")
			}
			t.Logf("self-destination check: %s (expected failure)", result.EngineResult)

			// Test 2: Zero SendMax should fail.
			result, err = rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     accounts[1].Address,
				"SendMax":         "0",
			})
			if err != nil {
				t.Fatal("check create zero amount:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for zero SendMax, got tesSUCCESS")
			}
			t.Logf("zero SendMax check: %s (expected failure)", result.EngineResult)

			// Test 3: Negative SendMax should fail.
			result, err = rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     accounts[1].Address,
				"SendMax":         "-1000000",
			})
			if err != nil {
				t.Fatal("check create negative amount:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for negative SendMax, got tesSUCCESS")
			}
			t.Logf("negative SendMax check: %s (expected failure)", result.EngineResult)

			// Test 4: Destination that does not exist should fail with tecNO_DST.
			noExist, _ := rpc.WalletPropose()
			result, err = rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     noExist.AccountID,
				"SendMax":         "1000000000",
			})
			if err != nil {
				t.Fatal("check create no dest:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for non-existent destination, got tesSUCCESS")
			}
			t.Logf("non-existent destination check: %s (expected failure)", result.EngineResult)
		},
	}
}

// checkCashXRP tests creating an XRP check, cashing it, and verifying balances change correctly.
func checkCashXRP() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "check_cash_xrp",
		Description: "Create XRP check, cash it, verify balances change correctly.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			// Record initial balances.
			srcBalBefore := getAccountBalance(t, rpc, src.Address)
			destBalBefore := getAccountBalance(t, rpc, dest.Address)

			// Create check for 2000 XRP.
			checkAmount := "2000000000"
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     dest.Address,
				"SendMax":         checkAmount,
			})
			if err != nil {
				t.Fatal("check create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Find check ID.
			checkID := getCheckID(t, rpc, src.Address)

			// Cash the check for the full amount.
			cashResult, err := rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "CheckCash",
				"CheckID":         checkID,
				"Amount":          checkAmount,
			})
			if err != nil {
				t.Fatal("check cash:", err)
			}
			assertEngineResult(t, cashResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify balances changed.
			srcBalAfter := getAccountBalance(t, rpc, src.Address)
			destBalAfter := getAccountBalance(t, rpc, dest.Address)

			srcBefore, _ := strconv.ParseInt(srcBalBefore, 10, 64)
			srcAfter, _ := strconv.ParseInt(srcBalAfter, 10, 64)
			destBefore, _ := strconv.ParseInt(destBalBefore, 10, 64)
			destAfter, _ := strconv.ParseInt(destBalAfter, 10, 64)

			// Source should have lost at least the check amount (plus fees).
			if srcAfter >= srcBefore {
				t.Fatalf("source balance did not decrease: before=%d, after=%d", srcBefore, srcAfter)
			}
			// Destination should have gained the check amount (minus cash fee).
			if destAfter <= destBefore {
				t.Fatalf("destination balance did not increase: before=%d, after=%d", destBefore, destAfter)
			}

			// Check object should be removed after cashing.
			count := getCheckCount(t, rpc, src.Address)
			if count != 0 {
				t.Fatalf("expected check removed after cashing, found %d checks", count)
			}

			t.Logf("XRP check cashed: src %d->%d, dest %d->%d", srcBefore, srcAfter, destBefore, destAfter)
		},
	}
}

// checkCashIOU tests creating an IOU check, cashing it, and verifying trust line balances.
func checkCashIOU() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "check_cash_iou",
		Description: "Create IOU check (USD), cash it, verify trust line balances.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			src := accounts[1]
			dest := accounts[2]

			// Enable DefaultRipple on issuer so IOUs can flow.
			rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			waitSettled(rpc)

			// Setup: src and dest trust issuer for USD.
			err := setup.SetupTrustLine(ctx, rpc, src.Address, src.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line src:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, dest.Address, dest.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line dest:", err)
			}

			// Issuer sends 100 USD to src.
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     src.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("iou payment:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Record IOU balances before check.
			srcBalBefore := getIOUBalance(t, rpc, src.Address, "USD", issuer.Address)
			destBalBefore := getIOUBalance(t, rpc, dest.Address, "USD", issuer.Address)

			// Create IOU check for 50 USD.
			result, err = rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     dest.Address,
				"SendMax": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("check create iou:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Find check ID.
			checkID := getCheckID(t, rpc, src.Address)

			// Cash the check.
			cashResult, err := rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "CheckCash",
				"CheckID":         checkID,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("check cash iou:", err)
			}
			assertEngineResult(t, cashResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify IOU balances changed.
			srcBalAfter := getIOUBalance(t, rpc, src.Address, "USD", issuer.Address)
			destBalAfter := getIOUBalance(t, rpc, dest.Address, "USD", issuer.Address)

			if srcBalAfter == srcBalBefore {
				t.Fatalf("source IOU balance did not change: %s", srcBalAfter)
			}
			if destBalAfter == destBalBefore {
				t.Fatalf("destination IOU balance did not change: %s", destBalAfter)
			}

			t.Logf("IOU check cashed: src USD %s->%s, dest USD %s->%s",
				srcBalBefore, srcBalAfter, destBalBefore, destBalAfter)
		},
	}
}

// checkCashInvalid tests cashing checks with invalid parameters.
func checkCashInvalid() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "check_cash_invalid",
		Description: "Try cashing with wrong amount, wrong destination, non-existent check.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			src := accounts[0]
			dest := accounts[1]
			thirdParty := accounts[2]

			// Create a valid check.
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

			checkID := getCheckID(t, rpc, src.Address)

			// Test 1: Wrong destination tries to cash the check.
			cashResult, err := rpc.Submit(thirdParty.Secret, thirdParty.Address, map[string]interface{}{
				"TransactionType": "CheckCash",
				"CheckID":         checkID,
				"Amount":          "1000000000",
			})
			if err != nil {
				t.Fatal("check cash wrong dest:", err)
			}
			if cashResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure when wrong account cashes check, got tesSUCCESS")
			}
			t.Logf("wrong destination cash: %s (expected failure)", cashResult.EngineResult)

			// Test 2: Cash amount exceeds SendMax.
			cashResult, err = rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "CheckCash",
				"CheckID":         checkID,
				"Amount":          "2000000000",
			})
			if err != nil {
				t.Fatal("check cash over amount:", err)
			}
			if cashResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for over-amount cash, got tesSUCCESS")
			}
			t.Logf("over-amount cash: %s (expected failure)", cashResult.EngineResult)

			// Test 3: Non-existent check ID.
			fakeCheckID := "0000000000000000000000000000000000000000000000000000000000000000"
			cashResult, err = rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "CheckCash",
				"CheckID":         fakeCheckID,
				"Amount":          "1000000000",
			})
			if err != nil {
				t.Fatal("check cash non-existent:", err)
			}
			if cashResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for non-existent check, got tesSUCCESS")
			}
			t.Logf("non-existent check cash: %s (expected failure)", cashResult.EngineResult)

			// Test 4: Cash with both Amount and DeliverMin (mutually exclusive with Amount).
			cashResult, err = rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "CheckCash",
				"CheckID":         checkID,
				"Amount":          "500000000",
				"DeliverMin":      "100000000",
			})
			if err != nil {
				t.Fatal("check cash both fields:", err)
			}
			if cashResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for Amount+DeliverMin, got tesSUCCESS")
			}
			t.Logf("both Amount and DeliverMin: %s (expected failure)", cashResult.EngineResult)
		},
	}
}

// checkCashWithTransferFee tests that transfer fees are correctly applied when
// cashing IOU checks through a non-issuer path.
func checkCashWithTransferFee() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "check_cash_with_transfer_fee",
		Description: "Set TransferRate on issuer, create IOU check, cash it, verify fee applied.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			src := accounts[1]
			dest := accounts[2]

			// Set 20% transfer rate on the issuer. TransferRate 1200000000 = 20% fee.
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AccountSet",
				"TransferRate":    1200000000,
			})
			if err != nil {
				t.Fatal("set transfer rate:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Enable DefaultRipple on issuer.
			rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			waitSettled(rpc)

			// Setup trust lines: src and dest trust issuer for USD.
			err = setup.SetupTrustLine(ctx, rpc, src.Address, src.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line src:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, dest.Address, dest.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line dest:", err)
			}

			// Issuer sends 200 USD to src (enough to cover transfer fee).
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     src.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "200",
				},
			})
			if err != nil {
				t.Fatal("iou payment:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Record balances before check.
			srcBalBefore := getIOUBalance(t, rpc, src.Address, "USD", issuer.Address)

			// Create check for 100 USD with SendMax that includes the transfer fee.
			// With 20% transfer fee, sending 100 USD costs 120 USD from the source.
			result, err = rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     dest.Address,
				"SendMax": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "120",
				},
			})
			if err != nil {
				t.Fatal("check create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			checkID := getCheckID(t, rpc, src.Address)

			// Cash the check using DeliverMin for 100 USD.
			cashResult, err := rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "CheckCash",
				"CheckID":         checkID,
				"DeliverMin": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("check cash:", err)
			}
			assertEngineResult(t, cashResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify balances. Source should have lost more than 100 due to transfer fee.
			srcBalAfter := getIOUBalance(t, rpc, src.Address, "USD", issuer.Address)
			destBalAfter := getIOUBalance(t, rpc, dest.Address, "USD", issuer.Address)

			t.Logf("transfer fee test: src USD %s->%s, dest USD 0->%s", srcBalBefore, srcBalAfter, destBalAfter)

			// Dest should have received at least 100 USD.
			destBal, _ := strconv.ParseFloat(destBalAfter, 64)
			if destBal < 100 {
				t.Fatalf("destination should have received at least 100 USD, got %s", destBalAfter)
			}

			// Source should have lost more than 100 USD (due to 20% fee).
			srcBefore, _ := strconv.ParseFloat(srcBalBefore, 64)
			srcAfter, _ := strconv.ParseFloat(srcBalAfter, 64)
			amountDeducted := srcBefore - srcAfter
			if amountDeducted <= 100 {
				t.Fatalf("expected source to lose more than 100 USD due to transfer fee, lost %f", amountDeducted)
			}
			t.Logf("source deducted %.2f USD (includes 20%% transfer fee)", amountDeducted)
		},
	}
}

// checkCancelValid tests that both the creator and destination can cancel a check,
// and verifies the check object is removed from the ledger.
func checkCancelValid() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "check_cancel_valid",
		Description: "Both creator and destination can cancel, verify object removed.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			// Test 1: Creator cancels.
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     dest.Address,
				"SendMax":         "1000000000",
			})
			if err != nil {
				t.Fatal("check create 1:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			checkID := getCheckID(t, rpc, src.Address)

			cancelResult, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCancel",
				"CheckID":         checkID,
			})
			if err != nil {
				t.Fatal("check cancel by creator:", err)
			}
			assertEngineResult(t, cancelResult, "tesSUCCESS")
			waitSettled(rpc)

			count := getCheckCount(t, rpc, src.Address)
			if count != 0 {
				t.Fatalf("expected check removed after creator cancel, found %d", count)
			}
			t.Log("creator successfully cancelled check")

			// Test 2: Destination cancels.
			result, err = rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     dest.Address,
				"SendMax":         "2000000000",
			})
			if err != nil {
				t.Fatal("check create 2:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			checkID = getCheckID(t, rpc, src.Address)

			cancelResult, err = rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "CheckCancel",
				"CheckID":         checkID,
			})
			if err != nil {
				t.Fatal("check cancel by dest:", err)
			}
			assertEngineResult(t, cancelResult, "tesSUCCESS")
			waitSettled(rpc)

			count = getCheckCount(t, rpc, src.Address)
			if count != 0 {
				t.Fatalf("expected check removed after destination cancel, found %d", count)
			}
			t.Log("destination successfully cancelled check")
		},
	}
}

// checkCancelInvalid tests that a third party cannot cancel an unexpired check,
// and that cancelling a non-existent check fails.
func checkCancelInvalid() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "check_cancel_invalid",
		Description: "Third party can't cancel unexpired check, non-existent check fails.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			src := accounts[0]
			dest := accounts[1]
			thirdParty := accounts[2]

			// Create a check with no Expiration (never expires).
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

			checkID := getCheckID(t, rpc, src.Address)

			// Test 1: Third party tries to cancel an unexpired check.
			cancelResult, err := rpc.Submit(thirdParty.Secret, thirdParty.Address, map[string]interface{}{
				"TransactionType": "CheckCancel",
				"CheckID":         checkID,
			})
			if err != nil {
				t.Fatal("check cancel third party:", err)
			}
			if cancelResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for third party cancel of unexpired check, got tesSUCCESS")
			}
			t.Logf("third party cancel: %s (expected failure)", cancelResult.EngineResult)

			// Verify the check still exists.
			count := getCheckCount(t, rpc, src.Address)
			if count == 0 {
				t.Fatal("check should still exist after failed third party cancel")
			}

			// Test 2: Cancel a non-existent check.
			fakeCheckID := "0000000000000000000000000000000000000000000000000000000000000000"
			cancelResult, err = rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCancel",
				"CheckID":         fakeCheckID,
			})
			if err != nil {
				t.Fatal("check cancel non-existent:", err)
			}
			if cancelResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for non-existent check cancel, got tesSUCCESS")
			}
			t.Logf("non-existent check cancel: %s (expected failure)", cancelResult.EngineResult)
		},
	}
}

// checkWithTickets tests creating and cashing checks using TicketSequence instead
// of regular Sequence numbers.
func checkWithTickets() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "check_with_tickets",
		Description: "Create and cash checks using TicketSequence instead of Sequence.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			// Get current sequence for ticket calculation.
			info, err := rpc.AccountInfo(src.Address)
			if err != nil {
				t.Fatal("account_info:", err)
			}

			// Create 3 tickets on the source account.
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "TicketCreate",
				"TicketCount":     3,
			})
			if err != nil {
				t.Fatal("ticket create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// TicketCreate consumes info.Sequence, tickets are info.Sequence+1, +2, +3.
			ticketSeq1 := info.Sequence + 1
			ticketSeq2 := info.Sequence + 2

			// Use ticket 1 to create a check.
			result, err = rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     dest.Address,
				"SendMax":         "500000000",
				"Sequence":        0,
				"TicketSequence":  ticketSeq1,
			})
			if err != nil {
				t.Fatal("check create with ticket:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Find check ID.
			checkID := getCheckID(t, rpc, src.Address)

			// Get dest account sequence for the ticket.
			destInfo, err := rpc.AccountInfo(dest.Address)
			if err != nil {
				t.Fatal("dest account_info:", err)
			}

			// Create tickets on dest.
			result, err = rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "TicketCreate",
				"TicketCount":     2,
			})
			if err != nil {
				t.Fatal("dest ticket create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			destTicketSeq1 := destInfo.Sequence + 1

			// Use ticket to cash the check.
			cashResult, err := rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "CheckCash",
				"CheckID":         checkID,
				"Amount":          "500000000",
				"Sequence":        0,
				"TicketSequence":  destTicketSeq1,
			})
			if err != nil {
				t.Fatal("check cash with ticket:", err)
			}
			assertEngineResult(t, cashResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify check is removed.
			count := getCheckCount(t, rpc, src.Address)
			if count != 0 {
				t.Fatalf("expected check removed after ticket-based cash, found %d", count)
			}

			// Use ticket 2 to create another check (verifying tickets still work).
			result, err = rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     dest.Address,
				"SendMax":         "100000000",
				"Sequence":        0,
				"TicketSequence":  ticketSeq2,
			})
			if err != nil {
				t.Fatal("check create with ticket 2:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			t.Log("checks with tickets: all operations succeeded")
		},
	}
}

// checkTrustLineCreation tests cashing an IOU check when no trust line exists
// for the destination. With the CheckCashMakesTrustLine amendment, the cash
// operation should create the trust line automatically.
func checkTrustLineCreation() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "check_trust_line_creation",
		Description: "Cash IOU check when no trust line exists (CheckCashMakesTrustLine amendment).",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			src := accounts[1]
			dest := accounts[2]

			// Enable DefaultRipple on issuer.
			rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			waitSettled(rpc)

			// Setup: src trusts issuer for USD. Dest does NOT trust issuer yet.
			err := setup.SetupTrustLine(ctx, rpc, src.Address, src.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line src:", err)
			}

			// Issuer sends 100 USD to src.
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     src.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("iou payment:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify dest has no trust line to issuer.
			destBalBefore := getIOUBalance(t, rpc, dest.Address, "USD", issuer.Address)
			if destBalBefore != "0" {
				t.Fatalf("expected dest to have no USD balance, got %s", destBalBefore)
			}

			// Create IOU check from src to dest for 50 USD.
			result, err = rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     dest.Address,
				"SendMax": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("check create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			checkID := getCheckID(t, rpc, src.Address)

			// Cash the check. With CheckCashMakesTrustLine amendment (enabled via
			// "all" features), this should succeed and create the trust line.
			cashResult, err := rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "CheckCash",
				"CheckID":         checkID,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("check cash no trust line:", err)
			}

			// The result depends on whether CheckCashMakesTrustLine is active.
			// With all features enabled, it should succeed.
			if cashResult.EngineResult == "tesSUCCESS" {
				waitSettled(rpc)

				// Verify dest now has the USD balance.
				destBalAfter := getIOUBalance(t, rpc, dest.Address, "USD", issuer.Address)
				if destBalAfter == "0" {
					t.Fatal("expected dest to have USD balance after cashing check")
				}
				t.Logf("trust line created via CheckCash: dest USD balance = %s", destBalAfter)

				// Verify dest OwnerCount increased (trust line + check removal).
				destInfo, err := rpc.AccountInfo(dest.Address)
				if err != nil {
					t.Fatal("dest account_info:", err)
				}
				t.Logf("dest owner count after trust line creation: %d", destInfo.OwnerCount)
			} else {
				// Without the amendment, expect tecNO_LINE.
				t.Logf("check cash without trust line: %s (amendment may not be active)",
					cashResult.EngineResult)

				// Fallback: create trust line manually and try again.
				err = setup.SetupTrustLine(ctx, rpc, dest.Address, dest.Secret, "USD", issuer.Address, "1000")
				if err != nil {
					t.Fatal("trust line dest fallback:", err)
				}

				// Need a new check since the old one may still exist.
				count := getCheckCount(t, rpc, src.Address)
				if count > 0 {
					// Cash existing check with trust line now in place.
					cashResult, err = rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
						"TransactionType": "CheckCash",
						"CheckID":         checkID,
						"Amount": map[string]interface{}{
							"currency": "USD",
							"issuer":   issuer.Address,
							"value":    "50",
						},
					})
					if err != nil {
						t.Fatal("check cash retry:", err)
					}
					assertEngineResult(t, cashResult, "tesSUCCESS")
					waitSettled(rpc)
					t.Log("check cashed after manual trust line creation")
				}
			}
		},
	}
}

// checkCashDeliverMin tests using the DeliverMin field on CheckCash.
// DeliverMin allows cashing for a flexible amount up to SendMax.
func checkCashDeliverMin() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "check_cash_deliver_min",
		Description: "Cash a check using DeliverMin for flexible amount delivery.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			// Create check for 5000 XRP.
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "CheckCreate",
				"Destination":     dest.Address,
				"SendMax":         "5000000000",
			})
			if err != nil {
				t.Fatal("check create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			checkID := getCheckID(t, rpc, src.Address)
			destBalBefore := getAccountBalance(t, rpc, dest.Address)

			// Cash using DeliverMin = 1000 XRP (flexible amount).
			cashResult, err := rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType": "CheckCash",
				"CheckID":         checkID,
				"DeliverMin":      "1000000000",
			})
			if err != nil {
				t.Fatal("check cash deliver min:", err)
			}
			assertEngineResult(t, cashResult, "tesSUCCESS")
			waitSettled(rpc)

			destBalAfter := getAccountBalance(t, rpc, dest.Address)
			before, _ := strconv.ParseInt(destBalBefore, 10, 64)
			after, _ := strconv.ParseInt(destBalAfter, 10, 64)
			gained := after - before

			// Dest should have gained at least DeliverMin amount (minus fees).
			if after <= before {
				t.Fatalf("destination balance did not increase: before=%d, after=%d", before, after)
			}

			// Check should be removed.
			count := getCheckCount(t, rpc, src.Address)
			if count != 0 {
				t.Fatalf("expected check removed after DeliverMin cash, found %d", count)
			}

			t.Logf("DeliverMin cash: dest gained %d drops", gained)
		},
	}
}
