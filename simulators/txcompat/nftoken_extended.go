package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
	"github.com/xrpl-commons/xrpl-hive/xrplsim/setup"
)

// --- Shared NFToken types and helpers ---

// accountRef is a lightweight struct holding address/secret for test accounts.
type accountRef struct {
	Address string
	Secret  string
}

// toRef converts a setup.Account to accountRef.
func toRef(acct setup.Account) accountRef {
	return accountRef{Address: acct.Address, Secret: acct.Secret}
}

// mintNFT mints an NFT with optional flags and returns its NFTokenID.
func mintNFT(t *xrplsim.T, rpc *xrplsim.RPCClient, acct accountRef, flags int) string {
	txJSON := map[string]interface{}{
		"TransactionType": "NFTokenMint",
		"NFTokenTaxon":    0,
	}
	if flags != 0 {
		txJSON["Flags"] = flags
	}
	result, err := rpc.Submit(acct.Secret, acct.Address, txJSON)
	if err != nil {
		t.Fatal("mint NFT:", err)
	}
	assertEngineResult(t, result, "tesSUCCESS")
	waitSettled(rpc)

	nfts := getAccountNFTs(t, rpc, acct.Address)
	if len(nfts) == 0 {
		t.Fatal("no NFTs after mint")
	}
	return nfts[len(nfts)-1].NFTokenID
}

// mintNFTWithFields mints an NFToken with arbitrary extra fields and returns the
// submit result and the new NFTokenID. If the mint does not succeed, nftID is empty.
func mintNFTWithFields(t *xrplsim.T, rpc *xrplsim.RPCClient, acct accountRef, txFields map[string]interface{}) (*xrplsim.SubmitResult, string) {
	beforeNFTs := getAccountNFTs(t, rpc, acct.Address)

	tx := map[string]interface{}{
		"TransactionType": "NFTokenMint",
	}
	for k, v := range txFields {
		tx[k] = v
	}

	result, err := rpc.Submit(acct.Secret, acct.Address, tx)
	if err != nil {
		t.Fatal("mintNFTWithFields submit:", err)
	}
	if result.EngineResult != "tesSUCCESS" {
		return result, ""
	}
	waitSettled(rpc)

	afterNFTs := getAccountNFTs(t, rpc, acct.Address)
	existing := make(map[string]bool)
	for _, nft := range beforeNFTs {
		existing[nft.NFTokenID] = true
	}
	for _, nft := range afterNFTs {
		if !existing[nft.NFTokenID] {
			return result, nft.NFTokenID
		}
	}
	if len(afterNFTs) > 0 {
		return result, afterNFTs[len(afterNFTs)-1].NFTokenID
	}
	return result, ""
}

// getNFTOfferIndex returns the first offer index for an NFT.
func getNFTOfferIndex(t *xrplsim.T, rpc *xrplsim.RPCClient, nftID string, isSell bool) string {
	method := "nft_buy_offers"
	if isSell {
		method = "nft_sell_offers"
	}
	raw, err := rpc.Call(method, map[string]interface{}{"nft_id": nftID})
	if err != nil {
		t.Fatal(method+":", err)
	}
	var resp struct {
		Offers []struct {
			Index string `json:"nft_offer_index"`
		} `json:"offers"`
	}
	json.Unmarshal(raw, &resp)
	if len(resp.Offers) == 0 {
		t.Fatalf("no %s offers found for %s", method, nftID)
	}
	return resp.Offers[0].Index
}

type nftInfo struct {
	NFTokenID string `json:"NFTokenID"`
	URI       string `json:"URI"`
	Flags     int    `json:"Flags"`
}

func getAccountNFTs(t *xrplsim.T, rpc *xrplsim.RPCClient, account string) []nftInfo {
	raw, err := rpc.Call("account_nfts", map[string]interface{}{
		"account":      account,
		"ledger_index": "current",
	})
	if err != nil {
		t.Fatal("account_nfts:", err)
	}
	var resp struct {
		NFTs []nftInfo `json:"account_nfts"`
	}
	json.Unmarshal(raw, &resp)
	return resp.NFTs
}

