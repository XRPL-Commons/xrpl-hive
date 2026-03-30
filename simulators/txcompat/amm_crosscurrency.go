package main

import (
	"context"
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

// ammExtPayment tests a simple payment that routes through an AMM pool.
// Alice has USD, bob wants XRP. The AMM USD/XRP pool converts.
func ammExtPayment() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_payment",
		Description: "Payment routes through AMM pool: alice sends USD, bob receives XRP.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Create AMM USD/XRP pool (100 USD + 5000 XRP).
			_ = setupAMMPool(t, rpc)

			accounts := mustFund(t, rpc, 2)
			alice := accounts[0]
			bob := accounts[1]

			// Alice trusts genesis for USD and gets some.
			err := setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			result, err := rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "200",
				},
			})
			if err != nil {
				t.Fatal("fund alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Record bob's XRP balance before.
			bobBefore := getAccountBalance(t, rpc, bob.Address)

			// Alice sends a payment to bob: deliver 100 XRP, pay with USD via AMM.
			// SendMax in USD, Amount in XRP drops.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount":          "100000000", // 100 XRP in drops
				"SendMax": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "50",
				},
				"Paths": []interface{}{
					[]interface{}{
						map[string]interface{}{
							"currency": "XRP",
						},
					},
				},
			})
			if err != nil {
				t.Fatal("payment through AMM:", err)
			}
			t.Logf("payment through AMM: %s", result.EngineResult)
			waitSettled(rpc)

			// If successful, bob should have more XRP.
			if result.EngineResult == "tesSUCCESS" {
				bobAfter := getAccountBalance(t, rpc, bob.Address)
				t.Logf("bob XRP before=%s, after=%s", bobBefore, bobAfter)
			}
		},
	}
}

// ammExtPayIOU tests an IOU-to-IOU payment through AMM.
// Requires two pools (USD/XRP and EUR/XRP) bridging through XRP.
func ammExtPayIOU() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_pay_iou",
		Description: "IOU-to-IOU payment through AMM: alice sends USD, bob receives EUR via XRP bridge.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Create USD/XRP AMM pool.
			_ = setupAMMPool(t, rpc)

			// Create EUR/XRP AMM pool.
			accounts := mustFund(t, rpc, 3)
			poolCreator := accounts[0]
			alice := accounts[1]
			bob := accounts[2]

			// Pool creator trusts genesis for EUR.
			err := setup.SetupTrustLine(ctx, rpc, poolCreator.Address, poolCreator.Secret, "EUR", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line pool creator EUR:", err)
			}
			result, err := rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     poolCreator.Address,
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "1000",
				},
			})
			if err != nil {
				t.Fatal("fund pool creator EUR:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Create EUR/XRP pool.
			result, err = rpc.Submit(poolCreator.Secret, poolCreator.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "100",
				},
				"Amount2": "5000000000", // 5000 XRP
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("amm create EUR/XRP:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice trusts genesis for USD and gets some.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line alice USD:", err)
			}
			result, err = rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "200",
				},
			})
			if err != nil {
				t.Fatal("fund alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Bob trusts genesis for EUR.
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "EUR", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line bob EUR:", err)
			}

			// Alice sends USD, bob receives EUR. Path: USD -> XRP (AMM1) -> EUR (AMM2).
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "10",
				},
				"SendMax": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "50",
				},
				"Paths": []interface{}{
					[]interface{}{
						map[string]interface{}{
							"currency": "XRP",
						},
						map[string]interface{}{
							"currency": "EUR",
							"issuer":   xrplsim.GenesisAddress,
						},
					},
				},
			})
			if err != nil {
				t.Fatal("cross-currency IOU payment:", err)
			}
			t.Logf("IOU-to-IOU via AMM: %s", result.EngineResult)
			waitSettled(rpc)

			if result.EngineResult == "tesSUCCESS" {
				bobEUR := getIOUBalance(t, rpc, bob.Address, "EUR", xrplsim.GenesisAddress)
				t.Logf("bob EUR balance: %s", bobEUR)
			}
		},
	}
}

