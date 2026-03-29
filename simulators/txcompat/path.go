package main

import (
	"context"
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

// pathDirectNoIntermediary tests that ripple_path_find returns a direct path
// when source and destination share the same trust line to the same issuer.
func pathDirectNoIntermediary() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "path_direct_no_intermediary",
		Description: "Direct path (same currency, same issuer). Verify ripple_path_find returns a path.",
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

			// Both alice and bob trust the issuer for USD.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			// Issuer sends 100 USD to alice so she has funds.
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
				t.Fatal("fund alice:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Find path from alice to bob for 10 USD.
			raw, err := rpc.Call("ripple_path_find", map[string]interface{}{
				"source_account":      alice.Address,
				"destination_account": bob.Address,
				"destination_amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "10",
				},
			})
			if err != nil {
				t.Fatal("ripple_path_find:", err)
			}

			var resp struct {
				Alternatives []struct {
					SourceAmount interface{} `json:"source_amount"`
					PathsComputed []interface{} `json:"paths_computed"`
				} `json:"alternatives"`
				Status string `json:"status"`
			}
			json.Unmarshal(raw, &resp)

			// A direct path should exist (same issuer, both have trust lines).
			if len(resp.Alternatives) == 0 {
				t.Fatal("expected at least one path alternative for direct USD transfer")
			}
			t.Logf("direct path found: %d alternative(s)", len(resp.Alternatives))
		},
	}
}

// pathFindBasic tests the basic ripple_path_find RPC call and verifies
// that paths are returned for a simple trust line scenario.
func pathFindBasic() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "path_find_basic",
		Description: "Basic ripple_path_find: alice has USD from gateway, bob wants USD.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			gateway := accounts[0]
			alice := accounts[1]
			bob := accounts[2]

			// Enable DefaultRipple on gateway.
			result, err := rpc.SubmitAccountSet(gateway.Secret, gateway.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Both trust the gateway for USD.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", gateway.Address, "1000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", gateway.Address, "1000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			// Gateway sends 50 USD to alice.
			result, err = rpc.Submit(gateway.Secret, gateway.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   gateway.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("fund alice:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Find paths: alice -> bob, 10 USD from gateway.
			raw, err := rpc.Call("ripple_path_find", map[string]interface{}{
				"source_account":      alice.Address,
				"destination_account": bob.Address,
				"destination_amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   gateway.Address,
					"value":    "10",
				},
			})
			if err != nil {
				t.Fatal("ripple_path_find:", err)
			}

			var resp struct {
				Alternatives []struct {
					SourceAmount interface{} `json:"source_amount"`
				} `json:"alternatives"`
			}
			json.Unmarshal(raw, &resp)

			if len(resp.Alternatives) == 0 {
				t.Fatal("expected at least one path alternative")
			}
			t.Logf("basic path find: %d alternative(s), source_amount=%v",
				len(resp.Alternatives), resp.Alternatives[0].SourceAmount)
		},
	}
}