func getNFTOfferCount(t *xrplsim.T, rpc *xrplsim.RPCClient, nftID string, isSell bool) int {
	method := "nft_buy_offers"
	if isSell {
		method = "nft_sell_offers"
	}
	raw, err := rpc.Call(method, map[string]interface{}{
		"nft_id": nftID,
	})
	if err != nil {
		// "object not found" means zero offers.
		return 0
	}
	var resp struct {
		Offers []json.RawMessage `json:"offers"`
	}
	json.Unmarshal(raw, &resp)
	return len(resp.Offers)
}

// --- Test functions ---

// 1. nftMintInvalid - Mint with invalid parameters.
func nftMintInvalid() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_mint_invalid",
		Description: "Mint with invalid taxon or missing NFTokenTaxon. Expect tem* errors.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Missing NFTokenTaxon field entirely.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "NFTokenMint",
			})
			if err != nil {
				t.Fatal("mint no taxon submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Error("expected failure for mint without NFTokenTaxon, got tesSUCCESS")
			} else {
				t.Logf("mint without NFTokenTaxon: %s (expected tem* error)", result.EngineResult)
			}

			// Invalid taxon: negative value (JSON sends as -1, should be rejected).
			result, err = rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "NFTokenMint",
				"NFTokenTaxon":    -1,
			})
			if err != nil {
				t.Fatal("mint negative taxon submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Error("expected failure for mint with negative taxon, got tesSUCCESS")
			} else {
				t.Logf("mint with negative taxon: %s (expected tem* error)", result.EngineResult)
			}

			t.Log("invalid mint cases validated")
		},
	}
}

// 2. nftMintTaxon - Mint multiple NFTs with different taxons.
func nftMintTaxon() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_mint_taxon",
		Description: "Mint NFTs with different taxons (0, 1, 42), verify each gets a unique NFTokenID.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := toRef(accounts[0])

			taxons := []int{0, 1, 42}
			ids := make(map[string]bool)

			for _, taxon := range taxons {
				result, nftID := mintNFTWithFields(t, rpc, acct, map[string]interface{}{
					"NFTokenTaxon": taxon,
					"Flags":        8, // tfTransferable
				})
				assertEngineResult(t, result, "tesSUCCESS")
				if nftID == "" {
					t.Fatalf("no NFT ID returned for taxon %d", taxon)
				}
				if ids[nftID] {
					t.Fatalf("duplicate NFTokenID %s for taxon %d", nftID, taxon)
				}
				ids[nftID] = true
				t.Logf("taxon %d -> NFTokenID %s", taxon, nftID)
			}

			// Verify account has 3 NFTs.
			nfts := getAccountNFTs(t, rpc, acct.Address)
			if len(nfts) != 3 {
				t.Fatalf("expected 3 NFTs, got %d", len(nfts))
			}
			t.Log("3 NFTs minted with different taxons, all unique IDs")
		},
	}
}

// 3. nftMintURI - Mint with URI field.
func nftMintURI() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_mint_uri",
		Description: "Mint NFT with URI field, verify it appears in account_nfts.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := toRef(accounts[0])

			uriText := "https://example.com"
			uriHex := hex.EncodeToString([]byte(uriText))

			result, nftID := mintNFTWithFields(t, rpc, acct, map[string]interface{}{
				"NFTokenTaxon": 0,
				"Flags":        8, // tfTransferable
				"URI":          uriHex,
			})
			assertEngineResult(t, result, "tesSUCCESS")
			if nftID == "" {
				t.Fatal("no NFT ID returned")
			}

			// Verify URI in account_nfts.
			nfts := getAccountNFTs(t, rpc, acct.Address)
			found := false
			for _, nft := range nfts {
				if nft.NFTokenID == nftID {
					// URI is returned as uppercase hex by the server.
					expectedURI := fmt.Sprintf("%X", []byte(uriText))
					if nft.URI != expectedURI && nft.URI != uriHex {
						t.Fatalf("URI mismatch: got %s, want %s", nft.URI, expectedURI)
					}
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("minted NFT %s not found in account_nfts", nftID)
			}
			t.Logf("NFT minted with URI: %s -> %s", uriText, nftID)
		},
	}
}

