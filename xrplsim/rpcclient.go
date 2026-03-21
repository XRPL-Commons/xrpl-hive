// Package xrplsim provides XRPL-specific helpers for hive simulators.
//
// rpcclient.go implements a JSON-RPC client for XRPL nodes.
package xrplsim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RPCClient is a JSON-RPC client for XRPL nodes.
type RPCClient struct {
	endpoint string
	client   *http.Client
}

// NewRPCClient creates a new RPC client for the given endpoint
// (e.g. "http://10.0.0.2:5005").
func NewRPCClient(endpoint string) *RPCClient {
	return &RPCClient{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// rpcRequest is the JSON-RPC request envelope.
type rpcRequest struct {
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

// rpcResponse is the JSON-RPC response envelope.
type rpcResponse struct {
	Result json.RawMessage `json:"result"`
}

// Call invokes an RPC method and returns the raw result.
func (c *RPCClient) Call(method string, params interface{}) (json.RawMessage, error) {
	p := []interface{}{}
	if params != nil {
		p = []interface{}{params}
	} else {
		p = []interface{}{map[string]interface{}{}}
	}

	req := rpcRequest{Method: method, Params: p}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.client.Post(c.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("http post to %s: %w", c.endpoint, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(data, &rpcResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(data))
	}

	return rpcResp.Result, nil
}

// ServerInfoResult holds relevant fields from the server_info RPC response.
type ServerInfoResult struct {
	ServerState  string `json:"server_state"`
	BuildVersion string `json:"build_version"`
	Peers        int    `json:"peers"`
	Validated    struct {
		Seq  int    `json:"seq"`
		Hash string `json:"hash"`
	}
}

// ServerInfo calls server_info and returns parsed results.
func (c *RPCClient) ServerInfo() (*ServerInfoResult, error) {
	raw, err := c.Call("server_info", nil)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Info struct {
			ServerState     string `json:"server_state"`
			BuildVersion    string `json:"build_version"`
			Peers           int    `json:"peers"`
			ValidatedLedger struct {
				Seq  int    `json:"seq"`
				Hash string `json:"hash"`
			} `json:"validated_ledger"`
		} `json:"info"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("parse server_info: %w", err)
	}

	return &ServerInfoResult{
		ServerState:  wrapper.Info.ServerState,
		BuildVersion: wrapper.Info.BuildVersion,
		Peers:        wrapper.Info.Peers,
		Validated: struct {
			Seq  int    `json:"seq"`
			Hash string `json:"hash"`
		}{
			Seq:  wrapper.Info.ValidatedLedger.Seq,
			Hash: wrapper.Info.ValidatedLedger.Hash,
		},
	}, nil
}

// LedgerResult holds the root hashes for a ledger.
type LedgerResult struct {
	Seq             int    `json:"seq"`
	LedgerHash      string `json:"ledger_hash"`
	AccountHash     string `json:"account_hash"`
	TransactionHash string `json:"transaction_hash"`
}

// Ledger fetches a specific ledger by sequence number.
func (c *RPCClient) Ledger(seq int) (*LedgerResult, error) {
	raw, err := c.Call("ledger", map[string]interface{}{
		"ledger_index": seq,
	})
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		Ledger struct {
			LedgerIndex     json.RawMessage `json:"ledger_index"`
			LedgerHash      string          `json:"ledger_hash"`
			AccountHash     string          `json:"account_hash"`
			TransactionHash string          `json:"transaction_hash"`
		} `json:"ledger"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("parse ledger: %w", err)
	}

	if wrapper.Status == "error" {
		return nil, fmt.Errorf("ledger %d not found", seq)
	}

	return &LedgerResult{
		Seq:             seq,
		LedgerHash:      wrapper.Ledger.LedgerHash,
		AccountHash:     wrapper.Ledger.AccountHash,
		TransactionHash: wrapper.Ledger.TransactionHash,
	}, nil
}

// AccountInfoResult holds relevant account_info fields.
type AccountInfoResult struct {
	Account  string `json:"account"`
	Balance  string `json:"balance"`
	Sequence int    `json:"sequence"`
}

// AccountInfo fetches account info.
func (c *RPCClient) AccountInfo(account string) (*AccountInfoResult, error) {
	raw, err := c.Call("account_info", map[string]interface{}{
		"account": account,
	})
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		AccountData struct {
			Account  string `json:"Account"`
			Balance  string `json:"Balance"`
			Sequence int    `json:"Sequence"`
		} `json:"account_data"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("parse account_info: %w", err)
	}

	if wrapper.Status == "error" {
		return nil, fmt.Errorf("account %s not found", account)
	}

	return &AccountInfoResult{
		Account:  wrapper.AccountData.Account,
		Balance:  wrapper.AccountData.Balance,
		Sequence: wrapper.AccountData.Sequence,
	}, nil
}

// WalletProposeResult holds the result of wallet_propose.
type WalletProposeResult struct {
	AccountID  string `json:"account_id"`
	MasterSeed string `json:"master_seed"`
	PublicKey  string `json:"public_key"`
}

// WalletPropose generates a new wallet keypair via the node's admin RPC.
func (c *RPCClient) WalletPropose() (*WalletProposeResult, error) {
	raw, err := c.Call("wallet_propose", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		AccountID  string `json:"account_id"`
		MasterSeed string `json:"master_seed"`
		PublicKey  string `json:"public_key"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse wallet_propose: %w", err)
	}

	return &WalletProposeResult{
		AccountID:  result.AccountID,
		MasterSeed: result.MasterSeed,
		PublicKey:  result.PublicKey,
	}, nil
}

// SubmitResult holds the result of a transaction submission.
type SubmitResult struct {
	EngineResult        string `json:"engine_result"`
	EngineResultCode    int    `json:"engine_result_code"`
	EngineResultMessage string `json:"engine_result_message"`
	TxHash              string `json:"tx_hash"`
	Status              string `json:"status"`
}

// Submit submits a generic transaction using sign-and-submit.
// The txJSON map should contain the transaction fields (TransactionType, Account, etc.).
func (c *RPCClient) Submit(secret, account string, txJSON map[string]interface{}) (*SubmitResult, error) {
	// Ensure Account is set.
	tx := make(map[string]interface{}, len(txJSON)+1)
	for k, v := range txJSON {
		tx[k] = v
	}
	tx["Account"] = account

	raw, err := c.Call("submit", map[string]interface{}{
		"secret":  secret,
		"tx_json": tx,
	})
	if err != nil {
		return nil, err
	}
	return parseSubmitResult(raw)
}

// SubmitPayment submits a Payment transaction using sign-and-submit.
func (c *RPCClient) SubmitPayment(secret, from, to, amount string) (*SubmitResult, error) {
	raw, err := c.Call("submit", map[string]interface{}{
		"secret": secret,
		"tx_json": map[string]interface{}{
			"TransactionType": "Payment",
			"Account":         from,
			"Destination":     to,
			"Amount":          amount,
		},
	})
	if err != nil {
		return nil, err
	}
	return parseSubmitResult(raw)
}

// SubmitTrustSet submits a TrustSet transaction using sign-and-submit.
func (c *RPCClient) SubmitTrustSet(secret, account, currency, issuer, limit string) (*SubmitResult, error) {
	raw, err := c.Call("submit", map[string]interface{}{
		"secret": secret,
		"tx_json": map[string]interface{}{
			"TransactionType": "TrustSet",
			"Account":         account,
			"LimitAmount": map[string]interface{}{
				"currency": currency,
				"issuer":   issuer,
				"value":    limit,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return parseSubmitResult(raw)
}

// SubmitOfferCreate submits an OfferCreate transaction using sign-and-submit.
// takerPays and takerGets can be either a string (for XRP drops) or a map
// with currency/issuer/value fields.
func (c *RPCClient) SubmitOfferCreate(secret, account string, takerPays, takerGets interface{}) (*SubmitResult, error) {
	raw, err := c.Call("submit", map[string]interface{}{
		"secret": secret,
		"tx_json": map[string]interface{}{
			"TransactionType": "OfferCreate",
			"Account":         account,
			"TakerPays":       takerPays,
			"TakerGets":       takerGets,
		},
	})
	if err != nil {
		return nil, err
	}
	return parseSubmitResult(raw)
}

// SubmitAccountSet submits an AccountSet transaction using sign-and-submit.
func (c *RPCClient) SubmitAccountSet(secret, account string, setFlag uint32) (*SubmitResult, error) {
	raw, err := c.Call("submit", map[string]interface{}{
		"secret": secret,
		"tx_json": map[string]interface{}{
			"TransactionType": "AccountSet",
			"Account":         account,
			"SetFlag":         setFlag,
		},
	})
	if err != nil {
		return nil, err
	}
	return parseSubmitResult(raw)
}

// Tx fetches a transaction by its hash.
func (c *RPCClient) Tx(hash string) (json.RawMessage, error) {
	return c.Call("tx", map[string]interface{}{
		"transaction": hash,
	})
}

// Connect tells a node to connect to a peer (admin RPC).
func (c *RPCClient) Connect(ip string, port int) error {
	_, err := c.Call("connect", map[string]interface{}{
		"ip":   ip,
		"port": port,
	})
	return err
}

// WaitForLedger polls the node until it has closed a ledger with sequence >= seq.
func (c *RPCClient) WaitForLedger(ctx context.Context, seq int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for ledger seq >= %d", seq)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		info, err := c.ServerInfo()
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if info.Validated.Seq >= seq {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// WaitForValidation polls the node until the given ledger sequence is validated.
// This is functionally equivalent to WaitForLedger but named for clarity when
// the caller specifically needs validation (not just closure).
func (c *RPCClient) WaitForValidation(ctx context.Context, seq int, timeout time.Duration) error {
	return c.WaitForLedger(ctx, seq, timeout)
}

// parseSubmitResult extracts submit fields from the raw JSON-RPC response.
func parseSubmitResult(raw json.RawMessage) (*SubmitResult, error) {
	var result struct {
		EngineResult        string `json:"engine_result"`
		EngineResultCode    int    `json:"engine_result_code"`
		EngineResultMessage string `json:"engine_result_message"`
		TxJSON              struct {
			Hash string `json:"hash"`
		} `json:"tx_json"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse submit result: %w", err)
	}

	return &SubmitResult{
		EngineResult:        result.EngineResult,
		EngineResultCode:    result.EngineResultCode,
		EngineResultMessage: result.EngineResultMessage,
		TxHash:              result.TxJSON.Hash,
		Status:              result.Status,
	}, nil
}
