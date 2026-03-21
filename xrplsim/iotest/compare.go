package iotest

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CompareJSON performs semantic comparison between expected and actual JSON.
// If specOnly is true, only structure (keys and types) is validated.
// The wildcard string "..." in expected matches any value.
// Extra fields in actual are allowed (implementations may add fields).
// Returns nil if the comparison passes.
func CompareJSON(expected, actual []byte, specOnly bool) error {
	var expVal, actVal interface{}
	if err := json.Unmarshal(expected, &expVal); err != nil {
		return fmt.Errorf("invalid expected JSON: %w", err)
	}
	if err := json.Unmarshal(actual, &actVal); err != nil {
		return fmt.Errorf("invalid actual JSON: %w", err)
	}

	// Redact error_message before comparison (implementations vary in error text).
	redactErrorMessage(expVal)
	redactErrorMessage(actVal)

	var errs []string
	compareValue("", expVal, actVal, specOnly, &errs)
	if len(errs) > 0 {
		return fmt.Errorf("comparison failed:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

func compareValue(path string, expected, actual interface{}, specOnly bool, errs *[]string) {
	if expected == nil {
		if actual != nil {
			*errs = append(*errs, fmt.Sprintf("%s: expected null, got %T", pathOrRoot(path), actual))
		}
		return
	}

	switch exp := expected.(type) {
	case string:
		if exp == "..." {
			return // Wildcard — matches anything.
		}
		if specOnly {
			if _, ok := actual.(string); !ok {
				*errs = append(*errs, fmt.Sprintf("%s: expected string, got %T", pathOrRoot(path), actual))
			}
			return
		}
		act, ok := actual.(string)
		if !ok {
			*errs = append(*errs, fmt.Sprintf("%s: expected string, got %T", pathOrRoot(path), actual))
			return
		}
		if exp != act {
			*errs = append(*errs, fmt.Sprintf("%s: expected %q, got %q", pathOrRoot(path), exp, act))
		}

	case float64:
		if specOnly {
			if _, ok := actual.(float64); !ok {
				*errs = append(*errs, fmt.Sprintf("%s: expected number, got %T", pathOrRoot(path), actual))
			}
			return
		}
		act, ok := actual.(float64)
		if !ok {
			*errs = append(*errs, fmt.Sprintf("%s: expected number, got %T", pathOrRoot(path), actual))
			return
		}
		if exp != act {
			*errs = append(*errs, fmt.Sprintf("%s: expected %v, got %v", pathOrRoot(path), exp, act))
		}

	case bool:
		if specOnly {
			if _, ok := actual.(bool); !ok {
				*errs = append(*errs, fmt.Sprintf("%s: expected bool, got %T", pathOrRoot(path), actual))
			}
			return
		}
		act, ok := actual.(bool)
		if !ok {
			*errs = append(*errs, fmt.Sprintf("%s: expected bool, got %T", pathOrRoot(path), actual))
			return
		}
		if exp != act {
			*errs = append(*errs, fmt.Sprintf("%s: expected %v, got %v", pathOrRoot(path), exp, act))
		}

	case map[string]interface{}:
		act, ok := actual.(map[string]interface{})
		if !ok {
			*errs = append(*errs, fmt.Sprintf("%s: expected object, got %T", pathOrRoot(path), actual))
			return
		}
		// All keys in expected must exist in actual.
		for k, v := range exp {
			childPath := path + "." + k
			actV, exists := act[k]
			if !exists {
				*errs = append(*errs, fmt.Sprintf("%s: missing key", childPath))
				continue
			}
			compareValue(childPath, v, actV, specOnly, errs)
		}

	case []interface{}:
		act, ok := actual.([]interface{})
		if !ok {
			*errs = append(*errs, fmt.Sprintf("%s: expected array, got %T", pathOrRoot(path), actual))
			return
		}
		if specOnly {
			return // For speconly, just check it's an array.
		}
		if len(exp) != len(act) {
			*errs = append(*errs, fmt.Sprintf("%s: expected array length %d, got %d", pathOrRoot(path), len(exp), len(act)))
			return
		}
		for i := range exp {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			compareValue(childPath, exp[i], act[i], specOnly, errs)
		}
	}
}

// redactErrorMessage removes error_message from error objects
// since implementations vary in error message text.
func redactErrorMessage(v interface{}) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return
	}
	if errObj, ok := m["error"].(map[string]interface{}); ok {
		delete(errObj, "error_message")
		delete(errObj, "message")
	}
	// Also check nested result.
	if result, ok := m["result"].(map[string]interface{}); ok {
		delete(result, "error_message")
	}
	// Recurse.
	for _, child := range m {
		redactErrorMessage(child)
	}
}

func pathOrRoot(path string) string {
	if path == "" {
		return "(root)"
	}
	return path
}
