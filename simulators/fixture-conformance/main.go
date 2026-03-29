package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xrpl-commons/xrpl-hive/xrplsim"
)

// fixtureEntry holds metadata for a discovered fixture file.
type fixtureEntry struct {
	Path     string // absolute path to the JSON file
	Suite    string // e.g., "ripple.app.Escrow"
	Testcase string // e.g., "Lockup"
}

func main() {
	fixturesDir := os.Getenv("FIXTURES_DIR")
	if fixturesDir == "" {
		fixturesDir = "/fixtures"
	}

	// Auto-detect the versioned subdirectory (e.g., /fixtures/rippled-2.6.2).
	fixturesDir = detectFixturesRoot(fixturesDir)

	entries, skipped := discoverFixtures(fixturesDir)
	fmt.Printf("fixture-conformance: discovered %d fixtures (%d skipped due to modify_state)\n", len(entries), skipped)

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "no fixtures found — check FIXTURES_DIR or fixture path")
		os.Exit(1)
	}

	suite := xrplsim.Suite{
		Name:        "fixture-conformance",
		Description: "Replay xrpl-fixtures test vectors via RPC for cross-implementation conformance testing.",
	}

	for _, entry := range entries {
		entry := entry // capture loop variable
		suite.Add(xrplsim.TestSpec{
			Name:        fmt.Sprintf("%s/%s", entry.Suite, entry.Testcase),
			Description: fmt.Sprintf("Replay %s/%s from xrpl-fixtures", entry.Suite, entry.Testcase),
			Run: func(t *xrplsim.T) {
				fixture, err := loadFixture(entry.Path)
				if err != nil {
					t.Fatal(err)
					return
				}

				// Get available client types and run against each.
				clients, err := t.Sim.ClientTypes()
				if err != nil || len(clients) == 0 {
					t.Fatal("no client types available")
					return
				}

				for _, ct := range clients {
					ct := ct
					t.Run(xrplsim.TestSpec{
						Name: ct.Name,
						Run: func(t *xrplsim.T) {
							client, rpc := startNodeWithEnv(t, ct.Name, fixture.Env)
							runner := &fixtureRunner{
								t:           t,
								rpc:         rpc,
								client:      client,
								clientType:  ct.Name,
								acctMgr:     NewAccountManager(rpc),
								fixture:     fixture,
								fixturesDir: fixturesDir,
							}
							runner.runWithDependencies()
						},
					})
				}
			},
		})
	}

	xrplsim.MustRun(xrplsim.New(), suite)
}

// detectFixturesRoot finds the actual fixtures root.
// If fixturesDir contains a single versioned subdirectory (e.g., "rippled-2.6.2"),
// return that subdirectory. Otherwise return fixturesDir as-is.
func detectFixturesRoot(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return dir
	}

	// Check if dir directly contains app/ or ledger/ subdirectories.
	for _, e := range entries {
		if e.IsDir() && (e.Name() == "app" || e.Name() == "ledger") {
			return dir
		}
	}

	// Look one level deeper for a versioned directory containing app/.
	for _, e := range entries {
		if e.IsDir() {
			candidate := filepath.Join(dir, e.Name())
			sub, err := os.ReadDir(candidate)
			if err != nil {
				continue
			}
			for _, s := range sub {
				if s.IsDir() && s.Name() == "app" {
					return candidate
				}
			}
		}
	}
	return dir
}

// discoverFixtures walks the fixtures directory and returns all valid fixture entries,
// skipping those that contain modify_state steps.
func discoverFixtures(root string) (entries []fixtureEntry, skipped int) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		data, err := readFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot read %s: %v\n", path, err)
			return nil
		}

		// Quick parse: only extract suite, testcase, and check for modify_state.
		var header struct {
			Suite    string `json:"suite"`
			Testcase string `json:"testcase"`
			Steps    []struct {
				Op string `json:"op"`
			} `json:"steps"`
		}
		if err := json.Unmarshal(data, &header); err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot parse %s: %v\n", path, err)
			return nil
		}

		// Skip empty fixtures.
		if len(header.Steps) == 0 {
			return nil
		}

		// Skip fixtures with modify_state.
		for _, s := range header.Steps {
			if s.Op == "modify_state" {
				skipped++
				return nil
			}
		}

		entries = append(entries, fixtureEntry{
			Path:     path,
			Suite:    header.Suite,
			Testcase: header.Testcase,
		})
		return nil
	})
	return
}

// readFile reads the entire contents of a file.
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
