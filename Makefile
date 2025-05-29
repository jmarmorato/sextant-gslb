# Makefile for GSLB project

# Binary names
BIN_DIR := bin
RESOLVER := $(BIN_DIR)/resolver
HEALTHCHECK := $(BIN_DIR)/healthcheck

# Package paths
RESOLVER_PKG := ./cmd/resolver
HEALTHCHECK_PKG := ./cmd/healthcheck

# Default target: build both
.PHONY: all
all: $(RESOLVER) $(HEALTHCHECK)

# Build resolver binary
$(RESOLVER): 
	@echo "Building resolver..."
	@mkdir -p $(BIN_DIR)
	go build -o $(RESOLVER) $(RESOLVER_PKG)

# Build healthcheck binary
$(HEALTHCHECK): 
	@echo "Building healthcheck..."
	@mkdir -p $(BIN_DIR)
	go build -o $(HEALTHCHECK) $(HEALTHCHECK_PKG)

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning binaries..."
	@rm -rf $(BIN_DIR)

# Run a specific binary
.PHONY: run-resolver run-healthcheck
run-resolver: $(RESOLVER)
	@$(RESOLVER)

run-healthcheck: $(HEALTHCHECK)
	@$(HEALTHCHECK)