// 4. nftMintFlagBurnable - Mint with tfBurnable, verify issuer can burn someone else's NFT.
func nftMintFlagBurnable() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_mint_flag_burnable",
		Description: "Mint with tfBurnable (1). Transfer to another account, then issuer burns it.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			issuer := toRef(accounts[0])
			holder := accounts[1]

			// Mint with tfBurnable (1) + tfTransferable (8) = 9.
			nftID := mintNFT(t, rpc, issuer, 9)

			// Create sell offer from issuer to holder.
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "0", // free transfer
				"Destination":     holder.Address,
				"Flags":           1, // tfSellNFToken
			})
			if err != nil {
				t.Fatal("create sell offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			offerIndex := getNFTOfferIndex(t, rpc, nftID, true)

			// Holder accepts.
			result, err = rpc.Submit(holder.Secret, holder.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": offerIndex,
			})
			if err != nil {
				t.Fatal("accept sell offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify holder owns it.
			holderNFTs := getAccountNFTs(t, rpc, holder.Address)
			if len(holderNFTs) == 0 {
				t.Fatal("holder should own the NFT")
			}

			// Issuer burns the NFT (allowed because tfBurnable was set).
			result, err = rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "NFTokenBurn",
				"NFTokenID":       nftID,
				"Owner":           holder.Address,
			})
			if err != nil {
				t.Fatal("issuer burn:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify holder no longer has it.
			holderNFTs = getAccountNFTs(t, rpc, holder.Address)
			if len(holderNFTs) != 0 {
				t.Fatal("holder should have no NFTs after issuer burn")
			}
			t.Log("issuer burned burnable NFT from holder successfully")
		},
	}
}

// 5. nftMintFlagOnlyXRP - Mint with tfOnlyXRP, IOU offer should fail.
func nftMintFlagOnlyXRP() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_mint_flag_only_xrp",
		Description: "Mint with tfOnlyXRP (2). IOU sell offer fails, XRP sell offer succeeds.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			acct := toRef(accounts[0])
			buyer := accounts[1]

			// Mint with tfOnlyXRP (2) + tfTransferable (8) = 10.
			nftID := mintNFT(t, rpc, acct, 10)

			// Try to create IOU sell offer - should fail.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount": map[string]interface{}{
					"currency": "USD",
					"issuer":   xrplsim.GenesisAddress,
					"value":    "10",
				},
				"Flags": 1, // tfSellNFToken
			})
			if err != nil {
				t.Fatal("iou sell offer submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Error("expected IOU sell offer to fail for tfOnlyXRP NFT, got tesSUCCESS")
			} else {
				t.Logf("IOU sell offer: %s (expected failure)", result.EngineResult)
			}

			// Create XRP sell offer - should succeed.
			result, err = rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "1000000", // 1 XRP
				"Flags":           1,         // tfSellNFToken
			})
			if err != nil {
				t.Fatal("xrp sell offer submit:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Buyer accepts.
			offerIndex := getNFTOfferIndex(t, rpc, nftID, true)
			result, err = rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": offerIndex,
			})
			if err != nil {
				t.Fatal("accept xrp offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			t.Log("tfOnlyXRP: IOU offer rejected, XRP offer accepted")
		},
	}
}