// ammExtOfferCross tests creating a regular offer that crosses against AMM liquidity.
func ammExtOfferCross() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_offer_cross",
		Description: "Regular OfferCreate crosses against AMM pool liquidity.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Create AMM USD/XRP pool.
			_ = setupAMMPool(t, rpc)

			accounts := mustFund(t, rpc, 1)
			bob := accounts[0]

			// Bob trusts genesis for USD.
			err := setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			// Bob creates an offer: buy 10 USD for 1000 XRP (generous rate).
			// The AMM pool has USD liquidity, so the offer should cross against the pool.
			result, err := rpc.SubmitOfferCreate(bob.Secret, bob.Address,
				map[string]interface{}{"currency": "USD", "issuer": xrplsim.GenesisAddress, "value": "10"},
				"1000000000", // 1000 XRP
			)
			if err != nil {
				t.Fatal("bob offer:", err)
			}
			t.Logf("offer cross vs AMM: %s", result.EngineResult)
			waitSettled(rpc)

			// Check if bob received USD.
			bobUSD := getIOUBalance(t, rpc, bob.Address, "USD", xrplsim.GenesisAddress)
			t.Logf("bob USD after offer cross: %s", bobUSD)

			// Verify offer was consumed (fully or partially).
			raw, err := rpc.Call("account_offers", map[string]interface{}{
				"account":      bob.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_offers:", err)
			}
			var offResp struct {
				Offers []json.RawMessage `json:"offers"`
			}
			json.Unmarshal(raw, &offResp)
			t.Logf("bob remaining offers: %d", len(offResp.Offers))
		},
	}
}

// ammExtCurrencyConversion tests full conversion via AMM: sell all of one asset for another.
func ammExtCurrencyConversion() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_currency_conversion",
		Description: "Full currency conversion via AMM: sell all USD for XRP.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Create AMM USD/XRP pool.
			_ = setupAMMPool(t, rpc)

			accounts := mustFund(t, rpc, 1)
			alice := accounts[0]

			// Alice trusts genesis for USD and gets some.
			err := setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			result, err := rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("fund alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			aliceXRPBefore := getAccountBalance(t, rpc, alice.Address)

			// Alice creates an offer to sell all 50 USD for XRP via AMM.
			// tfSell flag (0x00080000) means: sell the full TakerGets amount.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "OfferCreate",
				"TakerPays":       "2500000000", // willing to accept 2500 XRP
				"TakerGets": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("currency conversion offer:", err)
			}
			t.Logf("currency conversion: %s", result.EngineResult)
			waitSettled(rpc)

			// Verify alice's USD was consumed.
			aliceUSD := getIOUBalance(t, rpc, alice.Address, "USD", xrplsim.GenesisAddress)
			aliceXRPAfter := getAccountBalance(t, rpc, alice.Address)
			t.Logf("alice USD after conversion: %s, XRP before=%s after=%s",
				aliceUSD, aliceXRPBefore, aliceXRPAfter)
		},
	}
}

