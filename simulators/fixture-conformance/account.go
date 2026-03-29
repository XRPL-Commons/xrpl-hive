package main

import (
	"fmt"
	"sync"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

const (
	genesisAddress = "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"
	genesisSecret  = "snoPBrXtMeMyMHUVTgbuqAfg1SUTb"
)

// AccountInfo holds the resolved address and secret for a fixture account.
type AccountInfo struct {
	Name    string
	Address string
	Secret  string
}

// AccountManager resolves fixture account names to addresses and secrets
// using the wallet_propose RPC with passphrase derivation.
type AccountManager struct {
	rpc   *xrplsim.RPCClient
	mu    sync.Mutex
	cache map[string]*AccountInfo // keyed by account name
}

// NewAccountManager creates a new account manager.
func NewAccountManager(rpc *xrplsim.RPCClient) *AccountManager {
	return &AccountManager{
		rpc:   rpc,
		cache: make(map[string]*AccountInfo),
	}
}

// Resolve resolves an account name to its address and secret.
// It verifies the derived address matches the expected address from the fixture.
func (m *AccountManager) Resolve(name, expectedAddress string) (*AccountInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check cache first.
	if info, ok := m.cache[name]; ok {
		if info.Address != expectedAddress {
			return nil, fmt.Errorf("account %q: cached address %s != expected %s", name, info.Address, expectedAddress)
		}
		return info, nil
	}

	// Special case: master/genesis account.
	if name == "master" {
		info := &AccountInfo{
			Name:    "master",
			Address: genesisAddress,
			Secret:  genesisSecret,
		}
		m.cache[name] = info
		return info, nil
	}

	// Derive via wallet_propose with passphrase.
	result, err := m.rpc.WalletProposePassphrase(name)
	if err != nil {
		return nil, fmt.Errorf("wallet_propose for %q: %w", name, err)
	}

	if result.AccountID != expectedAddress {
		return nil, fmt.Errorf("account %q: derived address %s != expected %s", name, result.AccountID, expectedAddress)
	}

	info := &AccountInfo{
		Name:    name,
		Address: result.AccountID,
		Secret:  result.MasterSeed,
	}
	m.cache[name] = info
	return info, nil
}

// Get returns a cached account by name, or nil if not resolved yet.
func (m *AccountManager) Get(name string) *AccountInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cache[name]
}

// Reset clears the account cache (used on env_reset).
func (m *AccountManager) Reset(rpc *xrplsim.RPCClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rpc = rpc
	m.cache = make(map[string]*AccountInfo)
}
