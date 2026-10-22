BINARY_NAME=gha-local-lint
BIN_DIR=bin
SRC_CMD=./cmd/gha-local-lint

.PHONY: all build test clean install lint-self

all: test build

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) $(SRC_CMD)

test:
	go test -v -race ./...

clean:
	rm -rf $(BIN_DIR)

install:
	CGO_ENABLED=0 go install $(SRC_CMD)

lint-self: build
	./$(BIN_DIR)/$(BINARY_NAME) check testdata/sample-pipeline.yml
