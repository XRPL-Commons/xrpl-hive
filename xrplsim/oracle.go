package xrplsim

import (
	"context"
	"fmt"
	"time"
)

// Oracle compares ledger hashes across multiple XRPL nodes to detect
// consensus divergences.
type Oracle struct {
	nodes []OracleNode
}

// OracleNode is a named XRPL node with an RPC client, used by the Oracle.
type OracleNode struct {
	Name   string
	Client *RPCClient
}

// LedgerComparison holds the result of comparing a single ledger across nodes.
type LedgerComparison struct {
	Sequence    int               `json:"sequence"`
	Agreed      bool              `json:"agreed"`
	NodeHashes  map[string]string `json:"node_hashes"`
	Divergences []string          `json:"divergences,omitempty"`
	Errors      []string          `json:"errors,omitempty"`
}

// NewOracle creates a new Oracle that compares ledger state across the
// given nodes.
func NewOracle(nodes []OracleNode) *Oracle {
	return &Oracle{nodes: nodes}
}

// CompareAtSequence queries all nodes for a specific ledger sequence and
// compares their hashes. It waits for each node to have validated that
// sequence before fetching, using a default timeout of 120 seconds.
func (o *Oracle) CompareAtSequence(ctx context.Context, seq int) (*LedgerComparison, error) {
	result := &LedgerComparison{
		Sequence:   seq,
		Agreed:     true,
		NodeHashes: make(map[string]string, len(o.nodes)),
	}

	// Wait for all nodes to have validated this sequence.
	timeout := 120 * time.Second
	for _, node := range o.nodes {
		if err := o.waitForValidated(ctx, node, seq, timeout); err != nil {
			return nil, fmt.Errorf("node %s: %w", node.Name, err)
		}
	}

	// Fetch the ledger hash from each node.
	for _, node := range o.nodes {
		ledger, err := node.Client.Ledger(seq)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", node.Name, err))
			result.Agreed = false
			continue
		}
		result.NodeHashes[node.Name] = ledger.LedgerHash
	}

	// Compare all hashes. Use the first node's hash as reference.
	if len(result.NodeHashes) < 2 {
		return result, nil
	}

	var refName, refHash string
	for name, hash := range result.NodeHashes {
		if refName == "" {
			refName = name
			refHash = hash
			continue
		}
		if hash != refHash {
			result.Agreed = false
			result.Divergences = append(result.Divergences,
				fmt.Sprintf("ledger %d: %s has %s but %s has %s",
					seq, refName, refHash, name, hash))
		}
	}

	return result, nil
}

// waitForValidated polls a node until it has validated the given sequence.
func (o *Oracle) waitForValidated(ctx context.Context, node OracleNode, seq int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for validated seq >= %d", seq)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		info, err := node.Client.ServerInfo()
		if err != nil {
			// Node might not be ready yet, retry.
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if info.Validated.Seq >= seq {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
}
