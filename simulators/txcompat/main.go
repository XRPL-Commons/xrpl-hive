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

	// SetRegularKey
	suite.Add(setRegularKey())
	suite.Add(revokeRegularKey())
	suite.Add(disableMasterKey())
	suite.Add(reEnableMasterKey())
	suite.Add(regularKeyWithTicket())

	xrplsim.MustRun(xrplsim.New(), suite)
}
