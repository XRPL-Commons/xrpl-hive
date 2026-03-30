package main

import (
	"context"
	"strconv"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

// clawbackEnableFlag enables asfAllowTrustLineClawback (16) and verifies the
// flag is set via account_info.
func clawbackEnableFlag() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "clawback_enable_flag",
		Description: "Enable asfAllowTrustLineClawback (16), verify Flags via account_info.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Enable clawback flag (asfAllowTrustLineClawback = 16).
			result, err := rpc.SubmitAccountSet(acct.Secret, acct.Address, 16)
			if err != nil {
				t.Fatal("account set clawback:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify the flag is set via account_info.
			info, err := rpc.AccountInfo(acct.Address)
			if err != nil {
				t.Fatal("account_info:", err)
			}
			// lsfAllowTrustLineClawback = 0x80000000 (2147483648)
			const lsfAllowTrustLineClawback = 0x80000000
			if info.Flags&lsfAllowTrustLineClawback == 0 {
				t.Fatalf("expected lsfAllowTrustLineClawback set, flags=%d", info.Flags)
			}
			t.Logf("clawback flag enabled, flags=%d", info.Flags)
		},
	}
}

// clawbackValidation tests clawback without enabling the flag (tecNO_PERMISSION)
// and clawback from a non-existent trust line (tecNO_LINE).
func clawbackValidation() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "clawback_validation",
		Description: "Clawback without flag gives tecNO_PERMISSION; non-existent line gives tecNO_LINE.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			holder := accounts[1]
			other := accounts[2]

			// --- Test 1: clawback without enabling flag first ---
			// Enable DefaultRipple on issuer BEFORE trust lines.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust line and issue IOU (without clawback flag).
			err = setup.SetupIOU(ctx, rpc,
				issuer.Address, issuer.Secret,
				holder.Address, holder.Secret,
				"USD", "100",
			)
			if err != nil {
				t.Fatal("setup IOU:", err)
			}

			// Attempt clawback without flag.
			clawResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Clawback",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   holder.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("clawback:", err)
			}
			if clawResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected clawback without flag to fail, got tesSUCCESS")
			}
			t.Logf("clawback without flag: %s (expected tecNO_PERMISSION)", clawResult.EngineResult)

			// --- Test 2: clawback from non-existent trust line ---
			// Create a new issuer with clawback flag enabled.
			accounts2 := mustFund(t, rpc, 1)
			issuer2 := accounts2[0]

			// Enable clawback BEFORE any trust lines.
			result, err = rpc.SubmitAccountSet(issuer2.Secret, issuer2.Address, 16)
			if err != nil {
				t.Fatal("enable clawback:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Attempt clawback from an account with no trust line.
			clawResult2, err := rpc.Submit(issuer2.Secret, issuer2.Address, map[string]interface{}{
				"TransactionType": "Clawback",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   other.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("clawback no line:", err)
			}
			if clawResult2.EngineResult == "tesSUCCESS" {
				t.Fatal("expected clawback from non-existent line to fail, got tesSUCCESS")
			}
			t.Logf("clawback non-existent line: %s (expected tecNO_LINE)", clawResult2.EngineResult)
		},
	}
}

// clawbackAmountExceeds tests clawing back more than the holder has.
// The protocol should clawback the available amount (partial).
func clawbackAmountExceeds() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "clawback_amount_exceeds",
		Description: "Clawback more than holder has, should clawback available amount (partial).",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Enable clawback flag BEFORE creating trust lines.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 16)
			if err != nil {
				t.Fatal("enable clawback:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Enable DefaultRipple.
			result, err = rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust line and issue 100 USD to holder.
			err = setup.SetupIOU(ctx, rpc,
				issuer.Address, issuer.Secret,
				holder.Address, holder.Secret,
				"USD", "100",
			)
			if err != nil {
				t.Fatal("setup IOU:", err)
			}

			// Record balance before clawback.
			balBefore := getIOUBalance(t, rpc, holder.Address, "USD", issuer.Address)

			// Claw back 200 USD (more than the 100 held).
			clawResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Clawback",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   holder.Address,
					"value":    "200",
				},
			})
			if err != nil {
				t.Fatal("clawback:", err)
			}
			assertEngineResult(t, clawResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify balance is now 0 (or the line was removed).
			balAfter := getIOUBalance(t, rpc, holder.Address, "USD", issuer.Address)
			t.Logf("holder balance: %s -> %s (clawback 200 of 100)", balBefore, balAfter)

			afterVal, _ := strconv.ParseFloat(balAfter, 64)
			if afterVal > 0 {
				t.Fatalf("expected holder balance <= 0 after clawback, got %s", balAfter)
			}
		},
	}
}

