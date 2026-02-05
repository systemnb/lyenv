APP      := lyenv
DIST     := dist
PKG      := github.com/systemnb/lyenv/cmd/lyenv
GO       := go

ifeq ($(OS),Windows_NT)
  EXT := .exe
else
  EXT :=
endif

all: build

.PHONY: build
build:
	@mkdir -p $(DIST)
	$(GO) build -trimpath -ldflags "-s -w" -o $(DIST)/$(APP)$(EXT) $(PKG)
	@echo "Built: $(DIST)/$(APP)$(EXT)"

.PHONY: install
install: build
	@$(DIST)/$(APP)$(EXT) install

.PHONY: uninstall
uninstall:
	@$(DIST)/$(APP)$(EXT) uninstall

.PHONY: clean
clean:
	@rm -rf $(DIST)