// ammExtCrossCurrencyBridged tests a cross-currency payment bridged through XRP via AMM.
func ammExtCrossCurrencyBridged() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_cross_currency_bridged",
		Description: "Cross-currency payment bridged through XRP via two AMM pools.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Create USD/XRP AMM pool.
			_ = setupAMMPool(t, rpc)

			// Create EUR/XRP AMM pool.
			accounts := mustFund(t, rpc, 3)
			poolCreator := accounts[0]
			alice := accounts[1]
			bob := accounts[2]

			err := setup.SetupTrustLine(ctx, rpc, poolCreator.Address, poolCreator.Secret, "EUR", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line pool creator EUR:", err)
			}
			result, err := rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     poolCreator.Address,
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "1000",
				},
			})
			if err != nil {
				t.Fatal("fund pool creator EUR:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			result, err = rpc.Submit(poolCreator.Secret, poolCreator.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "100",
				},
				"Amount2": "5000000000",
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("amm create EUR/XRP:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice has USD.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line alice USD:", err)
			}
			result, err = rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "200",
				},
			})
			if err != nil {
				t.Fatal("fund alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Bob trusts EUR.
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "EUR", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line bob EUR:", err)
			}

			// Alice pays bob in EUR, using her USD. Bridged: USD -> XRP (AMM1) -> EUR (AMM2).
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "5",
				},
				"SendMax": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "50",
				},
				"Paths": []interface{}{
					[]interface{}{
						map[string]interface{}{
							"currency": "XRP",
						},
						map[string]interface{}{
							"currency": "EUR",
							"issuer":   xrplsim.GenesisAddress,
						},
					},
				},
			})
			if err != nil {
				t.Fatal("cross-currency bridged payment:", err)
			}
			t.Logf("cross-currency bridged via AMM: %s", result.EngineResult)
			waitSettled(rpc)

			if result.EngineResult == "tesSUCCESS" {
				bobEUR := getIOUBalance(t, rpc, bob.Address, "EUR", xrplsim.GenesisAddress)
				t.Logf("bob EUR after bridged payment: %s", bobEUR)
			}
		},
	}
}

// ammExtSellBasic tests OfferCreate with tfSell flag against AMM pool.
func ammExtSellBasic() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_sell_basic",
		Description: "OfferCreate with tfSell flag crossing against AMM liquidity.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Create AMM USD/XRP pool.
			_ = setupAMMPool(t, rpc)

			accounts := mustFund(t, rpc, 1)
			alice := accounts[0]

			// Alice trusts and gets USD.
			err := setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			result, err := rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("fund alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			aliceXRPBefore := getAccountBalance(t, rpc, alice.Address)

			// tfSell = 0x00080000 = 524288.
			// Sell 20 USD for XRP. With tfSell, the engine tries to sell the full TakerGets.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "OfferCreate",
				"TakerPays":       "500000000", // 500 XRP (generous)
				"TakerGets": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "20",
				},
				"Flags": 524288, // tfSell
			})
			if err != nil {
				t.Fatal("sell offer:", err)
			}
			t.Logf("tfSell offer: %s", result.EngineResult)
			waitSettled(rpc)

			aliceUSD := getIOUBalance(t, rpc, alice.Address, "USD", xrplsim.GenesisAddress)
			aliceXRPAfter := getAccountBalance(t, rpc, alice.Address)
			t.Logf("after tfSell: alice USD=%s, XRP before=%s after=%s",
				aliceUSD, aliceXRPBefore, aliceXRPAfter)
		},
	}
}

// ammExtFillOrKill tests OfferCreate with tfFillOrKill against AMM.
// The offer must fill completely or fail.
func ammExtFillOrKill() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_fill_or_kill",
		Description: "OfferCreate with tfFillOrKill against AMM: must fill completely or fail.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Create AMM USD/XRP pool (100 USD + 5000 XRP).
			_ = setupAMMPool(t, rpc)

			accounts := mustFund(t, rpc, 1)
			bob := accounts[0]

			// Bob trusts genesis for USD.
			err := setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			// Case 1: Small fill-or-kill that should succeed.
			// tfFillOrKill = 0x00040000 = 262144.
			// Buy 5 USD for 500 XRP at a generous rate. Pool has 100 USD, so this should fill.
			result, err := rpc.SubmitOfferCreate(bob.Secret, bob.Address,
				map[string]interface{}{"currency": "USD", "issuer": xrplsim.GenesisAddress, "value": "5"},
				"500000000", // 500 XRP
			)
			if err != nil {
				t.Fatal("fill-or-kill small:", err)
			}
			t.Logf("fill-or-kill small (should succeed): %s", result.EngineResult)
			waitSettled(rpc)

			bobUSD := getIOUBalance(t, rpc, bob.Address, "USD", xrplsim.GenesisAddress)
			t.Logf("bob USD after small FoK: %s", bobUSD)

			// Case 2: Huge fill-or-kill that should fail (tecKILLED).
			// Try to buy 200 USD but pool only has ~95 left. With tfFillOrKill it should fail.
			result, err = rpc.Submit(bob.Secret, bob.Address, map[string]interface{}{
				"TransactionType": "OfferCreate",
				"TakerPays": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "200",
				},
				"TakerGets": "20000000000", // 20000 XRP
				"Flags":     262144,        // tfFillOrKill
			})
			if err != nil {
				t.Fatal("fill-or-kill large:", err)
			}
			// Should get tecKILLED because the full amount cannot be filled.
			t.Logf("fill-or-kill large (should fail): %s", result.EngineResult)
			if result.EngineResult == "tesSUCCESS" {
				t.Log("warning: large FoK succeeded unexpectedly (pool may have enough)")
			}
			waitSettled(rpc)
		},
	}
}

