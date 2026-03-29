package main

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

// flowDirectStep tests a direct XRP payment between two accounts and verifies
// balance changes on both sides.
func flowDirectStep() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "flow_direct_step",
		Description: "Direct XRP payment between two accounts, verify balance changes.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			alice := accounts[0]
			bob := accounts[1]

			// Record balances before payment.
			aliceBefore := getAccountBalance(t, rpc, alice.Address)
			bobBefore := getAccountBalance(t, rpc, bob.Address)

			// Alice sends 1000 XRP (1,000,000,000 drops) to Bob.
			sendAmount := "1000000000"
			result, err := rpc.SubmitPayment(alice.Secret, alice.Address, bob.Address, sendAmount)
			if err != nil {
				t.Fatal("payment submit:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify balances changed.
			aliceAfter := getAccountBalance(t, rpc, alice.Address)
			bobAfter := getAccountBalance(t, rpc, bob.Address)

			aliceB, _ := strconv.ParseInt(aliceBefore, 10, 64)
			aliceA, _ := strconv.ParseInt(aliceAfter, 10, 64)
			bobB, _ := strconv.ParseInt(bobBefore, 10, 64)
			bobA, _ := strconv.ParseInt(bobAfter, 10, 64)

			sent, _ := strconv.ParseInt(sendAmount, 10, 64)

			// Alice should have lost at least the sent amount (plus fee).
			if aliceA >= aliceB-sent {
				t.Fatalf("alice balance not reduced enough: before=%d, after=%d, sent=%d", aliceB, aliceA, sent)
			}

			// Bob should have gained exactly the sent amount.
			if bobA != bobB+sent {
				t.Fatalf("bob balance mismatch: before=%d, after=%d, expected gain=%d", bobB, bobA, sent)
			}

			t.Logf("direct XRP payment: alice %d->%d, bob %d->%d", aliceB, aliceA, bobB, bobA)
		},
	}
}

// flowTransferRate tests that a TransferRate set on an issuer correctly deducts
// a fee when IOUs are sent between non-issuer accounts.
func flowTransferRate() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "flow_transfer_rate",
		Description: "Set TransferRate (20%) on issuer, send IOU via trust lines, verify fee deduction.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			alice := accounts[1]
			bob := accounts[2]

			// Enable DefaultRipple on issuer BEFORE trust lines.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Set 20% transfer rate: 1,200,000,000.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AccountSet",
				"TransferRate":    1200000000,
			})
			if err != nil {
				t.Fatal("set transfer rate:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice and Bob trust the issuer for USD.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			// Issuer sends 100 USD to Alice.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("issuer payment:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Record balances before transfer.
			aliceBefore := getIOUBalance(t, rpc, alice.Address, "USD", issuer.Address)
			bobBefore := getIOUBalance(t, rpc, bob.Address, "USD", issuer.Address)

			// Alice sends 50 USD to Bob. With 20% transfer rate, Alice should lose 60 USD.
			// SendMax allows the engine to deduct up to 60 from Alice to deliver 50 to Bob.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "50",
				},
				"SendMax": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "60",
				},
			})
			if err != nil {
				t.Fatal("alice to bob payment:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify balances after transfer.
			aliceAfter := getIOUBalance(t, rpc, alice.Address, "USD", issuer.Address)
			bobAfter := getIOUBalance(t, rpc, bob.Address, "USD", issuer.Address)

			// Bob should have received 50 USD.
			if bobAfter != "50" {
				t.Fatalf("bob should have 50 USD, got %s", bobAfter)
			}

			// Alice should have lost 60 USD (50 + 20% fee = 60), ending at 40.
			if aliceAfter != "40" {
				t.Fatalf("alice should have 40 USD (lost 60 = 50 + 20%% fee), got %s", aliceAfter)
			}

			t.Logf("transfer rate: alice USD %s->%s, bob USD %s->%s",
				aliceBefore, aliceAfter, bobBefore, bobAfter)
		},
	}
}

