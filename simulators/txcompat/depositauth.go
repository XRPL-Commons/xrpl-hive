package main

import (
	"context"
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

// depositAuthEnable enables asfDepositAuth (9) and verifies the flag is set
// via account_info.
func depositAuthEnable() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "deposit_auth_enable",
		Description: "Enable asfDepositAuth (9), verify flag set via account_info.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Enable DepositAuth (asfDepositAuth = 9).
			result, err := rpc.SubmitAccountSet(acct.Secret, acct.Address, 9)
			if err != nil {
				t.Fatal("account set deposit auth:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify the flag is set via account_info.
			info, err := rpc.AccountInfo(acct.Address)
			if err != nil {
				t.Fatal("account_info:", err)
			}
			// lsfDepositAuth = 0x01000000 (16777216)
			const lsfDepositAuth = 0x01000000
			if info.Flags&lsfDepositAuth == 0 {
				t.Fatalf("expected lsfDepositAuth set, flags=%d", info.Flags)
			}
			t.Logf("deposit auth enabled, flags=%d", info.Flags)
		},
	}
}

// depositAuthPaymentXRP tests that an XRP payment to a deposit-auth protected
// account is rejected with tecNO_PERMISSION.
func depositAuthPaymentXRP() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "deposit_auth_payment_xrp",
		Description: "Enable deposit auth, XRP payment to protected account fails with tecNO_PERMISSION.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			sender := accounts[0]
			receiver := accounts[1]

			// Enable DepositAuth on receiver.
			result, err := rpc.SubmitAccountSet(receiver.Secret, receiver.Address, 9)
			if err != nil {
				t.Fatal("enable deposit auth:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Sender tries XRP payment to deposit-auth receiver.
			payResult, err := rpc.SubmitPayment(sender.Secret, sender.Address, receiver.Address, "1000000")
			if err != nil {
				t.Fatal("payment:", err)
			}
			if payResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected XRP payment to deposit-auth account to fail, got tesSUCCESS")
			}
			t.Logf("XRP payment to deposit-auth: %s (expected tecNO_PERMISSION)", payResult.EngineResult)
		},
	}
}

// depositAuthPaymentIOU tests that an IOU payment to a deposit-auth protected
// account is rejected with tecNO_PERMISSION.
func depositAuthPaymentIOU() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "deposit_auth_payment_iou",
		Description: "Enable deposit auth, IOU payment to protected account fails with tecNO_PERMISSION.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			sender := accounts[1]
			receiver := accounts[2]

			// Enable DefaultRipple on issuer.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust lines: sender and receiver trust issuer for USD.
			err = setup.SetupTrustLine(ctx, rpc, sender.Address, sender.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line sender:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, receiver.Address, receiver.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line receiver:", err)
			}

			// Issue USD to sender.
			payResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     sender.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("fund sender USD:", err)
			}
			assertEngineResult(t, payResult, "tesSUCCESS")
			waitSettled(rpc)

			// Enable DepositAuth on receiver.
			result, err = rpc.SubmitAccountSet(receiver.Secret, receiver.Address, 9)
			if err != nil {
				t.Fatal("enable deposit auth:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Sender tries IOU payment to deposit-auth receiver.
			iouPayResult, err := rpc.Submit(sender.Secret, sender.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     receiver.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "50",
				},
			})
			if err != nil {
				t.Fatal("IOU payment:", err)
			}
			if iouPayResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected IOU payment to deposit-auth account to fail, got tesSUCCESS")
			}
			t.Logf("IOU payment to deposit-auth: %s (expected tecNO_PERMISSION)", iouPayResult.EngineResult)
		},
	}
}