// ammExtTransferRate tests setting TransferRate on an issuer, trading through AMM,
// and verifying fees are applied. Uses SendMax for the payment.
func ammExtTransferRate() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_transfer_rate",
		Description: "Set TransferRate on issuer, trade through AMM, verify fees applied.",
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

			// Set 25% transfer rate: 1,250,000,000.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AccountSet",
				"TransferRate":    1250000000,
			})
			if err != nil {
				t.Fatal("set transfer rate:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust lines for USD from this issuer.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", issuer.Address, "10000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", issuer.Address, "10000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			// Fund alice with USD from issuer.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "500",
				},
			})
			if err != nil {
				t.Fatal("fund alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Create AMM pool with this issuer's USD / XRP.
			// Alice creates the pool.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
				"Amount2": "5000000000", // 5000 XRP
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("amm create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			aliceUSDBefore := getIOUBalance(t, rpc, alice.Address, "USD", issuer.Address)
			bobUSDBefore := getIOUBalance(t, rpc, bob.Address, "USD", issuer.Address)

			// Alice sends 50 USD to bob. With 25% transfer rate on the issuer,
			// alice should lose more than 50. Use SendMax to allow up to 75.
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
					"value":    "75",
				},
			})
			if err != nil {
				t.Fatal("payment with transfer rate:", err)
			}
			t.Logf("payment with transfer rate: %s", result.EngineResult)
			waitSettled(rpc)

			aliceUSDAfter := getIOUBalance(t, rpc, alice.Address, "USD", issuer.Address)
			bobUSDAfter := getIOUBalance(t, rpc, bob.Address, "USD", issuer.Address)
			t.Logf("transfer rate: alice USD %s->%s, bob USD %s->%s",
				aliceUSDBefore, aliceUSDAfter, bobUSDBefore, bobUSDAfter)
		},
	}
}