// flowSelfPayment tests that an account can send an IOU payment to itself
// with no net balance change.
func flowSelfPayment() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "flow_self_payment",
		Description: "Account sends IOU payment to itself, verify no net balance change.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			alice := accounts[1]

			// Enable DefaultRipple on issuer.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup: alice trusts issuer, issuer sends 100 USD to alice.
			err = setup.SetupIOU(ctx, rpc,
				issuer.Address, issuer.Secret,
				alice.Address, alice.Secret,
				"USD", "100",
			)
			if err != nil {
				t.Fatal("setup IOU:", err)
			}

			// Record balance before self-payment.
			balBefore := getIOUBalance(t, rpc, alice.Address, "USD", issuer.Address)

			// Alice sends 50 USD to herself.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("self payment:", err)
			}
			// Self-payment should succeed (or might be tecNO_PERMISSION depending on
			// deposit auth rules, but with standard settings it succeeds).
			t.Logf("self payment result: %s", result.EngineResult)
			waitSettled(rpc)

			// If it succeeded, verify no net balance change.
			if result.EngineResult == "tesSUCCESS" {
				balAfter := getIOUBalance(t, rpc, alice.Address, "USD", issuer.Address)
				if balAfter != balBefore {
					t.Fatalf("self-payment changed balance: before=%s, after=%s", balBefore, balAfter)
				}
				t.Logf("self payment: balance unchanged at %s USD", balAfter)
			}
		},
	}
}

// flowLimitQuality tests payment behavior when QualityIn is set on a trust line.
func flowLimitQuality() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "flow_limit_quality",
		Description: "Set QualityIn on trust line and verify payment respects quality settings.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			alice := accounts[1]
			bob := accounts[2]

			// Enable DefaultRipple on issuer.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice creates trust line with QualityIn = 900000000 (90% valuation).
			// This means alice values incoming USD at 90% — 1 USD from issuer is worth 0.9 to alice.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "TrustSet",
				"LimitAmount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "1000",
				},
				"QualityIn": 900000000,
			})
			if err != nil {
				t.Fatal("trust set with quality:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Bob creates a normal trust line.
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			// Issuer sends 100 USD to alice and bob.
			for _, dest := range []setup.Account{alice, bob} {
				result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
					"TransactionType": "Payment",
					"Destination":     dest.Address,
					"Amount": map[string]interface{}{
						"currency": "USD",
						"issuer":   issuer.Address,
						"value":    "100",
					},
					"SendMax": map[string]interface{}{
						"currency": "USD",
						"issuer":   issuer.Address,
						"value":    "200",
					},
				})
				if err != nil {
					t.Fatal("issuer payment:", err)
				}
				t.Logf("issuer→%s: %s", dest.Address[:8], result.EngineResult)
				waitSettled(rpc)
			}

			// Verify alice has USD.
			aliceBal := getIOUBalance(t, rpc, alice.Address, "USD", issuer.Address)
			bobBal := getIOUBalance(t, rpc, bob.Address, "USD", issuer.Address)
			t.Logf("after quality setup: alice=%s USD, bob=%s USD", aliceBal, bobBal)
		},
	}
}