// 6. nftMintFlagTransferable - Mint with tfTransferable, verify re-sell works.
func nftMintFlagTransferable() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_mint_flag_transferable",
		Description: "Mint with tfTransferable (8). Sell to buyer, buyer can re-sell to third party.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			issuer := toRef(accounts[0])
			buyer := toRef(accounts[1])
			thirdParty := accounts[2]

			// Mint with tfTransferable.
			nftID := mintNFT(t, rpc, issuer, 8)

			// Issuer sells to buyer.
			result, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "1000000", // 1 XRP
				"Flags":           1,         // tfSellNFToken
			})
			if err != nil {
				t.Fatal("issuer sell offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			offerIndex := getNFTOfferIndex(t, rpc, nftID, true)
			result, err = rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": offerIndex,
			})
			if err != nil {
				t.Fatal("buyer accept:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify buyer owns it.
			buyerNFTs := getAccountNFTs(t, rpc, buyer.Address)
			if len(buyerNFTs) == 0 {
				t.Fatal("buyer should own the NFT")
			}

			// Buyer re-sells to thirdParty (allowed because tfTransferable).
			result, err = rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "2000000", // 2 XRP
				"Flags":           1,         // tfSellNFToken
			})
			if err != nil {
				t.Fatal("buyer re-sell offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			offerIndex = getNFTOfferIndex(t, rpc, nftID, true)
			result, err = rpc.Submit(thirdParty.Secret, thirdParty.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": offerIndex,
			})
			if err != nil {
				t.Fatal("thirdParty accept:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify thirdParty owns it.
			tpNFTs := getAccountNFTs(t, rpc, thirdParty.Address)
			if len(tpNFTs) == 0 {
				t.Fatal("thirdParty should own the NFT after re-sell")
			}
			t.Log("transferable NFT re-sold successfully: issuer -> buyer -> thirdParty")
		},
	}
}

// 7. nftMintTransferFee - Mint with TransferFee, verify issuer gets fee cut on secondary sale.
func nftMintTransferFee() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_mint_transfer_fee",
		Description: "Mint with TransferFee (5000 = 5%). Sell, then re-sell. Verify issuer gets fee.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			issuer := toRef(accounts[0])
			buyer := accounts[1]
			buyer2 := accounts[2]

			// Mint with tfTransferable (8) and TransferFee 5000 (5%).
			result, nftID := mintNFTWithFields(t, rpc, issuer, map[string]interface{}{
				"NFTokenTaxon": 0,
				"Flags":        8,    // tfTransferable
				"TransferFee":  5000, // 5% = 5000 basis points (out of 50000)
			})
			assertEngineResult(t, result, "tesSUCCESS")
			if nftID == "" {
				t.Fatal("no NFT ID returned")
			}

			// First sale: issuer -> buyer (no transfer fee on primary sale).
			result2, err := rpc.Submit(issuer.Secret, issuer.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "10000000", // 10 XRP
				"Flags":           1,          // tfSellNFToken
			})
			if err != nil {
				t.Fatal("issuer sell:", err)
			}
			assertEngineResult(t, result2, "tesSUCCESS")
			waitSettled(rpc)

			offerIndex := getNFTOfferIndex(t, rpc, nftID, true)
			result2, err = rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": offerIndex,
			})
			if err != nil {
				t.Fatal("buyer accept primary:", err)
			}
			assertEngineResult(t, result2, "tesSUCCESS")
			waitSettled(rpc)

			// Record issuer balance before secondary sale.
			issuerInfoBefore, _ := rpc.AccountInfo(issuer.Address)

			// Secondary sale: buyer -> buyer2 for 100 XRP.
			// Issuer should get 5% = 5 XRP.
			result2, err = rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "100000000", // 100 XRP
				"Flags":           1,           // tfSellNFToken
			})
			if err != nil {
				t.Fatal("buyer sell:", err)
			}
			assertEngineResult(t, result2, "tesSUCCESS")
			waitSettled(rpc)

			offerIndex = getNFTOfferIndex(t, rpc, nftID, true)
			result2, err = rpc.Submit(buyer2.Secret, buyer2.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": offerIndex,
			})
			if err != nil {
				t.Fatal("buyer2 accept:", err)
			}
			assertEngineResult(t, result2, "tesSUCCESS")
			waitSettled(rpc)

			// Verify issuer received transfer fee.
			issuerInfoAfter, _ := rpc.AccountInfo(issuer.Address)
			balBefore, _ := strconv.ParseInt(issuerInfoBefore.Balance, 10, 64)
			balAfter, _ := strconv.ParseInt(issuerInfoAfter.Balance, 10, 64)
			fee := balAfter - balBefore

			// 5% of 100 XRP = 5 XRP = 5,000,000 drops.
			if fee <= 0 {
				t.Fatalf("issuer should have received transfer fee, balance change: %d drops", fee)
			}
			t.Logf("issuer transfer fee received: %d drops (expected ~5000000)", fee)
		},
	}
}

