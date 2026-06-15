.PHONY: all build test test-integration clean lint fmt tidy check-coverage vuln install-hooks

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOFMT=gofmt

# Coverage threshold
MIN_COVERAGE=91.0

# Binary parameters
BINARY_NAME=pithos
BIN_DIR=bin
MAIN_PATH=./cmd/pithos

all: lint vuln check-coverage build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_PATH)

test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out -coverpkg=./internal/... ./...

test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -run="Test.*Pricing" ./internal/pipeline/...

check-coverage: test
	@if command -v powerword >/dev/null 2>&1; then \
		powerword check-coverage $(MIN_COVERAGE) coverage.out; \
	else \
		echo "Warning: 'powerword' CLI not found. Falling back to local awk verification..."; \
		go tool cover -func=coverage.out | awk -v min="$(MIN_COVERAGE)" 'BEGIN {matched=0} /total:/ {matched=1; print $$0; gsub("%","",$$NF); if($$NF < min) {print "FAIL: coverage " $$NF "% is below threshold " min "%"; exit 1} else {print "PASS: coverage " $$NF "% meets threshold " min "%"; exit 0}} END {if(matched==0) {print "Error: total coverage line not found or go tool cover failed"; exit 1}}'; \
	fi

lint:
	@echo "Running linter via powerword..."
	powerword lint-go

vuln:
	@echo "Checking for vulnerabilities..."
	@GOBIN=$$(go env GOBIN); \
	GOPATH=$$(go env GOPATH); \
	if [ -z "$$GOBIN" ]; then GOBIN=$$GOPATH/bin; fi; \
	if [ ! -f "$$GOBIN/govulncheck" ]; then \
		echo "Installing govulncheck..."; \
		$(GOCMD) install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi; \
	$$GOBIN/govulncheck ./...

fmt:
	$(GOFMT) -w -s .

tidy:
	$(GOCMD) mod tidy

install-hooks:
	@echo "Installing git hooks..."
	@mkdir -p $$(git rev-parse --git-path hooks)
	@cp scripts/git-hooks/pre-push $$(git rev-parse --git-path hooks)/pre-push
	@chmod +x $$(git rev-parse --git-path hooks)/pre-push

clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BIN_DIR)
	rm -f coverage.out
