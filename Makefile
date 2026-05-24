.PHONY: all build test vet fmt clean install run

BIN       ?= chimera
PREFIX    ?= /usr/local
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo v0.2.0)
BUILDTIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -s -w -X chimera/cmd.Version=$(VERSION) -X chimera/cmd.BuildTime=$(BUILDTIME)

all: build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

run: build
	./$(BIN) help

clean:
	rm -f $(BIN)

install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 0755 $(BIN) $(DESTDIR)$(PREFIX)/bin/$(BIN)