// 8. nftBurnInvalid - Burn non-existent NFT, burn someone else's non-burnable NFT.
func nftBurnInvalid() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_burn_invalid",
		Description: "Burn non-existent NFTokenID, burn another's NFT without permission. Expect tec* errors.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			alice := toRef(accounts[0])
			bob := accounts[1]

			// Try to burn a non-existent NFT.
			fakeID := "00000000000000000000000000000000000000000000000000000000DEADBEEF"
			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "NFTokenBurn",
				"NFTokenID":       fakeID,
			})
			if err != nil {
				t.Fatal("burn non-existent submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Error("expected failure burning non-existent NFT, got tesSUCCESS")
			} else {
				t.Logf("burn non-existent NFT: %s (expected tecNO_ENTRY or similar)", result.EngineResult)
			}

			// Alice mints a non-burnable NFT (tfTransferable only, no tfBurnable).
			nftID := mintNFT(t, rpc, alice, 8) // tfTransferable only

			// Transfer to bob.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "0",
				"Destination":     bob.Address,
				"Flags":           1, // tfSellNFToken
			})
			if err != nil {
				t.Fatal("sell offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			offerIndex := getNFTOfferIndex(t, rpc, nftID, true)
			result, err = rpc.Submit(bob.Secret, bob.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": offerIndex,
			})
			if err != nil {
				t.Fatal("bob accept:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Alice (issuer) tries to burn bob's NFT without tfBurnable.
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "NFTokenBurn",
				"NFTokenID":       nftID,
				"Owner":           bob.Address,
			})
			if err != nil {
				t.Fatal("issuer burn non-burnable submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Error("expected failure: issuer burning non-burnable NFT held by another")
			} else {
				t.Logf("issuer burn non-burnable: %s (expected tecNO_PERMISSION)", result.EngineResult)
			}

			t.Log("invalid burn cases validated")
		},
	}
}

// 9. nftCreateOfferInvalid - Create offer for non-existent NFT, sell offer for NFT you don't own, etc.
func nftCreateOfferInvalid() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_create_offer_invalid",
		Description: "Create offer for non-existent NFT, sell offer for unowned NFT, zero amount buy. Expect errors.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			alice := toRef(accounts[0])
			bob := accounts[1]

			// Offer for non-existent NFT.
			fakeID := "00000000000000000000000000000000000000000000000000000000DEADBEEF"
			result, err := rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       fakeID,
				"Amount":          "1000000",
				"Flags":           1, // tfSellNFToken
			})
			if err != nil {
				t.Fatal("offer non-existent submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Error("expected failure for offer on non-existent NFT")
			} else {
				t.Logf("offer non-existent NFT: %s (expected tec error)", result.EngineResult)
			}

			// Alice mints an NFT.
			nftID := mintNFT(t, rpc, alice, 8)

			// Bob tries to create sell offer for alice's NFT.
			result, err = rpc.Submit(bob.Secret, bob.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "1000000",
				"Flags":           1, // tfSellNFToken
			})
			if err != nil {
				t.Fatal("bob sell alice nft submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Error("expected failure: bob creating sell offer for alice's NFT")
			} else {
				t.Logf("bob sell alice's NFT: %s (expected tecNO_PERMISSION or similar)", result.EngineResult)
			}

			// Alice creates sell offer with zero amount (valid for sell offers per protocol).
			result, err = rpc.Submit(alice.Secret, alice.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "0",
				"Flags":           1, // tfSellNFToken
			})
			if err != nil {
				t.Fatal("zero amount sell offer submit:", err)
			}
			// Zero amount sell offers are allowed per the XRPL protocol.
			t.Logf("zero amount sell offer: %s", result.EngineResult)

			// Bob creates buy offer with zero amount (should fail - temBAD_AMOUNT).
			result, err = rpc.Submit(bob.Secret, bob.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "0",
				"Owner":           alice.Address,
				// No tfSellNFToken flag = buy offer
			})
			if err != nil {
				t.Fatal("zero amount buy offer submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Error("expected failure for zero amount buy offer")
			} else {
				t.Logf("zero amount buy offer: %s (expected temBAD_AMOUNT)", result.EngineResult)
			}

			t.Log("invalid create offer cases validated")
		},
	}
}

