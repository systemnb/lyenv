# Application configuration
APP_NAME := lyenv
PKG_MAIN := ./cmd/lyenv
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDTIME:= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Build flags
LDFLAGS  := -X lyenv/internal/version.Version=$(VERSION) \
            -X lyenv/internal/version.Commit=$(COMMIT) \
            -X lyenv/internal/version.BuildTime=$(BUILDTIME)

# Directories
BIN_DIR  := ./dist
BIN_PATH := $(BIN_DIR)/$(APP_NAME)

# Platform detection
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

# Go command detection
GO_CMD := go
GO_VERSION_MIN := 1.21

# Colors for better output
RED    := \033[0;31m
GREEN  := \033[0;32m
YELLOW := \033[0;33m
BLUE   := \033[0;34m
CYAN   := \033[0;36m
RESET  := \033[0m

.PHONY: all build build-all clean install uninstall check-go check-go-version help install-go ensure-go

.DEFAULT_GOAL := help

help: ## Show this help message
	@echo "$(GREEN)$(APP_NAME) Build System$(RESET)"
	@echo
	@echo "$(YELLOW)Usage:$(RESET)"
	@echo "  make [target]"
	@echo
	@echo "$(YELLOW)Targets:$(RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(RESET) %s\n", $$1, $$2}'
	@echo

check-go: ## Check if Go is installed
	@echo "$(BLUE)Checking Go installation...$(RESET)"
	@if ! command -v $(GO_CMD) >/dev/null 2>&1; then \
		echo "$(RED)ERROR: Go is not installed!$(RESET)"; \
		echo "$(CYAN)Attempting to install Go...$(RESET)"; \
		$(MAKE) install-go; \
		if ! command -v $(GO_CMD) >/dev/null 2>&1; then \
			echo "$(RED)Failed to install Go automatically$(RESET)"; \
			echo "$(YELLOW)Please install Go manually: https://golang.org/dl/$(RESET)"; \
			exit 127; \
		fi; \
	fi
	@echo "$(GREEN)✓ Go is installed$(RESET)"

check-go-version: check-go ## Check if Go version is sufficient
	@echo "$(BLUE)Checking Go version...$(RESET)"
	@GO_VERSION=$$($(GO_CMD) version | sed -E 's/.*go([0-9]+\.[0-9]+(\.[0-9]+)?).*/\1/'); \
	GO_VERSION_MAJOR=$$(echo $$GO_VERSION | cut -d. -f1); \
	GO_VERSION_MINOR=$$(echo $$GO_VERSION | cut -d. -f2); \
	REQUIRED_MAJOR=$$(echo $(GO_VERSION_MIN) | cut -d. -f1); \
	REQUIRED_MINOR=$$(echo $(GO_VERSION_MIN) | cut -d. -f2); \
	if [ $$GO_VERSION_MAJOR -lt $$REQUIRED_MAJOR ] || \
	   { [ $$GO_VERSION_MAJOR -eq $$REQUIRED_MAJOR ] && [ $$GO_VERSION_MINOR -lt $$REQUIRED_MINOR ]; }; then \
		echo "$(RED)ERROR: Go version $$GO_VERSION is too old!$(RESET)"; \
		echo "$(YELLOW)Required: Go $(GO_VERSION_MIN) or newer$(RESET)"; \
		echo "$(CYAN)Attempting to upgrade Go...$(RESET)"; \
		$(MAKE) install-go; \
		GO_VERSION=$$($(GO_CMD) version | sed -E 's/.*go([0-9]+\.[0-9]+(\.[0-9]+)?).*/\1/'); \
		GO_VERSION_MAJOR=$$(echo $$GO_VERSION | cut -d. -f1); \
		GO_VERSION_MINOR=$$(echo $$GO_VERSION | cut -d. -f2); \
		if [ $$GO_VERSION_MAJOR -lt $$REQUIRED_MAJOR ] || \
		   { [ $$GO_VERSION_MAJOR -eq $$REQUIRED_MAJOR ] && [ $$GO_VERSION_MINOR -lt $$REQUIRED_MINOR ]; }; then \
			echo "$(RED)Failed to upgrade Go to required version$(RESET)"; \
			echo "$(YELLOW)Please upgrade Go manually: https://golang.org/dl/$(RESET)"; \
			exit 1; \
		fi; \
	fi
	@echo "$(GREEN)✓ Go version $$GO_VERSION meets requirement ($(GO_VERSION_MIN) min)$(RESET)"

install-go: ## Install or upgrade Go automatically
	@echo "$(CYAN)Installing/upgrading Go...$(RESET)"
	@if [ "$(UNAME_S)" = "Darwin" ]; then \
		echo "$(YELLOW)macOS detected, using Homebrew...$(RESET)"; \
		if command -v brew >/dev/null 2>&1; then \
			echo "Upgrading Go via Homebrew..."; \
			brew update; \
			brew upgrade go; \
		else \
			echo "$(RED)Homebrew not found. Please install Homebrew first.$(RESET)"; \
			echo "Visit: https://brew.sh/"; \
			exit 1; \
		fi; \
	elif [ "$(UNAME_S)" = "Linux" ]; then \
		echo "$(YELLOW)Linux detected, checking distribution...$(RESET)"; \
		if [ -f /etc/os-release ]; then \
			. /etc/os-release; \
			if [ "$$ID" = "ubuntu" ] || [ "$$ID" = "debian" ]; then \
				echo "Ubuntu/Debian detected, adding PPA and installing Go..."; \
				echo "$(YELLOW)Adding longsleep/golang-backports PPA...$(RESET)"; \
				sudo apt-get update; \
				sudo apt-get install -y software-properties-common; \
				sudo add-apt-repository -y ppa:longsleep/golang-backports; \
				sudo apt-get update; \
				echo "$(YELLOW)Installing Go $(GO_VERSION_MIN)+...$(RESET)"; \
				sudo apt-get install -y golang-go; \
			elif [ "$$ID" = "centos" ] || [ "$$ID" = "rhel" ] || [ "$$ID" = "fedora" ]; then \
				echo "RHEL/CentOS/Fedora detected, installing via package manager..."; \
				if command -v dnf >/dev/null 2>&1; then \
					sudo dnf install -y golang; \
				elif command -v yum >/dev/null 2>&1; then \
					sudo yum install -y golang; \
				fi; \
			elif [ "$$ID" = "arch" ] || [ "$$ID" = "manjaro" ]; then \
				echo "Arch/Manjaro detected, installing via pacman..."; \
				sudo pacman -Sy --noconfirm go; \
			else \
				echo "$(RED)Unsupported Linux distribution.$(RESET)"; \
				echo "Please install Go manually from: https://golang.org/dl/"; \
				exit 1; \
			fi; \
		else \
			echo "$(RED)Could not detect Linux distribution.$(RESET)"; \
			echo "Please install Go manually from: https://golang.org/dl/"; \
			exit 1; \
		fi; \
	elif [ "$(UNAME_S)" = "Windows_NT" ]; then \
		echo "$(RED)Windows detected - cannot install Go automatically.$(RESET)"; \
		echo "Please download and install Go from: https://golang.org/dl/"; \
		exit 1; \
	else \
		echo "$(RED)Unsupported operating system: $(UNAME_S)$(RESET)"; \
		echo "Please install Go manually from: https://golang.org/dl/"; \
		exit 1; \
	fi
	@echo "$(GREEN)✓ Go installation/upgrade complete$(RESET)"

ensure-go: check-go-version ## Ensure Go is installed and at the right version
	@echo "$(GREEN)✓ Go is ready$(RESET)"

all: build ## Build the application (default)

build: ensure-go ## Build the application for current platform
	@echo "$(BLUE)Building $(APP_NAME) $(VERSION) (commit $(COMMIT))...$(RESET)"
	@mkdir -p $(BIN_DIR)
	@$(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_PATH) $(PKG_MAIN)
	@echo "$(GREEN)✓ Build complete$(RESET)"
	@echo "  Binary: $(BIN_PATH)"
	@echo "  Platform: $(UNAME_S)/$(UNAME_M)"

build-all: ensure-go ## Build for all major platforms
	@echo "$(BLUE)Building $(APP_NAME) for all platforms...$(RESET)"
	@mkdir -p $(BIN_DIR)
	@echo "$(YELLOW)Building for Linux...$(RESET)"
	@GOOS=linux GOARCH=amd64 $(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(APP_NAME)-linux-amd64 $(PKG_MAIN)
	@GOOS=linux GOARCH=arm64 $(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(APP_NAME)-linux-arm64 $(PKG_MAIN)
	@echo "$(YELLOW)Building for macOS...$(RESET)"
	@GOOS=darwin GOARCH=amd64 $(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(APP_NAME)-darwin-amd64 $(PKG_MAIN)
	@GOOS=darwin GOARCH=arm64 $(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(APP_NAME)-darwin-arm64 $(PKG_MAIN)
	@echo "$(YELLOW)Building for Windows...$(RESET)"
	@GOOS=windows GOARCH=amd64 $(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(APP_NAME)-windows-amd64.exe $(PKG_MAIN)
	@echo "$(GREEN)✓ Cross-compilation complete$(RESET)"
	@echo "  Binaries are in $(BIN_DIR)/"
	@ls -la $(BIN_DIR)/

clean: ## Clean build artifacts
	@echo "$(BLUE)Cleaning...$(RESET)"
	@rm -rf $(BIN_DIR)
	@$(GO_CMD) clean -cache
	@echo "$(GREEN)✓ Clean complete$(RESET)"

install: build ## Build and install the application
	@echo "$(BLUE)Installing $(APP_NAME)...$(RESET)"
	@if [ -f "scripts/install.sh" ]; then \
		bash scripts/install.sh "$(BIN_PATH)" "$(APP_NAME)"; \
	else \
		echo "$(YELLOW)Warning: install.sh not found, copying binary...$(RESET)"; \
		if [ "$(UNAME_S)" = "Darwin" ] || [ "$(UNAME_S)" = "Linux" ]; then \
			sudo cp $(BIN_PATH) /usr/local/bin/; \
			echo "$(GREEN)✓ Installed to /usr/local/bin/$(APP_NAME)$(RESET)"; \
		else \
			echo "$(RED)ERROR: Automatic installation not supported for $(UNAME_S)$(RESET)"; \
			exit 1; \
		fi; \
	fi

uninstall: ## Uninstall the application
	@echo "$(BLUE)Uninstalling $(APP_NAME)...$(RESET)"
	@if [ -f "scripts/uninstall.sh" ]; then \
		bash scripts/uninstall.sh "$(APP_NAME)"; \
	else \
		if [ "$(UNAME_S)" = "Darwin" ] || [ "$(UNAME_S)" = "Linux" ]; then \
			if [ -f "/usr/local/bin/$(APP_NAME)" ]; then \
				sudo rm -f /usr/local/bin/$(APP_NAME); \
				echo "$(GREEN)✓ Uninstalled from /usr/local/bin/$(RESET)"; \
			else \
				echo "$(YELLOW)Not installed in /usr/local/bin/$(RESET)"; \
			fi; \
		else \
			echo "$(RED)ERROR: Automatic uninstallation not supported for $(UNAME_S)$(RESET)"; \
			exit 1; \
		fi; \
	fi

test: ensure-go ## Run tests
	@echo "$(BLUE)Running tests...$(RESET)"
	@$(GO_CMD) test ./... -v
	@echo "$(GREEN)✓ Tests complete$(RESET)"

deps: ensure-go ## Download dependencies
	@echo "$(BLUE)Downloading dependencies...$(RESET)"
	@$(GO_CMD) mod download
	@echo "$(GREEN)✓ Dependencies downloaded$(RESET)"

tidy: ensure-go ## Tidy go.mod
	@echo "$(BLUE)Tidying go.mod...$(RESET)"
	@$(GO_CMD) mod tidy
	@echo "$(GREEN)✓ go.mod tidy complete$(RESET)"

fmt: ensure-go ## Format Go code
	@echo "$(BLUE)Formatting code...$(RESET)"
	@$(GO_CMD) fmt ./...
	@echo "$(GREEN)✓ Format complete$(RESET)"

lint: ensure-go ## Run linter (if available)
	@echo "$(BLUE)Running linter...$(RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "$(YELLOW)golangci-lint not found, installing...$(RESET)"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin; \
		golangci-lint run; \
	fi
	@echo "$(GREEN)✓ Lint complete$(RESET)"

# Quick development commands
dev: ensure-go ## Build and run for development
	@echo "$(BLUE)Running in development mode...$(RESET)"
	@$(GO_CMD) run $(PKG_MAIN)

watch: ensure-go ## Watch for changes and rebuild (requires fswatch on macOS or inotify-tools on Linux)
	@echo "$(BLUE)Watching for changes...$(RESET)"
	@if [ "$(UNAME_S)" = "Darwin" ]; then \
		if ! command -v fswatch >/dev/null 2>&1; then \
			echo "$(YELLOW)Installing fswatch...$(RESET)"; \
			brew install fswatch; \
		fi; \
		fswatch -o . | while read; do make build; done; \
	elif [ "$(UNAME_S)" = "Linux" ]; then \
		if ! command -v inotifywait >/dev/null 2>&1; then \
			echo "$(YELLOW)Installing inotify-tools...$(RESET)"; \
			if command -v apt >/dev/null 2>&1; then \
				sudo apt install inotify-tools; \
			elif command -v yum >/dev/null 2>&1; then \
				sudo yum install inotify-tools; \
			fi; \
		fi; \
		while true; do \
			inotifywait -r -e modify -e create -e delete .; \
			make build; \
		done; \
	else \
		echo "$(RED)File watching not supported on $(UNAME_S)$(RESET)"; \
	fi