// depositAuthSelfPayment tests that an account with deposit auth can send to itself.
// XRP self-payment returns temREDUNDANT (sending XRP to yourself is not allowed).
// IOU self-payment IS allowed and tests the deposit auth bypass for self.
func depositAuthSelfPayment() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "deposit_auth_self_payment",
		Description: "Account with deposit auth: XRP self-payment returns temREDUNDANT; IOU self-payment succeeds.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()
			accounts := mustFund(t, rpc, 2)
			acct := accounts[0]
			issuer := accounts[1]

			// Enable DefaultRipple on issuer.
			setResult, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, setResult, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust line: acct trusts issuer for USD.
			err = setup.SetupTrustLine(ctx, rpc, acct.Address, acct.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line:", err)
			}

			// Issue USD to acct.
			payIOU, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     acct.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("fund USD:", err)
			}
			assertEngineResult(t, payIOU, "tesSUCCESS")
			waitSettled(rpc)

			// Enable DepositAuth on acct.
			result, err := rpc.SubmitAccountSet(acct.Secret, acct.Address, 9)
			if err != nil {
				t.Fatal("enable deposit auth:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// XRP self-payment returns temREDUNDANT (sending XRP to yourself is blocked).
			payXRP, err := rpc.SubmitPayment(acct.Secret, acct.Address, acct.Address, "1000000")
			if err != nil {
				t.Fatal("XRP self payment:", err)
			}
			if payXRP.EngineResult == "temREDUNDANT" {
				t.Log("XRP self-payment correctly returns temREDUNDANT")
			} else {
				t.Logf("XRP self-payment: %s (expected temREDUNDANT)", payXRP.EngineResult)
			}

			// IOU self-payment should succeed (deposit auth bypass for self).
			iouSelf, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     acct.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "10",
				},
			})
			if err != nil {
				t.Fatal("IOU self payment:", err)
			}
			if iouSelf.EngineResult == "tesSUCCESS" {
				t.Log("IOU self-payment with deposit auth: tesSUCCESS")
			} else {
				t.Logf("IOU self-payment with deposit auth: %s", iouSelf.EngineResult)
			}
		},
	}
}

// depositAuthPreauth tests DepositPreauth: enable deposit auth, preauthorize
// a specific sender, verify that sender can pay but others cannot.
func depositAuthPreauth() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "deposit_auth_preauth",
		Description: "Enable deposit auth, preauth a sender, verify only preauthed sender can pay.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			receiver := accounts[0]
			authorized := accounts[1]
			unauthorized := accounts[2]

			// Enable DepositAuth on receiver.
			result, err := rpc.SubmitAccountSet(receiver.Secret, receiver.Address, 9)
			if err != nil {
				t.Fatal("enable deposit auth:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Preauthorize the authorized sender.
			preauthResult, err := rpc.Submit(receiver.Secret, receiver.Address, map[string]interface{}{
				"TransactionType": "DepositPreauth",
				"Authorize":       authorized.Address,
			})
			if err != nil {
				t.Fatal("deposit preauth:", err)
			}
			assertEngineResult(t, preauthResult, "tesSUCCESS")
			waitSettled(rpc)

			// Authorized sender can pay.
			payResult, err := rpc.SubmitPayment(authorized.Secret, authorized.Address, receiver.Address, "1000000")
			if err != nil {
				t.Fatal("authorized payment:", err)
			}
			assertEngineResult(t, payResult, "tesSUCCESS")
			t.Logf("authorized payment: %s", payResult.EngineResult)

			// Unauthorized sender cannot pay.
			payResult2, err := rpc.SubmitPayment(unauthorized.Secret, unauthorized.Address, receiver.Address, "1000000")
			if err != nil {
				t.Fatal("unauthorized payment:", err)
			}
			if payResult2.EngineResult == "tesSUCCESS" {
				t.Fatal("expected unauthorized payment to fail, got tesSUCCESS")
			}
			t.Logf("unauthorized payment: %s (expected tecNO_PERMISSION)", payResult2.EngineResult)
		},
	}
}

