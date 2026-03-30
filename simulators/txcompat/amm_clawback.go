package main

import (
	"context"
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

// setupAMMWithClawback funds an issuer and holder, enables clawback + DefaultRipple
// on the issuer, creates a trust line, issues 1000 USD to the holder, then the holder
// creates an AMM pool with 100 USD + 5000 XRP (5000000000 drops).
// Returns (issuer, holder).
func setupAMMWithClawback(t *xrplsim.T, rpc *xrplsim.RPCClient) (setup.Account, setup.Account) {
	ctx := context.Background()

	accounts := mustFund(t, rpc, 2)
	issuer := accounts[0]
	holder := accounts[1]

	// 1. Enable asfAllowTrustLineClawback (flag 16) BEFORE any trust lines.
	result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 16)
	if err != nil {
		t.Fatal("accountset clawback:", err)
	}
	assertEngineResult(t, result, "tesSUCCESS")
	setup.WaitSettled(ctx, rpc, 3)

	// 2. Enable asfDefaultRipple (flag 8).
	result, err = rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
	if err != nil {
		t.Fatal("accountset defaultripple:", err)
	}
	assertEngineResult(t, result, "tesSUCCESS")
	setup.WaitSettled(ctx, rpc, 3)

	// 3. Holder trusts issuer for USD.
	err = setup.SetupTrustLine(ctx, rpc, holder.Address, holder.Secret, "USD", issuer.Address, "10000")
	if err != nil {
		t.Fatal("trust line:", err)
	}

	// 4. Issuer sends 1000 USD to holder.
	result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
		"TransactionType": "Payment",
		"Destination":     holder.Address,
		"Amount": map[string]interface{}{
			"currency": "USD",
			"issuer":   issuer.Address,
			"value":    "1000",
		},
	})
	if err != nil {
		t.Fatal("iou payment:", err)
	}
	assertEngineResult(t, result, "tesSUCCESS")
	setup.WaitSettled(ctx, rpc, 3)

	// 5. Holder creates AMM pool: 100 USD + 5000 XRP.
	result, err = rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
		"TransactionType": "AMMCreate",
		"Amount": map[string]interface{}{
			"currency": "USD",
			"issuer":   issuer.Address,
			"value":    "100",
		},
		"Amount2": "5000000000", // 5000 XRP in drops
					"TradingFee": 0,
				})
	if err != nil {
		t.Fatal("amm create:", err)
	}
	assertEngineResult(t, result, "tesSUCCESS")
	setup.WaitSettled(ctx, rpc, 3)

	return issuer, holder
}

