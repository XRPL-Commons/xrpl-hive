package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

const asfDefaultRipple = 8

// fixtureRunner executes a single fixture against a running client via RPC.
type fixtureRunner struct {
	t           *xrplsim.T
	rpc         *xrplsim.RPCClient
	client      *xrplsim.Client
	clientType  string
	acctMgr     *AccountManager
	fixture     *Fixture
	fixturesDir string
}

// run executes all steps in the fixture sequentially.
func (r *fixtureRunner) run() {
	for i, step := range r.fixture.Steps {
		if r.t.Failed() {
			return
		}
		switch step.Op {
		case "fund":
			r.execFund(i, step)
		case "trust":
			r.execTrust(i, step)
		case "close":
			r.execClose(i)
		case "tx":
			r.execTx(i, step)
		case "env_reset":
			r.execEnvReset(i, step)
		case "enable_amendment":
			r.execEnableAmendment(i, step)
		case "modify_state":
			// Should have been filtered out at discovery time.
			r.t.Fatalf("step %d: modify_state not supported via RPC", i)
		case "retry":
			r.execRetry(i, step)
		default:
			r.t.Fatalf("step %d: unknown op %q", i, step.Op)
		}
	}
}

// execFund creates and optionally configures an account.
func (r *fixtureRunner) execFund(idx int, step Step) {
	// Resolve account secret via wallet_propose.
	info, err := r.acctMgr.Resolve(step.Account, step.Address)
	if err != nil {
		r.t.Fatalf("step %d (fund %s): %v", idx, step.Account, err)
		return
	}

	// Parse amount.
	amount, err := parseDropsAmount(step.Amount)
	if err != nil {
		r.t.Fatalf("step %d (fund %s): %v", idx, step.Account, err)
		return
	}

	// Set DefaultRipple if needed (default true).
	setDefaultRipple := step.SetDefaultRipple == nil || *step.SetDefaultRipple

	// If DefaultRipple will be set, add the AccountSet fee to the fund amount
	// so the account ends up with the exact balance the fixture expects.
	// rippled's test env reimburses this fee via direct ledger manipulation;
	// we compensate by over-funding.
	fundAmount := amount
	if setDefaultRipple {
		var n uint64
		fmt.Sscanf(amount, "%d", &n)
		fundAmount = fmt.Sprintf("%d", n+10)
	}

	// Send Payment from genesis to create the account.
	result, err := r.rpc.SubmitPayment(genesisSecret, genesisAddress, info.Address, fundAmount)
	if err != nil {
		r.t.Fatalf("step %d (fund %s): payment submit: %v", idx, step.Account, err)
		return
	}
	if result.EngineResult != "tesSUCCESS" {
		r.t.Fatalf("step %d (fund %s): payment got %s: %s", idx, step.Account, result.EngineResult, result.EngineResultMessage)
		return
	}

	if setDefaultRipple {
		result, err := r.rpc.SubmitAccountSet(info.Secret, info.Address, asfDefaultRipple)
		if err != nil {
			r.t.Fatalf("step %d (fund %s): account_set submit: %v", idx, step.Account, err)
			return
		}
		if result.EngineResult != "tesSUCCESS" {
			r.t.Fatalf("step %d (fund %s): account_set got %s: %s", idx, step.Account, result.EngineResult, result.EngineResultMessage)
			return
		}
	}

	r.t.Logf("step %d: funded %s (%s) with %s drops (defaultRipple=%v)", idx, step.Account, info.Address, amount, setDefaultRipple)
}

// execTrust creates a trust line.
func (r *fixtureRunner) execTrust(idx int, step Step) {
	info := r.acctMgr.Get(step.Account)
	if info == nil {
		// Try resolving if not cached yet.
		var err error
		info, err = r.acctMgr.Resolve(step.Account, step.Address)
		if err != nil {
			r.t.Fatalf("step %d (trust %s): %v", idx, step.Account, err)
			return
		}
	}

	if step.LimitAmount == nil {
		r.t.Fatalf("step %d (trust %s): missing limit_amount", idx, step.Account)
		return
	}

	result, err := r.rpc.SubmitTrustSet(
		info.Secret, info.Address,
		step.LimitAmount.Currency, step.LimitAmount.Issuer, step.LimitAmount.Value,
	)
	if err != nil {
		r.t.Fatalf("step %d (trust %s): submit: %v", idx, step.Account, err)
		return
	}
	if result.EngineResult != "tesSUCCESS" {
		r.t.Fatalf("step %d (trust %s): got %s: %s", idx, step.Account, result.EngineResult, result.EngineResultMessage)
		return
	}

	r.t.Logf("step %d: trust %s → %s/%s limit=%s", idx, step.Account, step.LimitAmount.Currency, step.LimitAmount.Issuer, step.LimitAmount.Value)
}

