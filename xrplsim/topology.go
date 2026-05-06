package xrplsim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Pre-generated validator keypairs (from xrpl-confluence topology.star).
var DefaultValidators = []ValidatorKey{
	{Seed: "sneWFZcEqA8TUA5BmJ38xsqaR7dFb", PubKey: "n9LXMXFTeVL6o9fxdFHfeVZWf6YzWCBzt7YyeK1HV7wZ4ZFRNgUV"},
	{Seed: "snjbY5o3g4zK8dtotD6wjdNV3i96r", PubKey: "n9KTo9UAFTV2XPZG8oUbuwNBhvwVF2fkyxz9jE88iGhJVoV3Sxy4"},
	{Seed: "sn8KuG4fs84rowCsqTuz6AtqEkmJ7", PubKey: "n9KVs96MmgjXmok33PNEr29xbRAfvqvw1HqQYGsWE9zBdJMYJ9Pc"},
	{Seed: "sha6zPXQHAEwVk1qEREAxZPqy7h5Z", PubKey: "n9KRLEqrFzXi5yK3XE6NUhcFx8XLHWZg3SczPb8doFCiryPSmvfr"},
	{Seed: "snPRr5dyXnYYZ4idydxHxhm2qnohc", PubKey: "n9Jjt6fFpdTzms5tpYAf2iFyQwXNZWrQgwtrbwQEvFWQN4kfRFPb"},
}

const (
	// GenesisAddress is the hard-coded genesis account on XRPL test networks.
	GenesisAddress = "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"
	// GenesisSecret is the master seed for the genesis account.
	GenesisSecret = "snoPBrXtMeMyMHUVTgbuqAfg1SUTb"
	// DefaultNetworkID is used for private test networks.
	DefaultNetworkID = 10000
	// DefaultPeerPort is the default XRPL peer protocol port.
	DefaultPeerPort = 51235
	// DefaultRPCPort is the default XRPL JSON-RPC port.
	DefaultRPCPort = 5005
	// DefaultWSPort is the default XRPL WebSocket port.
	DefaultWSPort = 6006
)

// ValidatorKey holds a validator's seed and public key.
type ValidatorKey struct {
	Seed   string
	PubKey string
}

// Topology describes a set of validators forming a private XRPL network.
type Topology struct {
	Validators []ValidatorKey
	NetworkID  int
}

// NewTopology creates a topology with the first numValidators entries from
// DefaultValidators. If numValidators exceeds the number of pre-generated
// keys, it is clamped to len(DefaultValidators).
func NewTopology(numValidators int) *Topology {
	if numValidators > len(DefaultValidators) {
		numValidators = len(DefaultValidators)
	}
	validators := make([]ValidatorKey, numValidators)
	copy(validators, DefaultValidators[:numValidators])
	return &Topology{
		Validators: validators,
		NetworkID:  DefaultNetworkID,
	}
}

// validatorsJSONPayload is the shape expected by rippled/goXRPLd configs.
type validatorsJSONPayload struct {
	Validators []string `json:"validators"`
}

// ValidatorsJSON returns the validator list as JSON in the format:
//
//	{"validators": ["n9LXM...", "n9KTo...", ...]}
func (t *Topology) ValidatorsJSON() []byte {
	keys := make([]string, len(t.Validators))
	for i, v := range t.Validators {
		keys[i] = v.PubKey
	}
	data, _ := json.Marshal(validatorsJSONPayload{Validators: keys})
	return data
}

// EnvForNode returns environment variable parameters for the node at the
// given index. peerAddrs should contain the "ip:port" addresses of other
// peers (boot nodes) that this node should connect to.
//
// The returned Params map contains:
//   - XRPL_NETWORK_ID
//   - XRPL_VALIDATOR_SEED (for the validator at index)
//   - XRPL_BOOTNODE (comma-separated peer addresses, if any)
//   - XRPL_PEER_PRIVATE=1
//   - XRPL_LOGLEVEL=3
func (t *Topology) EnvForNode(index int, peerAddrs []string) Params {
	p := Params{
		"XRPL_NETWORK_ID":   fmt.Sprintf("%d", t.NetworkID),
		"XRPL_PEER_PRIVATE": "1",
		"XRPL_LOGLEVEL":     "5",
	}

	if index >= 0 && index < len(t.Validators) {
		p["XRPL_VALIDATOR_SEED"] = t.Validators[index].Seed
	}

	if len(peerAddrs) > 0 {
		p["XRPL_BOOTNODE"] = strings.Join(peerAddrs, ",")
	}

	// For single-validator networks, set quorum to 1 so the node
	// can validate ledgers by itself.
	if len(t.Validators) == 1 {
		p["XRPL_VALIDATION_QUORUM"] = "1"
	}

	return p
}

// WithValidatorConfig returns a StartOption that configures a node as a
// validator using the topology. It sets the appropriate environment
// variables for the node at nodeIndex, uploads the validators.json UNL
// file, and connects to the given peer addresses.
func WithValidatorConfig(topo *Topology, nodeIndex int, peerAddrs []string) StartOption {
	return Bundle(
		topo.EnvForNode(nodeIndex, peerAddrs),
		WithDynamicFile("/xrpl/validators.json", func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(topo.ValidatorsJSON())), nil
		}),
	)
}
