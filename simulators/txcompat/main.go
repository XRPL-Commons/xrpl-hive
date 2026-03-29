// The txcompat simulator tests XRPL transaction types systematically.
// Each test: fund accounts → submit transaction → verify engine_result → verify ledger state.
package main

import (
	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

func main() {
	suite := xrplsim.Suite{
		Name:        "txcompat",
		Description: "Transaction type compatibility tests across XRPL implementations.",
	}

	// Payment
	suite.Add(paymentIOU())
	suite.Add(paymentInsufficientFunds())
	suite.Add(paymentNoDestination())

	// Check
	suite.Add(checkCreateAndCash())
	suite.Add(checkCreateAndCancel())

	// Check (extended)
	suite.Add(checkCreateValid())
	suite.Add(checkCreateInvalid())
	suite.Add(checkCashXRP())
	suite.Add(checkCashIOU())
	suite.Add(checkCashInvalid())
	suite.Add(checkCashWithTransferFee())
	suite.Add(checkCancelValid())
	suite.Add(checkCancelInvalid())
	suite.Add(checkWithTickets())
	suite.Add(checkTrustLineCreation())

	// Escrow
	suite.Add(escrowCreateAndCancel())
	suite.Add(escrowLockup())
	suite.Add(escrowFinishOnly())
	suite.Add(escrowCancelOnly())
	suite.Add(escrowTags())
	suite.Add(escrowMetadataToSelf())
	suite.Add(escrowMetadataToOther())
	suite.Add(escrowFailureCases())
	suite.Add(escrowWithTickets())
	suite.Add(escrowDisallowXRP())

	// Payment Channels
	suite.Add(payChannelCreateAndClaim())
	suite.Add(paychanSimple())
	suite.Add(paychanSettleDelay())
	suite.Add(paychanDstTag())
	suite.Add(paychanFund())
	suite.Add(paychanMultipleChannels())
	suite.Add(paychanDisallowIncoming())
	suite.Add(paychanWithTickets())
	suite.Add(paychanDepositAuth())
	suite.Add(paychanOptionalFields())

	// NFToken
	suite.Add(nftMintAndBurn())
	suite.Add(nftCreateAndAcceptOffer())

	// NFToken (extended)
	suite.Add(nftMintInvalid())
	suite.Add(nftMintTaxon())
	suite.Add(nftMintURI())
	suite.Add(nftMintFlagBurnable())
	suite.Add(nftMintFlagOnlyXRP())
	suite.Add(nftMintFlagTransferable())
	suite.Add(nftMintTransferFee())
	suite.Add(nftBurnInvalid())
	suite.Add(nftCreateOfferInvalid())
	suite.Add(nftCreateOfferDestination())
	suite.Add(nftCancelOffers())
	suite.Add(nftBrokeredSale())
	suite.Add(nftBrokeredSaleToSelf())
	suite.Add(nftWithTickets())
	suite.Add(nftBuyAndSellOffers())

	// NFToken Auth
	suite.Add(nftAuthUnauthorizedBuyerCreateOffer())
	suite.Add(nftAuthUnauthorizedBuyerAcceptSellOffer())
	suite.Add(nftAuthSellerAcceptBuyFromUnauth())
	suite.Add(nftAuthUnauthorizedSellerAcceptBuy())
	suite.Add(nftAuthBrokeredWithUnauthorized())
	suite.Add(nftAuthMinterTransferFee())

	// Offer / DEX
	suite.Add(offerCreateCrossed())

	// SignerList + Tickets
	suite.Add(signerListSetAndMultisign())
	suite.Add(ticketCreateAndUse())

	// Oracle
	suite.Add(oracleSetAndDelete())

	// Clawback
	suite.Add(clawbackIOU())
	suite.Add(clawbackWithoutFlag())

	// DID
	suite.Add(didSetAndDelete())

	// Credential
	suite.Add(credentialCreateAcceptDelete())

	// AMM

	// AccountSet
	suite.Add(accountSetFlags())

	// AccountDelete
	suite.Add(accountDeleteBasics())
	suite.Add(accountDeleteDestinationConstraints())
	suite.Add(accountDeleteOwnedTypes())
	suite.Add(accountDeleteTooManyOffers())
	suite.Add(accountDeleteWithTickets())
	suite.Add(accountDeleteBalanceTooSmall())
	suite.Add(accountDeleteResurrection())
	suite.Add(accountDeleteDirectories())

	// TrustSet
	suite.Add(trustSetMalformed())
	suite.Add(trustSetTwoFreeTrustlines())
	suite.Add(trustSetDisallowIncoming())
	suite.Add(trustSetDynamicReserve())
	suite.Add(trustSetWithTicket())

	// Flow (payment flow/routing)
	suite.Add(flowDirectStep())
	suite.Add(flowTransferRate())
	suite.Add(flowSelfPayment())
	suite.Add(flowLimitQuality())
	suite.Add(flowLineQuality())
	suite.Add(flowUnfundedOffer())
	suite.Add(flowCircularXRP())
	suite.Add(flowPaymentWithTicket())
	suite.Add(flowCrossCurrencyPayment())
	suite.Add(flowDeliverMin())

	// Path finding
	suite.Add(pathDirectNoIntermediary())
	suite.Add(pathFindBasic())
	suite.Add(pathPaymentAutoPathFind())
	suite.Add(pathNoPath())
	suite.Add(pathTrustAutoClear())
	suite.Add(pathSourceCurrencyLimits())
	suite.Add(pathHybridOfferPath())
	suite.Add(pathQualitySetAndTest())

	// SetRegularKey
	suite.Add(setRegularKey())
	suite.Add(revokeRegularKey())
	suite.Add(disableMasterKey())
	suite.Add(reEnableMasterKey())
	suite.Add(regularKeyWithTicket())

	xrplsim.MustRun(xrplsim.New(), suite)
}