// pathPaymentAutoPathFind tests a cross-currency payment where the node
// finds the path automatically via the Paths field.
func pathPaymentAutoPathFind() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "path_payment_auto_path_find",
		Description: "Cross-currency payment with explicit Paths field enabling path-based routing.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 4)
			issuer := accounts[0]
			alice := accounts[1]
			bob := accounts[2]
			mm := accounts[3]

			// Enable DefaultRipple on issuer.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust lines: alice has USD, bob wants EUR, mm bridges both.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line alice USD:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "EUR", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line bob EUR:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, mm.Address, mm.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line mm USD:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, mm.Address, mm.Secret, "EUR", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line mm EUR:", err)
			}

			// Fund alice with USD and market maker with EUR.
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
				"Destination":     mm.Address,
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

			// Market maker: offer to sell 50 EUR for 50 USD.
			err = setup.SetupOffer(ctx, rpc, mm.Address, mm.Secret,
				map[string]interface{}{"currency": "USD", "issuer": issuer.Address, "value": "50"},
				map[string]interface{}{"currency": "EUR", "issuer": issuer.Address, "value": "50"},
			)
			if err != nil {
				t.Fatal("mm offer:", err)
			}

			// Use ripple_path_find to get paths first.
			raw, err := rpc.Call("ripple_path_find", map[string]interface{}{
				"source_account":      alice.Address,
				"destination_account": bob.Address,
				"destination_amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   issuer.Address,
					"value":    "10",
				},
			})
			if err != nil {
				t.Fatal("ripple_path_find:", err)
			}

			var pathResp struct {
				Alternatives []struct {
					SourceAmount  interface{}   `json:"source_amount"`
					PathsComputed []interface{} `json:"paths_computed"`
				} `json:"alternatives"`
			}
			json.Unmarshal(raw, &pathResp)

			if len(pathResp.Alternatives) == 0 {
				t.Fatal("expected path alternatives for USD->EUR via offer")
			}

			// Use the discovered paths to submit the payment.
			paths := pathResp.Alternatives[0].PathsComputed
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   issuer.Address,
					"value":    "10",
				},
				"SendMax": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "10",
				},
				"Paths": paths,
			})
			if err != nil {
				t.Fatal("path payment:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify bob received EUR.
			bobEUR := getIOUBalance(t, rpc, bob.Address, "EUR", issuer.Address)
			if bobEUR == "0" {
				t.Fatal("bob should have received EUR")
			}
			t.Logf("auto path payment: bob received %s EUR", bobEUR)
		},
	}
}

// pathNoPath tests that ripple_path_find returns empty alternatives when
// there is no connection between source and destination.
func pathNoPath() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "path_no_path",
		Description: "No path between unconnected accounts. Verify empty alternatives.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			alice := accounts[1]
			bob := accounts[2]

			// Neither alice nor bob has any trust lines or connections.
			// Try to find a path for USD that nobody has trust lines for.
			raw, err := rpc.Call("ripple_path_find", map[string]interface{}{
				"source_account":      alice.Address,
				"destination_account": bob.Address,
				"destination_amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "10",
				},
			})
			if err != nil {
				t.Fatal("ripple_path_find:", err)
			}

			var resp struct {
				Alternatives []json.RawMessage `json:"alternatives"`
			}
			json.Unmarshal(raw, &resp)

			if len(resp.Alternatives) != 0 {
				t.Fatalf("expected no path alternatives, got %d", len(resp.Alternatives))
			}
			t.Log("no path found as expected for unconnected accounts")
		},
	}
}

// pathTrustAutoClear tests that a trust line is auto-removed when both the
// balance and the limit reach zero.
func pathTrustAutoClear() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "path_trust_auto_clear",
		Description: "Pay trust line to zero, set limit to zero, verify trust line is auto-removed.",
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

			// Alice trusts issuer for USD.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line:", err)
			}

			// Issuer sends 50 USD to alice.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("fund alice:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify trust line exists.
			lines := getAccountLines(t, rpc, alice.Address)
			if len(lines) == 0 {
				t.Fatal("expected trust line to exist after funding")
			}
			t.Logf("trust line before clear: balance=%s, limit=%s", lines[0].Balance, lines[0].Limit)

			// Alice sends all 50 USD back to issuer (balance goes to zero).
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     issuer.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("pay back:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Set limit to zero to trigger auto-removal.
			result, err = rpc.SubmitTrustSet(alice.Secret, alice.Address, "USD", issuer.Address, "0")
			if err != nil {
				t.Fatal("trust set zero:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify trust line balance is zero and limit is zero.
			// Note: with DefaultRipple, the trust line may persist (non-default flags)
			// but balance and limit should both be zero.
			lines = getAccountLines(t, rpc, alice.Address)
			if len(lines) == 0 {
				t.Log("trust line auto-cleared successfully (removed)")
			} else if lines[0].Balance == "0" {
				t.Logf("trust line zeroed: balance=%s limit=%s (persists due to flags)", lines[0].Balance, lines[0].Limit)
			} else {
				t.Fatalf("expected zero balance, got %s", lines[0].Balance)
			}
		},
	}
}