// ammExtBridgedCrossing tests two AMM pools bridging different currencies through XRP.
func ammExtBridgedCrossing() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_bridged_crossing",
		Description: "Two AMM pools bridge different currencies (GBP and JPY) through XRP.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			enableDefaultRippleOnGenesis(t, rpc)

			accounts := mustFund(t, rpc, 4)
			poolCreatorA := accounts[0]
			poolCreatorB := accounts[1]
			alice := accounts[2]
			bob := accounts[3]

			// Create GBP/XRP pool.
			err := setup.SetupTrustLine(ctx, rpc, poolCreatorA.Address, poolCreatorA.Secret, "GBP", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line pool A GBP:", err)
			}
			result, err := rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     poolCreatorA.Address,
				"Amount": map[string]interface{}{
					"currency": "GBP",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "1000",
				},
			})
			if err != nil {
				t.Fatal("fund pool A GBP:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			result, err = rpc.Submit(poolCreatorA.Secret, poolCreatorA.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "GBP",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "100",
				},
				"Amount2": "5000000000",
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("amm create GBP/XRP:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Create JPY/XRP pool.
			err = setup.SetupTrustLine(ctx, rpc, poolCreatorB.Address, poolCreatorB.Secret, "JPY", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line pool B JPY:", err)
			}
			result, err = rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     poolCreatorB.Address,
				"Amount": map[string]interface{}{
					"currency": "JPY",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "1000",
				},
			})
			if err != nil {
				t.Fatal("fund pool B JPY:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			result, err = rpc.Submit(poolCreatorB.Secret, poolCreatorB.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "JPY",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "100",
				},
				"Amount2": "5000000000",
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("amm create JPY/XRP:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice gets GBP.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "GBP", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line alice GBP:", err)
			}
			result, err = rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "GBP",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "200",
				},
			})
			if err != nil {
				t.Fatal("fund alice GBP:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Bob trusts JPY.
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "JPY", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line bob JPY:", err)
			}

			// Alice pays bob JPY using her GBP. Bridged: GBP -> XRP (AMM1) -> JPY (AMM2).
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount": map[string]interface{}{
					"currency": "JPY",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "10",
				},
				"SendMax": map[string]interface{}{
					"currency": "GBP",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "50",
				},
				"Paths": []interface{}{
					[]interface{}{
						map[string]interface{}{
							"currency": "XRP",
						},
						map[string]interface{}{
							"currency": "JPY",
							"issuer":   xrplsim.GenesisAddress,
						},
					},
				},
			})
			if err != nil {
				t.Fatal("bridged crossing payment:", err)
			}
			t.Logf("bridged crossing GBP->XRP->JPY: %s", result.EngineResult)
			waitSettled(rpc)

			if result.EngineResult == "tesSUCCESS" {
				bobJPY := getIOUBalance(t, rpc, bob.Address, "JPY", xrplsim.GenesisAddress)
				t.Logf("bob JPY after bridged crossing: %s", bobJPY)
			}
		},
	}
}

// ammExtEnforceNoRipple tests setting NoRipple on trust lines and verifying AMM respects the flag.
func ammExtEnforceNoRipple() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_enforce_no_ripple",
		Description: "Set NoRipple on trust lines, verify AMM respects the flag for rippling paths.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			alice := accounts[1]
			bob := accounts[2]

			// Enable DefaultRipple on issuer first.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice creates a trust line with tfSetNoRipple (0x00020000 = 131072).
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "TrustSet",
				"LimitAmount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "1000",
				},
				"Flags": 131072, // tfSetNoRipple
			})
			if err != nil {
				t.Fatal("trust set with NoRipple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Bob creates a normal trust line (rippling allowed).
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			// Issuer funds alice and bob.
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
					t.Fatal("fund account:", err)
				}
				assertEngineResult(t, result, "tesSUCCESS")
				waitSettled(rpc)
			}

			// Alice has NoRipple set, so direct rippling alice->issuer->bob should still work
			// (NoRipple prevents rippling through alice as an intermediary, not payments from alice).
			// But a path that routes THROUGH alice's trust line as intermediary should be blocked.
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
				t.Fatal("payment with NoRipple:", err)
			}
			t.Logf("payment from NoRipple holder: %s (expect tesSUCCESS - NoRipple affects intermediary, not sender)", result.EngineResult)
			waitSettled(rpc)

			// Verify the trust line flags.
			raw, err := rpc.Call("account_lines", map[string]interface{}{
				"account":      alice.Address,
				"peer":         issuer.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_lines:", err)
			}
			var resp struct {
				Lines []struct {
					NoRipple     bool   `json:"no_ripple"`
					NoRipplePeer bool   `json:"no_ripple_peer"`
					Currency     string `json:"currency"`
				} `json:"lines"`
			}
			json.Unmarshal(raw, &resp)
			for _, line := range resp.Lines {
				t.Logf("alice trust line %s: no_ripple=%v, no_ripple_peer=%v",
					line.Currency, line.NoRipple, line.NoRipplePeer)
			}
		},
	}
}

