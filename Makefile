.PHONY: build test bench profile lint install clean

GOEXPERIMENT ?= simd

build:
	GOEXPERIMENT=$(GOEXPERIMENT) go build -o bin/gogrep ./cmd/gogrep

test:
	GOEXPERIMENT=$(GOEXPERIMENT) GOGREP_SKIP_PCRE=1 go test -race ./...
	GOEXPERIMENT=$(GOEXPERIMENT) go test ./internal/matcher/ -run "PCRE"

bench:
	GOEXPERIMENT=$(GOEXPERIMENT) go test -bench=. -benchmem ./internal/matcher/ ./internal/input/ ./internal/simd/

profile:
	./scripts/profile.sh

lint:
	GOEXPERIMENT=$(GOEXPERIMENT) go vet ./...

install:
	GOEXPERIMENT=$(GOEXPERIMENT) go install ./cmd/gogrep

clean:
	rm -rf bin/