// 10. nftCreateOfferDestination - Sell offer with specific Destination.
func nftCreateOfferDestination() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_create_offer_destination",
		Description: "Create sell offer with Destination. Only destination can accept. Others get tecNO_PERMISSION.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			seller := toRef(accounts[0])
			dest := accounts[1]
			other := accounts[2]

			nftID := mintNFT(t, rpc, seller, 8) // tfTransferable

			// Create sell offer with specific Destination.
			result, err := rpc.Submit(seller.Secret, seller.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "1000000", // 1 XRP
				"Destination":     dest.Address,
				"Flags":           1, // tfSellNFToken
			})
			if err != nil {
				t.Fatal("sell offer with dest:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			offerIndex := getNFTOfferIndex(t, rpc, nftID, true)

			// Other tries to accept - should fail.
			result, err = rpc.Submit(other.Secret, other.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": offerIndex,
			})
			if err != nil {
				t.Fatal("other accept submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Error("expected failure: non-destination accepting targeted offer")
			} else {
				t.Logf("other accept targeted offer: %s (expected tecNO_PERMISSION)", result.EngineResult)
			}

			// Destination accepts - should succeed.
			result, err = rpc.Submit(dest.Secret, dest.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": offerIndex,
			})
			if err != nil {
				t.Fatal("dest accept:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify dest owns it.
			destNFTs := getAccountNFTs(t, rpc, dest.Address)
			if len(destNFTs) == 0 {
				t.Fatal("destination should own the NFT")
			}
			t.Log("targeted sell offer: only destination could accept")
		},
	}
}

// 11. nftCancelOffers - Create multiple sell offers, cancel one, verify removal.
func nftCancelOffers() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_cancel_offers",
		Description: "Create multiple sell offers, cancel one, verify it is removed from nft_sell_offers.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := toRef(accounts[0])

			nftID := mintNFT(t, rpc, acct, 8)

			// Create 2 sell offers.
			for i := 0; i < 2; i++ {
				result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
					"TransactionType": "NFTokenCreateOffer",
					"NFTokenID":       nftID,
					"Amount":          fmt.Sprintf("%d", (i+1)*1000000),
					"Flags":           1, // tfSellNFToken
				})
				if err != nil {
					t.Fatalf("create sell offer %d: %v", i, err)
				}
				assertEngineResult(t, result, "tesSUCCESS")
				waitSettled(rpc)
			}

			// Verify 2 sell offers exist.
			countBefore := getNFTOfferCount(t, rpc, nftID, true)
			if countBefore != 2 {
				t.Fatalf("expected 2 sell offers, got %d", countBefore)
			}

			// Get the first offer index.
			offerIndex := getNFTOfferIndex(t, rpc, nftID, true)

			// Cancel the first offer.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "NFTokenCancelOffer",
				"NFTokenOffers":   []string{offerIndex},
			})
			if err != nil {
				t.Fatal("cancel offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify only 1 sell offer remains.
			countAfter := getNFTOfferCount(t, rpc, nftID, true)
			if countAfter != 1 {
				t.Fatalf("expected 1 sell offer after cancel, got %d", countAfter)
			}
			t.Log("sell offer cancelled, 1 remaining")
		},
	}
}

