package main

import (
	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func setRegularKey() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "set_regular_key",
		Description: "Set a regular key on an account, then submit a Payment signed with it.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			acct := accounts[0]
			dest := accounts[1]

			// Generate a new key pair for the regular key.
			regularKey, err := rpc.WalletPropose()
			if err != nil {
				t.Fatal("wallet_propose for regular key:", err)
			}

			// Set the regular key on the account.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "SetRegularKey",
				"RegularKey":      regularKey.AccountID,
			})
			if err != nil {
				t.Fatal("set regular key:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Submit a Payment signed with the regular key to a funded dest.
			payResult, err := rpc.Submit(regularKey.MasterSeed, acct.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     dest.Address,
				"Amount":          "1000000",
			})
			if err != nil {
				t.Fatal("payment with regular key:", err)
			}
			assertEngineResult(t, payResult, "tesSUCCESS")
			t.Logf("payment with regular key: %s", payResult.EngineResult)
		},
	}
}

func revokeRegularKey() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "revoke_regular_key",
		Description: "Set a regular key, then remove it and verify the old key no longer works.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			acct, dest := accounts[0], accounts[1]

			regularKey, err := rpc.WalletPropose()
			if err != nil {
				t.Fatal("wallet_propose:", err)
			}

			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "SetRegularKey",
				"RegularKey":      regularKey.AccountID,
			})
			if err != nil {
				t.Fatal("set regular key:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			revokeResult, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "SetRegularKey",
			})
			if err != nil {
				t.Fatal("revoke regular key:", err)
			}
			assertEngineResult(t, revokeResult, "tesSUCCESS")
			waitSettled(rpc)

			payResult, err := rpc.Submit(regularKey.MasterSeed, acct.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     dest.Address,
				"Amount":          "1000000",
			})
			if err != nil {
				t.Fatal("payment with revoked key:", err)
			}
			if payResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected payment with revoked regular key to fail, got tesSUCCESS")
			}
			t.Logf("payment with revoked key: %s (expected tefBAD_AUTH or similar)", payResult.EngineResult)
		},
	}
}

func disableMasterKey() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "disable_master_key",
		Description: "Set regular key, disable master key, verify master fails but regular works.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Generate and set a regular key.
			regularKey, err := rpc.WalletPropose()
			if err != nil {
				t.Fatal("wallet_propose:", err)
			}

			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "SetRegularKey",
				"RegularKey":      regularKey.AccountID,
			})
			if err != nil {
				t.Fatal("set regular key:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Disable master key via AccountSet SetFlag=4 (asfDisableMaster).
			disableResult, err := rpc.SubmitAccountSet(acct.Secret, acct.Address, 4)
			if err != nil {
				t.Fatal("disable master:", err)
			}
			assertEngineResult(t, disableResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify the flag is set.
			info, err := rpc.AccountInfo(acct.Address)
			if err != nil {
				t.Fatal("account_info:", err)
			}
			t.Logf("flags after disable master: %d", info.Flags)

			accounts2 := mustFund(t, rpc, 1)
			dest := accounts2[0]

			// Payment with master key should fail.
			masterPayResult, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     dest.Address,
				"Amount":          "1000000",
			})
			if err != nil {
				t.Fatal("payment with disabled master:", err)
			}
			if masterPayResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected payment with disabled master key to fail, got tesSUCCESS")
			}
			t.Logf("payment with disabled master: %s (expected tefMASTER_DISABLED)", masterPayResult.EngineResult)

			// Payment with regular key should succeed.
			regPayResult, err := rpc.Submit(regularKey.MasterSeed, acct.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     dest.Address,
				"Amount":          "1000000",
			})
			if err != nil {
				t.Fatal("payment with regular key:", err)
			}
			assertEngineResult(t, regPayResult, "tesSUCCESS")
			t.Logf("payment with regular key (master disabled): %s", regPayResult.EngineResult)
		},
	}
}

func reEnableMasterKey() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "re_enable_master_key",
		Description: "Disable master key then re-enable it, verify master key works again.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Generate and set a regular key.
			regularKey, err := rpc.WalletPropose()
			if err != nil {
				t.Fatal("wallet_propose:", err)
			}

			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "SetRegularKey",
				"RegularKey":      regularKey.AccountID,
			})
			if err != nil {
				t.Fatal("set regular key:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Disable master key.
			disableResult, err := rpc.SubmitAccountSet(acct.Secret, acct.Address, 4)
			if err != nil {
				t.Fatal("disable master:", err)
			}
			assertEngineResult(t, disableResult, "tesSUCCESS")
			waitSettled(rpc)

			// Re-enable master key using the regular key (ClearFlag=4).
			reEnableResult, err := rpc.Submit(regularKey.MasterSeed, acct.Address, map[string]interface{}{
				"TransactionType": "AccountSet",
				"ClearFlag":       4,
			})
			if err != nil {
				t.Fatal("re-enable master:", err)
			}
			assertEngineResult(t, reEnableResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify master key works again.
			accounts2 := mustFund(t, rpc, 1)
			dest := accounts2[0]

			masterPayResult, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     dest.Address,
				"Amount":          "1000000",
			})
			if err != nil {
				t.Fatal("payment with re-enabled master:", err)
			}
			assertEngineResult(t, masterPayResult, "tesSUCCESS")
			t.Logf("payment with re-enabled master key: %s", masterPayResult.EngineResult)
		},
	}
}

func regularKeyWithTicket() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "regular_key_with_ticket",
		Description: "Set a regular key using a TicketSequence.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Get current sequence for ticket calculation.
			info, err := rpc.AccountInfo(acct.Address)
			if err != nil {
				t.Fatal("account_info:", err)
			}

			// Create a ticket.
			ticketResult, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "TicketCreate",
				"TicketCount":     1,
			})
			if err != nil {
				t.Fatal("ticket create:", err)
			}
			assertEngineResult(t, ticketResult, "tesSUCCESS")
			waitSettled(rpc)

			ticketSeq := info.Sequence + 1

			// Generate a new key pair for the regular key.
			regularKey, err := rpc.WalletPropose()
			if err != nil {
				t.Fatal("wallet_propose:", err)
			}

			// Set regular key using the ticket.
			setResult, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "SetRegularKey",
				"RegularKey":      regularKey.AccountID,
				"Sequence":        0,
				"TicketSequence":  ticketSeq,
			})
			if err != nil {
				t.Fatal("set regular key with ticket:", err)
			}
			assertEngineResult(t, setResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify the regular key works by sending a Payment.
			accounts2 := mustFund(t, rpc, 1)
			dest := accounts2[0]

			payResult, err := rpc.Submit(regularKey.MasterSeed, acct.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     dest.Address,
				"Amount":          "1000000",
			})
			if err != nil {
				t.Fatal("payment with regular key:", err)
			}
			assertEngineResult(t, payResult, "tesSUCCESS")
			t.Logf("payment with regular key (set via ticket): %s", payResult.EngineResult)
		},
	}
}