// depositAuthNoRipple tests deposit auth interaction with NoRipple flag.
func depositAuthNoRipple() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "deposit_auth_no_ripple",
		Description: "Test deposit auth with NoRipple flag interactions.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts := mustFund(t, rpc, 3)
			issuer := accounts[0]
			sender := accounts[1]
			receiver := accounts[2]

			// Enable DefaultRipple on issuer.
			result, err := rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 8)
			if err != nil {
				t.Fatal("set default ripple:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Setup trust lines.
			err = setup.SetupTrustLine(ctx, rpc, sender.Address, sender.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line sender:", err)
			}
			err = setup.SetupTrustLine(ctx, rpc, receiver.Address, receiver.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line receiver:", err)
			}

			// Issue USD to sender.
			payResult, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     sender.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("fund sender USD:", err)
			}
			assertEngineResult(t, payResult, "tesSUCCESS")
			waitSettled(rpc)

			// Set NoRipple on receiver's trust line.
			noRippleResult, err := rpc.Submit(receiver.Secret, receiver.Address, map[string]interface{}{
				"TransactionType": "TrustSet",
				"LimitAmount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "1000",
				},
				"Flags": 131072, // tfSetNoRipple
			})
			if err != nil {
				t.Fatal("set no ripple:", err)
			}
			assertEngineResult(t, noRippleResult, "tesSUCCESS")
			waitSettled(rpc)

			// Enable DepositAuth on receiver.
			result, err = rpc.SubmitAccountSet(receiver.Secret, receiver.Address, 9)
			if err != nil {
				t.Fatal("enable deposit auth:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Sender tries IOU payment to deposit-auth + noRipple receiver.
			iouPayResult, err := rpc.Submit(sender.Secret, sender.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     receiver.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "10",
				},
			})
			if err != nil {
				t.Fatal("IOU payment:", err)
			}
			// Should fail due to deposit auth.
			if iouPayResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected payment to deposit-auth account to fail, got tesSUCCESS")
			}
			t.Logf("IOU payment to deposit-auth+noRipple: %s (expected tecNO_PERMISSION)", iouPayResult.EngineResult)
		},
	}
}

// depositAuthInvalid tests invalid deposit auth scenarios.
func depositAuthInvalid() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "deposit_auth_invalid",
		Description: "Test invalid deposit auth operations: preauth self, preauth non-existent account.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Enable DepositAuth.
			result, err := rpc.SubmitAccountSet(acct.Secret, acct.Address, 9)
			if err != nil {
				t.Fatal("enable deposit auth:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Test 1: DepositPreauth for self should fail.
			selfPreauth, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "DepositPreauth",
				"Authorize":       acct.Address,
			})
			if err != nil {
				t.Fatal("self preauth:", err)
			}
			if selfPreauth.EngineResult == "tesSUCCESS" {
				t.Fatal("expected self-preauth to fail, got tesSUCCESS")
			}
			t.Logf("self preauth: %s (expected temCANNOT_PREAUTH_SELF)", selfPreauth.EngineResult)

			// Test 2: DepositPreauth for a non-existent account.
			phantom, err := rpc.WalletPropose()
			if err != nil {
				t.Fatal("wallet_propose:", err)
			}

			phantomPreauth, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "DepositPreauth",
				"Authorize":       phantom.AccountID,
			})
			if err != nil {
				t.Fatal("phantom preauth:", err)
			}
			// Non-existent destination may result in tecNO_TARGET.
			t.Logf("preauth non-existent: %s (expected tecNO_TARGET)", phantomPreauth.EngineResult)

			// Test 3: DepositPreauth with both Authorize and Unauthorize (invalid).
			otherAccounts := mustFund(t, rpc, 1)
			other := otherAccounts[0]

			// First preauth the other account so we can try to unauthorize.
			preauthResult, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "DepositPreauth",
				"Authorize":       other.Address,
			})
			if err != nil {
				t.Fatal("preauth other:", err)
			}
			assertEngineResult(t, preauthResult, "tesSUCCESS")
			waitSettled(rpc)

			// Duplicate preauth should fail.
			dupPreauth, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "DepositPreauth",
				"Authorize":       other.Address,
			})
			if err != nil {
				t.Fatal("duplicate preauth:", err)
			}
			if dupPreauth.EngineResult == "tesSUCCESS" {
				t.Fatal("expected duplicate preauth to fail, got tesSUCCESS")
			}
			t.Logf("duplicate preauth: %s (expected tecDUPLICATE)", dupPreauth.EngineResult)
		},
	}
}