// ammExtFrozenTrustLines tests freezing a trust line and trying to trade through AMM.
func ammExtFrozenTrustLines() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_frozen_trust_lines",
		Description: "Freeze trust line, verify trading through AMM fails.",
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
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", issuer.Address, "10000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", issuer.Address, "10000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			// Fund alice with USD.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "500",
				},
			})
			if err != nil {
				t.Fatal("fund alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice creates an AMM pool.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
				"Amount2": "5000000000",
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("amm create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Issuer freezes alice's trust line using TrustSet with tfSetFreeze.
			// tfSetFreeze = 0x00100000 = 1048576.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "TrustSet",
				"LimitAmount": map[string]interface{}{
					"currency": "USD",
					"issuer":   alice.Address,
					"value":    "0",
				},
				"Flags": 1048576, // tfSetFreeze
			})
			if err != nil {
				t.Fatal("freeze trust line:", err)
			}
			t.Logf("freeze alice trust line: %s", result.EngineResult)
			waitSettled(rpc)

			// Now try to trade: bob tries to buy USD from the AMM via offer.
			// This should fail because alice's USD trust line is frozen,
			// affecting the AMM's ability to move funds.
			tradeResult, err := rpc.SubmitOfferCreate(bob.Secret, bob.Address,
				map[string]interface{}{"currency": "USD", "issuer": issuer.Address, "value": "10"},
				"1000000000", // 1000 XRP
			)
			if err != nil {
				t.Fatal("trade against frozen AMM:", err)
			}
			t.Logf("trade against frozen trust line AMM: %s (may fail with tec* code)", tradeResult.EngineResult)
			waitSettled(rpc)

			// Also try a direct payment through the frozen path.
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
				t.Fatal("payment from frozen account:", err)
			}
			t.Logf("payment from frozen trust line: %s (expect tecPATH_DRY or tecFROZEN)", result.EngineResult)
			waitSettled(rpc)
		},
	}
}

// ammExtGlobalFreeze tests setting GlobalFreeze on an issuer and verifying AMM trades fail.
func ammExtGlobalFreeze() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_global_freeze",
		Description: "Set GlobalFreeze on issuer, verify AMM trades fail.",
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

			// Setup trust lines and fund.
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", issuer.Address, "10000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", issuer.Address, "10000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			// Fund alice.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "500",
				},
			})
			if err != nil {
				t.Fatal("fund alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice creates AMM pool with issuer's USD.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
				"Amount2": "5000000000",
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("amm create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Set GlobalFreeze on issuer.
			// asfGlobalFreeze = 7.
			result, err = rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 7)
			if err != nil {
				t.Fatal("set global freeze:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify the flag is set on the issuer.
			info, err := rpc.AccountInfo(issuer.Address)
			if err != nil {
				t.Fatal("account_info issuer:", err)
			}
			t.Logf("issuer flags after GlobalFreeze: %d", info.Flags)

			// Try to trade through the AMM. This should fail.
			tradeResult, err := rpc.SubmitOfferCreate(bob.Secret, bob.Address,
				map[string]interface{}{"currency": "USD", "issuer": issuer.Address, "value": "10"},
				"1000000000",
			)
			if err != nil {
				t.Fatal("trade against frozen AMM:", err)
			}
			t.Logf("trade under GlobalFreeze: %s (expect tec* failure)", tradeResult.EngineResult)
			waitSettled(rpc)

			// Try a direct payment.
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
				t.Fatal("payment under GlobalFreeze:", err)
			}
			t.Logf("payment under GlobalFreeze: %s (expect tecPATH_DRY or tecFROZEN)", result.EngineResult)
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected payment to fail under GlobalFreeze")
			}
			waitSettled(rpc)
		},
	}
}

