# Makefile for lyenv
# All comments in English for cross-platform readability.

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
REPO_BIN  := $(TOOLS_DIR)/repo

# Optional: allow user to override install prefix when PM can’t be detected.
# Example: make deps PKG_INSTALL="sudo apt install -y"
PKG_INSTALL ?=

.PHONY: all build clean install uninstall deps ensure-deps print-os print-pm

all: build

# ---- Dependencies (OS packages + repo script) ------------------------------

# Main entry to ensure all deps
deps: ensure-deps

ensure-deps: print-os print-pm $(REPO_BIN)
	@echo "Dependencies are ensured."

# Detect OS and package manager; store hints in files under dist/
print-os:
	@mkdir -p $(BIN_DIR)
	@echo "Detecting OS via /etc/os-release ..."
	@if [ -f /etc/os-release ]; then \
		. /etc/os-release; \
		echo "os_id=$$ID" > $(BIN_DIR)/os_detect.txt; \
		echo "os_version_id=$$VERSION_ID" >> $(BIN_DIR)/os_detect.txt; \
		echo "Detected: $$ID $$VERSION_ID"; \
	else \
		echo "os_id=unknown" > $(BIN_DIR)/os_detect.txt; \
		echo "os_version_id=unknown" >> $(BIN_DIR)/os_detect.txt; \
		echo "Warning: /etc/os-release not found. OS unknown."; \
	fi

print-pm:
	@mkdir -p $(BIN_DIR)
	@echo "Detecting package manager ..."
	@PM=""; \
	if command -v apt-get >/dev/null 2>&1; then PM="apt-get"; \
	elif command -v dnf >/dev/null 2>&1; then PM="dnf"; \
	elif command -v yum >/dev/null 2>&1; then PM="yum"; \
	elif command -v zypper >/dev/null 2>&1; then PM="zypper"; \
	elif command -v pacman >/dev/null 2>&1; then PM="pacman"; \
	elif command -v apk >/dev/null 2>&1; then PM="apk"; \
	else PM=""; fi; \
	echo "pkg_manager_name=$$PM" > $(BIN_DIR)/pm_detect.txt; \
	if [ -z "$$PM" ]; then \
		echo "No known package manager detected."; \
		if [ -z "$(PKG_INSTALL)" ]; then \
			echo ""; \
			echo "Please provide an install prefix via PKG_INSTALL (e.g., 'sudo apt install -y')."; \
			echo "Usage: make deps PKG_INSTALL='sudo apt install -y'"; \
		else \
			echo "Using user-provided PKG_INSTALL='$(PKG_INSTALL)'"; \
		fi; \
	else \
		echo "Detected package manager: $$PM"; \
	fi

# Download 'repo' tool to project-local tools dir
$(REPO_BIN):
	@mkdir -p $(TOOLS_DIR)
	@echo "Ensuring 'repo' tool locally at $(REPO_BIN) ..."
	@if [ -s "$(REPO_BIN)" ]; then \
		echo "'repo' already exists."; \
	else \
		# Try TUNA mirror first if user prefers; fall back to official.
		REPO_URL_PRIMARY="https://storage.googleapis.com/git-repo-downloads/repo"; \
		REPO_URL_TUNA="https://mirrors.tuna.tsinghua.edu.cn/git-repo-downloads/repo"; \
		DOWNLOAD_OK=""; \
		for URL in "$$REPO_URL_TUNA" "$$REPO_URL_PRIMARY"; do \
			if command -v curl >/dev/null 2>&1; then \
				echo "Downloading via curl: $$URL"; \
				if curl -L "$$URL" -o "$(REPO_BIN)"; then DOWNLOAD_OK="yes"; break; fi; \
			elif command -v wget >/dev/null 2>&1; then \
				echo "Downloading via wget: $$URL"; \
				if wget -O "$(REPO_BIN)" "$$URL"; then DOWNLOAD_OK="yes"; break; fi; \
			else \
				echo "Error: Neither curl nor wget found; cannot download 'repo'."; \
				break; \
			fi; \
		done; \
		if [ -z "$$DOWNLOAD_OK" ]; then \
			echo "Failed to download 'repo' from known URLs."; \
			echo "Please manually place the script at $(REPO_BIN)."; \
			exit 1; \
		fi; \
		chmod +x "$(REPO_BIN)"; \
		echo "Saved 'repo' to $(REPO_BIN)."; \
	fi

# Install OS-level packages (maps differ per PM). This rule is invoked by 'build' before compilation.
# It gracefully handles missing PM by using PKG_INSTALL when provided.
install-deps: print-pm
	@set -e; \
	PM=$$(sed -n 's/^pkg_manager_name=//p' $(BIN_DIR)/pm_detect.txt); \
	echo "Installing packages via PM='$$PM' ..."; \
	if [ "$$PM" = "apt-get" ]; then \
		sudo apt-get update || true; \
		sudo apt-get install -y build-essential flex bison libssl-dev libelf-dev bc python3 python-is-python3 rsync perl curl git unzip tar; \
	elif [ "$$PM" = "dnf" ]; then \
		sudo dnf install -y gcc gcc-c++ make flex bison openssl-devel elfutils-libelf-devel bc python3 rsync perl curl git unzip tar; \
	elif [ "$$PM" = "yum" ]; then \
		sudo yum install -y gcc gcc-c++ make flex bison openssl-devel elfutils-libelf-devel bc python3 rsync perl curl git unzip tar; \
	elif [ "$$PM" = "zypper" ]; then \
		sudo zypper refresh || true; \
		sudo zypper install -y gcc gcc-c++ make flex bison libopenssl-devel libelf-devel bc python3 rsync perl curl git unzip tar; \
	elif [ "$$PM" = "pacman" ]; then \
		sudo pacman -Sy || true; \
		sudo pacman -S --noconfirm base-devel flex bison openssl elfutils bc python rsync perl curl git unzip tar; \
	elif [ "$$PM" = "apk" ]; then \
		sudo apk update || true; \
		sudo apk add build-base flex bison openssl-dev elfutils-dev bc python3 rsync perl curl git unzip tar; \
	else \
		if [ -z "$(PKG_INSTALL)" ]; then \
			echo "No PM detected and PKG_INSTALL not provided."; \
			echo "Please re-run: make deps PKG_INSTALL='sudo apt install -y'"; \
			exit 1; \
		fi; \
		echo "Using custom installer prefix: $(PKG_INSTALL)"; \
		$(PKG_INSTALL) rsync perl curl git unzip tar flex bison bc || true; \
		# Try common dev packages (names may differ per distro): \
		$(PKG_INSTALL) gcc g++ make openssl-devel libssl-dev elfutils-libelf-devel libelf-dev python3 || true; \
	fi; \
	echo "OS packages installation attempted."

# ---- Build / Install / Uninstall / Clean -----------------------------------

build: deps install-deps
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