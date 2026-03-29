.PHONY: build test clean smoke full rpccompat rpccompat-stateful sync txcompat wscompat fixture-conformance

build:
	go build -o ./bin/xrpl-hive .

test:
	go test ./...

# Run smoke tests against specified clients
smoke: build
	./bin/xrpl-hive --sim smoke --client rippled,goxrpl

# Run all simulators against all clients
full: build
	./bin/xrpl-hive --sim ".*" --client rippled,goxrpl,rxrpl

# Run propagation tests
propagation: build
	./bin/xrpl-hive --sim propagation --client rippled,goxrpl

# Run consensus tests
consensus: build
	./bin/xrpl-hive --sim consensus --client rippled,goxrpl

# Run soak tests
soak: build
	./bin/xrpl-hive --sim soak --client rippled,goxrpl --sim.timelimit 10m

# Run RPC compatibility tests
rpccompat: build
	./bin/xrpl-hive --sim rpccompat --client rippled,goxrpl

# Run stateful RPC compatibility tests
rpccompat-stateful: build
	./bin/xrpl-hive --sim rpccompat-stateful --client rippled,goxrpl

# Run sync tests
sync: build
	./bin/xrpl-hive --sim sync --client rippled,goxrpl

# Run transaction compatibility tests
txcompat: build
	./bin/xrpl-hive --sim txcompat --client rippled,goxrpl

# Run WebSocket compatibility tests
wscompat: build
	./bin/xrpl-hive --sim wscompat --client rippled,goxrpl

# Run fixture-based conformance tests (all xrpl-fixtures via RPC)
fixture-conformance: build
	./bin/xrpl-hive --sim fixture-conformance --client rippled --sim.timelimit 30m

# Dev mode — start API only, no simulators
dev: build
	./bin/xrpl-hive --dev --client rippled,goxrpl

# Clean build artifacts and containers
clean:
	rm -rf bin/ workspace/
	./bin/xrpl-hive --cleanup 2>/dev/null || true
