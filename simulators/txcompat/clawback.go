package main

import (
	"context"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

func clawbackIOU() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "clawback_iou",
		Description: "Enable clawback flag, issue IOU, claw back from holder.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Enable clawback on issuer (asfAllowTrustLineClawback = 16).
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 16)
			if err != nil {
				t.Fatal("account set clawback:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			setup.WaitSettled(ctx, rpc, 3)

			// Create trust line and issue IOU.
			err = setup.SetupIOU(ctx, rpc,
				issuer.Address, issuer.Secret,
				holder.Address, holder.Secret,
				"USD", "100",
			)
			if err != nil {
				t.Fatal("setup IOU:", err)
			}

			// Claw back 50 USD.
			clawResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Clawback",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   holder.Address, // In clawback, issuer field is the holder.
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("clawback:", err)
			}
			assertEngineResult(t, clawResult, "tesSUCCESS")
			t.Logf("clawback: %s", clawResult.EngineResult)
		},
	}
}

func clawbackWithoutFlag() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "clawback_without_flag",
		Description: "Attempt clawback without enabling the flag — should fail.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			holder := accounts[1]

			// Do NOT enable clawback flag.
			// Create trust line and issue IOU.
			err := setup.SetupIOU(ctx, rpc,
				issuer.Address, issuer.Secret,
				holder.Address, holder.Secret,
				"USD", "100",
			)
			if err != nil {
				t.Fatal("setup IOU:", err)
			}

			// Attempt clawback — should fail.
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
				t.Fatal("expected clawback to fail without flag, got tesSUCCESS")
			}
			t.Logf("clawback without flag: %s (expected failure)", clawResult.EngineResult)
		},
	}
}
