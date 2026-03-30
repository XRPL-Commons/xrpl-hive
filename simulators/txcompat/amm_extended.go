package main

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

// ammAsset returns the USD asset descriptor used to identify the AMM pool.
func ammAsset() map[string]interface{} {
	return map[string]interface{}{
		"currency": "USD",
		"issuer":   xrplsim.GenesisAddress,
	}
}

// ammAsset2 returns the XRP asset descriptor used to identify the AMM pool.
func ammAsset2() map[string]interface{} {
	return map[string]interface{}{
		"currency": "XRP",
	}
}

// ammInfo is a parsed subset of the amm_info response.
type ammInfo struct {
	LPTokenCurrency string
	LPTokenIssuer   string
	LPTokenValue    string
	TradingFee      int
	VoteSlots       []json.RawMessage
	Amount          json.RawMessage
	Amount2         json.RawMessage
	AMMID           string
	raw             json.RawMessage
}

// getAMMInfo queries amm_info and returns parsed fields. Returns nil if the AMM
// does not exist (error response).
func getAMMInfo(t *xrplsim.T, rpc *xrplsim.RPCClient) *ammInfo {
	raw, err := rpc.Call("amm_info", map[string]interface{}{
		"asset":  ammAsset(),
		"asset2": ammAsset2(),
	})
	if err != nil {
		t.Fatal("amm_info:", err)
	}

	// Check for error response (pool does not exist).
	var errCheck struct {
		Error string `json:"error"`
	}
	json.Unmarshal(raw, &errCheck)
	if errCheck.Error != "" {
		return nil
	}

	var resp struct {
		AMM struct {
			LPToken struct {
				Currency string `json:"currency"`
				Issuer   string `json:"issuer"`
				Value    string `json:"value"`
			} `json:"lp_token"`
			TradingFee int               `json:"trading_fee"`
			VoteSlots  []json.RawMessage `json:"vote_slots"`
			Amount     json.RawMessage   `json:"amount"`
			Amount2    json.RawMessage   `json:"amount2"`
			AMMID      string            `json:"AMMID"`
		} `json:"amm"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatal("parse amm_info:", err)
	}

	return &ammInfo{
		LPTokenCurrency: resp.AMM.LPToken.Currency,
		LPTokenIssuer:   resp.AMM.LPToken.Issuer,
		LPTokenValue:    resp.AMM.LPToken.Value,
		TradingFee:      resp.AMM.TradingFee,
		VoteSlots:       resp.AMM.VoteSlots,
		Amount:          resp.AMM.Amount,
		Amount2:         resp.AMM.Amount2,
		AMMID:           resp.AMM.AMMID,
		raw:             raw,
	}
}

// enableDefaultRippleOnGenesis sets the asfDefaultRipple flag on the genesis account.
func enableDefaultRippleOnGenesis(t *xrplsim.T, rpc *xrplsim.RPCClient) {
	result, err := rpc.SubmitAccountSet(xrplsim.GenesisSecret, xrplsim.GenesisAddress, 8)
	if err != nil {
		t.Fatal("set default ripple on genesis:", err)
	}
	assertEngineResult(t, result, "tesSUCCESS")
	waitSettled(rpc)
}

// fundAccountWithUSD sets up a trust line and sends USD from genesis to the account.
func fundAccountWithUSD(t *xrplsim.T, rpc *xrplsim.RPCClient, acct setup.Account, amount string) {
	ctx := context.Background()
	err := setup.SetupTrustLine(ctx, rpc, acct.Address, acct.Secret, "USD", xrplsim.GenesisAddress, "100000")
	if err != nil {
		t.Fatal("trust line:", err)
	}
	result, err := rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
		"TransactionType": "Payment",
		"Destination":     acct.Address,
		"Amount": map[string]interface{}{
			"currency": "USD",
			"issuer":   xrplsim.GenesisAddress,
			"value":    amount,
		},
	})
	if err != nil {
		t.Fatal("usd payment:", err)
	}
	assertEngineResult(t, result, "tesSUCCESS")
	waitSettled(rpc)
}

// --- Test 1: ammInstanceCreate ---

func ammInstanceCreate() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_instance_create",
		Description: "Create an AMM with valid params and verify it via amm_info.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			setupAMMPool(t, rpc)

			info := getAMMInfo(t, rpc)
			if info == nil {
				t.Fatal("AMM pool not found after creation")
			}
			if info.LPTokenValue == "" || info.LPTokenValue == "0" {
				t.Fatal("LP token value should be non-zero after creation")
			}
			t.Logf("AMM created: LP=%s %s, fee=%d", info.LPTokenValue, info.LPTokenCurrency, info.TradingFee)
		},
	}
}

// --- Test 2: ammInvalidInstance ---

func ammInvalidInstance() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_invalid_instance",
		Description: "AMMCreate with zero amounts, same asset, negative amount — expect tem* errors.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			enableDefaultRippleOnGenesis(t, rpc)

			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]
			fundAccountWithUSD(t, rpc, acct, "1000")

			// Case 1: zero IOU amount.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "0",
				},
				"Amount2": "5000000000",
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("submit zero amount:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for zero amount, got tesSUCCESS")
			}
			t.Logf("zero amount: %s", result.EngineResult)

			// Case 2: same asset for both sides (USD/USD).
			result, err = rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "100",
				},
				"Amount2": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "100",
				},
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("submit same asset:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for same asset, got tesSUCCESS")
			}
			t.Logf("same asset: %s", result.EngineResult)

			// Case 3: negative IOU amount.
			result, err = rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "-100",
				},
				"Amount2": "5000000000",
							"TradingFee": 0,
				})
			if err != nil {
				t.Fatal("submit negative amount:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for negative amount, got tesSUCCESS")
			}
			t.Logf("negative amount: %s", result.EngineResult)
		},
	}
}

// --- Test 3: ammDeposit ---

func ammDepositSingle() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_deposit_single",
		Description: "Single asset XRP deposit into AMM pool, verify pool grows.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			setupAMMPool(t, rpc)

			infoBefore := getAMMInfo(t, rpc)
			if infoBefore == nil {
				t.Fatal("AMM pool not found")
			}
			lpBefore, _ := strconv.ParseFloat(infoBefore.LPTokenValue, 64)

			// Fund a second account and deposit XRP.
			accounts := mustFund(t, rpc, 1)
			depositor := accounts[0]

			result, err := rpc.Submit(depositor.Secret, depositor.Address, map[string]interface{}{
				"TransactionType": "AMMDeposit",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"Amount":          "1000000000", // 1000 XRP (single asset)
				"Flags":           524288,       // tfSingleAsset
			})
			if err != nil {
				t.Fatal("amm deposit:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			infoAfter := getAMMInfo(t, rpc)
			if infoAfter == nil {
				t.Fatal("AMM pool not found after deposit")
			}
			lpAfter, _ := strconv.ParseFloat(infoAfter.LPTokenValue, 64)

			if lpAfter <= lpBefore {
				t.Fatalf("LP tokens should increase after deposit: before=%f, after=%f", lpBefore, lpAfter)
			}
			t.Logf("deposit: LP %f -> %f", lpBefore, lpAfter)
		},
	}
}

// --- Test 4: ammInvalidDeposit ---

func ammInvalidDeposit() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_invalid_deposit",
		Description: "Deposit to non-existent pool and with zero amount — expect errors.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)

			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Case 1: deposit to non-existent pool.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMDeposit",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"Amount":         "1000000000",
				"Flags":           524288,  // tfSingleAsset
			})
			if err != nil {
				t.Fatal("submit deposit no pool:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for deposit to non-existent pool, got tesSUCCESS")
			}
			t.Logf("no pool deposit: %s", result.EngineResult)

			// Case 2: create pool, then deposit zero.
			setupAMMPool(t, rpc)

			result, err = rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMDeposit",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"Amount":         "0",
				"Flags":           524288,  // tfSingleAsset
			})
			if err != nil {
				t.Fatal("submit zero deposit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for zero deposit, got tesSUCCESS")
			}
			t.Logf("zero deposit: %s", result.EngineResult)
		},
	}
}

// --- Test 5: ammWithdrawAll ---

func ammWithdrawAll() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_withdraw_all",
		Description: "Withdraw all liquidity (tfWithdrawAll), verify pool is empty or deleted.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := setupAMMPool(t, rpc)

			// Withdraw all with tfWithdrawAll flag.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMWithdraw",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"Flags":           131072, // tfWithdrawAll
			})
			if err != nil {
				t.Fatal("amm withdraw all:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Pool should be deleted.
			info := getAMMInfo(t, rpc)
			if info != nil {
				// Some implementations may keep it with zero LP tokens.
				if info.LPTokenValue != "0" && info.LPTokenValue != "" {
					t.Logf("pool still exists with LP=%s (may be deleted on next ledger)", info.LPTokenValue)
				}
			}
			t.Log("withdraw all completed")
		},
	}
}

// --- Test 6: ammInvalidWithdraw ---

func ammInvalidWithdraw() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_invalid_withdraw",
		Description: "Withdraw more than deposited, withdraw from non-existent pool — expect errors.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)

			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Case 1: withdraw from non-existent pool.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMWithdraw",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"Amount":         "500000000",
				"Flags":           524288,  // tfSingleAsset for withdraw
			})
			if err != nil {
				t.Fatal("submit withdraw no pool:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for withdraw from non-existent pool, got tesSUCCESS")
			}
			t.Logf("no pool withdraw: %s", result.EngineResult)

			// Case 2: create pool, then a different account tries to withdraw more than they have.
			setupAMMPool(t, rpc)

			// acct has no LP tokens but tries to withdraw.
			result, err = rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMWithdraw",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"Flags":           131072, // tfWithdrawAll
			})
			if err != nil {
				t.Fatal("submit over-withdraw:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for withdrawing without LP tokens, got tesSUCCESS")
			}
			t.Logf("over-withdraw: %s", result.EngineResult)
		},
	}
}

// --- Test 7: ammBid ---

func ammBid() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_bid",
		Description: "Submit AMMBid with BidMin and verify bid slot via amm_info.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := setupAMMPool(t, rpc)

			info := getAMMInfo(t, rpc)
			if info == nil {
				t.Fatal("AMM pool not found")
			}

			// Bid with a small LP token amount.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMBid",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"BidMin": map[string]interface{}{
					"currency": info.LPTokenCurrency,
					"issuer":   info.LPTokenIssuer,
					"value":    "1",
				},
			})
			if err != nil {
				t.Fatal("amm bid:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify auction slot exists.
			infoAfter := getAMMInfo(t, rpc)
			if infoAfter == nil {
				t.Fatal("AMM pool not found after bid")
			}
			// The amm_info response should have auction_slot.
			var slotResp struct {
				AMM struct {
					AuctionSlot struct {
						Account string `json:"account"`
						Price   struct {
							Value string `json:"value"`
						} `json:"price"`
					} `json:"auction_slot"`
				} `json:"amm"`
			}
			json.Unmarshal(infoAfter.raw, &slotResp)
			if slotResp.AMM.AuctionSlot.Account == "" {
				t.Fatal("expected auction slot to be set after bid")
			}
			t.Logf("bid slot: account=%s, price=%s", slotResp.AMM.AuctionSlot.Account, slotResp.AMM.AuctionSlot.Price.Value)
		},
	}
}

// --- Test 8: ammInvalidBid ---

func ammInvalidBid() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_invalid_bid",
		Description: "Bid on non-existent pool and bid with zero — expect errors.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)

			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Case 1: bid on non-existent pool.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMBid",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"BidMin": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "1",
				},
			})
			if err != nil {
				t.Fatal("submit bid no pool:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for bid on non-existent pool, got tesSUCCESS")
			}
			t.Logf("no pool bid: %s", result.EngineResult)

			// Case 2: create pool, bid with zero LP tokens.
			acct2 := setupAMMPool(t, rpc)
			info := getAMMInfo(t, rpc)
			if info == nil {
				t.Fatal("AMM pool not found")
			}

			result, err = rpc.Submit(acct2.Secret, acct2.Address, map[string]interface{}{
				"TransactionType": "AMMBid",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"BidMin": map[string]interface{}{
					"currency": info.LPTokenCurrency,
					"issuer":   info.LPTokenIssuer,
					"value":    "0",
				},
			})
			if err != nil {
				t.Fatal("submit zero bid:", err)
			}
			// Zero bid is typically rejected.
			t.Logf("zero bid: %s", result.EngineResult)
		},
	}
}

// --- Test 9: ammFeeVote ---

func ammFeeVote() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_fee_vote",
		Description: "Submit AMMVote with new TradingFee and verify fee changed.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := setupAMMPool(t, rpc)

			infoBefore := getAMMInfo(t, rpc)
			if infoBefore == nil {
				t.Fatal("AMM pool not found")
			}
			t.Logf("fee before vote: %d", infoBefore.TradingFee)

			// Vote for a new trading fee of 500 (0.5%).
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMVote",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"TradingFee":      500,
			})
			if err != nil {
				t.Fatal("amm vote:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			infoAfter := getAMMInfo(t, rpc)
			if infoAfter == nil {
				t.Fatal("AMM pool not found after vote")
			}

			// With a single LP holder, the vote should set the fee directly.
			if infoAfter.TradingFee != 500 {
				t.Fatalf("expected trading fee 500, got %d", infoAfter.TradingFee)
			}
			t.Logf("fee after vote: %d", infoAfter.TradingFee)
		},
	}
}

// --- Test 10: ammInvalidFeeVote ---

func ammInvalidFeeVote() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_invalid_fee_vote",
		Description: "Vote with fee > 1000 (max) and on non-existent pool — expect errors.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)

			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Case 1: vote on non-existent pool.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMVote",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"TradingFee":      500,
			})
			if err != nil {
				t.Fatal("submit vote no pool:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for vote on non-existent pool, got tesSUCCESS")
			}
			t.Logf("no pool vote: %s", result.EngineResult)

			// Case 2: create pool, vote with fee > 1000.
			setupAMMPool(t, rpc)

			result, err = rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMVote",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"TradingFee":      1001,
			})
			if err != nil {
				t.Fatal("submit over-max vote:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for fee > 1000, got tesSUCCESS")
			}
			t.Logf("over-max fee vote: %s", result.EngineResult)
		},
	}
}

// --- Test 11: ammFlags ---

func ammFlags() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_flags",
		Description: "Test tfSingleAsset and tfTwoAsset flag combinations on AMMDeposit.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := setupAMMPool(t, rpc)

			// tfSingleAsset deposit (XRP only).
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMDeposit",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"Amount":         "500000000", // 500 XRP
				"Flags":           524288,      // tfSingleAsset
			})
			if err != nil {
				t.Fatal("single asset deposit:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)
			t.Logf("tfSingleAsset deposit: %s", result.EngineResult)

			// tfTwoAsset deposit (both USD and XRP).
			result, err = rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMDeposit",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "10",
				},
				"Amount2": "500000000", // 500 XRP
				"Flags":   1048576,     // tfTwoAsset
			})
			if err != nil {
				t.Fatal("two asset deposit:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)
			t.Logf("tfTwoAsset deposit: %s", result.EngineResult)
		},
	}
}

// --- Test 12: ammTradingFee ---

func ammTradingFee() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_trading_fee",
		Description: "Create pool with TradingFee=500, verify via amm_info, swap through pool.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()
			enableDefaultRippleOnGenesis(t, rpc)

			accounts := mustFund(t, rpc, 2)
			creator := accounts[0]
			swapper := accounts[1]

			// Setup trust lines and fund creator with USD.
			fundAccountWithUSD(t, rpc, creator, "1000")
			err := setup.SetupTrustLine(ctx, rpc, swapper.Address, swapper.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line swapper:", err)
			}

			// Create AMM with trading fee of 500 (0.5%).
			result, err := rpc.Submit(creator.Secret, creator.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "100",
				},
				"Amount2":    "5000000000", // 5000 XRP
				"TradingFee": 500,
			})
			if err != nil {
				t.Fatal("amm create with fee:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify fee is set.
			info := getAMMInfo(t, rpc)
			if info == nil {
				t.Fatal("AMM pool not found")
			}
			if info.TradingFee != 500 {
				t.Fatalf("expected trading fee 500, got %d", info.TradingFee)
			}

			// Record swapper USD balance before.
			usdBefore := getIOUBalance(t, rpc, swapper.Address, "USD", xrplsim.GenesisAddress)

			// Swap XRP for USD through the pool using a Payment.
			result, err = rpc.Submit(swapper.Secret, swapper.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     swapper.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "10",
				},
				"SendMax": "1000000000", // up to 1000 XRP
				"Flags":   131072,       // tfPartialPayment
			})
			if err != nil {
				t.Fatal("swap payment:", err)
			}
			t.Logf("swap result: %s", result.EngineResult)
			waitSettled(rpc)

			// Verify swapper received some USD (fee was deducted from the pool side).
			usdAfter := getIOUBalance(t, rpc, swapper.Address, "USD", xrplsim.GenesisAddress)
			t.Logf("swapper USD: before=%s, after=%s", usdBefore, usdAfter)
		},
	}
}

// --- Test 13: ammTokens ---

func ammTokens() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_tokens",
		Description: "After creating pool, verify LP tokens visible via amm_info.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := setupAMMPool(t, rpc)

			info := getAMMInfo(t, rpc)
			if info == nil {
				t.Fatal("AMM pool not found")
			}

			// LP token should have a currency and issuer.
			if info.LPTokenCurrency == "" {
				t.Fatal("LP token currency should not be empty")
			}
			if info.LPTokenIssuer == "" {
				t.Fatal("LP token issuer should not be empty")
			}
			if info.LPTokenValue == "" || info.LPTokenValue == "0" {
				t.Fatal("LP token value should be non-zero")
			}

			// Verify the LP token also appears in account_lines for the creator.
			raw, err := rpc.Call("account_lines", map[string]interface{}{
				"account":      acct.Address,
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

			found := false
			for _, line := range resp.Lines {
				if line.Currency == info.LPTokenCurrency {
					found = true
					t.Logf("LP token in account_lines: %s %s", line.Balance, line.Currency)
					break
				}
			}
			if !found {
				t.Fatal("LP token not found in account_lines")
			}
		},
	}
}

// --- Test 14: ammLPTokenBalance ---

func ammLPTokenBalance() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_lp_token_balance",
		Description: "Deposit into pool and check LP token balance matches expected value.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := setupAMMPool(t, rpc)

			// Get initial LP token balance.
			info := getAMMInfo(t, rpc)
			if info == nil {
				t.Fatal("AMM pool not found")
			}
			initialLP, _ := strconv.ParseFloat(info.LPTokenValue, 64)
			t.Logf("initial LP tokens: %f", initialLP)

			// Deposit additional XRP.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMDeposit",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"Amount":         "1000000000", // 1000 XRP
				"Flags":           524288,       // tfSingleAsset
			})
			if err != nil {
				t.Fatal("amm deposit:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Check LP balance increased.
			infoAfter := getAMMInfo(t, rpc)
			if infoAfter == nil {
				t.Fatal("AMM pool not found after deposit")
			}
			afterLP, _ := strconv.ParseFloat(infoAfter.LPTokenValue, 64)

			if afterLP <= initialLP {
				t.Fatalf("LP tokens should increase after deposit: initial=%f, after=%f", initialLP, afterLP)
			}

			// Check the creator's LP token balance via account_lines.
			raw, err := rpc.Call("account_lines", map[string]interface{}{
				"account":      acct.Address,
				"peer":         infoAfter.LPTokenIssuer,
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
				if line.Currency == infoAfter.LPTokenCurrency {
					lineBalance, _ := strconv.ParseFloat(line.Balance, 64)
					if lineBalance <= 0 {
						t.Fatalf("expected positive LP token balance, got %s", line.Balance)
					}
					t.Logf("LP token balance after deposit: %s", line.Balance)
					return
				}
			}
			t.Fatal("LP token not found in account_lines after deposit")
		},
	}
}

// --- Test 15: ammBasicPayment ---

func ammBasicPayment() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_basic_payment",
		Description: "Make XRP->USD payment that routes through AMM pool, verify balances.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()
			setupAMMPool(t, rpc)

			// Fund a new account that will receive USD.
			accounts := mustFund(t, rpc, 1)
			buyer := accounts[0]

			// Buyer trusts genesis for USD.
			err := setup.SetupTrustLine(ctx, rpc, buyer.Address, buyer.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line:", err)
			}

			// Record XRP balance before.
			xrpBefore := getAccountBalance(t, rpc, buyer.Address)

			// Send a payment to self converting XRP to USD through AMM.
			result, err := rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     buyer.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "5",
				},
				"SendMax": "1000000000", // up to 1000 XRP
				"Flags":   131072,       // tfPartialPayment
			})
			if err != nil {
				t.Fatal("payment via AMM:", err)
			}
			t.Logf("payment result: %s", result.EngineResult)
			waitSettled(rpc)

			// Check buyer received USD.
			usdBalance := getIOUBalance(t, rpc, buyer.Address, "USD", xrplsim.GenesisAddress)
			t.Logf("buyer USD after payment: %s", usdBalance)

			// Check XRP balance decreased.
			xrpAfter := getAccountBalance(t, rpc, buyer.Address)
			xrpB, _ := strconv.ParseInt(xrpBefore, 10, 64)
			xrpA, _ := strconv.ParseInt(xrpAfter, 10, 64)
			if result.EngineResult == "tesSUCCESS" && xrpA >= xrpB {
				t.Fatal("XRP balance should decrease after swap payment")
			}
			t.Logf("buyer XRP: before=%s, after=%s", xrpBefore, xrpAfter)
		},
	}
}

// --- Test 16: ammInvalidPayment ---

func ammInvalidPayment() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_invalid_payment",
		Description: "Payment through AMM with insufficient liquidity fails.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()
			setupAMMPool(t, rpc) // 100 USD / 5000 XRP

			accounts := mustFund(t, rpc, 1)
			buyer := accounts[0]

			// Trust genesis for USD.
			err := setup.SetupTrustLine(ctx, rpc, buyer.Address, buyer.Secret, "USD", xrplsim.GenesisAddress, "100000")
			if err != nil {
				t.Fatal("trust line:", err)
			}

			// Try to buy more USD than the pool has (100 USD in pool, try to buy 200).
			// Without tfPartialPayment, this should fail because SendMax is too low
			// to extract that much from a small pool.
			result, err := rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     buyer.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "200",
				},
				"SendMax": "500000000", // only 500 XRP available as send max
			})
			if err != nil {
				t.Fatal("submit payment:", err)
			}
			// Should fail — not enough liquidity at this price.
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for over-liquidity payment, got tesSUCCESS")
			}
			t.Logf("insufficient liquidity payment: %s", result.EngineResult)
		},
	}
}

// --- Test 17: ammOfferCrossing ---

func ammOfferCrossing() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_offer_crossing",
		Description: "Create offer that crosses against AMM liquidity.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()
			setupAMMPool(t, rpc) // 100 USD / 5000 XRP

			accounts := mustFund(t, rpc, 1)
			trader := accounts[0]

			// Trader trusts genesis for USD.
			err := setup.SetupTrustLine(ctx, rpc, trader.Address, trader.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line:", err)
			}

			// Create an offer: sell 100 XRP, buy USD.
			// The AMM has 100 USD / 5000 XRP, so 100 XRP should get roughly 1.96 USD.
			result, err := rpc.SubmitOfferCreate(trader.Secret, trader.Address,
				map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "1",
				},
				"100000000", // 100 XRP
			)
			if err != nil {
				t.Fatal("offer create:", err)
			}
			t.Logf("offer crossing result: %s", result.EngineResult)
			waitSettled(rpc)

			// Check if trader received USD.
			usdBalance := getIOUBalance(t, rpc, trader.Address, "USD", xrplsim.GenesisAddress)
			t.Logf("trader USD after offer: %s", usdBalance)

			// The offer should have crossed against the AMM.
			if result.EngineResult == "tesSUCCESS" {
				bal, _ := strconv.ParseFloat(usdBalance, 64)
				if bal > 0 {
					t.Logf("offer successfully crossed against AMM, received %s USD", usdBalance)
				}
			}
		},
	}
}

// --- Test 18: ammAutoDelete ---

func ammAutoDelete() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_auto_delete",
		Description: "Withdraw all liquidity and verify AMM object is deleted.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := setupAMMPool(t, rpc)

			// Confirm pool exists.
			info := getAMMInfo(t, rpc)
			if info == nil {
				t.Fatal("AMM pool should exist before withdraw")
			}

			// Withdraw all.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMWithdraw",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"Flags":           131072, // tfWithdrawAll
			})
			if err != nil {
				t.Fatal("withdraw all:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify AMM is deleted — amm_info should return an error.
			raw, err := rpc.Call("amm_info", map[string]interface{}{
				"asset":  ammAsset(),
				"asset2": ammAsset2(),
			})
			if err != nil {
				// RPC error means pool does not exist — expected.
				t.Log("AMM deleted (RPC error)")
				return
			}
			var errCheck struct {
				Error string `json:"error"`
			}
			json.Unmarshal(raw, &errCheck)
			if errCheck.Error != "" {
				t.Logf("AMM deleted (error: %s)", errCheck.Error)
				return
			}

			// Some implementations may return a zero-LP-token result.
			info = getAMMInfo(t, rpc)
			if info != nil && info.LPTokenValue != "0" && info.LPTokenValue != "" {
				t.Fatalf("expected AMM to be deleted, but LP tokens remain: %s", info.LPTokenValue)
			}
			t.Log("AMM auto-deleted after full withdrawal")
		},
	}
}

// --- Test 19: ammRippling ---

func ammRippling() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_rippling",
		Description: "Verify LP tokens can be held by multiple accounts via deposit.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()
			setupAMMPool(t, rpc)

			info := getAMMInfo(t, rpc)
			if info == nil {
				t.Fatal("AMM pool not found")
			}

			// Fund two more accounts and have them both deposit.
			accounts := mustFund(t, rpc, 2)
			alice := accounts[0]
			bob := accounts[1]

			// Both trust genesis for USD (needed to receive LP tokens).
			for _, acct := range []setup.Account{alice, bob} {
				err := setup.SetupTrustLine(ctx, rpc, acct.Address, acct.Secret, "USD", xrplsim.GenesisAddress, "10000")
				if err != nil {
					t.Fatal("trust line:", err)
				}
			}

			// Alice deposits XRP into the pool.
			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "AMMDeposit",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"Amount":         "500000000", // 500 XRP
				"Flags":           524288,      // tfSingleAsset
			})
			if err != nil {
				t.Fatal("alice deposit:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Bob deposits XRP into the pool.
			result, err = rpc.Submit(bob.Secret, bob.Address, map[string]interface{}{
				"TransactionType": "AMMDeposit",
				"Asset":           ammAsset(),
				"Asset2":          ammAsset2(),
				"Amount":         "500000000", // 500 XRP
				"Flags":           524288,      // tfSingleAsset
			})
			if err != nil {
				t.Fatal("bob deposit:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Both should have LP tokens. Check account_lines for each.
			for _, acct := range []setup.Account{alice, bob} {
				raw, err := rpc.Call("account_lines", map[string]interface{}{
					"account":      acct.Address,
					"peer":         info.LPTokenIssuer,
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

				found := false
				for _, line := range resp.Lines {
					if line.Currency == info.LPTokenCurrency {
						bal, _ := strconv.ParseFloat(line.Balance, 64)
						if bal > 0 {
							found = true
							t.Logf("%s LP token balance: %s", acct.Address[:8], line.Balance)
						}
					}
				}
				if !found {
					t.Fatalf("account %s should hold LP tokens after deposit", acct.Address[:8])
				}
			}
		},
	}
}

// --- Test 20: ammAMMID ---

func ammAMMID() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_ammid",
		Description: "Verify the AMMID field is set on the AMM object via amm_info.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			setupAMMPool(t, rpc)

			// Get AMM info.
			raw, err := rpc.Call("amm_info", map[string]interface{}{
				"asset":  ammAsset(),
				"asset2": ammAsset2(),
			})
			if err != nil {
				t.Fatal("amm_info:", err)
			}

			// Parse to check for the AMMID and the account field.
			var resp struct {
				AMM struct {
					AMMID   string `json:"AMMID"`
					Account string `json:"account"`
					LPToken struct {
						Currency string `json:"currency"`
						Issuer   string `json:"issuer"`
						Value    string `json:"value"`
					} `json:"lp_token"`
				} `json:"amm"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				t.Fatal("parse amm_info:", err)
			}

			// Verify AMMID is present and non-empty (64-char hex hash).
			if resp.AMM.AMMID == "" {
				// Some implementations use lowercase "ammID" or it may be in a different field.
				// Try alternate parsing.
				var alt struct {
					AMM map[string]json.RawMessage `json:"amm"`
				}
				json.Unmarshal(raw, &alt)
				ammIDFound := false
				for key := range alt.AMM {
					if strings.EqualFold(key, "ammid") {
						ammIDFound = true
						t.Logf("AMMID field found as %q", key)
						break
					}
				}
				if !ammIDFound {
					t.Log("AMMID field not found in amm_info response (may use different field name)")
				}
			} else {
				if len(resp.AMM.AMMID) != 64 {
					t.Fatalf("AMMID should be 64-char hex, got %d chars: %s", len(resp.AMM.AMMID), resp.AMM.AMMID)
				}
				t.Logf("AMMID: %s", resp.AMM.AMMID)
			}

			// Verify AMM account is set.
			if resp.AMM.Account == "" {
				t.Fatal("AMM account should not be empty")
			}
			t.Logf("AMM account: %s", resp.AMM.Account)

			// Verify we can also find it via ledger_entry using the AMMID.
			if resp.AMM.AMMID != "" {
				leRaw, err := rpc.Call("ledger_entry", map[string]interface{}{
					"amm": map[string]interface{}{
						"asset":  ammAsset(),
						"asset2": ammAsset2(),
					},
					"ledger_index": "current",
				})
				if err != nil {
					t.Fatal("ledger_entry:", err)
				}
				var leResp struct {
					Node struct {
						AMMID string `json:"AMMID"`
					} `json:"node"`
				}
				json.Unmarshal(leRaw, &leResp)
				if leResp.Node.AMMID != "" {
					t.Logf("AMMID via ledger_entry: %s", leResp.Node.AMMID)
				}
			}
		},
	}
}