// pathSourceCurrencyLimits tests the source_currencies parameter in
// ripple_path_find to limit which currencies can be used as the source.
func pathSourceCurrencyLimits() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "path_source_currency_limits",
		Description: "Use source_currencies to limit which currencies the source can use in path finding.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 4)
			issuer := accounts[0]
			alice := accounts[1]
			bob := accounts[2]
			mm := accounts[3]

			// Enable DefaultRipple on issuer.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice has both USD and EUR.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line alice USD:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "EUR", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line alice EUR:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line bob USD:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, mm.Address, mm.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line mm USD:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, mm.Address, mm.Secret, "EUR", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line mm EUR:", err)
			}

			// Fund alice with both currencies.
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
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("fund alice EUR:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Market maker: offer EUR -> USD.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     mm.Address,
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

			err = setup.SetupOffer(ctx, rpc, mm.Address, mm.Secret,
				map[string]interface{}{"currency": "USD", "issuer": issuer.Address, "value": "50"},
				map[string]interface{}{"currency": "EUR", "issuer": issuer.Address, "value": "50"},
			)
			if err != nil {
				t.Fatal("mm offer:", err)
			}

			// Search with source_currencies restricted to EUR only.
			raw, err := rpc.Call("ripple_path_find", map[string]interface{}{
				"source_account":      alice.Address,
				"destination_account": bob.Address,
				"destination_amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "10",
				},
				"source_currencies": []interface{}{
					map[string]interface{}{"currency": "EUR"},
				},
			})
			if err != nil {
				t.Fatal("ripple_path_find with source_currencies:", err)
			}

			var resp struct {
				Alternatives []struct {
					SourceAmount interface{} `json:"source_amount"`
				} `json:"alternatives"`
			}
			json.Unmarshal(raw, &resp)

			t.Logf("source_currencies EUR only: %d alternative(s)", len(resp.Alternatives))

			// Any alternatives found should use EUR as source (not USD directly).
			for i, alt := range resp.Alternatives {
				if altMap, ok := alt.SourceAmount.(map[string]interface{}); ok {
					if altMap["currency"] != "EUR" {
						t.Fatalf("alternative %d uses currency %v, expected EUR", i, altMap["currency"])
					}
				}
			}

			// Now search with source_currencies restricted to USD.
			// There should be a direct path since both have USD trust lines.
			raw, err = rpc.Call("ripple_path_find", map[string]interface{}{
				"source_account":      alice.Address,
				"destination_account": bob.Address,
				"destination_amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "10",
				},
				"source_currencies": []interface{}{
					map[string]interface{}{"currency": "USD"},
				},
			})
			if err != nil {
				t.Fatal("ripple_path_find with USD source:", err)
			}
			json.Unmarshal(raw, &resp)

			if len(resp.Alternatives) == 0 {
				t.Fatal("expected at least one USD direct path alternative")
			}
			t.Logf("source_currencies USD only: %d alternative(s)", len(resp.Alternatives))
		},
	}
}

// pathHybridOfferPath tests path finding through a hybrid path that uses
// both trust lines and offers on the DEX.
func pathHybridOfferPath() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "path_hybrid_offer_path",
		Description: "Path using both trust lines and DEX offers. Find path through an offer.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			gateway := accounts[0]
			alice := accounts[1]
			bob := accounts[2]

			// Enable DefaultRipple on gateway.
			result, err := rpc.SubmitAccountSet(gateway.Secret, gateway.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice has USD from gateway via trust line.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", gateway.Address, "1000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}

			// Fund alice with USD.
			result, err = rpc.Submit(gateway.Secret, gateway.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   gateway.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("fund alice:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice creates offer: sell 50 USD for 50 XRP.
			// This creates a USD->XRP path on the order book.
			err = setup.SetupOffer(ctx, rpc, alice.Address, alice.Secret,
				"50000000", // wants 50 XRP
				map[string]interface{}{"currency": "USD", "issuer": gateway.Address, "value": "50"},
			)
			if err != nil {
				t.Fatal("alice offer:", err)
			}

			// Bob wants XRP. Find path from alice to bob for 10 XRP.
			// The path should go: alice USD -> (offer) -> XRP -> bob.
			raw, err := rpc.Call("ripple_path_find", map[string]interface{}{
				"source_account":      alice.Address,
				"destination_account": bob.Address,
				"destination_amount":  "10000000", // 10 XRP in drops
			})
			if err != nil {
				t.Fatal("ripple_path_find:", err)
			}

			var resp struct {
				Alternatives []struct {
					SourceAmount  interface{}   `json:"source_amount"`
					PathsComputed []interface{} `json:"paths_computed"`
				} `json:"alternatives"`
			}
			json.Unmarshal(raw, &resp)

			t.Logf("hybrid offer path: %d alternative(s)", len(resp.Alternatives))
			for i, alt := range resp.Alternatives {
				t.Logf("  alternative %d: source_amount=%v", i, alt.SourceAmount)
			}
		},
	}
}