// execClose advances the ledger.
func (r *fixtureRunner) execClose(idx int) {
	_, err := r.rpc.Call("ledger_accept", nil)
	if err != nil {
		r.t.Fatalf("step %d (close): %v", idx, err)
	}
}

// execTx submits a pre-signed transaction blob and validates the result.
func (r *fixtureRunner) execTx(idx int, step Step) {
	// Handle empty blob.
	if step.TxBlob == "" {
		if strings.HasPrefix(step.ExpectTER, "tem") || step.ExpectTER == "telENV_RPC_FAILED" {
			r.t.Logf("step %d (tx): empty blob, expected %s — skipping", idx, step.ExpectTER)
			return
		}
		r.t.Fatalf("step %d (tx): empty tx_blob with unexpected TER %s", idx, step.ExpectTER)
		return
	}

	result, err := r.rpc.SubmitBlob(step.TxBlob)
	if err != nil {
		// RPC-level error (e.g., invalid blob). Accept for tem*/tel* expectations.
		if strings.HasPrefix(step.ExpectTER, "tem") || step.ExpectTER == "telENV_RPC_FAILED" {
			r.t.Logf("step %d (tx): RPC error (expected %s): %v", idx, step.ExpectTER, err)
			return
		}
		r.t.Fatalf("step %d (tx): submit: %v", idx, err)
		return
	}

	// Check TER code.
	if !r.matchTER(result.EngineResult, step.ExpectTER) {
		r.t.Errorf("step %d (tx): TER mismatch: got %q, want %q (%s)", idx, result.EngineResult, step.ExpectTER, result.EngineResultMessage)
	} else {
		r.t.Logf("step %d (tx): %s (expected %s)", idx, result.EngineResult, step.ExpectTER)
	}

	// Validate post-state for applied transactions (tes* or tec*).
	if step.PostState != nil && isApplied(result.EngineResult) {
		r.validatePostState(idx, step.PostState)
	}
}

// execEnvReset stops the current client and starts a fresh one with new config.
func (r *fixtureRunner) execEnvReset(idx int, step Step) {
	r.t.Logf("step %d: env_reset — restarting client", idx)

	env := step.Env
	if env == nil && r.fixture.Env != nil {
		env = r.fixture.Env
	}

	r.client, r.rpc = startNodeWithEnv(r.t, r.clientType, env)
	r.acctMgr.Reset(r.rpc)
}

// execEnableAmendment enables an amendment via the feature admin RPC.
func (r *fixtureRunner) execEnableAmendment(idx int, step Step) {
	if err := r.rpc.Feature(step.Amendment, false); err != nil {
		r.t.Fatalf("step %d (enable_amendment %s): %v", idx, step.Amendment, err)
		return
	}
	// Close ledger to activate.
	if _, err := r.rpc.Call("ledger_accept", nil); err != nil {
		r.t.Fatalf("step %d (enable_amendment %s): ledger_accept: %v", idx, step.Amendment, err)
	}
	r.t.Logf("step %d: enabled amendment %s", idx, step.Amendment)
}

// execRetry re-submits a transaction blob.
func (r *fixtureRunner) execRetry(idx int, step Step) {
	if step.TxBlob == "" {
		r.t.Logf("step %d (retry): empty blob — skipping", idx)
		return
	}

	result, err := r.rpc.SubmitBlob(step.TxBlob)
	if err != nil {
		r.t.Errorf("step %d (retry): submit: %v", idx, err)
		return
	}

	if !r.matchTER(result.EngineResult, step.ExpectTER) {
		r.t.Errorf("step %d (retry): TER mismatch: got %q, want %q", idx, result.EngineResult, step.ExpectTER)
	} else {
		r.t.Logf("step %d (retry): %s (expected %s)", idx, result.EngineResult, step.ExpectTER)
	}

	if step.PostState != nil && isApplied(result.EngineResult) {
		r.validatePostState(idx, step.PostState)
	}
}

