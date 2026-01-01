APP_NAME := lyenv
PKG_MAIN := ./cmd/lyenv
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILDTIME:= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS  := -X lyenv/internal/version.Version=$(VERSION) \
            -X lyenv/internal/version.Commit=$(COMMIT) \
            -X lyenv/internal/version.BuildTime=$(BUILDTIME)

BIN_DIR  := ./dist
BIN_PATH := $(BIN_DIR)/$(APP_NAME)

.PHONY: all build clean install uninstall

all: build

build:
	@mkdir -p $(BIN_DIR)
	@echo "Building $(APP_NAME) $(VERSION) (commit $(COMMIT))..."
	@go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_PATH) $(PKG_MAIN)
	@echo "Binary: $(BIN_PATH)"

clean:
	@rm -rf $(BIN_DIR)
	@echo "Clean done."

install: build
	@bash scripts/install.sh "$(BIN_PATH)" "$(APP_NAME)"

uninstall:
	@bash scripts/uninstall.sh "$(APP_NAME)"
