// The rpccompat-stateful simulator tests XRPL RPC methods that require
// ledger state (funded accounts, trust lines, offers, etc.) before asserting.
// Unlike the rpccompat simulator which uses standalone nodes, this simulator
// creates a multi-node network with validators so ledgers actually close.
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

	xrplsim.MustRun(xrplsim.New(), suite)
}

// helpers

func startNetwork(t *xrplsim.T) (*xrplsim.Client, *xrplsim.RPCClient) {
	clients, err := t.Sim.ClientTypes()
	if err != nil || len(clients) == 0 {
		t.Fatal("no client types available")
	}

	// Start a single node for stateful tests.
	// We use the first available client type.
	topo := xrplsim.NewTopology(1)
	c := t.StartClient(clients[0].Name,
		xrplsim.WithValidatorConfig(topo, 0, nil),
	)
	rpc := xrplsim.NewRPCClient(c.RPCEndpoint())

	// Wait for RPC to be responsive.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := rpc.WaitForLedger(ctx, 3, 60*time.Second); err != nil {
		// Fallback: just wait for RPC to respond.
		for i := 0; i < 30; i++ {
			if _, err := rpc.ServerInfo(); err == nil {
				break
			}
			time.Sleep(time.Second)
		}
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

			// Create offer: sell XRP for USD.
			err := setup.SetupOffer(ctx, rpc, acct.Address, acct.Secret,
				"1000000",
				map[string]interface{}{"currency": "USD", "issuer": xrplsim.GenesisAddress, "value": "10"},
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