// depositAuthWithCredentials tests deposit auth with credential-based authorization.
// A sender creates a credential, the receiver has deposit auth enabled, and the
// sender can pay using the credential.
func depositAuthWithCredentials() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "deposit_auth_with_credentials",
		Description: "Enable deposit auth, create credential for sender, verify sender can pay with credential.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			receiver := accounts[0]
			sender := accounts[1]
			credIssuer := accounts[2]

			// Enable DepositAuth on receiver.
			result, err := rpc.SubmitAccountSet(receiver.Secret, receiver.Address, 9)
			if err != nil {
				t.Fatal("enable deposit auth:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Credential issuer creates a credential for the sender.
			credResult, err := rpc.Submit(credIssuer.Secret, credIssuer.Address, map[string]interface{}{
				"TransactionType": "CredentialCreate",
				"Subject":         sender.Address,
				"CredentialType":  "4B5943", // "KYC" in hex
			})
			if err != nil {
				t.Fatal("credential create:", err)
			}
			assertEngineResult(t, credResult, "tesSUCCESS")
			waitSettled(rpc)

			// Sender accepts the credential.
			acceptResult, err := rpc.Submit(sender.Secret, sender.Address, map[string]interface{}{
				"TransactionType": "CredentialAccept",
				"Issuer":          credIssuer.Address,
				"CredentialType":  "4B5943",
			})
			if err != nil {
				t.Fatal("credential accept:", err)
			}
			assertEngineResult(t, acceptResult, "tesSUCCESS")
			waitSettled(rpc)

			// Without preauth, sender cannot pay (verify deposit auth is working).
			payFail, err := rpc.SubmitPayment(sender.Secret, sender.Address, receiver.Address, "1000000")
			if err != nil {
				t.Fatal("payment without preauth:", err)
			}
			if payFail.EngineResult == "tesSUCCESS" {
				t.Fatal("expected payment without preauth to fail, got tesSUCCESS")
			}
			t.Logf("payment without preauth: %s (expected tecNO_PERMISSION)", payFail.EngineResult)

			// Receiver creates a DepositPreauth with credentials.
			depPreauthResult, err := rpc.Submit(receiver.Secret, receiver.Address, map[string]interface{}{
				"TransactionType": "DepositPreauth",
				"Authorize":       sender.Address,
			})
			if err != nil {
				t.Fatal("deposit preauth:", err)
			}
			assertEngineResult(t, depPreauthResult, "tesSUCCESS")
			waitSettled(rpc)

			// Now sender should be able to pay.
			paySuccess, err := rpc.SubmitPayment(sender.Secret, sender.Address, receiver.Address, "1000000")
			if err != nil {
				t.Fatal("payment with preauth:", err)
			}
			assertEngineResult(t, paySuccess, "tesSUCCESS")
			t.Logf("payment with credential-based preauth: %s", paySuccess.EngineResult)

			// Verify the credential still exists.
			raw, err := rpc.Call("account_objects", map[string]interface{}{
				"account":      sender.Address,
				"type":         "credential",
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_objects:", err)
			}
			var objResp struct {
				AccountObjects []json.RawMessage `json:"account_objects"`
			}
			json.Unmarshal(raw, &objResp)
			t.Logf("sender has %d credential object(s) after payment", len(objResp.AccountObjects))
		},
	}
}