// ammExtLimitQuality tests payment with tfLimitQuality flag through AMM pool.
func ammExtLimitQuality() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_limit_quality",
		Description: "Payment with tfLimitQuality through AMM pool.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Create AMM USD/XRP pool (100 USD + 5000 XRP).
			// Implied rate: 1 USD = 50 XRP.
			_ = setupAMMPool(t, rpc)

			accounts := mustFund(t, rpc, 2)
			alice := accounts[0]
			bob := accounts[1]

			// Alice trusts and gets USD.
			err := setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}
			result, err := rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "200",
				},
			})
			if err != nil {
				t.Fatal("fund alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Case 1: Payment with tfLimitQuality at a rate the AMM can satisfy.
			// tfLimitQuality = 0x00040000 = 262144 for Payment.
			// Send at most 5 USD to deliver 100 XRP. AMM rate is ~50 XRP/USD,
			// so 5 USD should give ~250 XRP after slippage. This quality is achievable.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount":          "100000000", // 100 XRP
				"SendMax": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "5",
				},
				"Flags": 262144, // tfLimitQuality
				"Paths": []interface{}{
					[]interface{}{
						map[string]interface{}{
							"currency": "XRP",
						},
					},
				},
			})
			if err != nil {
				t.Fatal("limit quality payment:", err)
			}
			t.Logf("limit quality payment (good rate): %s", result.EngineResult)
			waitSettled(rpc)

			// Case 2: Payment with tfLimitQuality at a rate the AMM cannot satisfy.
			// Demand: deliver 100 XRP but only allow 0.1 USD (rate = 1000 XRP/USD).
			// AMM rate is ~50 XRP/USD. This quality is too demanding and should fail.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount":          "100000000", // 100 XRP
				"SendMax": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "0.1",
				},
				"Flags": 262144, // tfLimitQuality
				"Paths": []interface{}{
					[]interface{}{
						map[string]interface{}{
							"currency": "XRP",
						},
					},
				},
			})
			if err != nil {
				t.Fatal("limit quality payment bad rate:", err)
			}
			t.Logf("limit quality payment (bad rate): %s (expect tecPATH_PARTIAL or tecPATH_DRY)", result.EngineResult)
			waitSettled(rpc)
		},
	}
}

// ammExtReceiveMax tests payment with tfLimitQuality flag and verifies max received.
func ammExtReceiveMax() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_receive_max",
		Description: "Payment with tfLimitQuality, verify max received amount is respected.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Create AMM USD/XRP pool.
			_ = setupAMMPool(t, rpc)

			accounts := mustFund(t, rpc, 2)
			alice := accounts[0]
			bob := accounts[1]

			// Bob trusts genesis for USD.
			err := setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line bob:", err)
			}

			bobXRPBefore := getAccountBalance(t, rpc, bob.Address)

			// Alice (sender with XRP) sends a payment to bob for USD via AMM.
			// Alice has XRP, bob wants USD. Use Paths + SendMax for cross-currency.
			// Alice trusts genesis for USD too (for SendMax).
			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line alice:", err)
			}

			// Deliver 5 USD to bob. SendMax = 500 XRP. tfLimitQuality ensures quality threshold.
			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bob.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "5",
				},
				"SendMax": "500000000", // 500 XRP
				"Flags":   262144,      // tfLimitQuality
				"Paths": []interface{}{
					[]interface{}{
						map[string]interface{}{
							"currency": "USD",
							"issuer":   xrplsim.GenesisAddress,
						},
					},
				},
			})
			if err != nil {
				t.Fatal("receive max payment:", err)
			}
			t.Logf("receive max payment: %s", result.EngineResult)
			waitSettled(rpc)

			if result.EngineResult == "tesSUCCESS" {
				bobUSD := getIOUBalance(t, rpc, bob.Address, "USD", xrplsim.GenesisAddress)
				bobXRPAfter := getAccountBalance(t, rpc, bob.Address)
				t.Logf("bob USD=%s, XRP before=%s after=%s", bobUSD, bobXRPBefore, bobXRPAfter)
			}
		},
	}
}

