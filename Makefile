VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/hettiger/crannog/crannog/cmd.version=$(VERSION)

.PHONY: all build test vet fmt fmt-check check clean THIRD_PARTY_NOTICES

all: check build

# crannog runs on macOS on Apple Silicon only (ADR 0008).
build:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/ ./...

# On macOS the tests run in Seatbelt and can write only to the temporary
# directory and the Go build cache (ADR 0030). The modules are downloaded
# before: the tests cannot write to the module cache. Without $TMPDIR, Go
# uses /tmp; cd with an empty argument would stay in the repository.
ifeq ($(shell uname -s),Darwin)
test:
	go mod download
	mkdir -p "$$(go env GOCACHE)"
	/usr/bin/sandbox-exec -f internal/seatbelt/test.sb \
		-D "TEMP_DIR=$$(cd -P -- "$${TMPDIR:-/tmp}" && pwd -P)" \
		-D "GO_CACHE=$$(cd -P -- "$$(go env GOCACHE)" && pwd -P)" \
		go test ./...
else
test:
	go test ./...
endif

vet:
	go vet ./...

fmt:
	gofmt -w .

fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "$$out"; exit 1; fi

check: fmt-check vet test

clean:
	rm -rf dist

THIRD_PARTY_NOTICES:
	rm -rf THIRD_PARTY_NOTICES
	go run github.com/google/go-licenses/v2@v2.0.1 \
		save ./... \
		--ignore github.com/hettiger/crannog \
		--save_path=THIRD_PARTY_NOTICES
