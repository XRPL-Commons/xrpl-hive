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

	// Escrow
	suite.Add(escrowCreateAndFinish())
	suite.Add(escrowCreateAndCancel())

	// Payment Channels
	suite.Add(payChannelCreateAndClaim())

	// NFToken
	suite.Add(nftMintAndBurn())
	suite.Add(nftCreateAndAcceptOffer())

	// Offer / DEX
	suite.Add(offerCreateCrossed())
	suite.Add(offerCancel())

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
	suite.Add(ammCreateAndDeposit())
	suite.Add(ammWithdraw())

	// AccountSet
	suite.Add(accountSetFlags())

	xrplsim.MustRun(xrplsim.New(), suite)
}
