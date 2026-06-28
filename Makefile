BINARY_SERVER := urapt-server
BINARY_CLI    := urapt
GO           := go
LDFLAGS      := -s -w

# Version is injected into the binary via -X. Falls back to the git describe
# output (tag or commit), then to "dev" when not in a git checkout.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
RELEASE_LDFLAGS := -s -w -X urapt/shared/version.Version=$(VERSION)

# Release targets: <goos>-<goarch>. Windows is omitted (no apt client there).
RELEASE_TARGETS := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64

.PHONY: all build build-server build-cli test vet fmt lint run-server run-cli clean tidy release release-binaries checksums

all: build

build: build-server build-cli

build-server:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY_SERVER) ./cmd/urapt-server

build-cli:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY_CLI) ./cmd/urapt

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

lint: vet
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed; skipping"

run-server: build-server
	./$(BINARY_SERVER)

run-cli: build-cli
	./$(BINARY_CLI)

tidy:
	$(GO) mod tidy

clean:
	rm -f $(BINARY_SERVER) $(BINARY_CLI)
	rm -rf bin/ dist/

# release builds static, CGO-free binaries for all release targets into dist/,
# bundles each pair (server + CLI) into a tar.gz, and produces a checksums file.
release: release-binaries checksums

release-binaries:
	@mkdir -p dist
	@for target in $(RELEASE_TARGETS); do \
		os=$${target%-*}; \
		arch=$${target#*-}; \
		echo "==> building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(RELEASE_LDFLAGS)" -o dist/$(BINARY_SERVER)-$$os-$$arch ./cmd/urapt-server; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(RELEASE_LDFLAGS)" -o dist/$(BINARY_CLI)-$$os-$$arch ./cmd/urapt; \
		tar -czf dist/urapt-$(VERSION)-$$os-$$arch.tar.gz -C dist $(BINARY_SERVER)-$$os-$$arch $(BINARY_CLI)-$$os-$$arch; \
	done
	@echo "==> release artifacts in dist/"

checksums:
	@cd dist && sha256sum *.tar.gz > checksums-$(VERSION).txt
	@echo "==> wrote dist/checksums-$(VERSION).txt"
