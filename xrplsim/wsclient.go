// Package xrplsim provides XRPL-specific helpers for hive simulators.
//
// wsclient.go implements a WebSocket client for XRPL nodes.
package xrplsim

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WSClient is a WebSocket client for XRPL nodes.
type WSClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
	id   int
}

// NewWSClient dials the given WebSocket endpoint and returns a client.
func NewWSClient(endpoint string) (*WSClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", endpoint, err)
	}
	return &WSClient{conn: conn}, nil
}

// wsRequest is a WebSocket JSON-RPC request.
type wsRequest struct {
	ID      int         `json:"id"`
	Command string      `json:"command"`
	Params  interface{} `json:"-"`
}

// Call sends a synchronous JSON-RPC request over WebSocket and returns the response.
func (c *WSClient) Call(command string, params map[string]interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	c.id++
	id := c.id
	c.mu.Unlock()

	// Build request: merge params into the top-level object.
	req := map[string]interface{}{
		"id":      id,
		"command": command,
	}
	for k, v := range params {
		req[k] = v
	}

	if err := c.conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("write %s: %w", command, err)
	}

	// Read response matching our ID.
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("read response for %s: %w", command, err)
		}

		var resp struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Status string          `json:"status"`
			Type   string          `json:"type"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal response: %w", err)
		}

		// Skip stream notifications (they have type but no matching id).
		if resp.Type != "" && resp.ID != id {
			continue
		}

		if resp.ID == id {
			return resp.Result, nil
		}
	}
}

// Subscribe sends a subscribe command for the given streams.
func (c *WSClient) Subscribe(streams []string) error {
	_, err := c.Call("subscribe", map[string]interface{}{
		"streams": streams,
	})
	return err
}

// Unsubscribe sends an unsubscribe command for the given streams.
func (c *WSClient) Unsubscribe(streams []string) error {
	_, err := c.Call("unsubscribe", map[string]interface{}{
		"streams": streams,
	})
	return err
}

// ReadMessage reads the next WebSocket message with a timeout.
// This is used to receive stream notifications after subscribing.
func (c *WSClient) ReadMessage(timeout time.Duration) (json.RawMessage, error) {
	c.conn.SetReadDeadline(time.Now().Add(timeout))
	_, data, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// Close closes the WebSocket connection.
func (c *WSClient) Close() error {
	return c.conn.Close()
}
