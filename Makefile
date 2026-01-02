# Makefile for lyenv
# Goal: ensure Go compiler exists before building lyenv.
# If 'go' is not available in PATH, download a local toolchain to ./dist/tools/go
# and use it only for this build (no system-wide changes).

APP_NAME := lyenv
PKG_MAIN := ./cmd/lyenv
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDTIME:= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS  := -X lyenv/internal/version.Version=$(VERSION) \
            -X lyenv/internal/version.Commit=$(COMMIT) \
            -X lyenv/internal/version.BuildTime=$(BUILDTIME)

BIN_DIR   := ./dist
BIN_PATH  := $(BIN_DIR)/$(APP_NAME)
TOOLS_DIR := $(BIN_DIR)/tools

# ---------------------------------------------------------------------------
# Go toolchain settings (local bootstrap when 'go' not found)
# Change GO_VERSION when you want a different toolchain.
GO_VERSION ?= 1.22.5

# Detect OS/ARCH for official Go tarballs
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

# Map uname to Go's naming
# Supported: Linux x86_64/aarch64, Darwin x86_64/arm64
GO_OS := $(shell \
	if [ "$(UNAME_S)" = "Linux" ]; then echo "linux"; \
	elif [ "$(UNAME_S)" = "Darwin" ]; then echo "darwin"; \
	else echo "unsupported"; fi)
GO_ARCH := $(shell \
	if [ "$(UNAME_M)" = "x86_64" ]; then echo "amd64"; \
	elif [ "$(UNAME_M)" = "aarch64" ] || [ "$(UNAME_M)" = "arm64" ]; then echo "arm64"; \
	else echo "unsupported"; fi)

# Local GOROOT under dist/tools/go
GO_LOCAL_ROOT := $(TOOLS_DIR)/go-$(GO_VERSION)-$(GO_OS)-$(GO_ARCH)
GO_LOCAL_BIN  := $(GO_LOCAL_ROOT)/go/bin/go
GO_TARBALL    := $(TOOLS_DIR)/go$(GO_VERSION).$(GO_OS)-$(GO_ARCH).tar.gz
GO_URL        := https://go.dev/dl/go$(GO_VERSION).$(GO_OS)-$(GO_ARCH).tar.gz

# Helper: does system 'go' exist?
HAVE_GO := $(shell command -v go >/dev/null 2>&1 && echo yes || echo no)

# Allow overriding download command if needed:
CURL := $(shell command -v curl >/dev/null 2>&1 && echo curl || echo "")
WGET := $(shell command -v wget >/dev/null 2>&1 && echo wget || echo "")

.PHONY: all build clean install uninstall go-ensure go-download go-extract go-print go-local-env

all: build

# ---------------------------------------------------------------------------
# Ensure a usable Go compiler (system or local)
go-ensure: go-print
	@set -e; \
	if [ "$(HAVE_GO)" = "yes" ]; then \
		echo "[go-ensure] Found system 'go' at: $$(command -v go)"; \
	else \
		if [ "$(GO_OS)" = "unsupported" ] || [ "$(GO_ARCH)" = "unsupported" ]; then \
			echo "[go-ensure] Unsupported platform: $(UNAME_S)/$(UNAME_M)"; \
			echo "Please install Go $(GO_VERSION) manually and ensure 'go' is in PATH."; \
			exit 1; \
		fi; \
		$(MAKE) go-download; \
		$(MAKE) go-extract; \
	fi

# Print environment summary
go-print:
	@mkdir -p $(TOOLS_DIR)
	@echo "[go-print] System OS: $(UNAME_S), ARCH: $(UNAME_M)"
	@echo "[go-print] Mapped GO_OS=$(GO_OS), GO_ARCH=$(GO_ARCH), GO_VERSION=$(GO_VERSION)"
	@echo "[go-print] System 'go' present? $(HAVE_GO)"
	@echo "[go-print] Local GOROOT: $(GO_LOCAL_ROOT)"

# Download Go tarball to tools dir (uses curl or wget)
go-download:
	@mkdir -p $(TOOLS_DIR)
	@echo "[go-download] Fetching $(GO_URL) -> $(GO_TARBALL)"
	@if [ -s "$(GO_TARBALL)" ]; then \
		echo "[go-download] Tarball already present."; \
	else \
		if [ -n "$(CURL)" ]; then \
			$(CURL) -L "$(GO_URL)" -o "$(GO_TARBALL)"; \
		elif [ -n "$(WGET)" ]; then \
			$(WGET) -O "$(GO_TARBALL)" "$(GO_URL)"; \
		else \
			echo "[go-download] Neither 'curl' nor 'wget' is available."; \
			echo "Please download $(GO_URL) manually to $(GO_TARBALL)"; \
			exit 1; \
		fi; \
	fi
	@test -s "$(GO_TARBALL)" || (echo "[go-download] Failed to download Go tarball."; exit 1)

# Extract tarball into $(GO_LOCAL_ROOT), idempotent
go-extract:
	@mkdir -p $(GO_LOCAL_ROOT)
	@echo "[go-extract] Extracting $(GO_TARBALL) to $(GO_LOCAL_ROOT)"
	@tar -xzf "$(GO_TARBALL)" -C "$(GO_LOCAL_ROOT)"
	@# After extraction, official tarball layout is: $(GO_LOCAL_ROOT)/go/bin/go
	@test -x "$(GO_LOCAL_BIN)" || (echo "[go-extract] go binary not found after extraction."; exit 1)
	@echo "[go-extract] Local Go ready at $(GO_LOCAL_BIN)"

# Expose how to use local Go (for debugging)
go-local-env:
	@echo "To use local Go:"
	@echo "  export GOROOT=$(GO_LOCAL_ROOT)/go"
	@echo "  export PATH=\$$GOROOT/bin:\$$PATH"
	@echo "  go version"

# ---------------------------------------------------------------------------
# Build lyenv (uses system 'go' if present; otherwise uses local GOROOT/PATH)
build: go-ensure
	@mkdir -p $(BIN_DIR)
	@echo "Building $(APP_NAME) $(VERSION) (commit $(COMMIT))..."
	@set -e; \
	if [ "$(HAVE_GO)" = "yes" ]; then \
		GO_BIN=$$(command -v go); \
		GOROOT=$$(go env GOROOT); \
		echo "[build] Using system Go: $$GO_BIN (GOROOT=$$GOROOT)"; \
		go version; \
		go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_PATH) $(PKG_MAIN); \
	else \
		GOROOT_LOCAL="$(GO_LOCAL_ROOT)/go"; \
		export GOROOT="$$GOROOT_LOCAL"; \
		export PATH="$$GOROOT/bin:$$PATH"; \
		echo "[build] Using local Go at $$GOROOT_LOCAL"; \
		go version; \
		go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_PATH) $(PKG_MAIN); \
	fi
	@echo "Binary: $(BIN_PATH)"

clean:
	@rm -rf $(BIN_DIR)
	@echo "Clean done."

install: build
	@bash scripts/install.sh "$(BIN_PATH)" "$(APP_NAME)"

uninstall:
	@bash scripts/uninstall.sh "$(APP_NAME)"