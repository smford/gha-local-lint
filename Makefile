BINARY_NAME=gha-local-lint
BIN_DIR=bin
SRC_CMD=./cmd/gha-local-lint

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.3.0")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS = -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: all build test bench clean install lint-self lint-scripts pre-commit

all: test build

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) $(SRC_CMD)

test:
	go test -v -race ./...

bench:
	go test -bench=. -benchmem ./...

lint-scripts:
	shellcheck scripts/*.sh

pre-commit:
	pre-commit run --all-files

clean:
	rm -rf $(BIN_DIR)

install:
	CGO_ENABLED=0 go install -ldflags="$(LDFLAGS)" $(SRC_CMD)

lint-self: build
	./$(BIN_DIR)/$(BINARY_NAME) check --require-sha-pin .github/workflows/*.yml
