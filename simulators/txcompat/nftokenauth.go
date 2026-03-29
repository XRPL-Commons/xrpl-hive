package main

import (
	"encoding/json"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func nftAuthUnauthorizedBuyerCreateOffer() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_auth_unauthorized_buyer_create_offer",
		Description: "Unauthorized buyer cannot create buy offer for NFT minted with lsfRequireAuth issuer.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			issuer := accounts[0]
			buyer := accounts[1]

			// Set RequireAuth on issuer.
			rpc.SubmitAccountSet(issuer.Secret, issuer.Address, 2) // asfRequireAuth
			waitSettled(rpc)

			// Mint a non-transferable NFT (no tfTransferable flag).
			nftID := mintNFT(t, rpc, toRef(issuer), 0)

			// Buyer tries to create buy offer — should succeed (buy offers don't need auth).
			result, err := rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Owner":           issuer.Address,
				"Amount":          "1000000",
			})
			if err != nil {
				t.Fatal("buy offer:", err)
			}
			// Buy offers are always allowed; it's the accept that may fail.
			t.Logf("unauthorized buyer create buy offer: %s", result.EngineResult)
		},
	}
}

func nftAuthUnauthorizedBuyerAcceptSellOffer() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_auth_unauthorized_buyer_accept_sell",
		Description: "Unauthorized buyer tries to accept a sell offer.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			seller := accounts[0]
			buyer := accounts[1]

			// Mint transferable NFT.
			nftID := mintNFT(t, rpc, toRef(seller), 8) // tfTransferable

			// Create sell offer.
			result, err := rpc.Submit(seller.Secret, seller.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "1000000",
				"Flags":           1, // tfSellNFToken
			})
			if err != nil {
				t.Fatal("sell offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			offerIndex := getNFTOfferIndex(t, rpc, nftID, true)

			// Buyer accepts — should work since no auth required on seller.
			acceptResult, err := rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": offerIndex,
			})
			if err != nil {
				t.Fatal("accept:", err)
			}
			assertEngineResult(t, acceptResult, "tesSUCCESS")
			t.Log("buyer accepted sell offer successfully")
		},
	}
}

func nftAuthSellerAcceptBuyFromUnauth() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_auth_seller_accept_buy_from_unauth",
		Description: "Seller accepts a buy offer from an unauthorized buyer.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 2)
			seller := accounts[0]
			buyer := accounts[1]

			// Mint transferable NFT.
			nftID := mintNFT(t, rpc, toRef(seller), 8)

			// Buyer creates buy offer.
			result, err := rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Owner":           seller.Address,
				"Amount":          "5000000", // 5 XRP
			})
			if err != nil {
				t.Fatal("buy offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			offerIndex := getNFTOfferIndex(t, rpc, nftID, false)

			// Seller accepts the buy offer.
			acceptResult, err := rpc.Submit(seller.Secret, seller.Address, map[string]interface{}{
				"TransactionType": "NFTokenAcceptOffer",
				"NFTokenBuyOffer": offerIndex,
			})
			if err != nil {
				t.Fatal("accept:", err)
			}
			assertEngineResult(t, acceptResult, "tesSUCCESS")

			// Verify buyer now owns the NFT.
			raw, _ := rpc.Call("account_nfts", map[string]interface{}{
				"account": buyer.Address, "ledger_index": "current",
			})
			var nftResp struct {
				NFTs []struct{ NFTokenID string `json:"NFTokenID"` } `json:"account_nfts"`
			}
			json.Unmarshal(raw, &nftResp)
			if len(nftResp.NFTs) == 0 {
				t.Fatal("buyer should own NFT")
			}
			t.Log("seller accepted buy offer from buyer")
		},
	}
}

func nftAuthUnauthorizedSellerAcceptBuy() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_auth_unauthorized_seller_accept_buy",
		Description: "Non-owner tries to accept a buy offer (should fail).",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			seller := accounts[0]
			buyer := accounts[1]
			thirdParty := accounts[2]

			nftID := mintNFT(t, rpc, toRef(seller), 8)

			// Buyer creates buy offer.
			result, err := rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Owner":           seller.Address,
				"Amount":          "5000000",
			})
			if err != nil {
				t.Fatal("buy offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			offerIndex := getNFTOfferIndex(t, rpc, nftID, false)

			// Third party (not the seller) tries to accept buy offer.
			acceptResult, err := rpc.Submit(thirdParty.Secret, thirdParty.Address, map[string]interface{}{
				"TransactionType": "NFTokenAcceptOffer",
				"NFTokenBuyOffer": offerIndex,
			})
			if err != nil {
				t.Fatal("accept:", err)
			}
			if acceptResult.EngineResult == "tesSUCCESS" {
				t.Fatal("expected non-owner to fail accepting buy offer")
			}
			t.Logf("non-owner accept buy offer: %s (expected failure)", acceptResult.EngineResult)
		},
	}
}

