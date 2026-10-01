VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
PREFIX  ?= $(HOME)/.local
BINDIR  ?= $(PREFIX)/bin
MODULE  := github.com/aleslanger/claude-code-theme-designer
LDFLAGS := -s -w -X $(MODULE)/internal/cli.Version=$(VERSION)

.PHONY: build test cover lint install uninstall dist clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o claude-theme ./cmd/claude-theme

test:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1

lint:
	gofmt -l . | (! grep .) && go vet ./...

install: build
	install -d "$(BINDIR)"
	install -m 755 claude-theme "$(BINDIR)/claude-theme"
	@echo "Installed $(BINDIR)/claude-theme ($(VERSION))"

uninstall:
	rm -f "$(BINDIR)/claude-theme"
	@echo "Removed $(BINDIR)/claude-theme (designer data in ~/.config/claude-theme-designer is kept)"

dist:
	scripts/dist.sh $(VERSION)

clean:
	rm -rf claude-theme dist coverage.out