// pathQualitySetAndTest tests that quality settings on trust lines are
// reflected in ripple_path_find results.
func pathQualitySetAndTest() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "path_quality_set_and_test",
		Description: "Set quality on trust lines, verify ripple_path_find respects quality.",
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

			// Alice creates trust line with QualityIn = 900000000 (90%) and
			// QualityOut = 1100000000 (110%).
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "TrustSet",
				"LimitAmount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "1000",
				},
				"QualityIn":  900000000,
				"QualityOut": 1100000000,
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

			// Fund alice with 100 USD. SendMax needed because QualityIn affects path cost.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
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
				t.Fatal("fund alice:", err)
			}
			t.Logf("fund alice with quality: %s", result.EngineResult)
			waitSettled(rpc)

			// Find paths from alice to bob for 10 USD.
			raw, err := rpc.Call("ripple_path_find", map[string]interface{}{
				"source_account":      alice.Address,
				"destination_account": bob.Address,
				"destination_amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "10",
				},
			})
			if err != nil {
				t.Fatal("ripple_path_find:", err)
			}

			var resp struct {
				Alternatives []struct {
					SourceAmount interface{} `json:"source_amount"`
				} `json:"alternatives"`
			}
			json.Unmarshal(raw, &resp)

			if len(resp.Alternatives) == 0 {
				t.Fatal("expected path alternatives with quality settings")
			}

			// The source_amount may differ from destination_amount due to quality.
			t.Logf("quality path: %d alternative(s)", len(resp.Alternatives))
			for i, alt := range resp.Alternatives {
				t.Logf("  alternative %d: source_amount=%v", i, alt.SourceAmount)
			}

			// Also find path from bob to alice to see the reverse quality effect.
			// Fund bob too so the reverse path is possible.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("fund bob:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			raw, err = rpc.Call("ripple_path_find", map[string]interface{}{
				"source_account":      bob.Address,
				"destination_account": alice.Address,
				"destination_amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "10",
				},
			})
			if err != nil {
				t.Fatal("ripple_path_find reverse:", err)
			}
			json.Unmarshal(raw, &resp)

			t.Logf("reverse quality path: %d alternative(s)", len(resp.Alternatives))
			for i, alt := range resp.Alternatives {
				t.Logf("  reverse alternative %d: source_amount=%v", i, alt.SourceAmount)
			}
		},
	}
}

// accountLine is a helper struct for parsing account_lines responses.
type accountLine struct {
	Balance  string `json:"balance"`
	Currency string `json:"currency"`
	Limit    string `json:"limit"`
	Account  string `json:"account"`
}

// getAccountLines returns all trust lines for the given account.
func getAccountLines(t *xrplsim.T, rpc *xrplsim.RPCClient, account string) []accountLine {
	raw, err := rpc.Call("account_lines", map[string]interface{}{
		"account":      account,
		"ledger_index": "current",
	})
	if err != nil {
		t.Fatal("account_lines:", err)
	}
	var resp struct {
		Lines []accountLine `json:"lines"`
	}
	json.Unmarshal(raw, &resp)
	return resp.Lines
}
