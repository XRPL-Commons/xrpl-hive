.PHONY: build test clean smoke full

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

# Dev mode — start API only, no simulators
dev: build
	./bin/xrpl-hive --dev --client rippled,goxrpl

# Clean build artifacts and containers
clean:
	rm -rf bin/ workspace/
	./bin/xrpl-hive --cleanup 2>/dev/null || true
