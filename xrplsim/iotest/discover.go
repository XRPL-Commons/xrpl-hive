package iotest

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Discover finds all .io test files under the given root directory,
// parses them, and returns them sorted by name.
func Discover(root string) ([]*IOTest, error) {
	var tests []*IOTest

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".io") {
			return nil
		}

		test, err := ParseFile(path)
		if err != nil {
			return err
		}

		// Derive test name from path: tests/server/ping.io -> server/ping
		rel, _ := filepath.Rel(root, path)
		test.Name = strings.TrimSuffix(filepath.ToSlash(rel), ".io")

		tests = append(tests, test)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(tests, func(i, j int) bool {
		return tests[i].Name < tests[j].Name
	})

	return tests, nil
}
