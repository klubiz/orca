.PHONY: build clean test run

BINARY=orca
BUILD_DIR=bin

build:
	@echo "Building $(BINARY)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/orca

clean:
	@rm -rf $(BUILD_DIR)
	@rm -f orca

test:
	go test ./...

run: build
	./$(BUILD_DIR)/$(BINARY) serve

install: build
	cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet

.DEFAULT_GOAL := build