// validatePostState checks account balances, owner counts, sequences, and flags.
func (r *fixtureRunner) validatePostState(stepIdx int, ps *PostState) {
	for _, expected := range ps.Accounts {
		info, err := r.rpc.AccountInfoLedger(expected.Address, "current")
		if err != nil {
			r.t.Errorf("step %d post_state: account_info(%s): %v", stepIdx, expected.Address, err)
			continue
		}

		if info.Balance != expected.XRPBalance {
			r.t.Errorf("step %d post_state %s (%s): balance got %s, want %s",
				stepIdx, expected.Name, expected.Address, info.Balance, expected.XRPBalance)
		}

		if info.OwnerCount != expected.OwnerCount {
			r.t.Errorf("step %d post_state %s (%s): owner_count got %d, want %d",
				stepIdx, expected.Name, expected.Address, info.OwnerCount, expected.OwnerCount)
		}

		if expected.Sequence != nil && uint32(info.Sequence) != *expected.Sequence {
			r.t.Errorf("step %d post_state %s (%s): sequence got %d, want %d",
				stepIdx, expected.Name, expected.Address, info.Sequence, *expected.Sequence)
		}

		if expected.Flags != nil && info.Flags != *expected.Flags {
			r.t.Errorf("step %d post_state %s (%s): flags got %d, want %d",
				stepIdx, expected.Name, expected.Address, info.Flags, *expected.Flags)
		}
	}
}

// matchTER compares actual vs expected TER codes with special-case handling.
func (r *fixtureRunner) matchTER(got, want string) bool {
	if got == want {
		return true
	}

	// telENV_RPC_FAILED: accept any non-applied result.
	if want == "telENV_RPC_FAILED" {
		return !isApplied(got)
	}

	return false
}

// isApplied returns true if the TER code indicates the tx was applied.
func isApplied(ter string) bool {
	return strings.HasPrefix(ter, "tes") || strings.HasPrefix(ter, "tec")
}

// startNodeWithEnv starts a standalone XRPL node with the given environment config.
func startNodeWithEnv(t *xrplsim.T, clientType string, env *EnvConfig) (*xrplsim.Client, *xrplsim.RPCClient) {
	params := xrplsim.Params{
		"XRPL_STANDALONE":   "1",
		"XRPL_LOGLEVEL":     "3",
		"XRPL_PEER_PRIVATE": "1",
	}

	// Fixture tx_blobs are pre-signed without NetworkID.
	// Set network_id=0 so rippled doesn't require it in transactions.
	if env != nil && env.NetworkID != nil {
		params["XRPL_NETWORK_ID"] = fmt.Sprintf("%d", *env.NetworkID)
	} else {
		params["XRPL_NETWORK_ID"] = "0"
	}

	if env != nil && len(env.AmendmentsEnabled) > 0 {
		params["XRPL_FEATURES"] = strings.Join(env.AmendmentsEnabled, ",")
	} else {
		params["XRPL_FEATURES"] = "all"
	}

	c := t.StartClient(clientType, params)
	rpc := xrplsim.NewRPCClient(c.RPCEndpoint())

	// Wait for node readiness.
	for i := 0; i < 30; i++ {
		if _, err := rpc.ServerInfo(); err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	// Advance to the fixture's initial ledger sequence.
	// rippled standalone starts with open ledger 2 (genesis = ledger 1).
	// Each ledger_accept increments the open ledger by 1.
	// With DeletableAccounts, account sequences start at open ledger index,
	// so we must match the fixture's expected starting point exactly.
	targetOpen := uint32(3) // default: open ledger 3
	if env != nil && env.InitialLedgerSeq != nil {
		targetOpen = *env.InitialLedgerSeq
	}
	// Close until the open ledger reaches the target.
	for {
		raw, err := rpc.Call("ledger_current", nil)
		if err != nil {
			break
		}
		var resp struct {
			LedgerCurrentIndex uint32 `json:"ledger_current_index"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			break
		}
		if resp.LedgerCurrentIndex >= targetOpen {
			break
		}
		rpc.Call("ledger_accept", nil)
		time.Sleep(100 * time.Millisecond)
	}

	return c, rpc
}

// run executes the fixture, including any dependency chain.
func (r *fixtureRunner) runWithDependencies() {
	if r.fixture.DependsOn != "" {
		depPath := r.fixturesDir + "/" + r.fixture.DependsOn
		depFixture, err := loadFixture(depPath)
		if err != nil {
			r.t.Fatalf("failed to load dependency %s: %v", r.fixture.DependsOn, err)
			return
		}
		// Replay prerequisite steps first (on the same node).
		depRunner := &fixtureRunner{
			t:           r.t,
			rpc:         r.rpc,
			client:      r.client,
			clientType:  r.clientType,
			acctMgr:     r.acctMgr,
			fixture:     depFixture,
			fixturesDir: r.fixturesDir,
		}
		depRunner.runWithDependencies()
		if r.t.Failed() {
			return
		}
	}
	r.run()
}

// loadFixture reads and parses a fixture JSON file.
func loadFixture(path string) (*Fixture, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixture %s: %w", path, err)
	}
	var f Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse fixture %s: %w", path, err)
	}
	return &f, nil
}
