# =========================
# lyenv - Minimal Makefile
# =========================

# Application
APP_NAME := lyenv
PKG_MAIN := ./cmd/lyenv

# Version info (best-effort, fallback if git/CI metadata is unavailable)
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDTIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Build flags
LDFLAGS := -X lyenv/internal/version.Version=$(VERSION) \
           -X lyenv/internal/version.Commit=$(COMMIT) \
           -X lyenv/internal/version.BuildTime=$(BUILDTIME)

# Output
BIN_DIR  := ./dist
BIN_PATH := $(BIN_DIR)/$(APP_NAME)

# Go tool
GO_CMD := go

# Colors (optional)
GREEN := \033[0;32m
YELLOW:= \033[0;33m
BLUE  := \033[0;34m
RESET := \033[0m

# Allow targeted builds (e.g., make build-target TARGET_OS=android TARGET_ARCH=arm64)
TARGET_OS   ?= $(shell $(GO_CMD) env GOOS)
TARGET_ARCH ?= $(shell $(GO_CMD) env GOARCH)

GUI_FRONTEND_DIR := ./gui-mvp/frontend
GUI_BACKEND_DIR  := ./gui-mvp/backend
GUI_BIN_NAME     := lyenv-gui
GUI_BIN_PATH     := $(BIN_DIR)/$(GUI_BIN_NAME)$(if $(filter $(TARGET_OS),windows),.exe,)

.PHONY: all help build build-target build-all clean test tidy fmt

.DEFAULT_GOAL := help

help: ## Show this help
	@echo "$(GREEN)$(APP_NAME) — Minimal Build Targets$(RESET)"
	@echo
	@echo "  make build           # Build for current platform"
	@echo "  make build-target    # Build for TARGET_OS/TARGET_ARCH (vars below)"
	@echo "  make build-all       # Build for common platforms (incl. Android)"
	@echo "  make clean           # Remove ./dist and go build cache"
	@echo "  make test            # Run tests"
	@echo "  make tidy            # go mod tidy"
	@echo "  make fmt             # go fmt ./..."
	@echo
	@echo "$(YELLOW)Variables for build-target:$(RESET)"
	@echo "  TARGET_OS   (default: $$($(GO_CMD) env GOOS))"
	@echo "  TARGET_ARCH (default: $$($(GO_CMD) env GOARCH))"
	@echo
	@echo "$(YELLOW)Examples:$(RESET)"
	@echo "  make build"
	@echo "  make build-target TARGET_OS=windows TARGET_ARCH=amd64"
	@echo "  make build-target TARGET_OS=android TARGET_ARCH=arm64"
	@echo "  make build-all"

all: build ## Default: build

build: ## Build for current platform
	@echo "$(BLUE)Building $(APP_NAME) $(VERSION) (commit $(COMMIT)) for $$(go env GOOS)/$$(go env GOARCH)...$(RESET)"
	@mkdir -p $(BIN_DIR)
	@$(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_PATH) $(PKG_MAIN)
	@echo "$(GREEN)✓ Build complete$(RESET)"
	@echo "  -> $(BIN_PATH)"

build-target: ## Build for TARGET_OS/TARGET_ARCH (e.g., make build-target TARGET_OS=android TARGET_ARCH=arm64)
	@echo "$(BLUE)Building $(APP_NAME) $(VERSION) for $(TARGET_OS)/$(TARGET_ARCH)...$(RESET)"
	@mkdir -p $(BIN_DIR)
	@GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) $(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' \
	  -o $(BIN_DIR)/$(APP_NAME)-$(TARGET_OS)-$(TARGET_ARCH)$(if $(filter $(TARGET_OS),windows),.exe,) \
	  $(PKG_MAIN)
	@echo "$(GREEN)✓ Cross build complete$(RESET)"
	@ls -la $(BIN_DIR) | sed -n '1,999p' >/dev/null 2>&1 || true

build-all: ## Build for common platforms (linux/darwin/windows/android)
	@echo "$(BLUE)Building $(APP_NAME) for common platforms...$(RESET)"
	@mkdir -p $(BIN_DIR)
	@echo "$(YELLOW)linux/amd64$(RESET)";   GOOS=linux   GOARCH=amd64 $(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(APP_NAME)-linux-amd64   $(PKG_MAIN)
	@echo "$(YELLOW)linux/arm64$(RESET)";   GOOS=linux   GOARCH=arm64 $(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(APP_NAME)-linux-arm64   $(PKG_MAIN)
	@echo "$(YELLOW)darwin/amd64$(RESET)";  GOOS=darwin  GOARCH=amd64 $(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(APP_NAME)-darwin-amd64  $(PKG_MAIN)
	@echo "$(YELLOW)darwin/arm64$(RESET)";  GOOS=darwin  GOARCH=arm64 $(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(APP_NAME)-darwin-arm64  $(PKG_MAIN)
	@echo "$(YELLOW)windows/amd64$(RESET)"; GOOS=windows GOARCH=amd64 $(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(APP_NAME)-windows-amd64.exe $(PKG_MAIN)
	@echo "$(YELLOW)android/arm64$(RESET)"; GOOS=android GOARCH=arm64 $(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(APP_NAME)-android-arm64 $(PKG_MAIN)
	@echo "$(YELLOW)android/arm$(RESET)";   GOOS=android GOARCH=arm   $(GO_CMD) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(APP_NAME)-android-arm   $(PKG_MAIN)
	@echo "$(GREEN)✓ Cross compilation complete$(RESET)"
	@ls -la $(BIN_DIR)

clean: ## Clean build artifacts and go build cache
	@echo "$(BLUE)Cleaning...$(RESET)"
	@rm -rf $(BIN_DIR)
	@$(GO_CMD) clean -cache
	@echo "$(GREEN)✓ Clean complete$(RESET)"

test: ## Run tests
	@$(GO_CMD) test ./... -v

tidy: ## go mod tidy
	@$(GO_CMD) mod tidy

fmt: ## go fmt ./...
	@$(GO_CMD) fmt ./...

PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

.PHONY: install uninstall

PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

# Termux default (user can still override PREFIX/BINDIR)
ifeq ($(shell uname -o 2>/dev/null),Android)
  PREFIX ?= /data/data/com.termux/files/usr
  BINDIR ?= $(PREFIX)/bin
endif

GUI_FRONTEND_DIR := ./gui-mvp/frontend
GUI_BACKEND_DIR  := ./gui-mvp/backend
GUI_BIN_NAME     := lyenv-gui
GUI_BIN_PATH     := $(BIN_DIR)/$(GUI_BIN_NAME)$(if $(filter $(TARGET_OS),windows),.exe,)

.PHONY: gui-assets build-gui clean-gui install uninstall

gui-assets:
	@echo "$(BLUE)Building GUI frontend (vite)...$(RESET)"
	@cd $(GUI_FRONTEND_DIR) && npm install
	@cd $(GUI_FRONTEND_DIR) && npm run build:prod
	@echo "$(GREEN)✓ GUI frontend built and copied to backend/dist$(RESET)"

build-gui: gui-assets
	@echo "$(BLUE)Building GUI backend binary...$(RESET)"
	@mkdir -p $(BIN_DIR)
	@cd $(GUI_BACKEND_DIR) && $(GO_CMD) build -trimpath -o ../../$(GUI_BIN_PATH) .
	@echo "$(GREEN)✓ GUI backend build complete$(RESET)"
	@echo "  -> $(GUI_BIN_PATH)"

install: build build-gui
	@echo "Installing $(APP_NAME) and $(GUI_BIN_NAME) to $(BINDIR)..."
	@if [ "$$OS" = "Windows_NT" ]; then \
	  echo "Windows detected."; \
	  echo "Please manually copy:"; \
	  echo "  dist\\$(APP_NAME)-windows-amd64.exe -> a directory in PATH as lyenv.exe"; \
	  echo "  dist\\$(GUI_BIN_NAME)-windows-amd64.exe -> a directory in PATH as lyenv-gui.exe"; \
	else \
	  install -d -m 0755 "$(BINDIR)"; \
	  install -m 0755 "$(BIN_PATH)" "$(BINDIR)/$(APP_NAME)"; \
	  install -m 0755 "$(GUI_BIN_PATH)" "$(BINDIR)/$(GUI_BIN_NAME)"; \
	  echo "✔ Installed $(APP_NAME) into $(BINDIR)"; \
	  echo "✔ Installed $(GUI_BIN_NAME) into $(BINDIR)"; \
	  echo "Initializing global GUI dir: $$HOME/.lyenv/gui ..."; \
	  mkdir -p "$$HOME/.lyenv/gui/logs"; \
	  if [ ! -f "$$HOME/.lyenv/gui/config.yaml" ]; then \
	    printf '%s\n' \
	      'version: 1' \
	      'envs:' \
	      '  pinned: []' \
	      '  scan:' \
	      "    - root: \"$$HOME/lyenv-envs\"" \
	      '      depth: 3' \
	      '      pattern: "lyenv.yaml"' \
	      "    - root: \"$$HOME/projects\"" \
	      '      depth: 3' \
	      '      pattern: "lyenv.yaml"' \
	      'server:' \
	      '  addr: "127.0.0.1:18888"' \
	      > "$$HOME/.lyenv/gui/config.yaml"; \
	    echo "✔ Created $$HOME/.lyenv/gui/config.yaml"; \
	  else \
	    echo "✔ Kept existing $$HOME/.lyenv/gui/config.yaml"; \
	  fi; \
	  echo "Done."; \
	fi

uninstall:
	@echo "Uninstalling $(APP_NAME) and $(GUI_BIN_NAME) from $(BINDIR)..."
	@if [ "$$OS" = "Windows_NT" ]; then \
	  echo "Windows detected."; \
	  echo "Please manually remove lyenv(.exe) and lyenv-gui(.exe) from your PATH directory."; \
	else \
	  rm -f "$(BINDIR)/$(APP_NAME)" || true; \
	  rm -f "$(BINDIR)/$(GUI_BIN_NAME)" || true; \
	  echo "✔ Removed binaries from $(BINDIR)"; \
	  echo "NOTE: global GUI data kept at $$HOME/.lyenv/gui (remove manually if needed)."; \
	fi


# --- GUI ---
.PHONY: gui-assets build-gui clean-gui

gui-assets: ## Build GUI frontend assets and copy to backend/dist (vite build:prod)
	@echo "$(BLUE)Building GUI frontend (vite)...$(RESET)"
	@cd $(GUI_FRONTEND_DIR) && npm install
	@cd $(GUI_FRONTEND_DIR) && npm run build:prod
	@echo "$(GREEN)✓ GUI frontend built and copied to backend/dist$(RESET)"

build-gui: gui-assets ## Build GUI backend binary (lyenv-gui) into ./dist
	@echo "$(BLUE)Building GUI backend binary...$(RESET)"
	@mkdir -p $(BIN_DIR)
	@cd $(GUI_BACKEND_DIR) && $(GO_CMD) build -trimpath -o ../../$(GUI_BIN_PATH) .
	@echo "$(GREEN)✓ GUI backend build complete$(RESET)"
	@echo "  -> $(GUI_BIN_PATH)"

clean-gui: ## Remove GUI build outputs
	@rm -rf $(GUI_BACKEND_DIR)/dist
	@rm -f $(GUI_BIN_PATH)