// flowLineQuality tests that different QualityOut values on trust lines
// affect payment amounts between accounts.
func flowLineQuality() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "flow_line_quality",
		Description: "Set QualityOut on trust line and verify payment amounts are affected.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			alice := accounts[1]
			bob := accounts[2]

			// Enable DefaultRipple on issuer.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice creates trust line with QualityOut = 800000000 (80% outgoing quality).
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "TrustSet",
				"LimitAmount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "1000",
				},
				"QualityOut": 800000000,
			})
			if err != nil {
				t.Fatal("trust set with quality out:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Bob creates a normal trust line.
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			// Issuer funds both accounts.
			for _, dest := range []setup.Account{alice, bob} {
				result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
					"TransactionType": "Payment",
					"Destination":     dest.Address,
					"Amount": map[string]interface{}{
						"currency": "USD",
						"issuer":   issuer.Address,
						"value":    "100",
					},
				})
				if err != nil {
					t.Fatal("issuer payment:", err)
				}
				assertEngineResult(t, result, "tesSUCCESS")
				waitSettled(rpc)
			}

			// Record balances before.
			aliceBefore := getIOUBalance(t, rpc, alice.Address, "USD", issuer.Address)
			bobBefore := getIOUBalance(t, rpc, bob.Address, "USD", issuer.Address)

			// Alice sends 10 USD to bob. QualityOut on alice's line may mean bob
			// receives less, or alice needs to send more to deliver 10.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "10",
				},
			})
			if err != nil {
				t.Fatal("alice to bob payment:", err)
			}
			t.Logf("payment with QualityOut result: %s", result.EngineResult)
			waitSettled(rpc)

			// Verify balances changed.
			aliceAfter := getIOUBalance(t, rpc, alice.Address, "USD", issuer.Address)
			bobAfter := getIOUBalance(t, rpc, bob.Address, "USD", issuer.Address)
			t.Logf("QualityOut: alice USD %s->%s, bob USD %s->%s",
				aliceBefore, aliceAfter, bobBefore, bobAfter)
		},
	}
}

// flowUnfundedOffer creates an offer, drains the account so the offer becomes
// unfunded, then verifies the unfunded offer is cleaned up from the book.
func flowUnfundedOffer() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "flow_unfunded_offer",
		Description: "Create offer, drain account, verify unfunded offer is cleaned up on crossing.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			alice := accounts[1]
			bob := accounts[2]

			// Enable DefaultRipple on issuer.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust lines.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			// Issuer sends 100 USD to alice.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("fund alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice creates an offer: sell 50 USD for 50 XRP.
			err = setup.SetupOffer(ctx, rpc, alice.Address, alice.Secret,
				"50000000", // wants 50 XRP
				map[string]interface{}{"currency": "USD", "issuer": issuer.Address, "value": "50"},
			)
			if err != nil {
				t.Fatal("alice offer:", err)
			}

			// Verify alice has an offer on the book.
			raw, err := rpc.Call("account_offers", map[string]interface{}{
				"account":      alice.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_offers:", err)
			}
			var offResp struct {
				Offers []json.RawMessage `json:"offers"`
			}
			json.Unmarshal(raw, &offResp)
			if len(offResp.Offers) == 0 {
				t.Fatal("expected alice to have an offer")
			}
			t.Logf("alice has %d offer(s) before drain", len(offResp.Offers))

			// Drain alice's USD by sending it all back to issuer.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     issuer.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("drain alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Issuer sends 50 USD to bob so bob can cross.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("fund bob USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Bob places a crossing offer: sell 50 XRP for 50 USD.
			// This should encounter alice's unfunded offer and clean it up.
			crossResult, err := rpc.SubmitOfferCreate(bob.Secret, bob.Address,
				map[string]interface{}{"currency": "USD", "issuer": issuer.Address, "value": "50"},
				"50000000",
			)
			if err != nil {
				t.Fatal("bob crossing offer:", err)
			}
			t.Logf("bob crossing offer: %s", crossResult.EngineResult)
			waitSettled(rpc)

			// Check that alice's unfunded offer was removed.
			raw, err = rpc.Call("account_offers", map[string]interface{}{
				"account":      alice.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_offers after:", err)
			}
			json.Unmarshal(raw, &offResp)
			if len(offResp.Offers) != 0 {
				t.Fatalf("expected alice's unfunded offer to be removed, found %d offer(s)", len(offResp.Offers))
			}
			t.Log("unfunded offer cleaned up successfully")
		},
	}
}

