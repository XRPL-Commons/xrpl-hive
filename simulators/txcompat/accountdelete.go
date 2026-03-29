package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

// advanceLedgers closes n ledgers via ledger_accept. This is needed for
// AccountDelete which requires the account to have existed for at least
// 256 ledgers.
func advanceLedgers(rpc *xrplsim.RPCClient, n int) {
	for i := 0; i < n; i++ {
		rpc.Call("ledger_accept", nil)
	}
}

func accountDeleteBasics() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_delete_basics",
		Description: "Delete an account after 256 ledgers, verify destination receives funds and account is gone.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			alice := accounts[0]
			bob := accounts[1]

			// Record destination balance before deletion.
			bobInfoBefore, err := rpc.AccountInfo(bob.Address)
			if err != nil {
				t.Fatal("bob account_info before:", err)
			}
			bobBalanceBefore, _ := strconv.ParseInt(bobInfoBefore.Balance, 10, 64)

			// Record source balance before deletion.
			aliceInfo, err := rpc.AccountInfo(alice.Address)
			if err != nil {
				t.Fatal("alice account_info:", err)
			}
			aliceBalance, _ := strconv.ParseInt(aliceInfo.Balance, 10, 64)
			t.Logf("alice balance before delete: %d drops", aliceBalance)

			// Advance 256 ledgers so alice's account is old enough to delete.
			advanceLedgers(rpc, 256)

			// Delete alice's account, sending remaining funds to bob.
			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AccountDelete",
				"Destination":     bob.Address,
				"Fee":             "2000000",
			})
			if err != nil {
				t.Fatal("account delete:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify alice's account is gone.
			_, err = rpc.AccountInfo(alice.Address)
			if err == nil {
				t.Fatal("expected alice account to be deleted, but account_info succeeded")
			}
			t.Log("alice account deleted successfully")

			// Verify bob received the funds (alice balance minus 2 XRP fee).
			bobInfoAfter, err := rpc.AccountInfo(bob.Address)
			if err != nil {
				t.Fatal("bob account_info after:", err)
			}
			bobBalanceAfter, _ := strconv.ParseInt(bobInfoAfter.Balance, 10, 64)
			received := bobBalanceAfter - bobBalanceBefore
			expectedReceived := aliceBalance - 2000000 // balance minus fee
			t.Logf("bob received %d drops (expected %d)", received, expectedReceived)
			if received != expectedReceived {
				t.Fatalf("bob balance mismatch: received %d, expected %d", received, expectedReceived)
			}
		},
	}
}

func accountDeleteDestinationConstraints() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_delete_destination_constraints",
		Description: "Cannot delete to self; cannot delete to non-existent account.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			alice := accounts[0]

			// Advance 256 ledgers.
			advanceLedgers(rpc, 256)

			// Try to delete to self -- should fail with temDST_IS_SRC.
			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AccountDelete",
				"Destination":     alice.Address,
				"Fee":             "2000000",
			})
			if err != nil {
				t.Fatal("delete to self submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure when deleting to self, got tesSUCCESS")
			}
			t.Logf("delete to self: %s (expected temDST_IS_SRC)", result.EngineResult)

			// Try to delete to a non-existent account -- should fail with tecNO_DST.
			nonExistent, _ := rpc.WalletPropose()
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AccountDelete",
				"Destination":     nonExistent.AccountID,
				"Fee":             "2000000",
			})
			if err != nil {
				t.Fatal("delete to non-existent submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure when deleting to non-existent destination, got tesSUCCESS")
			}
			t.Logf("delete to non-existent: %s (expected tecNO_DST)", result.EngineResult)
		},
	}
}

func accountDeleteOwnedTypes() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_delete_owned_types",
		Description: "Cannot delete account that owns objects (offers, trust lines, etc.).",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()
			accounts := mustFund(t, rpc, 2)
			alice := accounts[0]
			bob := accounts[1]

			// Create a trust line on alice (gives her an owned object).
			err := setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", xrplsim.GenesisAddress, "1000")
			if err != nil {
				t.Fatal("setup trust line:", err)
			}

			// Verify alice owns objects.
			aliceInfo, err := rpc.AccountInfo(alice.Address)
			if err != nil {
				t.Fatal("alice account_info:", err)
			}
			if aliceInfo.OwnerCount == 0 {
				t.Fatal("expected alice to have owned objects")
			}
			t.Logf("alice owner_count: %d", aliceInfo.OwnerCount)

			// Advance 256 ledgers.
			advanceLedgers(rpc, 256)

			// Try to delete alice -- should fail with tecHAS_OBLIGATIONS.
			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AccountDelete",
				"Destination":     bob.Address,
				"Fee":             "2000000",
			})
			if err != nil {
				t.Fatal("delete with objects submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure deleting account with owned objects, got tesSUCCESS")
			}
			t.Logf("delete with trust line: %s (expected tecHAS_OBLIGATIONS)", result.EngineResult)
		},
	}
}

