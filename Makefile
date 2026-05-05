.PHONY: build run test lint fmt license-check

BINARY := bin/daimon
CONFIG  := examples/config.yaml

build:
	@mkdir -p bin
	go build -o $(BINARY) ./cmd/daimon

run: build
	$(BINARY) serve --config $(CONFIG)

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

license-check:
	@echo "Checking license headers..."
	@missing=$$(grep -rL "SPDX-License-Identifier: Apache-2.0" --include="*.go" . 2>/dev/null | grep -v "vendor"); \
	if [ -n "$$missing" ]; then \
		echo "Files missing license header:"; \
		echo "$$missing"; \
		exit 1; \
	fi
	@echo "License headers OK"