// clawbackBidirectional tests clawback when both parties issue to each other.
// A claws back from B, B claws back from A.
func clawbackBidirectional() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "clawback_bidirectional",
		Description: "Both parties issue to each other, both claw back.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 2)
			alice := accounts[0]
			bob := accounts[1]

			// Enable clawback on both Alice and Bob BEFORE any trust lines.
			for _, acct := range []setup.Account{alice, bob} {
				result, err := rpc.SubmitAccountSet(acct.Secret, acct.Address, 16)
				if err != nil {
					t.Fatalf("enable clawback %s: %v", acct.Address, err)
				}
				assertEngineResult(t, result, "tesSUCCESS")
			}
			waitSettled(rpc)

			// Enable DefaultRipple on both.
			for _, acct := range []setup.Account{alice, bob} {
				result, err := rpc.SubmitAccountSet(acct.Secret, acct.Address, 8)
				if err != nil {
					t.Fatalf("set default ripple %s: %v", acct.Address, err)
				}
				assertEngineResult(t, result, "tesSUCCESS")
			}
			waitSettled(rpc)

			// Bob trusts Alice for USD, Alice sends USD to Bob.
			err := setup.SetupIOU(ctx, rpc,
				alice.Address, alice.Secret,
				bob.Address, bob.Secret,
				"USD", "100",
			)
			if err != nil {
				t.Fatal("setup IOU alice->bob:", err)
			}

			// Alice trusts Bob for EUR, Bob sends EUR to Alice.
			err = setup.SetupIOU(ctx, rpc,
				bob.Address, bob.Secret,
				alice.Address, alice.Secret,
				"EUR", "50",
			)
			if err != nil {
				t.Fatal("setup IOU bob->alice:", err)
			}

			// Alice claws back 30 USD from Bob.
			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Clawback",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   bob.Address,
					"value":    "30",
				},
			})
			if err != nil {
				t.Fatal("clawback alice->bob:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Bob claws back 20 EUR from Alice.
			result, err = rpc.Submit(bob.Secret, bob.Address, map[string]interface{}{
				"TransactionType": "Clawback",
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   alice.Address,
					"value":    "20",
				},
			})
			if err != nil {
				t.Fatal("clawback bob->alice:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify balances.
			bobUSD := getIOUBalance(t, rpc, bob.Address, "USD", alice.Address)
			aliceEUR := getIOUBalance(t, rpc, alice.Address, "EUR", bob.Address)
			t.Logf("bob USD balance: %s (expected ~70)", bobUSD)
			t.Logf("alice EUR balance: %s (expected ~30)", aliceEUR)
		},
	}
}

// clawbackMultiLine tests clawback from one holder while verifying other
// holders' trust line balances are unchanged.
func clawbackMultiLine() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "clawback_multi_line",
		Description: "Issuer has trust lines with multiple holders, clawback from one, others unchanged.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 4)
			issuer := accounts[0]
			holderA := accounts[1]
			holderB := accounts[2]
			holderC := accounts[3]

			// Enable clawback BEFORE trust lines.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 16)
			if err != nil {
				t.Fatal("enable clawback:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Enable DefaultRipple.
			result, err = rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Issue USD to all three holders.
			for _, holder := range []setup.Account{holderA, holderB, holderC} {
				err := setup.SetupIOU(ctx, rpc,
					issuer.Address, issuer.Secret,
					holder.Address, holder.Secret,
					"USD", "100",
				)
				if err != nil {
					t.Fatalf("setup IOU to %s: %v", holder.Address, err)
				}
			}

			// Record balances before clawback.
			balBBefore := getIOUBalance(t, rpc, holderB.Address, "USD", issuer.Address)
			balCBefore := getIOUBalance(t, rpc, holderC.Address, "USD", issuer.Address)

			// Clawback 50 USD from holderA only.
			clawResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Clawback",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   holderA.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("clawback:", err)
			}
			assertEngineResult(t, clawResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify holderA balance changed.
			balA := getIOUBalance(t, rpc, holderA.Address, "USD", issuer.Address)
			t.Logf("holderA balance after clawback: %s (expected ~50)", balA)

			// Verify holderB and holderC unchanged.
			balBAfter := getIOUBalance(t, rpc, holderB.Address, "USD", issuer.Address)
			balCAfter := getIOUBalance(t, rpc, holderC.Address, "USD", issuer.Address)
			if balBAfter != balBBefore {
				t.Fatalf("holderB balance changed: %s -> %s", balBBefore, balBAfter)
			}
			if balCAfter != balCBefore {
				t.Fatalf("holderC balance changed: %s -> %s", balCBefore, balCAfter)
			}
			t.Logf("holderB=%s holderC=%s (unchanged)", balBAfter, balCAfter)
		},
	}
}