func accountDeleteTooManyOffers() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_delete_too_many_offers",
		Description: "Account with offers cannot be deleted (OwnerCount > 0).",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Fund alice with extra XRP for reserves.
			accts, err := setup.FundN(ctx, rpc, 2, "50000000000")
			if err != nil {
				t.Fatal("fund:", err)
			}
			alice := accts[0]
			bob := accts[1]

			// Set up a trust line so alice can create IOU offers.
			setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", xrplsim.GenesisAddress, "10000")

			// Create several offers.
			for i := 0; i < 5; i++ {
				err := setup.SetupOffer(ctx, rpc, alice.Address, alice.Secret,
					fmt.Sprintf("%d", (i+1)*1000000),
					map[string]interface{}{"currency": "USD", "issuer": xrplsim.GenesisAddress, "value": fmt.Sprintf("%d", i+1)},
				)
				if err != nil {
					t.Fatal("create offer:", err)
				}
			}

			// Verify alice has offers.
			aliceInfo, err := rpc.AccountInfo(alice.Address)
			if err != nil {
				t.Fatal("alice account_info:", err)
			}
			t.Logf("alice owner_count with offers: %d", aliceInfo.OwnerCount)

			// Advance 256 ledgers.
			advanceLedgers(rpc, 256)

			// Try to delete -- should fail because of owned offers.
			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AccountDelete",
				"Destination":     bob.Address,
				"Fee":             "2000000",
			})
			if err != nil {
				t.Fatal("delete with offers submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure deleting account with offers, got tesSUCCESS")
			}
			t.Logf("delete with offers: %s (expected tecHAS_OBLIGATIONS)", result.EngineResult)
		},
	}
}

func accountDeleteWithTickets() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_delete_with_tickets",
		Description: "Delete an account using a ticket.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			alice := accounts[0]
			bob := accounts[1]

			// Get alice's current sequence before creating tickets.
			aliceInfo, err := rpc.AccountInfo(alice.Address)
			if err != nil {
				t.Fatal("alice account_info:", err)
			}

			// Create a ticket.
			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "TicketCreate",
				"TicketCount":     1,
			})
			if err != nil {
				t.Fatal("ticket create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// The ticket sequence is the sequence at the time of TicketCreate + 1.
			ticketSeq := aliceInfo.Sequence + 1

			// Advance 256 ledgers so account is old enough.
			advanceLedgers(rpc, 256)

			// Verify alice has the ticket (OwnerCount should be 1).
			aliceInfoNow, err := rpc.AccountInfo(alice.Address)
			if err != nil {
				t.Fatal("alice account_info now:", err)
			}
			t.Logf("alice owner_count with ticket: %d", aliceInfoNow.OwnerCount)

			// Delete using the ticket. The ticket is consumed as part of the
			// AccountDelete, so OwnerCount drops to 0 and the deletion proceeds.
			deleteResult, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AccountDelete",
				"Destination":     bob.Address,
				"Fee":             "2000000",
				"Sequence":        0,
				"TicketSequence":  ticketSeq,
			})
			if err != nil {
				t.Fatal("delete with ticket:", err)
			}
			assertEngineResult(t, deleteResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify alice's account is gone.
			_, err = rpc.AccountInfo(alice.Address)
			if err == nil {
				t.Fatal("expected alice account to be deleted after ticket-based delete")
			}
			t.Log("account deleted using ticket")
		},
	}
}

func accountDeleteBalanceTooSmall() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_delete_balance_too_small",
		Description: "Account with balance less than the AccountDelete fee (2 XRP) cannot be deleted.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Fund two accounts: alice with minimal balance, bob as destination.
			accts, err := setup.FundN(ctx, rpc, 2, "10000000000")
			if err != nil {
				t.Fatal("fund:", err)
			}
			alice := accts[0]
			bob := accts[1]

			// Advance 256 ledgers.
			advanceLedgers(rpc, 256)

			// Drain alice's balance by sending most of it to bob, leaving less
			// than 2 XRP (the AccountDelete fee) but above the reserve.
			// Reserve is 200 XRP = 200000000 drops. We want alice to have just
			// above reserve but below reserve + fee (200 XRP + 2 XRP).
			// Send away most of alice's funds, leaving ~201 XRP.
			aliceInfo, err := rpc.AccountInfo(alice.Address)
			if err != nil {
				t.Fatal("alice account_info:", err)
			}
			aliceBalance, _ := strconv.ParseInt(aliceInfo.Balance, 10, 64)
			// Leave 201000000 drops (201 XRP) -- above reserve but barely enough for fee.
			sendAmount := aliceBalance - 201000000 - 12 // subtract 12 drops for payment fee
			if sendAmount > 0 {
				result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
					"TransactionType": "Payment",
					"Destination":     bob.Address,
					"Amount":          fmt.Sprintf("%d", sendAmount),
				})
				if err != nil {
					t.Fatal("drain payment:", err)
				}
				assertEngineResult(t, result, "tesSUCCESS")
				waitSettled(rpc)
			}

			// Now alice has ~201 XRP. After paying the 2 XRP fee, she would have
			// 199 XRP left which is below the 200 XRP reserve. However, since the
			// account is being deleted, the reserve is released. The real constraint
			// is that the balance must be >= fee.
			// Let's drain further so alice has less than 2 XRP (2000000 drops).
			aliceInfo, err = rpc.AccountInfo(alice.Address)
			if err != nil {
				t.Fatal("alice account_info 2:", err)
			}
			aliceBalance, _ = strconv.ParseInt(aliceInfo.Balance, 10, 64)
			// Leave only 1000000 drops (1 XRP) -- less than the 2 XRP fee.
			sendAmount = aliceBalance - 1000000 - 12
			if sendAmount > 0 {
				result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
					"TransactionType": "Payment",
					"Destination":     bob.Address,
					"Amount":          fmt.Sprintf("%d", sendAmount),
				})
				if err != nil {
					t.Fatal("drain payment 2:", err)
				}
				// This might succeed or fail depending on reserve; log either way.
				t.Logf("drain payment 2: %s", result.EngineResult)
				waitSettled(rpc)
			}

			// Try to delete alice with 2 XRP fee. Should fail because balance < fee.
			aliceInfo, err = rpc.AccountInfo(alice.Address)
			if err != nil {
				t.Fatal("alice account_info final:", err)
			}
			t.Logf("alice balance before delete attempt: %s drops", aliceInfo.Balance)

			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AccountDelete",
				"Destination":     bob.Address,
				"Fee":             "2000000",
			})
			if err != nil {
				t.Fatal("delete with low balance:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure deleting account with insufficient balance for fee, got tesSUCCESS")
			}
			t.Logf("delete with low balance: %s (expected terINSUF_FEE_B)", result.EngineResult)
		},
	}
}

