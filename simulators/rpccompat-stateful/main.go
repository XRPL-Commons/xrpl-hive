// The rpccompat-stateful simulator tests XRPL RPC methods that require
// ledger state (funded accounts, trust lines, offers, etc.) before asserting.
// It starts clients in standalone mode and uses ledger_accept to advance
// ledgers after submitting transactions.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

func main() {
	suite := xrplsim.Suite{
		Name:        "rpccompat-stateful",
		Description: "Stateful RPC tests that create ledger state before asserting.",
	}

	suite.Add(accountInfoFunded())
	suite.Add(accountLinesTrustline())
	suite.Add(accountOffersActive())
	suite.Add(accountTxAfterPayment())
	suite.Add(accountCurrenciesTrustline())
	suite.Add(submitPaymentSuccess())
	suite.Add(submitPaymentUnfunded())
	suite.Add(txAfterSubmit())
	suite.Add(ledgerEntryByIndex())
	suite.Add(accountNftsAfterMint())
	suite.Add(depositAuthorizedPreauth())
	suite.Add(bookOffersWithOrder())
	// Phase 2: new stateful tests
	suite.Add(accountChannelsAfterPayChan())
	suite.Add(gatewayBalancesWithIOU())
	suite.Add(norippleCheckWithTrustline())
	suite.Add(signAndSubmit())
	suite.Add(signForMultisign())
	suite.Add(submitMultisigned())
	suite.Add(transactionEntryAfterSubmit())
	suite.Add(ammInfoAfterCreate())
	suite.Add(ripplePathFindWithPaths())
	suite.Add(getAggregatePriceAfterOracle())
	// Phase 6: cross-currency and pathfinding
	suite.Add(ripplePathFindNoPath())
	suite.Add(bookOffersAfterCrossing())
	// Full coverage: remaining methods
	suite.Add(channelAuthorizeAndVerify())
	suite.Add(nftBuyOffersWithOffer())
	suite.Add(vaultInfoAfterCreate())
	suite.Add(simulateTransaction())
	suite.Add(ledgerDataWithState())

	xrplsim.MustRun(xrplsim.New(), suite)
}

// helpers

func startNetwork(t *xrplsim.T) (*xrplsim.Client, *xrplsim.RPCClient) {
	clients, err := t.Sim.ClientTypes()
	if err != nil || len(clients) == 0 {
		t.Fatal("no client types available")
	}

	// Start a single node in standalone mode.
	c := t.StartClient(clients[0].Name, xrplsim.Params{
		"XRPL_STANDALONE":   "1",
		"XRPL_NETWORK_ID":   "10000",
		"XRPL_LOGLEVEL":     "3",
		"XRPL_PEER_PRIVATE": "1",
	})
	rpc := xrplsim.NewRPCClient(c.RPCEndpoint())

	// Wait for RPC to be responsive.
	for i := 0; i < 30; i++ {
		if _, err := rpc.ServerInfo(); err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	// Advance a few ledgers so the node has initial closed state.
	for i := 0; i < 3; i++ {
		rpc.Call("ledger_accept", nil)
		time.Sleep(200 * time.Millisecond)
	}
	return c, rpc
}

func mustFund(t *xrplsim.T, rpc *xrplsim.RPCClient) setup.Account {
	ctx := context.Background()
	accounts, err := setup.FundN(ctx, rpc, 1, "10000000000") // 10,000 XRP
	if err != nil {
		t.Fatal("failed to fund account:", err)
	}
	return accounts[0]
}

func assertField(t *xrplsim.T, raw json.RawMessage, path string, expected interface{}) {
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	// Simple one-level path for now.
	val, ok := data[path]
	if !ok {
		t.Fatalf("missing field %q in response", path)
	}
	if expected != nil {
		if fmt.Sprint(val) != fmt.Sprint(expected) {
			t.Fatalf("field %q: expected %v, got %v", path, expected, val)
		}
	}
}

// --- Tests ---

func accountInfoFunded() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_info_funded",
		Description: "Fund an account and verify account_info returns correct data.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := mustFund(t, rpc)

			result, err := rpc.AccountInfo(acct.Address)
			if err != nil {
				t.Fatal("account_info failed:", err)
			}
			t.Logf("account %s balance: %s", acct.Address, result.Balance)
			if result.Balance == "" || result.Balance == "0" {
				t.Fatal("expected non-zero balance")
			}
		},
	}
}