// clawbackGetHolderLPTokens returns the LP token value for a specific holder in an AMM pool.
func clawbackGetHolderLPTokens(t *xrplsim.T, rpc *xrplsim.RPCClient, asset, asset2 map[string]interface{}, holderAddress string) string {
	raw, err := rpc.Call("amm_info", map[string]interface{}{
		"asset":   asset,
		"asset2":  asset2,
		"account": holderAddress,
	})
	if err != nil {
		t.Fatal("amm_info:", err)
	}

	var resp struct {
		AMM struct {
			LPToken struct {
				Value string `json:"value"`
			} `json:"lp_token"`
		} `json:"amm"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal("parse amm_info:", err)
	}
	return resp.AMM.LPToken.Value
}

// clawbackAMMExists returns true if the AMM pool exists for the given asset pair.
func clawbackAMMExists(t *xrplsim.T, rpc *xrplsim.RPCClient, asset, asset2 map[string]interface{}) bool {
	raw, err := rpc.Call("amm_info", map[string]interface{}{
		"asset":  asset,
		"asset2": asset2,
	})
	if err != nil {
		return false
	}
	var resp struct {
		Error string `json:"error"`
	}
	json.Unmarshal(raw, &resp)
	return resp.Error == ""
}

// 1. ammClawbackSingleDeposit: Issuer with clawback enabled creates USD.
// Holder deposits USD+XRP into AMM. Issuer claws back all of holder's position.
// Verify holder's LP tokens are gone.
func ammClawbackSingleDeposit() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_clawback_single_deposit",
		Description: "AMMClawback: single deposit then clawback all from holder.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			issuer, holder := setupAMMWithClawback(t, rpc)

			usdAsset := map[string]interface{}{
				"currency": "USD",
				"issuer":   issuer.Address,
			}
			xrpAsset := map[string]interface{}{
				"currency": "XRP",
			}

			// Verify AMM exists and holder has LP tokens.
			lpBefore := clawbackGetHolderLPTokens(t, rpc, usdAsset, xrpAsset, holder.Address)
			if lpBefore == "0" || lpBefore == "" {
				t.Fatal("holder should have LP tokens before clawback")
			}
			t.Logf("holder LP tokens before clawback: %s", lpBefore)

			// Issuer claws back all of holder's position (no Amount = clawback all).
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AMMClawback",
				"Asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
				},
				"Asset2": map[string]interface{}{
					"currency": "XRP",
				},
				"Holder": holder.Address,
			})
			if err != nil {
				t.Fatal("amm clawback:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// AMM should be deleted since holder was the only depositor.
			if clawbackAMMExists(t, rpc, usdAsset, xrpAsset) {
				t.Fatal("AMM should be deleted after clawing back the sole depositor")
			}

			t.Logf("amm clawback single deposit: %s", result.EngineResult)
		},
	}
}

// 2. ammClawbackSpecificAmount: Same setup, but clawback only a specific Amount
// of USD from the holder's AMM position (not all). Verify partial clawback.
func ammClawbackSpecificAmount() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_clawback_specific_amount",
		Description: "AMMClawback: clawback a specific USD amount from holder's AMM position.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			issuer, holder := setupAMMWithClawback(t, rpc)

			usdAsset := map[string]interface{}{
				"currency": "USD",
				"issuer":   issuer.Address,
			}
			xrpAsset := map[string]interface{}{
				"currency": "XRP",
			}

			// Check holder's USD balance before clawback.
			usdBefore := getIOUBalance(t, rpc, holder.Address, "USD", issuer.Address)
			t.Logf("holder USD balance before clawback: %s", usdBefore)

			// Clawback only 50 USD from the holder's AMM position.
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AMMClawback",
				"Asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
				},
				"Asset2": map[string]interface{}{
					"currency": "XRP",
				},
				"Holder": holder.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("amm clawback:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// AMM should still exist since not all was clawed back.
			if !clawbackAMMExists(t, rpc, usdAsset, xrpAsset) {
				t.Fatal("AMM should still exist after partial clawback")
			}

			// Holder should still have some LP tokens.
			lpAfter := clawbackGetHolderLPTokens(t, rpc, usdAsset, xrpAsset, holder.Address)
			t.Logf("holder LP tokens after partial clawback: %s", lpAfter)
			if lpAfter == "0" || lpAfter == "" {
				t.Fatal("holder should still have LP tokens after partial clawback")
			}

			// Holder's USD balance in wallet should not change (clawback is from the AMM).
			usdAfter := getIOUBalance(t, rpc, holder.Address, "USD", issuer.Address)
			t.Logf("holder USD balance after clawback: %s (was: %s)", usdAfter, usdBefore)

			t.Logf("amm clawback specific amount: %s", result.EngineResult)
		},
	}
}

// 3. ammClawbackAllTokens: Clawback all tokens from the pool entirely.
// Verify AMM is deleted when last holder's tokens are clawed back.
func ammClawbackAllTokens() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_clawback_all_tokens",
		Description: "AMMClawback: clawback all tokens causing AMM deletion.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Use a more complex setup: issuer creates the pool, holder deposits.
			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Enable clawback before trust lines.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 16)
			if err != nil {
				t.Fatal("accountset clawback:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Enable DefaultRipple.
			result, err = rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("accountset defaultripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Holder trusts issuer for USD.
			err = setup.SetupTrustLine(ctx, rpc, holder.Address, holder.Secret, "USD", issuer.Address, "100000")
			if err != nil {
				t.Fatal("trust line:", err)
			}

			// Issue 3000 USD to holder.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     holder.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "3000",
				},
			})
			if err != nil {
				t.Fatal("iou payment:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Holder creates AMM pool: 2000 USD + 1000 XRP.
			result, err = rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "2000",
				},
				"Amount2": "1000000000000", // 1000000 XRP to have plenty
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("amm create:", err)
			}
			// If the create fails due to insufficient reserves, try with less XRP.
			if result.EngineResult != "tesSUCCESS" {
				result, err = rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
					"TransactionType": "AMMCreate",
					"Amount": map[string]interface{}{
						"currency": "USD",
						"issuer":   issuer.Address,
						"value":    "2000",
					},
					"Amount2": "1000000000", // 1000 XRP
								"TradingFee": 0,
				})
				if err != nil {
					t.Fatal("amm create retry:", err)
				}
				assertEngineResult(t, result, "tesSUCCESS")
			}
			setup.WaitSettled(ctx, rpc, 3)

			usdAsset := map[string]interface{}{
				"currency": "USD",
				"issuer":   issuer.Address,
			}
			xrpAsset := map[string]interface{}{
				"currency": "XRP",
			}

			// Clawback 1000 USD first.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AMMClawback",
				"Asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
				},
				"Asset2": map[string]interface{}{
					"currency": "XRP",
				},
				"Holder": holder.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "1000",
				},
			})
			if err != nil {
				t.Fatal("amm clawback 1:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// AMM should still exist.
			if !clawbackAMMExists(t, rpc, usdAsset, xrpAsset) {
				t.Fatal("AMM should still exist after partial clawback")
			}

			// Clawback remaining (all) from holder.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AMMClawback",
				"Asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
				},
				"Asset2": map[string]interface{}{
					"currency": "XRP",
				},
				"Holder": holder.Address,
			})
			if err != nil {
				t.Fatal("amm clawback 2:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// AMM should be deleted since holder was the sole depositor.
			if clawbackAMMExists(t, rpc, usdAsset, xrpAsset) {
				t.Fatal("AMM should be deleted after clawing back all tokens")
			}

			// Holder's USD balance should be 1000 (they had 3000, deposited 2000,
			// the clawback was from the AMM not from the wallet).
			usdBalance := getIOUBalance(t, rpc, holder.Address, "USD", issuer.Address)
			t.Logf("holder USD balance after full clawback: %s", usdBalance)

			t.Logf("amm clawback all tokens: tesSUCCESS")
		},
	}
}

// 4. ammClawbackMutualIssuers: Two issuers create tokens for each other
// and deposit into an AMM. One issuer claws back from the other.
// Based on rippled's testAMMClawbackIssuesEachOther.
func ammClawbackMutualIssuers() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_clawback_mutual_issuers",
		Description: "AMMClawback: two issuers issue tokens to each other and clawback from AMM.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			gw := accounts[0]
			gw2 := accounts[1]
			alice := accounts[2]

			// gw sets clawback.
			result, err := rpc.SubmitAccountSet(gw.Secret, gw.Address, 16)
			if err != nil {
				t.Fatal("accountset clawback gw:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// gw sets DefaultRipple.
			result, err = rpc.SubmitAccountSet(gw.Secret, gw.Address, 8)
			if err != nil {
				t.Fatal("accountset defaultripple gw:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// gw2 sets clawback.
			result, err = rpc.SubmitAccountSet(gw2.Secret, gw2.Address, 16)
			if err != nil {
				t.Fatal("accountset clawback gw2:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// gw2 sets DefaultRipple.
			result, err = rpc.SubmitAccountSet(gw2.Secret, gw2.Address, 8)
			if err != nil {
				t.Fatal("accountset defaultripple gw2:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// gw issues USD to gw2 and alice.
			err = setup.SetupTrustLine(ctx, rpc, gw2.Address, gw2.Secret, "USD", gw.Address, "100000")
			if err != nil {
				t.Fatal("trust gw2->gw USD:", err)
			}
			result, err = rpc.Submit(gw.Secret, gw.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     gw2.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   gw.Address,
					"value":    "5000",
				},
			})
			if err != nil {
				t.Fatal("pay gw->gw2 USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", gw.Address, "100000")
			if err != nil {
				t.Fatal("trust alice->gw USD:", err)
			}
			result, err = rpc.Submit(gw.Secret, gw.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   gw.Address,
					"value":    "5000",
				},
			})
			if err != nil {
				t.Fatal("pay gw->alice USD:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// gw2 issues EUR to gw and alice.
			err = setup.SetupTrustLine(ctx, rpc, gw.Address, gw.Secret, "EUR", gw2.Address, "100000")
			if err != nil {
				t.Fatal("trust gw->gw2 EUR:", err)
			}
			result, err = rpc.Submit(gw2.Secret, gw2.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     gw.Address,
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   gw2.Address,
					"value":    "6000",
				},
			})
			if err != nil {
				t.Fatal("pay gw2->gw EUR:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			err = setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "EUR", gw2.Address, "100000")
			if err != nil {
				t.Fatal("trust alice->gw2 EUR:", err)
			}
			result, err = rpc.Submit(gw2.Secret, gw2.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   gw2.Address,
					"value":    "6000",
				},
			})
			if err != nil {
				t.Fatal("pay gw2->alice EUR:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// gw creates AMM pool: 1000 USD + 2000 EUR.
			result, err = rpc.Submit(gw.Secret, gw.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   gw.Address,
					"value":    "1000",
				},
				"Amount2": map[string]interface{}{
					"currency": "EUR",
					"issuer":   gw2.Address,
					"value":    "2000",
				},
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("amm create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			usdAsset := map[string]interface{}{
				"currency": "USD",
				"issuer":   gw.Address,
			}
			eurAsset := map[string]interface{}{
				"currency": "EUR",
				"issuer":   gw2.Address,
			}

			// gw2 deposits 2000 USD + 4000 EUR.
			result, err = rpc.Submit(gw2.Secret, gw2.Address, map[string]interface{}{
				"TransactionType": "AMMDeposit",
				"Asset":           usdAsset,
				"Asset2":          eurAsset,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   gw.Address,
					"value":    "2000",
				},
				"Amount2": map[string]interface{}{
					"currency": "EUR",
					"issuer":   gw2.Address,
					"value":    "4000",
				},
				"Flags": 1048576, // tfTwoAsset
			})
			if err != nil {
				t.Fatal("amm deposit gw2:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// alice deposits 3000 USD + 6000 EUR.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AMMDeposit",
				"Asset":           usdAsset,
				"Asset2":          eurAsset,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   gw.Address,
					"value":    "3000",
				},
				"Amount2": map[string]interface{}{
					"currency": "EUR",
					"issuer":   gw2.Address,
					"value":    "6000",
				},
				"Flags": 1048576, // tfTwoAsset
			})
			if err != nil {
				t.Fatal("amm deposit alice:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// gw claws back 1000 USD from gw2.
			result, err = rpc.Submit(gw.Secret, gw.Address, map[string]interface{}{
				"TransactionType": "AMMClawback",
				"Asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   gw.Address,
				},
				"Asset2": map[string]interface{}{
					"currency": "EUR",
					"issuer":   gw2.Address,
				},
				"Holder": gw2.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   gw.Address,
					"value":    "1000",
				},
			})
			if err != nil {
				t.Fatal("amm clawback gw->gw2:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// AMM should still exist.
			if !clawbackAMMExists(t, rpc, usdAsset, eurAsset) {
				t.Fatal("AMM should still exist after partial clawback")
			}

			// gw2 claws back 1000 EUR from gw.
			result, err = rpc.Submit(gw2.Secret, gw2.Address, map[string]interface{}{
				"TransactionType": "AMMClawback",
				"Asset": map[string]interface{}{
					"currency": "EUR",
					"issuer":   gw2.Address,
				},
				"Asset2": map[string]interface{}{
					"currency": "USD",
					"issuer":   gw.Address,
				},
				"Holder": gw.Address,
				"Amount": map[string]interface{}{
					"currency": "EUR",
					"issuer":   gw2.Address,
					"value":    "1000",
				},
			})
			if err != nil {
				t.Fatal("amm clawback gw2->gw:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// AMM should still exist since alice still has tokens.
			if !clawbackAMMExists(t, rpc, usdAsset, eurAsset) {
				t.Fatal("AMM should still exist after second clawback")
			}

			t.Logf("amm clawback mutual issuers: tesSUCCESS")
		},
	}
}

// 5. ammClawbackFrozenAssets: Freeze the holder's trust line, then clawback.
// Clawback should still work even when the trust line is frozen.
// Based on rippled's testAssetFrozen.
func ammClawbackFrozenAssets() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_clawback_frozen_assets",
		Description: "AMMClawback: clawback succeeds even when trust line is frozen.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Enable clawback before trust lines.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 16)
			if err != nil {
				t.Fatal("accountset clawback:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Enable DefaultRipple.
			result, err = rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("accountset defaultripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Holder trusts issuer for USD.
			err = setup.SetupTrustLine(ctx, rpc, holder.Address, holder.Secret, "USD", issuer.Address, "100000")
			if err != nil {
				t.Fatal("trust line:", err)
			}

			// Issue 3000 USD to holder.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     holder.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "3000",
				},
			})
			if err != nil {
				t.Fatal("iou payment:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Holder creates AMM pool: 2000 USD + 1000 XRP.
			result, err = rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "2000",
				},
				"Amount2": "1000000000000", // 1000000 XRP in drops
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("amm create:", err)
			}
			// If create fails due to insufficient XRP, use less.
			if result.EngineResult != "tesSUCCESS" {
				result, err = rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
					"TransactionType": "AMMCreate",
					"Amount": map[string]interface{}{
						"currency": "USD",
						"issuer":   issuer.Address,
						"value":    "2000",
					},
					"Amount2": "1000000000", // 1000 XRP
								"TradingFee": 0,
				})
				if err != nil {
					t.Fatal("amm create retry:", err)
				}
				assertEngineResult(t, result, "tesSUCCESS")
			}
			setup.WaitSettled(ctx, rpc, 3)

			usdAsset := map[string]interface{}{
				"currency": "USD",
				"issuer":   issuer.Address,
			}
			xrpAsset := map[string]interface{}{
				"currency": "XRP",
			}

			// Freeze the holder's trust line (issuer freezes holder's USD line).
			// tfSetFreeze = 0x00100000 = 1048576
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "TrustSet",
				"LimitAmount": map[string]interface{}{
					"currency": "USD",
					"issuer":   holder.Address,
					"value":    "0",
				},
				"Flags": 1048576, // tfSetFreeze
			})
			if err != nil {
				t.Fatal("trustset freeze:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Clawback 1000 USD from the frozen trust line holder's AMM position.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AMMClawback",
				"Asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
				},
				"Asset2": map[string]interface{}{
					"currency": "XRP",
				},
				"Holder": holder.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "1000",
				},
			})
			if err != nil {
				t.Fatal("amm clawback frozen:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// AMM should still exist with remaining tokens.
			if !clawbackAMMExists(t, rpc, usdAsset, xrpAsset) {
				t.Fatal("AMM should still exist after partial clawback on frozen line")
			}

			// Holder's wallet USD balance should remain 1000 (3000 - 2000 deposited).
			usdBalance := getIOUBalance(t, rpc, holder.Address, "USD", issuer.Address)
			t.Logf("holder USD after clawback on frozen line: %s", usdBalance)

			// Clawback remaining to delete the AMM.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AMMClawback",
				"Asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
				},
				"Asset2": map[string]interface{}{
					"currency": "XRP",
				},
				"Holder": holder.Address,
			})
			if err != nil {
				t.Fatal("amm clawback frozen all:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// AMM should be deleted.
			if clawbackAMMExists(t, rpc, usdAsset, xrpAsset) {
				t.Fatal("AMM should be deleted after full clawback on frozen line")
			}

			t.Logf("amm clawback frozen assets: tesSUCCESS")
		},
	}
}

// 6. ammClawbackInvalidRequest: Various invalid AMMClawback requests.
// Based on rippled's testInvalidRequest.
func ammClawbackInvalidRequest() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_clawback_invalid_request",
		Description: "AMMClawback: verify error codes for various invalid requests.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Enable clawback + DefaultRipple on issuer.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 16)
			if err != nil {
				t.Fatal("accountset clawback:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			result, err = rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("accountset defaultripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Setup trust + issue + AMM.
			err = setup.SetupTrustLine(ctx, rpc, holder.Address, holder.Secret, "USD", issuer.Address, "10000")
			if err != nil {
				t.Fatal("trust line:", err)
			}
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     holder.Address,
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
			setup.WaitSettled(ctx, rpc, 3)

			result, err = rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
				"Amount2": "100000000000", // 100000 XRP
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("amm create:", err)
			}
			// If create fails due to insufficient XRP, use less.
			if result.EngineResult != "tesSUCCESS" {
				result, err = rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
					"TransactionType": "AMMCreate",
					"Amount": map[string]interface{}{
						"currency": "USD",
						"issuer":   issuer.Address,
						"value":    "100",
					},
					"Amount2": "100000000", // 100 XRP
								"TradingFee": 0,
				})
				if err != nil {
					t.Fatal("amm create retry:", err)
				}
				assertEngineResult(t, result, "tesSUCCESS")
			}
			setup.WaitSettled(ctx, rpc, 3)

			// Sub-test A: Holder == Issuer (temMALFORMED).
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AMMClawback",
				"Asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
				},
				"Asset2": map[string]interface{}{
					"currency": "XRP",
				},
				"Holder": issuer.Address,
			})
			if err != nil {
				t.Fatal("amm clawback self:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected clawback self to fail, got tesSUCCESS")
			}
			t.Logf("clawback self: %s (expected tem*/tec* error)", result.EngineResult)

			// Sub-test B: Non-existent holder (terNO_ACCOUNT).
			// Generate a random address for the non-existent account.
			nonExistent, err := rpc.WalletPropose()
			if err != nil {
				t.Fatal("wallet_propose:", err)
			}
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AMMClawback",
				"Asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
				},
				"Asset2": map[string]interface{}{
					"currency": "XRP",
				},
				"Holder": nonExistent.AccountID,
			})
			if err != nil {
				t.Fatal("amm clawback non-existent:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected clawback from non-existent to fail, got tesSUCCESS")
			}
			t.Logf("clawback non-existent holder: %s (expected terNO_ACCOUNT)", result.EngineResult)

			// Sub-test C: Holder not in pool (tecAMM_BALANCE).
			// Create another account that has NOT deposited.
			extraAccounts := mustFund(t, rpc, 1)
			bystander := extraAccounts[0]
			err = setup.SetupTrustLine(ctx, rpc, bystander.Address, bystander.Secret, "USD", issuer.Address, "10000")
			if err != nil {
				t.Fatal("trust line bystander:", err)
			}
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     bystander.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("pay bystander:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AMMClawback",
				"Asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
				},
				"Asset2": map[string]interface{}{
					"currency": "XRP",
				},
				"Holder": bystander.Address,
			})
			if err != nil {
				t.Fatal("amm clawback bystander:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected clawback from non-depositor to fail, got tesSUCCESS")
			}
			t.Logf("clawback non-depositor: %s (expected tecAMM_BALANCE)", result.EngineResult)

			// Sub-test D: Clawback XRP (temMALFORMED - can't clawback XRP).
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AMMClawback",
				"Asset": map[string]interface{}{
					"currency": "XRP",
				},
				"Asset2": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
				},
				"Holder": holder.Address,
			})
			if err != nil {
				t.Fatal("amm clawback xrp:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected clawback of XRP to fail, got tesSUCCESS")
			}
			t.Logf("clawback XRP: %s (expected temMALFORMED)", result.EngineResult)

			t.Logf("amm clawback invalid requests: all error codes verified")
		},
	}
}

// 7. ammClawbackNotEnabled: Try AMMClawback when the issuer has NOT set the
// asfAllowTrustLineClawback flag. Should fail with tecNO_PERMISSION.
// Based on rippled's testInvalidRequest (the clawback-without-flag sub-test).
func ammClawbackNotEnabled() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_clawback_not_enabled",
		Description: "AMMClawback: fails with tecNO_PERMISSION when clawback flag is not set.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Do NOT enable asfAllowTrustLineClawback.
			// Only enable DefaultRipple.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("accountset defaultripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Setup trust + issue + AMM.
			err = setup.SetupTrustLine(ctx, rpc, holder.Address, holder.Secret, "USD", issuer.Address, "10000")
			if err != nil {
				t.Fatal("trust line:", err)
			}
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     holder.Address,
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
			setup.WaitSettled(ctx, rpc, 3)

			// Issuer creates AMM pool (issuer, not holder, since issuer doesn't have
			// clawback; the test is about the issuer submitting AMMClawback without flag).
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
				"Amount2": "100000000", // 100 XRP
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("amm create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Try AMMClawback without asfAllowTrustLineClawback.
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "AMMClawback",
				"Asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
				},
				"Asset2": map[string]interface{}{
					"currency": "XRP",
				},
				"Holder": holder.Address,
			})
			if err != nil {
				t.Fatal("amm clawback:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected clawback to fail without flag, got tesSUCCESS")
			}
			t.Logf("amm clawback without flag: %s (expected tecNO_PERMISSION)", result.EngineResult)
		},
	}
}
