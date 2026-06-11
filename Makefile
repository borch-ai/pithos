.PHONY: all build test clean lint fmt tidy check-coverage check-plans vuln

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



check-coverage: test
	@go run scripts/check_coverage/main.go $(MIN_COVERAGE) coverage.out

lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, running basic go vet..."; \
		$(GOCMD) vet ./...; \
	fi

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
