package main

import (
	"context"
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

// setupAMMPool creates an AMM pool for USD/XRP and returns the account used.
func setupAMMPool(t *xrplsim.T, rpc *xrplsim.RPCClient) setup.Account {
	ctx := context.Background()

	// Enable DefaultRipple on genesis (USD issuer).
	enableDefaultRippleOnGenesis(t, rpc)

	accounts := mustFund(t, rpc, 1)
	acct := accounts[0]

	// Trust genesis for USD.
	err := setup.SetupTrustLine(ctx, rpc, acct.Address, acct.Secret, "USD", xrplsim.GenesisAddress, "10000")
	if err != nil {
		t.Fatal("trust line:", err)
	}

	// Get some USD.
	_, err = rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
		"TransactionType": "Payment",
		"Destination":     acct.Address,
		"Amount": map[string]interface{}{
			"currency": "USD",
			"issuer":   xrplsim.GenesisAddress,
			"value":    "1000",
		},
	})
	if err != nil {
		t.Fatal("iou payment:", err)
	}
	setup.WaitSettled(ctx, rpc, 3)

	// AMMCreate.
	result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
		"TransactionType": "AMMCreate",
		"Amount": map[string]interface{}{
			"currency": "USD",
			"issuer":   xrplsim.GenesisAddress,
			"value":    "100",
		},
		"Amount2":   "5000000000", // 5000 XRP
		"TradingFee": 0,
	})
	if err != nil {
		t.Fatal("amm create:", err)
	}
	assertEngineResult(t, result, "tesSUCCESS")
	setup.WaitSettled(ctx, rpc, 3)

	return acct
}

func ammCreateAndDeposit() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_create_and_deposit",
		Description: "Create AMM pool and deposit additional liquidity.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := setupAMMPool(t, rpc)

			// Deposit more XRP into the pool.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMDeposit",
				"Asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
				},
				"Asset2": map[string]interface{}{
					"currency": "XRP",
				},
				"Amount2": "1000000000", // 1000 XRP
				"Flags":   524288,       // tfSingleAsset
			})
			if err != nil {
				t.Fatal("amm deposit:", err)
			}
			t.Logf("amm deposit: %s", result.EngineResult)
		},
	}
}

func ammWithdraw() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_withdraw",
		Description: "Create AMM pool and withdraw liquidity.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := setupAMMPool(t, rpc)

			// Get AMM info to find LP token.
			raw, err := rpc.Call("amm_info", map[string]interface{}{
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
					LPToken struct {
						Currency string `json:"currency"`
						Issuer   string `json:"issuer"`
						Value    string `json:"value"`
					} `json:"lp_token"`
				} `json:"amm"`
			}
			json.Unmarshal(raw, &ammResp)

			// Withdraw some LP tokens.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMWithdraw",
				"Asset": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
				},
				"Asset2": map[string]interface{}{
					"currency": "XRP",
				},
				"Amount": "500000000", // withdraw 500 XRP (single asset)
				"Flags":  524288,      // tfSingleAsset
			})
			if err != nil {
				t.Fatal("amm withdraw:", err)
			}
			t.Logf("amm withdraw: %s", result.EngineResult)
		},
	}
}