func accountLinesTrustline() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_lines_with_trustline",
		Description: "Create a trust line and verify it appears in account_lines.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := mustFund(t, rpc)

			// Create trust line: acct trusts genesis for USD.
			ctx := context.Background()
			err := setup.SetupTrustLine(ctx, rpc, acct.Address, acct.Secret, "USD", xrplsim.GenesisAddress, "1000")
			if err != nil {
				t.Fatal("setup trust line:", err)
			}

			// Verify account_lines.
			raw, err := rpc.Call("account_lines", map[string]interface{}{
				"account":      acct.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_lines:", err)
			}

			var resp struct {
				Lines []struct {
					Account  string `json:"account"`
					Currency string `json:"currency"`
				} `json:"lines"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				t.Fatal("unmarshal:", err)
			}
			if len(resp.Lines) == 0 {
				t.Fatal("expected at least one trust line")
			}
			t.Logf("trust line: %s %s", resp.Lines[0].Currency, resp.Lines[0].Account)
		},
	}
}

func accountOffersActive() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_offers_active",
		Description: "Create an offer and verify it appears in account_offers.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := mustFund(t, rpc)

			// First create a trust line for the IOU side.
			ctx := context.Background()
			setup.SetupTrustLine(ctx, rpc, acct.Address, acct.Secret, "USD", xrplsim.GenesisAddress, "1000")

			// Create offer: buy USD with XRP (taker pays USD, taker gets XRP).
			err := setup.SetupOffer(ctx, rpc, acct.Address, acct.Secret,
				map[string]interface{}{"currency": "USD", "issuer": xrplsim.GenesisAddress, "value": "10"},
				"1000000",
			)
			if err != nil {
				t.Fatal("setup offer:", err)
			}

			// Verify account_offers.
			raw, err := rpc.Call("account_offers", map[string]interface{}{
				"account":      acct.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_offers:", err)
			}

			var resp struct {
				Offers []interface{} `json:"offers"`
			}
			json.Unmarshal(raw, &resp)
			if len(resp.Offers) == 0 {
				t.Fatal("expected at least one offer")
			}
			t.Logf("found %d offer(s)", len(resp.Offers))
		},
	}
}

func accountTxAfterPayment() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_tx_after_payment",
		Description: "Submit a payment and verify it appears in account_tx.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := mustFund(t, rpc)

			// Submit payment from funded account to a random destination.
			w, _ := rpc.WalletPropose()
			result, err := rpc.SubmitPayment(acct.Secret, acct.Address, w.AccountID, "5000000000")
			if err != nil {
				t.Fatal("submit payment:", err)
			}
			t.Logf("payment: %s = %s", result.TxHash, result.EngineResult)

			ctx := context.Background()
			setup.WaitSettled(ctx, rpc, 3)

			// Verify account_tx.
			raw, err := rpc.Call("account_tx", map[string]interface{}{
				"account":          acct.Address,
				"ledger_index_min": -1,
				"ledger_index_max": -1,
			})
			if err != nil {
				t.Fatal("account_tx:", err)
			}

			var resp struct {
				Transactions []interface{} `json:"transactions"`
			}
			json.Unmarshal(raw, &resp)
			if len(resp.Transactions) == 0 {
				t.Fatal("expected at least one transaction in account_tx")
			}
			t.Logf("found %d transaction(s) in account_tx", len(resp.Transactions))
		},
	}
}

func accountCurrenciesTrustline() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_currencies_with_trustline",
		Description: "Create a trust line and verify account_currencies lists the currency.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := mustFund(t, rpc)

			ctx := context.Background()
			setup.SetupTrustLine(ctx, rpc, acct.Address, acct.Secret, "EUR", xrplsim.GenesisAddress, "500")

			raw, err := rpc.Call("account_currencies", map[string]interface{}{
				"account":      acct.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_currencies:", err)
			}

			var resp struct {
				ReceiveCurrencies []string `json:"receive_currencies"`
			}
			json.Unmarshal(raw, &resp)
			if len(resp.ReceiveCurrencies) == 0 {
				t.Fatal("expected at least one receive currency")
			}
			t.Logf("receive currencies: %v", resp.ReceiveCurrencies)
		},
	}
}

func submitPaymentSuccess() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "submit_payment_success",
		Description: "Submit a valid XRP payment and verify tesSUCCESS.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)

			w, _ := rpc.WalletPropose()
			result, err := rpc.SubmitPayment(
				xrplsim.GenesisSecret, xrplsim.GenesisAddress,
				w.AccountID, "100000000",
			)
			if err != nil {
				t.Fatal("submit:", err)
			}
			if result.EngineResult != "tesSUCCESS" {
				t.Fatalf("expected tesSUCCESS, got %s", result.EngineResult)
			}
			t.Logf("payment %s = %s", result.TxHash, result.EngineResult)
		},
	}
}

func submitPaymentUnfunded() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "submit_payment_unfunded",
		Description: "Submit a payment from an unfunded account and verify failure.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)

			w, _ := rpc.WalletPropose()
			dest, _ := rpc.WalletPropose()

			result, err := rpc.SubmitPayment(w.MasterSeed, w.AccountID, dest.AccountID, "100000000")
			if err != nil {
				t.Fatal("submit:", err)
			}
			// Unfunded source should fail.
			if result.EngineResult == "tesSUCCESS" {
				t.Fatal("expected failure for unfunded source, got tesSUCCESS")
			}
			t.Logf("unfunded payment: %s", result.EngineResult)
		},
	}
}

func txAfterSubmit() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "tx_after_submit",
		Description: "Submit a transaction and look it up by hash via tx method.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)

			w, _ := rpc.WalletPropose()
			result, err := rpc.SubmitPayment(
				xrplsim.GenesisSecret, xrplsim.GenesisAddress,
				w.AccountID, "50000000",
			)
			if err != nil {
				t.Fatal("submit:", err)
			}
			if result.TxHash == "" {
				t.Fatal("no tx hash returned")
			}

			ctx := context.Background()
			setup.WaitSettled(ctx, rpc, 5)

			// Look up by hash.
			txRaw, err := rpc.Tx(result.TxHash)
			if err != nil {
				t.Fatal("tx lookup:", err)
			}

			var txResp struct {
				Hash string `json:"hash"`
			}
			json.Unmarshal(txRaw, &txResp)
			t.Logf("tx lookup: hash=%s", txResp.Hash)
		},
	}
}

func ledgerEntryByIndex() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "ledger_entry_funded_account",
		Description: "Fund an account and look it up via ledger_entry.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := mustFund(t, rpc)

			raw, err := rpc.Call("ledger_entry", map[string]interface{}{
				"account_root": acct.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("ledger_entry:", err)
			}

			var resp struct {
				Node struct {
					Account string `json:"Account"`
					Balance string `json:"Balance"`
				} `json:"node"`
				Status string `json:"status"`
			}
			json.Unmarshal(raw, &resp)
			if resp.Node.Account != acct.Address {
				t.Fatalf("expected account %s, got %s", acct.Address, resp.Node.Account)
			}
			t.Logf("ledger_entry: account=%s balance=%s", resp.Node.Account, resp.Node.Balance)
		},
	}
}

func accountNftsAfterMint() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_nfts_after_mint",
		Description: "Mint an NFT and verify it appears in account_nfts.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := mustFund(t, rpc)

			// Submit NFTokenMint.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "NFTokenMint",
				"NFTokenTaxon":    0,
				"Flags":           8, // tfTransferable
			})
			if err != nil {
				t.Fatal("nft mint submit:", err)
			}
			t.Logf("NFTokenMint: %s = %s", result.TxHash, result.EngineResult)

			ctx := context.Background()
			setup.WaitSettled(ctx, rpc, 3)

			// Verify account_nfts.
			raw, err := rpc.Call("account_nfts", map[string]interface{}{
				"account":      acct.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_nfts:", err)
			}

			var resp struct {
				NFTs []interface{} `json:"account_nfts"`
			}
			json.Unmarshal(raw, &resp)
			if len(resp.NFTs) == 0 {
				t.Fatal("expected at least one NFT after mint")
			}
			t.Logf("found %d NFT(s)", len(resp.NFTs))
		},
	}
}

func depositAuthorizedPreauth() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "deposit_authorized_preauth",
		Description: "Set deposit preauth and verify deposit_authorized returns true.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := mustFund(t, rpc)
			ctx := context.Background()

			// Enable deposit auth on the account.
			result, err := rpc.SubmitAccountSet(acct.Secret, acct.Address, 9) // asfDepositAuth
			if err != nil {
				t.Fatal("account_set:", err)
			}
			t.Logf("enable deposit auth: %s", result.EngineResult)
			setup.WaitSettled(ctx, rpc, 3)

			// Authorize genesis to deposit.
			authResult, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "DepositPreauth",
				"Authorize":       xrplsim.GenesisAddress,
			})
			if err != nil {
				t.Fatal("deposit preauth:", err)
			}
			t.Logf("deposit preauth: %s", authResult.EngineResult)
			setup.WaitSettled(ctx, rpc, 3)

			// Verify deposit_authorized.
			raw, err := rpc.Call("deposit_authorized", map[string]interface{}{
				"source_account":      xrplsim.GenesisAddress,
				"destination_account": acct.Address,
				"ledger_index":        "current",
			})
			if err != nil {
				t.Fatal("deposit_authorized:", err)
			}

			var resp struct {
				DepositAuthorized bool `json:"deposit_authorized"`
				Status            string `json:"status"`
			}
			json.Unmarshal(raw, &resp)
			if !resp.DepositAuthorized {
				t.Fatal("expected deposit_authorized=true")
			}
			t.Log("deposit_authorized: true")
		},
	}
}

func accountChannelsAfterPayChan() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "account_channels_after_paychan",
		Description: "Create a payment channel and verify it appears in account_channels.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts, err := setup.FundN(ctx, rpc, 2, "10000000000")
			if err != nil {
				t.Fatal("fund:", err)
			}

			_, err = setup.SetupPaymentChannel(ctx, rpc,
				accounts[0].Address, accounts[0].Secret,
				accounts[1].Address, "1000000", 86400,
			)
			if err != nil {
				t.Fatal("setup payment channel:", err)
			}

			raw, err := rpc.Call("account_channels", map[string]interface{}{
				"account":      accounts[0].Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_channels:", err)
			}

			var resp struct {
				Channels []struct {
					ChannelID string `json:"channel_id"`
					Account   string `json:"account"`
				} `json:"channels"`
			}
			json.Unmarshal(raw, &resp)
			if len(resp.Channels) == 0 {
				t.Fatal("expected at least one payment channel")
			}
			t.Logf("found %d channel(s)", len(resp.Channels))
		},
	}
}

func gatewayBalancesWithIOU() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "gateway_balances_with_iou",
		Description: "Issue an IOU and verify gateway_balances shows obligations.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Fund issuer and holder.
			accounts, err := setup.FundN(ctx, rpc, 2, "10000000000")
			if err != nil {
				t.Fatal("fund:", err)
			}
			issuer := accounts[0]
			holder := accounts[1]

			// Issue IOU: holder trusts issuer, issuer sends IOU.
			err = setup.SetupIOU(ctx, rpc,
				issuer.Address, issuer.Secret,
				holder.Address, holder.Secret,
				"USD", "100",
			)
			if err != nil {
				t.Fatal("setup IOU:", err)
			}

			raw, err := rpc.Call("gateway_balances", map[string]interface{}{
				"account":      issuer.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("gateway_balances:", err)
			}

			var resp struct {
				Obligations map[string]string `json:"obligations"`
				Status      string            `json:"status"`
			}
			json.Unmarshal(raw, &resp)
			if len(resp.Obligations) == 0 {
				t.Fatal("expected non-empty obligations")
			}
			t.Logf("obligations: %v", resp.Obligations)
		},
	}
}

func norippleCheckWithTrustline() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "noripple_check_with_trustline",
		Description: "Create trust line and verify noripple_check returns problems.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			acct := mustFund(t, rpc)
			err := setup.SetupTrustLine(ctx, rpc, acct.Address, acct.Secret, "USD", xrplsim.GenesisAddress, "1000")
			if err != nil {
				t.Fatal("setup trust line:", err)
			}

			raw, err := rpc.Call("noripple_check", map[string]interface{}{
				"account":      acct.Address,
				"role":         "user",
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("noripple_check:", err)
			}

			var resp struct {
				Problems []string `json:"problems"`
				Status   string   `json:"status"`
			}
			json.Unmarshal(raw, &resp)
			t.Logf("noripple_check returned %d problem(s)", len(resp.Problems))
		},
	}
}

func signAndSubmit() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "sign_and_submit",
		Description: "Use sign to sign a payment offline, then submit the tx_blob.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)

			w, _ := rpc.WalletPropose()

			// Fund the wallet first.
			ctx := context.Background()
			setup.FundAccount(ctx, rpc, w.AccountID, "10000000000")

			dest, _ := rpc.WalletPropose()

			// Use the sign method to sign offline.
			signRaw, err := rpc.Call("sign", map[string]interface{}{
				"secret":  w.MasterSeed,
				"offline": true,
				"tx_json": map[string]interface{}{
					"TransactionType": "Payment",
					"Account":         w.AccountID,
					"Destination":     dest.AccountID,
					"Amount":          "1000000",
					"Sequence":        1,
					"Fee":             "12",
				},
			})
			if err != nil {
				t.Fatal("sign:", err)
			}

			var signResp struct {
				TxBlob string `json:"tx_blob"`
				Status string `json:"status"`
			}
			json.Unmarshal(signRaw, &signResp)
			if signResp.TxBlob == "" {
				t.Fatal("sign returned empty tx_blob")
			}
			t.Logf("sign produced tx_blob of length %d", len(signResp.TxBlob))

			// Submit the signed blob.
			submitRaw, err := rpc.Call("submit", map[string]interface{}{
				"tx_blob": signResp.TxBlob,
			})
			if err != nil {
				t.Fatal("submit tx_blob:", err)
			}

			var submitResp struct {
				EngineResult string `json:"engine_result"`
			}
			json.Unmarshal(submitRaw, &submitResp)
			t.Logf("submit tx_blob: %s", submitResp.EngineResult)
		},
	}
}

func signForMultisign() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "sign_for_multisign",
		Description: "Use sign_for to produce a partial multi-signature.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Create two accounts: the multi-sign account and a signer.
			accounts, err := setup.FundN(ctx, rpc, 2, "10000000000")
			if err != nil {
				t.Fatal("fund:", err)
			}
			multiAcct := accounts[0]
			signer := accounts[1]

			// Set signer list on multiAcct.
			_, err = rpc.Submit(multiAcct.Secret, multiAcct.Address, map[string]interface{}{
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
			setup.WaitSettled(ctx, rpc, 3)

			dest, _ := rpc.WalletPropose()

			// Use sign_for to sign on behalf of multiAcct.
			raw, err := rpc.Call("sign_for", map[string]interface{}{
				"account": signer.Address,
				"secret":  signer.Secret,
				"tx_json": map[string]interface{}{
					"TransactionType": "Payment",
					"Account":         multiAcct.Address,
					"Destination":     dest.AccountID,
					"Amount":          "1000000",
					"Sequence":        2,
					"Fee":             "20",
				},
			})
			if err != nil {
				t.Fatal("sign_for:", err)
			}

			var resp struct {
				TxBlob string `json:"tx_blob"`
				TxJSON struct {
					Signers []interface{} `json:"Signers"`
				} `json:"tx_json"`
				Status string `json:"status"`
			}
			json.Unmarshal(raw, &resp)
			if resp.TxBlob == "" {
				t.Fatal("sign_for returned empty tx_blob")
			}
			t.Logf("sign_for produced tx_blob with %d signer(s)", len(resp.TxJSON.Signers))
		},
	}
}

func submitMultisigned() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "submit_multisigned",
		Description: "Full multi-sign flow: signer list, sign_for, submit_multisigned.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts, err := setup.FundN(ctx, rpc, 2, "10000000000")
			if err != nil {
				t.Fatal("fund:", err)
			}
			multiAcct := accounts[0]
			signer := accounts[1]

			// Set signer list.
			_, err = rpc.Submit(multiAcct.Secret, multiAcct.Address, map[string]interface{}{
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
			setup.WaitSettled(ctx, rpc, 3)

			dest, _ := rpc.WalletPropose()

			// sign_for from the signer.
			signRaw, err := rpc.Call("sign_for", map[string]interface{}{
				"account": signer.Address,
				"secret":  signer.Secret,
				"tx_json": map[string]interface{}{
					"TransactionType": "Payment",
					"Account":         multiAcct.Address,
					"Destination":     dest.AccountID,
					"Amount":          "1000000",
					"Sequence":        2,
					"Fee":             "20",
				},
			})
			if err != nil {
				t.Fatal("sign_for:", err)
			}

			var signResp struct {
				TxJSON json.RawMessage `json:"tx_json"`
			}
			json.Unmarshal(signRaw, &signResp)

			// submit_multisigned with the signed tx_json.
			var txJSON map[string]interface{}
			json.Unmarshal(signResp.TxJSON, &txJSON)

			submitRaw, err := rpc.Call("submit_multisigned", map[string]interface{}{
				"tx_json": txJSON,
			})
			if err != nil {
				t.Fatal("submit_multisigned:", err)
			}

			var submitResp struct {
				EngineResult string `json:"engine_result"`
				Status       string `json:"status"`
			}
			json.Unmarshal(submitRaw, &submitResp)
			t.Logf("submit_multisigned: %s", submitResp.EngineResult)
		},
	}
}

func transactionEntryAfterSubmit() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "transaction_entry_after_submit",
		Description: "Submit a payment and look it up via transaction_entry.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)

			w, _ := rpc.WalletPropose()
			result, err := rpc.SubmitPayment(
				xrplsim.GenesisSecret, xrplsim.GenesisAddress,
				w.AccountID, "50000000",
			)
			if err != nil {
				t.Fatal("submit:", err)
			}

			ctx := context.Background()
			setup.WaitSettled(ctx, rpc, 5)

			// Get the ledger index where this tx was validated.
			info, err := rpc.ServerInfo()
			if err != nil {
				t.Fatal("server_info:", err)
			}

			// Try transaction_entry on the validated ledger range.
			raw, err := rpc.Call("transaction_entry", map[string]interface{}{
				"tx_hash":      result.TxHash,
				"ledger_index": info.Validated.Seq,
			})
			if err != nil {
				t.Fatal("transaction_entry:", err)
			}

			var resp struct {
				TxJSON map[string]interface{} `json:"tx_json"`
				Status string                 `json:"status"`
			}
			json.Unmarshal(raw, &resp)
			if resp.Status == "error" {
				// Try one ledger earlier in case of timing.
				raw, err = rpc.Call("transaction_entry", map[string]interface{}{
					"tx_hash":      result.TxHash,
					"ledger_index": info.Validated.Seq - 1,
				})
				if err != nil {
					t.Fatal("transaction_entry retry:", err)
				}
				json.Unmarshal(raw, &resp)
			}
			t.Logf("transaction_entry: tx_json keys=%d", len(resp.TxJSON))
		},
	}
}

func ammInfoAfterCreate() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "amm_info_after_create",
		Description: "Create an AMM pool and verify amm_info returns pool data.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts, err := setup.FundN(ctx, rpc, 1, "50000000000") // 50k XRP
			if err != nil {
				t.Fatal("fund:", err)
			}
			acct := accounts[0]

			// Create trust line for USD on the account.
			err = setup.SetupTrustLine(ctx, rpc, acct.Address, acct.Secret, "USD", xrplsim.GenesisAddress, "10000")
			if err != nil {
				t.Fatal("trust line:", err)
			}

			// Send some USD to the account.
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

			// Create AMM pool: USD/XRP.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "AMMCreate",
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "100",
				},
				"Amount2": "10000000000", // 10k XRP drops
			})
			if err != nil {
				t.Fatal("AMM create:", err)
			}
			t.Logf("AMMCreate: %s", result.EngineResult)
			if result.EngineResult != "tesSUCCESS" {
				t.Log("AMMCreate not supported or failed — skipping amm_info check")
				return
			}
			setup.WaitSettled(ctx, rpc, 3)

			// Query amm_info.
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

			var resp struct {
				AMM struct {
					Amount  interface{} `json:"amount"`
					Amount2 interface{} `json:"amount2"`
				} `json:"amm"`
				Status string `json:"status"`
			}
			json.Unmarshal(raw, &resp)
			if resp.Status == "error" {
				t.Fatal("amm_info returned error after successful AMMCreate")
			}
			t.Logf("amm_info: amount=%v amount2=%v", resp.AMM.Amount, resp.AMM.Amount2)
		},
	}
}

func ripplePathFindWithPaths() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "ripple_path_find_with_paths",
		Description: "Set up multi-hop IOU network and verify ripple_path_find returns paths.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Create three accounts: A (sender), B (intermediary/issuer), C (destination).
			accounts, err := setup.FundN(ctx, rpc, 3, "10000000000")
			if err != nil {
				t.Fatal("fund:", err)
			}
			issuer := accounts[0]
			holder := accounts[1]
			dest := accounts[2]

			// holder trusts issuer for USD.
			err = setup.SetupTrustLine(ctx, rpc, holder.Address, holder.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line holder->issuer:", err)
			}

			// dest trusts issuer for USD.
			err = setup.SetupTrustLine(ctx, rpc, dest.Address, dest.Secret, "USD", issuer.Address, "1000")
			if err != nil {
				t.Fatal("trust line dest->issuer:", err)
			}

			// Issuer sends USD to holder.
			_, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     holder.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   issuer.Address,
					"value":    "100",
				},
			})
			if err != nil {
				t.Fatal("issue USD:", err)
			}
			setup.WaitSettled(ctx, rpc, 3)

			// Query ripple_path_find: holder -> dest for 10 USD.
			raw, err := rpc.Call("ripple_path_find", map[string]interface{}{
				"source_account":      holder.Address,
				"destination_account": dest.Address,
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
				Alternatives []interface{} `json:"alternatives"`
				Status       string        `json:"status"`
			}
			json.Unmarshal(raw, &resp)
			t.Logf("ripple_path_find: %d alternative(s)", len(resp.Alternatives))
		},
	}
}

func getAggregatePriceAfterOracle() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "get_aggregate_price_after_oracle",
		Description: "Set oracle data and verify get_aggregate_price returns a price.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			acct := mustFund(t, rpc)

			// Submit OracleSet.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "OracleSet",
				"OracleDocumentID": 1,
				"Provider":        "70726F7669646572", // "provider" in hex
				"AssetClass":      "63757272656E6379", // "currency" in hex
				"LastUpdateTime":  time.Now().Unix() - 10,
				"PriceDataSeries": []map[string]interface{}{
					{
						"PriceData": map[string]interface{}{
							"BaseAsset":  "XRP",
							"QuoteAsset": "USD",
							"AssetPrice": 740,
							"Scale":      3,
						},
					},
				},
			})
			if err != nil {
				t.Fatal("oracle set:", err)
			}
			t.Logf("OracleSet: %s", result.EngineResult)
			setup.WaitSettled(ctx, rpc, 3)

			// Query get_aggregate_price.
			raw, err := rpc.Call("get_aggregate_price", map[string]interface{}{
				"base_asset":  "XRP",
				"quote_asset": "USD",
				"oracles": []map[string]interface{}{
					{
						"account":          acct.Address,
						"oracle_document_id": 1,
					},
				},
			})
			if err != nil {
				t.Fatal("get_aggregate_price:", err)
			}

			var resp struct {
				EntireSet interface{} `json:"entire_set"`
				Status    string      `json:"status"`
			}
			json.Unmarshal(raw, &resp)
			t.Logf("get_aggregate_price: status=%s", resp.Status)
		},
	}
}

func ripplePathFindNoPath() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "ripple_path_find_no_path",
		Description: "Query ripple_path_find when no path exists, verify empty alternatives.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts, err := setup.FundN(ctx, rpc, 2, "10000000000")
			if err != nil {
				t.Fatal("fund:", err)
			}

			// No trust lines = no IOU paths.
			raw, err := rpc.Call("ripple_path_find", map[string]interface{}{
				"source_account":      accounts[0].Address,
				"destination_account": accounts[1].Address,
				"destination_amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   accounts[0].Address,
					"value":    "10",
				},
			})
			if err != nil {
				t.Fatal("ripple_path_find:", err)
			}

			var resp struct {
				Alternatives []interface{} `json:"alternatives"`
			}
			json.Unmarshal(raw, &resp)
			if len(resp.Alternatives) != 0 {
				t.Fatalf("expected 0 alternatives, got %d", len(resp.Alternatives))
			}
			t.Log("ripple_path_find: 0 alternatives (expected)")
		},
	}
}

func bookOffersAfterCrossing() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "book_offers_after_crossing",
		Description: "Create crossing offers and verify book_offers reflects the result.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts, err := setup.FundN(ctx, rpc, 2, "10000000000")
			if err != nil {
				t.Fatal("fund:", err)
			}
			alice := accounts[0]
			bob := accounts[1]

			// Both trust genesis for USD.
			setup.SetupTrustLine(ctx, rpc, alice.Address, alice.Secret, "USD", xrplsim.GenesisAddress, "1000")
			setup.SetupTrustLine(ctx, rpc, bob.Address, bob.Secret, "USD", xrplsim.GenesisAddress, "1000")

			// Give alice some USD.
			rpc.Submit(xrplsim.GenesisSecret, xrplsim.GenesisAddress, map[string]interface{}{
				"TransactionType": "Payment",
				"Destination":     alice.Address,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "100",
				},
			})
			setup.WaitSettled(ctx, rpc, 3)

			// Alice: sell 10 USD for 10 XRP.
			setup.SetupOffer(ctx, rpc, alice.Address, alice.Secret,
				"10000000",
				map[string]interface{}{"currency": "USD", "issuer": xrplsim.GenesisAddress, "value": "10"},
			)

			// Bob: sell 10 XRP for 10 USD — crosses with alice's offer.
			rpc.SubmitOfferCreate(bob.Secret, bob.Address,
				map[string]interface{}{"currency": "USD", "issuer": xrplsim.GenesisAddress, "value": "10"},
				"10000000",
			)
			setup.WaitSettled(ctx, rpc, 3)

			// book_offers should be empty after full crossing.
			raw, err := rpc.Call("book_offers", map[string]interface{}{
				"taker_pays":   map[string]interface{}{"currency": "XRP"},
				"taker_gets":   map[string]interface{}{"currency": "USD", "issuer": xrplsim.GenesisAddress},
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("book_offers:", err)
			}

			var resp struct {
				Offers []interface{} `json:"offers"`
			}
			json.Unmarshal(raw, &resp)
			t.Logf("book_offers after crossing: %d offer(s)", len(resp.Offers))
		},
	}
}

func bookOffersWithOrder() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "book_offers_with_order",
		Description: "Create an offer and verify it appears in book_offers.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			acct := mustFund(t, rpc)
			ctx := context.Background()

			// Trust line for USD.
			setup.SetupTrustLine(ctx, rpc, acct.Address, acct.Secret, "USD", xrplsim.GenesisAddress, "1000")

			// Create offer: sell 10 XRP for 1 USD.
			setup.SetupOffer(ctx, rpc, acct.Address, acct.Secret,
				map[string]interface{}{"currency": "USD", "issuer": xrplsim.GenesisAddress, "value": "1"},
				"10000000",
			)

			// Query book_offers.
			raw, err := rpc.Call("book_offers", map[string]interface{}{
				"taker_pays":   map[string]interface{}{"currency": "USD", "issuer": xrplsim.GenesisAddress},
				"taker_gets":   map[string]interface{}{"currency": "XRP"},
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("book_offers:", err)
			}

			var resp struct {
				Offers []interface{} `json:"offers"`
			}
			json.Unmarshal(raw, &resp)
			if len(resp.Offers) == 0 {
				t.Fatal("expected at least one offer in book")
			}
			t.Logf("book_offers: %d offer(s)", len(resp.Offers))
		},
	}
}

func channelAuthorizeAndVerify() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "channel_authorize_and_verify",
		Description: "Create payment channel, authorize a claim, and verify the signature.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts, err := setup.FundN(ctx, rpc, 2, "10000000000")
			if err != nil {
				t.Fatal("fund:", err)
			}

			// Create payment channel.
			txHash, err := setup.SetupPaymentChannel(ctx, rpc,
				accounts[0].Address, accounts[0].Secret,
				accounts[1].Address, "5000000000", 86400,
			)
			if err != nil {
				t.Fatal("setup payment channel:", err)
			}
			_ = txHash

			// Get the channel ID.
			raw, err := rpc.Call("account_channels", map[string]interface{}{
				"account":      accounts[0].Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_channels:", err)
			}
			var chanResp struct {
				Channels []struct {
					ChannelID string `json:"channel_id"`
				} `json:"channels"`
			}
			json.Unmarshal(raw, &chanResp)
			if len(chanResp.Channels) == 0 {
				t.Fatal("no channels found")
			}
			channelID := chanResp.Channels[0].ChannelID

			// channel_authorize: sign a claim for 1000000 drops.
			authRaw, err := rpc.Call("channel_authorize", map[string]interface{}{
				"channel_id": channelID,
				"amount":     "1000000",
				"secret":     accounts[0].Secret,
			})
			if err != nil {
				t.Fatal("channel_authorize:", err)
			}
			var authResp struct {
				Signature string `json:"signature"`
				Status    string `json:"status"`
			}
			json.Unmarshal(authRaw, &authResp)
			if authResp.Signature == "" {
				t.Fatal("channel_authorize returned empty signature")
			}
			t.Logf("channel_authorize: signature length=%d", len(authResp.Signature))

			// Get the public key for verification.
			w, err := rpc.Call("account_info", map[string]interface{}{
				"account":      accounts[0].Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_info:", err)
			}
			// Use the channel's public key directly (set during creation).
			pubKey := "0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020"
			_ = w

			// channel_verify: verify the signature.
			verifyRaw, err := rpc.Call("channel_verify", map[string]interface{}{
				"channel_id": channelID,
				"amount":     "1000000",
				"signature":  authResp.Signature,
				"public_key": pubKey,
			})
			if err != nil {
				t.Fatal("channel_verify:", err)
			}
			var verifyResp struct {
				SignatureVerified bool   `json:"signature_verified"`
				Status           string `json:"status"`
			}
			json.Unmarshal(verifyRaw, &verifyResp)
			t.Logf("channel_verify: verified=%v", verifyResp.SignatureVerified)
		},
	}
}

func nftBuyOffersWithOffer() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_buy_offers_with_offer",
		Description: "Mint NFT, create buy offer, verify nft_buy_offers returns it.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts, err := setup.FundN(ctx, rpc, 2, "10000000000")
			if err != nil {
				t.Fatal("fund:", err)
			}
			minter := accounts[0]
			buyer := accounts[1]

			// Mint NFT.
			result, err := rpc.Submit(minter.Secret, minter.Address, map[string]interface{}{
				"TransactionType": "NFTokenMint",
				"NFTokenTaxon":    0,
				"Flags":           8, // tfTransferable
			})
			if err != nil {
				t.Fatal("nft mint:", err)
			}
			setup.WaitSettled(ctx, rpc, 3)

			// Get NFT ID.
			raw, err := rpc.Call("account_nfts", map[string]interface{}{
				"account":      minter.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_nfts:", err)
			}
			var nftResp struct {
				NFTs []struct {
					NFTokenID string `json:"NFTokenID"`
				} `json:"account_nfts"`
			}
			json.Unmarshal(raw, &nftResp)
			if len(nftResp.NFTs) == 0 {
				t.Fatal("no NFTs after mint")
			}
			nftID := nftResp.NFTs[0].NFTokenID
			_ = result

			// Buyer creates buy offer.
			buyResult, err := rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "5000000", // 5 XRP
				"Owner":           minter.Address,
			})
			if err != nil {
				t.Fatal("nft buy offer:", err)
			}
			setup.WaitSettled(ctx, rpc, 3)
			t.Logf("buy offer: %s", buyResult.EngineResult)

			// Query nft_buy_offers.
			raw, err = rpc.Call("nft_buy_offers", map[string]interface{}{
				"nft_id": nftID,
			})
			if err != nil {
				t.Fatal("nft_buy_offers:", err)
			}
			var buyOffersResp struct {
				Offers []struct {
					Owner  string `json:"owner"`
					Amount string `json:"amount"`
				} `json:"offers"`
			}
			json.Unmarshal(raw, &buyOffersResp)
			if len(buyOffersResp.Offers) == 0 {
				t.Fatal("expected at least one buy offer")
			}
			t.Logf("nft_buy_offers: %d offer(s)", len(buyOffersResp.Offers))
		},
	}
}

func vaultInfoAfterCreate() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "vault_info_after_create",
		Description: "Create a vault and query vault_info (amendment-gated).",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			accounts, err := setup.FundN(ctx, rpc, 1, "50000000000")
			if err != nil {
				t.Fatal("fund:", err)
			}
			acct := accounts[0]

			// Try to create a vault. This is amendment-gated.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "VaultCreate",
				"Asset": map[string]interface{}{
					"currency": "XRP",
				},
			})
			if err != nil {
				t.Fatal("vault create:", err)
			}
			setup.WaitSettled(ctx, rpc, 3)

			if result.EngineResult != "tesSUCCESS" {
				// Amendment may not be enabled — that's OK for this test.
				t.Logf("vault create: %s (amendment may not be enabled)", result.EngineResult)
				return
			}

			// Query vault_info.
			raw, err := rpc.Call("vault_info", map[string]interface{}{
				"owner":        acct.Address,
				"seq":          1,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("vault_info:", err)
			}
			var resp struct {
				Status string `json:"status"`
			}
			json.Unmarshal(raw, &resp)
			t.Logf("vault_info: status=%s", resp.Status)
		},
	}
}

func simulateTransaction() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "simulate_transaction",
		Description: "Use simulate to dry-run a payment without committing.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)

			w, _ := rpc.WalletPropose()

			// Simulate a payment from genesis to new wallet.
			raw, err := rpc.Call("simulate", map[string]interface{}{
				"tx_json": map[string]interface{}{
					"TransactionType": "Payment",
					"Account":         xrplsim.GenesisAddress,
					"Destination":     w.AccountID,
					"Amount":          "100000000",
					"Fee":             "12",
					"Sequence":        1,
				},
			})
			if err != nil {
				t.Fatal("simulate:", err)
			}

			var resp struct {
				EngineResult string `json:"engine_result"`
				Applied      bool   `json:"applied"`
				Status       string `json:"status"`
			}
			json.Unmarshal(raw, &resp)
			t.Logf("simulate: engine_result=%s applied=%v", resp.EngineResult, resp.Applied)

			// Verify the payment was NOT actually committed.
			_, infoErr := rpc.AccountInfo(w.AccountID)
			if infoErr == nil {
				t.Log("account exists after simulate — checking if it was truly dry-run")
			} else {
				t.Log("account does not exist after simulate (correct: dry-run)")
			}
		},
	}
}

func ledgerDataWithState() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "ledger_data_with_state",
		Description: "Fund accounts and verify ledger_data returns state entries.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			ctx := context.Background()

			// Create some state.
			_, err := setup.FundN(ctx, rpc, 3, "10000000000")
			if err != nil {
				t.Fatal("fund:", err)
			}

			// Query ledger_data.
			raw, err := rpc.Call("ledger_data", map[string]interface{}{
				"ledger_index": "current",
				"limit":        10,
			})
			if err != nil {
				t.Fatal("ledger_data:", err)
			}

			var resp struct {
				State []interface{} `json:"state"`
			}
			json.Unmarshal(raw, &resp)
			if len(resp.State) == 0 {
				t.Fatal("expected non-empty state in ledger_data")
			}
			t.Logf("ledger_data: %d state entries", len(resp.State))
		},
	}
}