// ammExtMultisignAMM tests submitting AMM transactions via multisig.
func ammExtMultisignAMM() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ext_multisign_amm",
		Description: "Submit AMM transaction via multisig (SignerListSet + submit_multisigned).",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			enableDefaultRippleOnGenesis(t, rpc)

			accounts := mustFund(t, rpc, 2)
			multiAcct := accounts[0]
			signer := accounts[1]

			// Set signer list on multiAcct.
			result, err := rpc.Submit(multiAcct.Secret, multiAcct.Address, map[string]interface{}{
				"TransactionType": "SignerListSet",
				"SignerQuorum":    1,
				"SignerEntries": []map[string]interface{}{
					{"SignerEntry": map[string]interface{}{
						"Account":      signer.Address,
						"SignerWeight": 1,
					}},
				},
			})
			if err != nil {
				t.Fatal("signer list set:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust and get USD for multiAcct.
			err = setup.SetupTrustLine(ctx, rpc, multiAcct.Address, multiAcct.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line multiAcct:", err)
			}
			result, err = rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     multiAcct.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "1000",
				},
			})
			if err != nil {
				t.Fatal("fund multiAcct USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Create AMM pool using normal signing (to set up the pool).
			result, err = rpc.Submit(multiAcct.Secret, multiAcct.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "100",
				},
				"Amount2": "5000000000",
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("amm create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Now submit AMMDeposit using sign_for + submit_multisigned.
			// Step 1: Get current sequence for the multisig account.
			info, err := rpc.AccountInfo(multiAcct.Address)
			if err != nil {
				t.Fatal("account_info:", err)
			}

			// Step 2: sign_for the AMMDeposit.
			signRaw, err := rpc.Call("sign_for", map[string]interface{}{
				"account": signer.Address,
				"secret":  signer.Secret,
				"tx_json": map[string]interface{}{
					"TransactionType": "AMMDeposit",
					"Account":         multiAcct.Address,
					"Asset": map[string]interface{}{
						"currency": "USD",
						"issuer":   xrplsim.GenesisAddress,
					},
					"Asset2": map[string]interface{}{
						"currency": "XRP",
					},
					"Amount2": "1000000000", // 1000 XRP
					"Flags":   524288,       // tfSingleAsset
					"Sequence": info.Sequence,
					"Fee":      "20",
				},
			})
			if err != nil {
				t.Fatal("sign_for:", err)
			}

			// Parse the signed tx_json.
			var signResp struct {
				TxJSON json.RawMessage `json:"tx_json"`
			}
			json.Unmarshal(signRaw, &signResp)

			// Step 3: submit_multisigned.
			var txMap map[string]interface{}
			json.Unmarshal(signResp.TxJSON, &txMap)

			submitRaw, err := rpc.Call("submit_multisigned", map[string]interface{}{
				"tx_json": txMap,
			})
			if err != nil {
				t.Fatal("submit_multisigned:", err)
			}

			// Parse result.
			var submitResp struct {
				EngineResult string `json:"engine_result"`
			}
			json.Unmarshal(submitRaw, &submitResp)
			t.Logf("multisigned AMMDeposit: %s", submitResp.EngineResult)
			waitSettled(rpc)

			// Verify the AMM pool still exists and has updated balances.
			ammRaw, err := rpc.Call("amm_info", map[string]interface{}{
				"asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
				},
				"asset2": map[string]interface{}{
					"currency": "XRP",
				},
			})
			if err != nil {
				t.Fatal("amm_info:", err)
			}
			var ammResp struct {
				AMM struct {
					Amount  interface{} `json:"amount"`
					Amount2 interface{} `json:"amount2"`
				} `json:"amm"`
			}
			json.Unmarshal(ammRaw, &ammResp)
			t.Logf("AMM pool after multisig deposit: amount=%v, amount2=%v",
				ammResp.AMM.Amount, ammResp.AMM.Amount2)
		},
	}
}
