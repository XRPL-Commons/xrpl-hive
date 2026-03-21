package iotest

import (
	"encoding/json"
	"fmt"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

// RunFile executes all request/response exchanges in an .io test file
// against the given RPC client, reporting results via t.
func RunFile(t *xrplsim.T, rpc *xrplsim.RPCClient, test *IOTest) {
	var lastResponse []byte

	for i, msg := range test.Messages {
		if msg.Send {
			// Send request.
			t.Logf(">> %s", msg.Data)
			resp, err := rpc.CallRaw([]byte(msg.Data))
			if err != nil {
				t.Fatalf("exchange %d: request failed: %v", i, err)
				return
			}
			lastResponse = resp
			t.Logf("<< %s", string(resp))
		} else {
			// Validate response against expected.
			if lastResponse == nil {
				t.Fatalf("exchange %d: expected response before any request", i)
				return
			}

			err := CompareJSON([]byte(msg.Data), lastResponse, test.SpecOnly)
			if err != nil {
				t.Logf("expected: %s", msg.Data)
				t.Logf("actual:   %s", string(lastResponse))
				t.Fatalf("exchange %d: %v", i, err)
				return
			}
			lastResponse = nil
		}
	}

	// Check for unhandled response.
	if lastResponse != nil {
		t.Logf("unhandled response: %s", string(lastResponse))
	}
}

// TestName derives a test name from an IOTest (strips directory prefix and .io extension).
func TestName(test *IOTest) string {
	return test.Name
}

// Category derives a category from the test name (first path component).
func Category(test *IOTest) string {
	parts := splitPath(test.Name)
	if len(parts) > 1 {
		return parts[0]
	}
	return ""
}

func splitPath(name string) []string {
	var parts []string
	current := ""
	for _, c := range name {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// CallRawRequest extracts the method and params from a raw JSON-RPC request
// and returns the full response. This is used by the runner to send .io requests.
func CallRawRequest(rpc *xrplsim.RPCClient, requestJSON []byte) ([]byte, error) {
	var req struct {
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(requestJSON, &req); err != nil {
		return nil, fmt.Errorf("invalid request JSON: %w", err)
	}

	return rpc.CallRaw(requestJSON)
}