func accountDeleteResurrection() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_delete_resurrection",
		Description: "Delete an account, then re-create it by sending XRP to the same address.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			alice := accounts[0]
			bob := accounts[1]
			aliceAddr := alice.Address

			// Advance 256 ledgers.
			advanceLedgers(rpc, 256)

			// Delete alice's account.
			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AccountDelete",
				"Destination":     bob.Address,
				"Fee":             "2000000",
			})
			if err != nil {
				t.Fatal("account delete:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify account is gone.
			_, err = rpc.AccountInfo(aliceAddr)
			if err == nil {
				t.Fatal("expected alice account to be deleted")
			}

			// Re-create alice by sending XRP from bob (must be >= reserve of 200 XRP).
			resurrectResult, err := rpc.SubmitPayment(bob.Secret, bob.Address, aliceAddr, "200000000")
			if err != nil {
				t.Fatal("resurrect payment:", err)
			}
			assertEngineResult(t, resurrectResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify the account exists again.
			aliceInfo, err := rpc.AccountInfo(aliceAddr)
			if err != nil {
				t.Fatal("alice account_info after resurrect:", err)
			}
			t.Logf("alice resurrected: balance=%s, sequence=%d", aliceInfo.Balance, aliceInfo.Sequence)

			// Sequence should be reset to 1 for a newly created account.
			if aliceInfo.Sequence != 1 {
				t.Fatalf("expected sequence 1 for resurrected account, got %d", aliceInfo.Sequence)
			}
		},
	}
}

func accountDeleteDirectories() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_delete_directories",
		Description: "Verify directory cleanup: no account_objects remain after deletion.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			alice := accounts[0]
			bob := accounts[1]

			// Advance 256 ledgers.
			advanceLedgers(rpc, 256)

			// Check alice's objects before deletion.
			raw, err := rpc.Call("account_objects", map[string]interface{}{
				"account":      alice.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_objects before:", err)
			}
			var objResp struct {
				AccountObjects []json.RawMessage `json:"account_objects"`
			}
			json.Unmarshal(raw, &objResp)
			t.Logf("alice objects before delete: %d", len(objResp.AccountObjects))

			// Delete alice's account.
			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AccountDelete",
				"Destination":     bob.Address,
				"Fee":             "2000000",
			})
			if err != nil {
				t.Fatal("account delete:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify account is gone -- account_objects should return an error.
			raw, err = rpc.Call("account_objects", map[string]interface{}{
				"account":      alice.Address,
				"ledger_index": "current",
			})
			if err != nil {
				// Error is expected -- account doesn't exist.
				t.Log("account_objects correctly returned error for deleted account")
				return
			}

			// Some implementations return a result with status "error" instead of
			// an HTTP error.
			var errResp struct {
				Error  string `json:"error"`
				Status string `json:"status"`
			}
			json.Unmarshal(raw, &errResp)
			if errResp.Status == "error" || errResp.Error != "" {
				t.Logf("account_objects returned error status: %s", errResp.Error)
				return
			}

			// If we got a response, verify no objects remain.
			json.Unmarshal(raw, &objResp)
			if len(objResp.AccountObjects) != 0 {
				t.Fatalf("expected no objects after account deletion, got %d", len(objResp.AccountObjects))
			}
			t.Log("directory cleanup verified: no objects after deletion")
		},
	}
}
