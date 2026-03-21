// Package iotest provides a data-driven RPC test framework using .io files.
//
// An .io file contains a sequence of JSON-RPC request/response pairs:
//
//	// Comment describing the test
//	// speconly: true
//	>> {"method":"server_info","params":[{}]}
//	<< {"result":{"info":{"server_state":"..."}}}
//
// Lines starting with >> are requests sent to the server.
// Lines starting with << are expected responses.
// Lines starting with // are comments (only before the first >> or <<).
// The string "..." in expected values acts as a wildcard (matches anything).
// The "speconly:" flag in comments enables structure-only validation.
package iotest

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// IOTest represents a parsed .io test file.
type IOTest struct {
	Name     string        // Test name derived from file path.
	Comment  string        // Comment block from header.
	SpecOnly bool          // If true, only validate response structure.
	Messages []IOMessage   // Sequence of request/response messages.
	FilePath string        // Original file path.
}

// IOMessage is a single request or expected response.
type IOMessage struct {
	Data string // JSON string.
	Send bool   // true = request (>>), false = expected response (<<).
}

// ParseFile parses an .io test file.
func ParseFile(path string) (*IOTest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	test := &IOTest{FilePath: path}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	inHeader := true
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip empty lines.
		if trimmed == "" {
			continue
		}

		// Header comments.
		if inHeader && strings.HasPrefix(trimmed, "//") {
			comment := strings.TrimPrefix(trimmed, "//")
			comment = strings.TrimLeft(comment, " ")
			test.Comment += comment + "\n"
			if strings.Contains(strings.ToLower(comment), "speconly:") {
				test.SpecOnly = true
			}
			continue
		}

		// Request line.
		if strings.HasPrefix(trimmed, ">>") {
			inHeader = false
			data := strings.TrimSpace(trimmed[2:])
			if data == "" {
				return nil, fmt.Errorf("%s:%d: empty request", path, lineNum)
			}
			test.Messages = append(test.Messages, IOMessage{Data: data, Send: true})
			continue
		}

		// Response line.
		if strings.HasPrefix(trimmed, "<<") {
			inHeader = false
			data := strings.TrimSpace(trimmed[2:])
			if data == "" {
				return nil, fmt.Errorf("%s:%d: empty response", path, lineNum)
			}
			test.Messages = append(test.Messages, IOMessage{Data: data, Send: false})
			continue
		}

		// Ignore comments after header.
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		return nil, fmt.Errorf("%s:%d: unexpected line: %s", path, lineNum, trimmed)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: scan error: %w", path, err)
	}

	if len(test.Messages) == 0 {
		return nil, fmt.Errorf("%s: no request/response messages found", path)
	}

	return test, nil
}