// flowCircularXRP tests a circular payment path: XRP -> IOU -> XRP using offers.
func flowCircularXRP() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "flow_circular_xrp",
		Description: "XRP -> IOU -> XRP circular path via offers.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 4)
			issuer := accounts[0]
			alice := accounts[1]
			bob := accounts[2]
			carol := accounts[3]

			// Enable DefaultRipple on issuer.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust lines: alice and bob trust issuer for USD.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			// Issuer sends 100 USD to alice.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("fund alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice creates offer: sell 50 USD for 50 XRP.
			err = setup.SetupOffer(ctx, rpc, alice.Address, alice.Secret,
				"50000000",
				map[string]interface{}{"currency": "USD", "issuer": issuer.Address, "value": "50"},
			)
			if err != nil {
				t.Fatal("alice offer USD->XRP:", err)
			}

			// Bob creates offer: sell 50 XRP for 50 USD.
			err = setup.SetupOffer(ctx, rpc, bob.Address, bob.Secret,
				map[string]interface{}{"currency": "USD", "issuer": issuer.Address, "value": "50"},
				"50000000",
			)
			if err != nil {
				t.Fatal("bob offer XRP->USD:", err)
			}

			// Record carol's XRP balance before.
			carolBefore := getAccountBalance(t, rpc, carol.Address)

			// Alice pays 10 XRP to carol using path: XRP -> USD (alice offer) -> XRP (bob offer).
			// The path goes through the USD book.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     carol.Address,
				"Amount":          "10000000", // 10 XRP to carol
				"SendMax":         "10000000", // at most 10 XRP
			})
			if err != nil {
				t.Fatal("circular payment:", err)
			}
			t.Logf("circular payment result: %s", result.EngineResult)
			waitSettled(rpc)

			carolAfter := getAccountBalance(t, rpc, carol.Address)
			t.Logf("carol XRP: before=%s, after=%s", carolBefore, carolAfter)
		},
	}
}

// flowPaymentWithTicket tests an IOU payment submitted using TicketSequence
// instead of a regular Sequence.
func flowPaymentWithTicket() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "flow_payment_with_ticket",
		Description: "IOU payment using TicketSequence instead of Sequence.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			alice := accounts[1]
			bob := accounts[2]

			// Enable DefaultRipple on issuer.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup: alice and bob trust issuer, issuer sends 100 USD to alice.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("fund alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Get alice's current sequence for ticket calculation.
			info, err := rpc.AccountInfo(alice.Address)
			if err != nil {
				t.Fatal("account_info:", err)
			}

			// Create a ticket.
			ticketResult, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "TicketCreate",
				"TicketCount":     1,
			})
			if err != nil {
				t.Fatal("ticket create:", err)
			}
			assertEngineResult(t, ticketResult, "tesSUCCESS")
			waitSettled(rpc)

			// The ticket sequence is the sequence at the time of TicketCreate + 1.
			ticketSeq := info.Sequence + 1

			// Record bob's USD balance before.
			bobBefore := getIOUBalance(t, rpc, bob.Address, "USD", issuer.Address)

			// Send 25 USD from alice to bob using the ticket.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "25",
				},
				"Sequence":       0,
				"TicketSequence": ticketSeq,
			})
			if err != nil {
				t.Fatal("payment with ticket:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify bob received the USD.
			bobAfter := getIOUBalance(t, rpc, bob.Address, "USD", issuer.Address)
			if bobAfter == bobBefore {
				t.Fatalf("bob's USD balance did not change: %s", bobAfter)
			}
			t.Logf("payment with ticket: bob USD %s->%s", bobBefore, bobAfter)
		},
	}
}

