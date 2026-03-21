package main

import (
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func nftMintAndBurn() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_mint_and_burn",
		Description: "Mint an NFT and burn it, verify removal from account_nfts.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 1)
			acct := accounts[0]

			// Mint.
			result, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "NFTokenMint",
				"NFTokenTaxon":    0,
				"Flags":           8, // tfTransferable
			})
			if err != nil {
				t.Fatal("nft mint:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Get the NFT ID.
			raw, err := rpc.Call("account_nfts", map[string]interface{}{
				"account":      acct.Address,
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

			// Burn.
			burnResult, err := rpc.Submit(acct.Secret, acct.Address, map[string]interface{}{
				"TransactionType": "NFTokenBurn",
				"NFTokenID":       nftID,
			})
			if err != nil {
				t.Fatal("nft burn:", err)
			}
			assertEngineResult(t, burnResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify removed.
			raw, err = rpc.Call("account_nfts", map[string]interface{}{
				"account":      acct.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("account_nfts after burn:", err)
			}
			json.Unmarshal(raw, &nftResp)
			if len(nftResp.NFTs) != 0 {
				t.Fatal("expected no NFTs after burn")
			}
			t.Log("NFT minted and burned successfully")
		},
	}
}

func nftCreateAndAcceptOffer() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_create_and_accept_offer",
		Description: "Mint NFT, create sell offer, accept it, verify ownership transfer.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			seller := accounts[0]
			buyer := accounts[1]

			// Mint.
			result, err := rpc.Submit(seller.Secret, seller.Address, map[string]interface{}{
				"TransactionType": "NFTokenMint",
				"NFTokenTaxon":    0,
				"Flags":           8, // tfTransferable
			})
			if err != nil {
				t.Fatal("nft mint:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Get NFT ID.
			raw, err := rpc.Call("account_nfts", map[string]interface{}{
				"account":      seller.Address,
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

			// Create sell offer.
			offerResult, err := rpc.Submit(seller.Secret, seller.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "1000000", // 1 XRP
				"Flags":           1,         // tfSellNFToken
			})
			if err != nil {
				t.Fatal("nft create offer:", err)
			}
			assertEngineResult(t, offerResult, "tesSUCCESS")
			waitSettled(rpc)

			// Find the offer from nft_sell_offers.
			raw, err = rpc.Call("nft_sell_offers", map[string]interface{}{
				"nft_id": nftID,
			})
			if err != nil {
				t.Fatal("nft_sell_offers:", err)
			}
			var sellResp struct {
				Offers []struct {
					Index string `json:"nft_offer_index"`
				} `json:"offers"`
			}
			json.Unmarshal(raw, &sellResp)
			if len(sellResp.Offers) == 0 {
				t.Fatal("no sell offers found")
			}
			offerIndex := sellResp.Offers[0].Index

			// Buyer accepts the offer.
			acceptResult, err := rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": offerIndex,
			})
			if err != nil {
				t.Fatal("nft accept offer:", err)
			}
			assertEngineResult(t, acceptResult, "tesSUCCESS")
			waitSettled(rpc)

			// Verify ownership transferred to buyer.
			raw, err = rpc.Call("account_nfts", map[string]interface{}{
				"account":      buyer.Address,
				"ledger_index": "current",
			})
			if err != nil {
				t.Fatal("buyer account_nfts:", err)
			}
			json.Unmarshal(raw, &nftResp)
			if len(nftResp.NFTs) == 0 {
				t.Fatal("buyer should own the NFT after accepting offer")
			}
			t.Logf("NFT transferred: seller -> buyer, id=%s", nftID)
		},
	}
}
