package main

import (
	"encoding/json"
	"fmt"
)

// Fixture represents a single v2 fixture file extracted from rippled's test suite.
type Fixture struct {
	RippledVersion string    `json:"rippled_version"`
	Suite          string    `json:"suite"`
	Testcase       string    `json:"testcase"`
	DependsOn      string    `json:"depends_on,omitempty"`
	Env            *EnvConfig `json:"env,omitempty"`
	Steps          []Step    `json:"steps"`
}

// EnvConfig holds the ledger environment configuration.
type EnvConfig struct {
	AmendmentsEnabled []string `json:"amendments_enabled"`
	BaseFee           uint64   `json:"base_fee"`
	ReserveBase       uint64   `json:"reserve_base"`
	ReserveIncrement  uint64   `json:"reserve_increment"`
	NetworkID         *uint32  `json:"network_id,omitempty"`
	InitialLedgerSeq  *uint32  `json:"initial_ledger_seq,omitempty"`
}

// Step represents a single operation in a fixture.
type Step struct {
	Op string `json:"op"`

	// fund fields
	Account          string          `json:"account,omitempty"`
	Address          string          `json:"address,omitempty"`
	Amount           json.RawMessage `json:"amount,omitempty"`
	SetDefaultRipple *bool           `json:"set_default_ripple,omitempty"`

	// trust fields
	LimitAmount *LimitAmount `json:"limit_amount,omitempty"`

	// tx fields
	TxBlob    string          `json:"tx_blob,omitempty"`
	TxJSON    json.RawMessage `json:"tx_json,omitempty"`
	ExpectTER string          `json:"expect_ter,omitempty"`
	PostState *PostState      `json:"post_state,omitempty"`

	// v2 timing/sequencing fields
	LedgerSeq       *uint32 `json:"ledger_seq,omitempty"`
	ParentCloseTime *uint32 `json:"parent_close_time,omitempty"`
	CloseTime       *uint32 `json:"close_time,omitempty"`

	// env_reset fields
	Env *EnvConfig `json:"env,omitempty"`

	// enable_amendment fields
	Amendment string `json:"amendment,omitempty"`

	// modify_state fields
	ModifyState *ModifyState `json:"modify_state,omitempty"`
}

// LimitAmount represents a trust line limit.
type LimitAmount struct {
	Currency string `json:"currency"`
	Issuer   string `json:"issuer"`
	Value    string `json:"value"`
}

// PostState holds the expected account states after a step.
type PostState struct {
	Accounts []AccountState `json:"accounts"`
}

// AccountState holds expected state for a single account.
type AccountState struct {
	Name       string  `json:"name"`
	Address    string  `json:"address"`
	XRPBalance string  `json:"xrp_balance"`
	OwnerCount uint32  `json:"owner_count"`
	Sequence   *uint32 `json:"sequence,omitempty"`
	Flags      *uint32 `json:"flags,omitempty"`
}

// ModifyState represents direct ledger manipulation (not available via RPC).
type ModifyState struct {
	Account string `json:"account,omitempty"`
}

// parseDropsAmount parses the amount field from a fund step.
// It handles both string ("1000000000") and integer (1000000000) representations.
func parseDropsAmount(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var n uint64
	if err := json.Unmarshal(raw, &n); err == nil {
		return fmt.Sprintf("%d", n), nil
	}
	return "", fmt.Errorf("invalid amount: %s", string(raw))
}

// hasModifyState returns true if any step in the fixture uses modify_state.
func hasModifyState(steps []Step) bool {
	for _, s := range steps {
		if s.Op == "modify_state" {
			return true
		}
	}
	return false
}