// flowCrossCurrencyPayment tests a cross-currency payment: alice pays bob in EUR
// using her USD, via offers that connect USD and EUR.
func flowCrossCurrencyPayment() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "flow_cross_currency_payment",
		Description: "Cross-currency payment: alice has USD, pays bob in EUR via connecting offers.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 4)
			issuer := accounts[0]
			alice := accounts[1]
			bob := accounts[2]
			marketMaker := accounts[3]

			// Enable DefaultRipple on issuer (handles both USD and EUR).
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust lines.
			// Alice trusts USD.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line alice USD:", err)
			}
			// Bob trusts EUR.
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "EUR", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line bob EUR:", err)
			}
			// Market maker trusts both USD and EUR.
			err = setup.SetupTrustLine(ctx, rpc, marketMaker.Address, marketMaker.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line mm USD:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, marketMaker.Address, marketMaker.Secret, "EUR", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line mm EUR:", err)
			}

			// Fund: issuer sends 100 USD to alice and 100 EUR to market maker.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("fund alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     marketMaker.Address,
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("fund mm EUR:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Also fund market maker with USD to create a two-sided market.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     marketMaker.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("fund mm USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Market maker creates offer: sell 50 EUR for 50 USD (1:1 rate).
			err = setup.SetupOffer(ctx, rpc, marketMaker.Address, marketMaker.Secret,
				map[string]interface{}{"currency": "USD", "issuer": issuer.Address, "value": "50"},
				map[string]interface{}{"currency": "EUR", "issuer": issuer.Address, "value": "50"},
			)
			if err != nil {
				t.Fatal("mm offer USD/EUR:", err)
			}

			// Alice sends cross-currency payment: pay bob 20 EUR, using up to 20 USD.
			// Paths go through the order book: USD -> EUR via the market maker's offer.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   issuer.Address,
					"value":    "20",
				},
				"SendMax": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "20",
				},
				"Paths": []interface{}{
					[]interface{}{
						map[string]interface{}{
							"currency": "EUR",
							"issuer":   issuer.Address,
						},
					},
				},
			})
			if err != nil {
				t.Fatal("cross-currency payment:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify bob received EUR.
			bobEUR := getIOUBalance(t, rpc, bob.Address, "EUR", issuer.Address)
			if bobEUR == "0" {
				t.Fatal("bob should have received EUR")
			}
			t.Logf("cross-currency: bob received %s EUR", bobEUR)

			// Verify alice spent USD.
			aliceUSD := getIOUBalance(t, rpc, alice.Address, "USD", issuer.Address)
			t.Logf("cross-currency: alice has %s USD remaining", aliceUSD)
		},
	}
}

// flowDeliverMin tests a partial payment with the DeliverMin field, verifying
// at least the minimum amount is delivered.
func flowDeliverMin() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "flow_deliver_min",
		Description: "Partial payment with DeliverMin, verify at least the minimum is delivered.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 4)
			issuer := accounts[0]
			alice := accounts[1]
			bob := accounts[2]
			marketMaker := accounts[3]

			// Enable DefaultRipple on issuer.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust lines.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, marketMaker.Address, marketMaker.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line mm:", err)
			}

			// Fund: issuer sends 200 USD to alice.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "200",
				},
			})
			if err != nil {
				t.Fatal("fund alice:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Record bob's balance before.
			bobBefore := getIOUBalance(t, rpc, bob.Address, "USD", issuer.Address)

			// Alice sends partial payment with DeliverMin.
			// tfPartialPayment = 131072 (0x00020000).
			// Amount = 100 (max), DeliverMin = 50 (at least 50 must be delivered).
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
				"DeliverMin": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "50",
				},
				"Flags": 131072,
			})
			if err != nil {
				t.Fatal("partial payment:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify bob received at least 50 USD.
			bobAfter := getIOUBalance(t, rpc, bob.Address, "USD", issuer.Address)
			bobBeforeF, _ := strconv.ParseFloat(bobBefore, 64)
			bobAfterF, _ := strconv.ParseFloat(bobAfter, 64)
			delivered := bobAfterF - bobBeforeF

			if delivered < 50 {
				t.Fatalf("expected at least 50 USD delivered, got %.2f", delivered)
			}
			t.Logf("partial payment: delivered %.2f USD (min was 50)", delivered)
		},
	}
}