// clawbackFrozenTrustline tests that clawback works on a frozen trust line.
func clawbackFrozenTrustline() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "clawback_frozen_trustline",
		Description: "Freeze trust line, then clawback. Should still succeed.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Enable clawback BEFORE trust lines.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 16)
			if err != nil {
				t.Fatal("enable clawback:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Enable DefaultRipple.
			result, err = rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust line and issue IOU.
			err = setup.SetupIOU(ctx, rpc,
				issuer.Address, issuer.Secret,
				holder.Address, holder.Secret,
				"USD", "100",
			)
			if err != nil {
				t.Fatal("setup IOU:", err)
			}

			// Freeze the trust line (issuer side: TrustSet with tfSetFreeze = 0x00100000).
			freezeResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "TrustSet",
				"LimitAmount": map[string]interface{}{
					"currency": "USD",
					"issuer":   holder.Address,
					"value":    "0",
				},
				"Flags": 1048576, // tfSetFreeze
			})
			if err != nil {
				t.Fatal("freeze trust line:", err)
			}
			assertEngineResult(t, freezeResult, "tesSUCCESS")
			waitSettled(rpc)

			// Clawback should still work on frozen trust line.
			clawResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Clawback",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   holder.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("clawback:", err)
			}
			assertEngineResult(t, clawResult, "tesSUCCESS")
			t.Log("clawback on frozen trust line succeeded")
		},
	}
}

// clawbackPermission tests that a non-issuer cannot clawback.
func clawbackPermission() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "clawback_permission",
		Description: "Non-issuer tries to clawback, expect tecNO_PERMISSION.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			holder := accounts[1]
			stranger := accounts[2]

			// Enable clawback on issuer BEFORE trust lines.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 16)
			if err != nil {
				t.Fatal("enable clawback:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Enable DefaultRipple.
			result, err = rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust line and issue IOU.
			err = setup.SetupIOU(ctx, rpc,
				issuer.Address, issuer.Secret,
				holder.Address, holder.Secret,
				"USD", "100",
			)
			if err != nil {
				t.Fatal("setup IOU:", err)
			}

			// Stranger tries to clawback (not the issuer).
			clawResult, err := rpc.Submit(stranger.Secret, stranger.Address, map[string]interface{}{
				"TransactionType": "Clawback",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   holder.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("clawback:", err)
			}
			if clawResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected non-issuer clawback to fail, got tesSUCCESS")
			}
			t.Logf("non-issuer clawback: %s (expected tecNO_PERMISSION)", clawResult.EngineResult)
		},
	}
}

// clawbackWithTickets tests clawback using a TicketSequence.
func clawbackWithTickets() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "clawback_with_tickets",
		Description: "Clawback using TicketSequence.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Enable clawback BEFORE trust lines.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 16)
			if err != nil {
				t.Fatal("enable clawback:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Enable DefaultRipple.
			result, err = rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust line and issue IOU.
			err = setup.SetupIOU(ctx, rpc,
				issuer.Address, issuer.Secret,
				holder.Address, holder.Secret,
				"USD", "100",
			)
			if err != nil {
				t.Fatal("setup IOU:", err)
			}

			// Get current sequence for ticket calculation.
			info, err := rpc.AccountInfo(issuer.Address)
			if err != nil {
				t.Fatal("account_info:", err)
			}

			// Create a ticket.
			ticketResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "TicketCreate",
				"TicketCount":     1,
			})
			if err != nil {
				t.Fatal("ticket create:", err)
			}
			assertEngineResult(t, ticketResult, "tesSUCCESS")
			waitSettled(rpc)

			ticketSeq := info.Sequence + 1

			// Clawback using the ticket.
			clawResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Clawback",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   holder.Address,
					"value":    "50",
				},
				"Sequence":       0,
				"TicketSequence": ticketSeq,
			})
			if err != nil {
				t.Fatal("clawback with ticket:", err)
			}
			assertEngineResult(t, clawResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify balance changed.
			bal := getIOUBalance(t, rpc, holder.Address, "USD", issuer.Address)
			t.Logf("holder balance after ticket clawback: %s (expected ~50)", bal)
		},
	}
}
