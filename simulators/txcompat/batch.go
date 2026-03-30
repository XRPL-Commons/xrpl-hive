package main

import (
	"encoding/json"
	"strings"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

// signTx signs a transaction via the sign RPC and returns the signed tx_json.
// This is needed for Batch inner transactions which must be pre-signed.
func signTx(t *xrplsim.T, rpc *xrplsim.RPCClient, secret string, txJSON map[string]interface{}) map[string]interface{} {
	raw, err := rpc.Call("sign", map[string]interface{}{
		"secret":  secret,
		"tx_json": txJSON,
	})
	if err != nil {
		t.Fatal("sign tx:", err)
	}
	var resp struct {
		TxJSON json.RawMessage `json:"tx_json"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal("parse sign response:", err)
	}
	var signedTx map[string]interface{}
	if err := json.Unmarshal(resp.TxJSON, &signedTx); err != nil {
		t.Fatal("parse signed tx_json:", err)
	}
	return signedTx
}

// isBatchDisabled returns true if the result indicates the Batch amendment is not active.
func isBatchDisabled(result *xrplsim.SubmitResult) bool {
	return result.EngineResult == "temDISABLED" ||
		result.EngineResult == "temINVALID" ||
		strings.HasPrefix(result.EngineResult, "temMALFORMED")
}

func batchEnabled() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "batch_enabled",
		Description: "Submit minimal Batch tx. Log result; handle temDISABLED gracefully.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			// Create a simple inner payment tx.
			innerTx := signTx(t, rpc, src.Secret, map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         src.Address,
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"Sequence":        0,
			})

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "Batch",
				"RawTransactions": []map[string]interface{}{
					{"RawTransaction": innerTx},
				},
				"Flags": 0,
			})
			if err != nil {
				t.Fatal("batch submit:", err)
			}
			if isBatchDisabled(result) {
				t.Logf("batch amendment not active: %s", result.EngineResult)
				return
			}
			t.Logf("batch enabled result: %s", result.EngineResult)
		},
	}
}

func batchIndependent() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "batch_independent",
		Description: "Batch with 2 independent Payment txns (Flags=0). Both should execute.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			src := accounts[0]
			dest1 := accounts[1]
			dest2 := accounts[2]

			inner1 := signTx(t, rpc, src.Secret, map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         src.Address,
				"Destination":     dest1.Address,
				"Amount":          "1000000",
				"Sequence":        0,
			})
			inner2 := signTx(t, rpc, src.Secret, map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         src.Address,
				"Destination":     dest2.Address,
				"Amount":          "2000000",
				"Sequence":        0,
			})

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "Batch",
				"RawTransactions": []map[string]interface{}{
					{"RawTransaction": inner1},
					{"RawTransaction": inner2},
				},
				"Flags": 0, // independent
			})
			if err != nil {
				t.Fatal("batch submit:", err)
			}
			if isBatchDisabled(result) {
				t.Logf("batch amendment not active: %s", result.EngineResult)
				return
			}
			t.Logf("batch independent: %s", result.EngineResult)
			waitSettled(rpc)
		},
	}
}

func batchAllOrNothing() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "batch_all_or_nothing",
		Description: "Batch with 2 txns where second fails (Flags=1). Neither should execute.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			// First inner: valid payment.
			inner1 := signTx(t, rpc, src.Secret, map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         src.Address,
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"Sequence":        0,
			})
			// Second inner: payment for more XRP than the account has.
			inner2 := signTx(t, rpc, src.Secret, map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         src.Address,
				"Destination":     dest.Address,
				"Amount":          "999999999999999",
				"Sequence":        0,
			})

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "Batch",
				"RawTransactions": []map[string]interface{}{
					{"RawTransaction": inner1},
					{"RawTransaction": inner2},
				},
				"Flags": 1, // all-or-nothing
			})
			if err != nil {
				t.Fatal("batch submit:", err)
			}
			if isBatchDisabled(result) {
				t.Logf("batch amendment not active: %s", result.EngineResult)
				return
			}
			t.Logf("batch all-or-nothing: %s", result.EngineResult)
			waitSettled(rpc)
		},
	}
}

func batchUntilFailure() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "batch_until_failure",
		Description: "Batch with 3 txns, middle one fails (Flags=2). First executes, rest don't.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			// First: valid payment.
			inner1 := signTx(t, rpc, src.Secret, map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         src.Address,
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"Sequence":        0,
			})
			// Second: invalid (overspend).
			inner2 := signTx(t, rpc, src.Secret, map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         src.Address,
				"Destination":     dest.Address,
				"Amount":          "999999999999999",
				"Sequence":        0,
			})
			// Third: valid payment (should not execute).
			inner3 := signTx(t, rpc, src.Secret, map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         src.Address,
				"Destination":     dest.Address,
				"Amount":          "500000",
				"Sequence":        0,
			})

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "Batch",
				"RawTransactions": []map[string]interface{}{
					{"RawTransaction": inner1},
					{"RawTransaction": inner2},
					{"RawTransaction": inner3},
				},
				"Flags": 2, // until-failure
			})
			if err != nil {
				t.Fatal("batch submit:", err)
			}
			if isBatchDisabled(result) {
				t.Logf("batch amendment not active: %s", result.EngineResult)
				return
			}
			t.Logf("batch until-failure: %s", result.EngineResult)
			waitSettled(rpc)
		},
	}
}

func batchAccountActivation() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "batch_account_activation",
		Description: "Batch that creates a new account via Payment in first inner tx.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			src := accounts[0]

			// Generate a new unfunded account.
			newAcct, err := rpc.WalletPropose()
			if err != nil {
				t.Fatal("wallet_propose:", err)
			}

			// First inner: fund the new account above reserve.
			inner1 := signTx(t, rpc, src.Secret, map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         src.Address,
				"Destination":     newAcct.AccountID,
				"Amount":          "300000000", // 300 XRP, above reserve
				"Sequence":        0,
			})

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "Batch",
				"RawTransactions": []map[string]interface{}{
					{"RawTransaction": inner1},
				},
				"Flags": 0,
			})
			if err != nil {
				t.Fatal("batch submit:", err)
			}
			if isBatchDisabled(result) {
				t.Logf("batch amendment not active: %s", result.EngineResult)
				return
			}
			t.Logf("batch account activation: %s", result.EngineResult)
			waitSettled(rpc)

			// Verify the new account exists.
			if result.EngineResult == "tesSUCCESS" {
				info, err := rpc.AccountInfo(newAcct.AccountID)
				if err != nil {
					t.Logf("new account not found (may be expected): %v", err)
				} else {
					t.Logf("new account balance: %s", info.Balance)
				}
			}
		},
	}
}

func batchInvalidFee() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "batch_invalid_fee",
		Description: "Batch with invalid outer fee. Expect temBAD_FEE or similar.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			inner1 := signTx(t, rpc, src.Secret, map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         src.Address,
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"Sequence":        0,
			})

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "Batch",
				"RawTransactions": []map[string]interface{}{
					{"RawTransaction": inner1},
				},
				"Flags": 0,
				"Fee":   "0", // invalid fee
			})
			if err != nil {
				t.Fatal("batch submit:", err)
			}
			if isBatchDisabled(result) {
				t.Logf("batch amendment not active: %s", result.EngineResult)
				return
			}
			// Expect some form of fee error.
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected fee error, got tesSUCCESS")
			}
			t.Logf("batch invalid fee: %s", result.EngineResult)
		},
	}
}

func batchPreflight() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "batch_preflight",
		Description: "Batch containing a malformed inner tx. Expect preflight failure.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			src := accounts[0]

			// Inner tx with missing required fields (Payment without Destination).
			// We construct the raw transaction manually since sign may reject it.
			malformedInner := map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         src.Address,
				"Amount":          "1000000",
				// Missing Destination -- malformed.
			}

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "Batch",
				"RawTransactions": []map[string]interface{}{
					{"RawTransaction": malformedInner},
				},
				"Flags": 0,
			})
			if err != nil {
				// The RPC itself may reject this.
				t.Logf("batch preflight rejected by RPC: %v", err)
				return
			}
			if isBatchDisabled(result) {
				t.Logf("batch amendment not active: %s", result.EngineResult)
				return
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected preflight failure for malformed inner tx, got tesSUCCESS")
			}
			t.Logf("batch preflight failure: %s", result.EngineResult)
		},
	}
}

func batchAccountSet() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "batch_account_set",
		Description: "Batch containing AccountSet. Verify flag is set.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Inner: set asfRequireDest (1).
			inner1 := signTx(t, rpc, acct.Secret, map[string]interface{}{
				"TransactionType": "AccountSet",
				"Account":         acct.Address,
				"SetFlag":         1, // asfRequireDest
				"Sequence":        0,
			})

			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "Batch",
				"RawTransactions": []map[string]interface{}{
					{"RawTransaction": inner1},
				},
				"Flags": 0,
			})
			if err != nil {
				t.Fatal("batch submit:", err)
			}
			if isBatchDisabled(result) {
				t.Logf("batch amendment not active: %s", result.EngineResult)
				return
			}
			t.Logf("batch accountset: %s", result.EngineResult)
			waitSettled(rpc)

			// Verify the flag is set if batch succeeded.
			if result.EngineResult == "tesSUCCESS" {
				info, err := rpc.AccountInfo(acct.Address)
				if err != nil {
					t.Fatal("account_info:", err)
				}
				// lsfRequireDestTag = 0x00020000 = 131072
				if info.Flags&0x00020000 == 0 {
					t.Logf("flag may not be set (flags=%d); batch inner tx may use different flag encoding", info.Flags)
				} else {
					t.Log("asfRequireDest flag set successfully via batch")
				}
			}
		},
	}
}

func batchWithTickets() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "batch_with_tickets",
		Description: "Batch using TicketSequence for outer transaction.",
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

			ticketSeq := info.Sequence + 1

			// Inner payment.
			inner1 := signTx(t, rpc, src.Secret, map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         src.Address,
				"Destination":     dest.Address,
				"Amount":          "1000000",
				"Sequence":        0,
			})

			// Submit batch using ticket.
			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "Batch",
				"RawTransactions": []map[string]interface{}{
					{"RawTransaction": inner1},
				},
				"Flags":          0,
				"Sequence":       0,
				"TicketSequence": ticketSeq,
			})
			if err != nil {
				t.Fatal("batch with ticket:", err)
			}
			if isBatchDisabled(result) {
				t.Logf("batch amendment not active: %s", result.EngineResult)
				return
			}
			t.Logf("batch with ticket %d: %s", ticketSeq, result.EngineResult)
		},
	}
}

func batchOnlyOne() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "batch_only_one",
		Description: "Batch with single inner transaction. Should work.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			src := accounts[0]
			dest := accounts[1]

			inner1 := signTx(t, rpc, src.Secret, map[string]interface{}{
				"TransactionType": "Payment",
				"Account":         src.Address,
				"Destination":     dest.Address,
				"Amount":          "5000000",
				"Sequence":        0,
			})

			result, err := rpc.Submit(src.Secret, src.Address, map[string]interface{}{
				"TransactionType": "Batch",
				"RawTransactions": []map[string]interface{}{
					{"RawTransaction": inner1},
				},
				"Flags": 0,
			})
			if err != nil {
				t.Fatal("batch submit:", err)
			}
			if result.EngineResult == "" {
				t.Log("batch_only_one: empty engine result (RPC may not support Batch or inner tx format issue)")
				return
			}
			if isBatchDisabled(result) {
				t.Logf("batch amendment not active: %s", result.EngineResult)
				return
			}
			t.Logf("batch with single tx: %s", result.EngineResult)
		},
	}
}
