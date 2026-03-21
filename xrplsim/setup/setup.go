// Package setup provides reusable test state builders for XRPL hive simulators.
package setup

import (
	"context"
	"fmt"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

// Account holds an XRPL account address and secret.
type Account struct {
	Address string
	Secret  string
}

// FundAccount sends XRP from the genesis account to the given address.
// It waits for the transaction to be validated.
func FundAccount(ctx context.Context, rpc *xrplsim.RPCClient, address, amount string) error {
	result, err := rpc.SubmitPayment(xrplsim.GenesisSecret, xrplsim.GenesisAddress, address, amount)
	if err != nil {
		return fmt.Errorf("fund %s: submit failed: %w", address, err)
	}
	if result.EngineResult != "tesSUCCESS" {
		return fmt.Errorf("fund %s: engine result %s", address, result.EngineResult)
	}
	// Wait a few ledgers for settlement.
	return WaitSettled(ctx, rpc, 3)
}

// FundN creates N new accounts via wallet_propose and funds each with the given amount.
func FundN(ctx context.Context, rpc *xrplsim.RPCClient, n int, amount string) ([]Account, error) {
	accounts := make([]Account, 0, n)
	for i := 0; i < n; i++ {
		w, err := rpc.WalletPropose()
		if err != nil {
			return nil, fmt.Errorf("wallet_propose %d: %w", i, err)
		}
		accounts = append(accounts, Account{Address: w.AccountID, Secret: w.MasterSeed})
	}

	// Fund all accounts.
	for i, acct := range accounts {
		result, err := rpc.SubmitPayment(xrplsim.GenesisSecret, xrplsim.GenesisAddress, acct.Address, amount)
		if err != nil {
			return nil, fmt.Errorf("fund account %d (%s): %w", i, acct.Address, err)
		}
		if result.EngineResult != "tesSUCCESS" {
			return nil, fmt.Errorf("fund account %d: %s", i, result.EngineResult)
		}
	}

	// Wait for all funding to settle.
	if err := WaitSettled(ctx, rpc, 5); err != nil {
		return nil, fmt.Errorf("wait for funding: %w", err)
	}

	return accounts, nil
}

// SetupTrustLine creates a trust line from the account to the issuer.
func SetupTrustLine(ctx context.Context, rpc *xrplsim.RPCClient, account, secret, currency, issuer, limit string) error {
	result, err := rpc.SubmitTrustSet(secret, account, currency, issuer, limit)
	if err != nil {
		return fmt.Errorf("trust set: %w", err)
	}
	if result.EngineResult != "tesSUCCESS" {
		return fmt.Errorf("trust set: %s", result.EngineResult)
	}
	return WaitSettled(ctx, rpc, 3)
}

// SetupOffer creates an offer on the DEX.
func SetupOffer(ctx context.Context, rpc *xrplsim.RPCClient, account, secret string, takerPays, takerGets interface{}) error {
	result, err := rpc.SubmitOfferCreate(secret, account, takerPays, takerGets)
	if err != nil {
		return fmt.Errorf("offer create: %w", err)
	}
	if result.EngineResult != "tesSUCCESS" {
		return fmt.Errorf("offer create: %s", result.EngineResult)
	}
	return WaitSettled(ctx, rpc, 3)
}

// WaitSettled waits for ledgersAhead ledgers to close, ensuring pending transactions settle.
func WaitSettled(ctx context.Context, rpc *xrplsim.RPCClient, ledgersAhead int) error {
	info, err := rpc.ServerInfo()
	if err != nil {
		// If we can't get server_info, just wait a bit.
		time.Sleep(5 * time.Second)
		return nil
	}
	targetSeq := info.Validated.Seq + ledgersAhead
	if targetSeq < 3 {
		targetSeq = 3
	}
	return rpc.WaitForLedger(ctx, targetSeq, 60*time.Second)
}