// 12. nftBrokeredSale - Broker facilitates sale between seller and buyer.
func nftBrokeredSale() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_brokered_sale",
		Description: "Seller creates sell offer, buyer creates buy offer, broker accepts both.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			seller := toRef(accounts[0])
			buyer := accounts[1]
			broker := accounts[2]

			nftID := mintNFT(t, rpc, seller, 8) // tfTransferable

			// Seller creates sell offer for 10 XRP.
			result, err := rpc.Submit(seller.Secret, seller.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "10000000", // 10 XRP
				"Flags":           1,          // tfSellNFToken
			})
			if err != nil {
				t.Fatal("seller sell offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			sellOfferIndex := getNFTOfferIndex(t, rpc, nftID, true)

			// Buyer creates buy offer for 10 XRP.
			result, err = rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "10000000", // 10 XRP
				"Owner":           seller.Address,
				// No tfSellNFToken = buy offer
			})
			if err != nil {
				t.Fatal("buyer buy offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			buyOfferIndex := getNFTOfferIndex(t, rpc, nftID, false)

			// Broker accepts both offers.
			result, err = rpc.Submit(broker.Secret, broker.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": sellOfferIndex,
				"NFTokenBuyOffer":  buyOfferIndex,
			})
			if err != nil {
				t.Fatal("broker accept:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify buyer owns the NFT.
			buyerNFTs := getAccountNFTs(t, rpc, buyer.Address)
			found := false
			for _, nft := range buyerNFTs {
				if nft.NFTokenID == nftID {
					found = true
					break
				}
			}
			if !found {
				t.Fatal("buyer should own the NFT after brokered sale")
			}

			// Verify seller no longer has it.
			sellerNFTs := getAccountNFTs(t, rpc, seller.Address)
			for _, nft := range sellerNFTs {
				if nft.NFTokenID == nftID {
					t.Fatal("seller should no longer own the NFT")
				}
			}
			t.Logf("brokered sale complete: seller -> buyer via broker, nft=%s", nftID)
		},
	}
}

// 13. nftBrokeredSaleToSelf - Brokered sale where buyer == seller (should fail).
func nftBrokeredSaleToSelf() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_brokered_sale_to_self",
		Description: "Brokered sale where buyer is also the seller. Should fail.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			owner := toRef(accounts[0])

			nftID := mintNFT(t, rpc, owner, 8) // tfTransferable

			// Owner creates sell offer.
			result, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "10000000",
				"Flags":           1, // tfSellNFToken
			})
			if err != nil {
				t.Fatal("owner sell offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			sellOfferIndex := getNFTOfferIndex(t, rpc, nftID, true)

			// Owner also creates buy offer for their own NFT (buying from self).
			// This should fail since you can't create a buy offer for your own NFT.
			result, err = rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "10000000",
				"Owner":           owner.Address,
			})
			if err != nil {
				t.Fatal("self buy offer submit:", err)
			}
			if result.EngineResult == "tesSUCCESS" {
				t.Error("expected failure: owner creating buy offer for own NFT")
			} else {
				t.Logf("self buy offer: %s (expected tem* error)", result.EngineResult)
			}

			t.Logf("sell offer %s still exists (self-buy correctly rejected)", sellOfferIndex)
			t.Log("brokered sale to self correctly prevented")
		},
	}
}