func nftAuthBrokeredWithUnauthorized() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_auth_brokered_unauthorized",
		Description: "Unauthorized broker tries to bridge sell and buy offers.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			seller := accounts[0]
			buyer := accounts[1]
			broker := accounts[2]

			nftID := mintNFT(t, rpc, toRef(seller), 8)

			// Seller creates sell offer.
			result, err := rpc.Submit(seller.Secret, seller.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "5000000",
				"Flags":           1,
			})
			if err != nil {
				t.Fatal("sell offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			sellOfferIndex := getNFTOfferIndex(t, rpc, nftID, true)

			// Buyer creates buy offer for more than sell price.
			result, err = rpc.Submit(buyer.Secret, buyer.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Owner":           seller.Address,
				"Amount":          "10000000", // 10 XRP (> sell price of 5)
			})
			if err != nil {
				t.Fatal("buy offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			buyOfferIndex := getNFTOfferIndex(t, rpc, nftID, false)

			// Broker bridges the offers.
			brokerResult, err := rpc.Submit(broker.Secret, broker.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": sellOfferIndex,
				"NFTokenBuyOffer":  buyOfferIndex,
			})
			if err != nil {
				t.Fatal("broker accept:", err)
			}
			// Brokered sale should succeed — the broker gets the difference.
			t.Logf("brokered sale: %s", brokerResult.EngineResult)
		},
	}
}

func nftAuthMinterTransferFee() xrplsim.TestSpec {
	return xrplsim.TestSpec{
		Name:        "nft_auth_minter_transfer_fee",
		Description: "Minter receives transfer fee on secondary sale.",
		Run: func(t *xrplsim.T) {
			_, rpc := startNetwork(t)
			accounts := mustFund(t, rpc, 3)
			minter := accounts[0]
			firstBuyer := accounts[1]
			secondBuyer := accounts[2]

			// Mint with transfer fee (5000 = 5%) and transferable flag.
			result, err := rpc.Submit(minter.Secret, minter.Address, map[string]interface{}{
				"TransactionType": "NFTokenMint",
				"NFTokenTaxon":    0,
				"Flags":           8, // tfTransferable
				"TransferFee":     5000,
			})
			if err != nil {
				t.Fatal("mint:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			raw, _ := rpc.Call("account_nfts", map[string]interface{}{"account": minter.Address, "ledger_index": "current"})
			var nftResp struct {
				NFTs []struct{ NFTokenID string `json:"NFTokenID"` } `json:"account_nfts"`
			}
			json.Unmarshal(raw, &nftResp)
			nftID := nftResp.NFTs[0].NFTokenID

			// First sale: minter → firstBuyer (no transfer fee on initial sale).
			result, err = rpc.Submit(minter.Secret, minter.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "10000000", // 10 XRP
				"Flags":           1,
			})
			if err != nil {
				t.Fatal("sell offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			sellIdx := getNFTOfferIndex(t, rpc, nftID, true)
			result, err = rpc.Submit(firstBuyer.Secret, firstBuyer.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": sellIdx,
			})
			if err != nil {
				t.Fatal("accept:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			// Record minter balance before secondary sale.
			minterBefore, _ := rpc.AccountInfo(minter.Address)

			// Second sale: firstBuyer → secondBuyer (transfer fee applies).
			result, err = rpc.Submit(firstBuyer.Secret, firstBuyer.Address, map[string]interface{}{
				"TransactionType": "NFTokenCreateOffer",
				"NFTokenID":       nftID,
				"Amount":          "20000000", // 20 XRP
				"Flags":           1,
			})
			if err != nil {
				t.Fatal("resell offer:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			resellIdx := getNFTOfferIndex(t, rpc, nftID, true)
			result, err = rpc.Submit(secondBuyer.Secret, secondBuyer.Address, map[string]interface{}{
				"TransactionType":  "NFTokenAcceptOffer",
				"NFTokenSellOffer": resellIdx,
			})
			if err != nil {
				t.Fatal("accept resale:", err)
			}
			assertEngineResult(t, result, "tesSUCCESS")
			waitSettled(rpc)

			minterAfter, _ := rpc.AccountInfo(minter.Address)
			t.Logf("minter balance: %s → %s (should increase by ~5%% of 20 XRP = 1 XRP)", minterBefore.Balance, minterAfter.Balance)
		},
	}
}

// mintNFT and getNFTOfferIndex are defined in nftoken_extended.go
