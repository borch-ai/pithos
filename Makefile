.PHONY: all build patch-gomod install test test-integration clean lint fmt tidy check-coverage vuln install-hooks

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

# patch-gomod creates a ../powerword symlink in CI environments only.
# Background: go.mod contains `replace github.com/borch-ai/powerword => ../powerword`
# so the Go toolchain expects the sibling directory at that path. We cannot use
# `go mod edit -replace=github.com/borch-ai/powerword=./powerword` because that
# modifies go.mod and causes the CI workflow's `git diff --exit-code go.mod go.sum`
# hygiene check to fail. The symlink is the only approach that satisfies the replace
# directive without altering the tracked go.mod file. The $CI guard ensures this
# never runs locally.
patch-gomod:
	@if [ -n "$$CI" ]; then \
		if [ -d "../powerword" ]; then \
			echo "CI detected: ../powerword already exists, no patch needed"; \
		elif [ -d "./powerword" ]; then \
			echo "CI detected: creating symlink ../powerword -> ./powerword"; \
			if [ -e "../powerword" ] && [ ! -L "../powerword" ]; then \
				echo "ERROR: ../powerword exists and is not a symlink; refusing to overwrite" >&2; exit 1; \
			fi; \
			rm -f ../powerword; \
			ln -sf "$(CURDIR)/powerword" ../powerword; \
		else \
			echo "ERROR: running in CI but neither ../powerword nor ./powerword exists; cannot satisfy go.mod replace directive" >&2; exit 1; \
		fi; \
	fi

build: patch-gomod
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_PATH)

install: patch-gomod
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(MAIN_PATH)

test: patch-gomod
	@echo "Running tests..."
	@rm -f coverage.out
	$(GOTEST) -v -race -coverprofile=coverage.out -coverpkg=./internal/... ./...

test-integration: patch-gomod
	@echo "Running integration tests..."
	$(GOTEST) -v -run="Test.*Pricing" ./internal/pipeline/...

check-coverage: test
	@if command -v powerword >/dev/null 2>&1; then \
		powerword check-coverage $(MIN_COVERAGE) coverage.out; \
	else \
		echo "Warning: 'powerword' CLI not found. Falling back to local awk verification..."; \
		go tool cover -func=coverage.out | awk -v min="$(MIN_COVERAGE)" 'BEGIN {matched=0} /total:/ {matched=1; print $$0; gsub("%","",$$NF); if($$NF < min) {print "FAIL: coverage " $$NF "% is below threshold " min "%"; exit 1} else {print "PASS: coverage " $$NF "% meets threshold " min "%"; exit 0}} END {if(matched==0) {print "Error: total coverage line not found or go tool cover failed"; exit 1}}'; \
	fi

lint: patch-gomod
	@if command -v powerword >/dev/null 2>&1; then \
		echo "Running linter via powerword..."; \
		powerword lint-go; \
	else \
		echo "Warning: 'powerword' CLI not found. Falling back to local golangci-lint..."; \
		if command -v golangci-lint >/dev/null 2>&1; then \
			if [ -f "../powerword/.golangci.yml" ]; then \
				golangci-lint run --config=../powerword/.golangci.yml; \
			else \
				golangci-lint run; \
			fi; \
		else \
			echo "Warning: golangci-lint not installed, running basic go vet..."; \
			$(GOCMD) vet ./...; \
		fi; \
	fi

vuln: patch-gomod
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

tidy: patch-gomod
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