// 14. nftWithTickets - Mint, create offer, accept offer all using TicketSequence.
func nftWithTickets() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_with_tickets",
		Description: "Mint, create offer, and accept offer using TicketSequence.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			seller := accounts[0]
			buyer := accounts[1]

			// Create tickets for seller.
			sellerInfo, _ := rpc.AccountInfo(seller.Address)
			result, err := rpc.Submit(seller.Secret, seller.Address, map[string]interface{}{
				"TransactionType": "TicketCreate",
				"TicketCount":     3,
			})
			if err != nil {
				t.Fatal("seller ticket create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// TicketCreate consumes sellerInfo.Sequence. Tickets are at +1, +2, +3.
			sellerTicket1 := sellerInfo.Sequence + 1
			sellerTicket2 := sellerInfo.Sequence + 2

			// Create tickets for buyer.
			buyerInfo, _ := rpc.AccountInfo(buyer.Address)
			result, err = rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType": "TicketCreate",
				"TicketCount":     2,
			})
			if err != nil {
				t.Fatal("buyer ticket create:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			buyerTicket1 := buyerInfo.Sequence + 1

			// Mint NFT using ticket.
			result, err = rpc.Submit(seller.Secret, seller.Address, map[string]interface{}{
				"TransactionType": "NFTokenMint",
				"NFTokenTaxon":    0,
				"Flags":           8, // tfTransferable
				"Sequence":        0,
				"TicketSequence":  sellerTicket1,
			})
			if err != nil {
				t.Fatal("mint with ticket:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Get NFT ID.
			sellerNFTs := getAccountNFTs(t, rpc, seller.Address)
			if len(sellerNFTs) == 0 {
				t.Fatal("no NFTs after ticket mint")
			}
			nftID := sellerNFTs[0].NFTokenID

			// Create sell offer using ticket.
			result, err = rpc.Submit(seller.Secret, seller.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "1000000",
				"Flags":           1, // tfSellNFToken
				"Sequence":        0,
				"TicketSequence":  sellerTicket2,
			})
			if err != nil {
				t.Fatal("sell offer with ticket:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			offerIndex := getNFTOfferIndex(t, rpc, nftID, true)

			// Buyer accepts using ticket.
			result, err = rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": offerIndex,
				"Sequence":         0,
				"TicketSequence":   buyerTicket1,
			})
			if err != nil {
				t.Fatal("accept with ticket:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Verify buyer owns NFT.
			buyerNFTs := getAccountNFTs(t, rpc, buyer.Address)
			if len(buyerNFTs) == 0 {
				t.Fatal("buyer should own NFT after ticket-based accept")
			}
			t.Log("NFT mint, offer, and accept all completed with tickets")
		},
	}
}

// 15. nftBuyAndSellOffers - Create both buy and sell offers, query both endpoints.
func nftBuyAndSellOffers() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_buy_and_sell_offers",
		Description: "Create both buy and sell offers for same NFT, verify both visible via RPC.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			owner := toRef(accounts[0])
			buyer := accounts[1]

			nftID := mintNFT(t, rpc, owner, 8) // tfTransferable

			// Owner creates sell offer for 10 XRP.
			result, err := rpc.Submit(owner.Secret, owner.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "10000000", // 10 XRP
				"Flags":           1,          // tfSellNFToken
			})
			if err != nil {
				t.Fatal("sell offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Buyer creates buy offer for 5 XRP.
			result, err = rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "5000000", // 5 XRP
				"Owner":           owner.Address,
				// No tfSellNFToken = buy offer
			})
			if err != nil {
				t.Fatal("buy offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Query nft_sell_offers.
			sellCount := getNFTOfferCount(t, rpc, nftID, true)
			if sellCount != 1 {
				t.Fatalf("expected 1 sell offer, got %d", sellCount)
			}

			// Query nft_buy_offers.
			buyCount := getNFTOfferCount(t, rpc, nftID, false)
			if buyCount != 1 {
				t.Fatalf("expected 1 buy offer, got %d", buyCount)
			}

			// Verify sell offer details.
			raw, err := rpc.Call("nft_sell_offers", map[string]interface{}{"nft_id": nftID})
			if err != nil {
				t.Fatal("nft_sell_offers:", err)
			}
			var sellResp struct {
				Offers []struct {
					Amount string `json:"amount"`
					Owner  string `json:"owner"`
				} `json:"offers"`
			}
			json.Unmarshal(raw, &sellResp)
			if len(sellResp.Offers) > 0 && sellResp.Offers[0].Amount != "10000000" {
				t.Fatalf("sell offer amount: got %s, want 10000000", sellResp.Offers[0].Amount)
			}

			// Verify buy offer details.
			raw, err = rpc.Call("nft_buy_offers", map[string]interface{}{"nft_id": nftID})
			if err != nil {
				t.Fatal("nft_buy_offers:", err)
			}
			var buyResp struct {
				Offers []struct {
					Amount string `json:"amount"`
					Owner  string `json:"owner"`
				} `json:"offers"`
			}
			json.Unmarshal(raw, &buyResp)
			if len(buyResp.Offers) > 0 && buyResp.Offers[0].Amount != "5000000" {
				t.Fatalf("buy offer amount: got %s, want 5000000", buyResp.Offers[0].Amount)
			}

			t.Log("both buy and sell offers visible via nft_buy_offers and nft_sell_offers")
		},
	}
}
