VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: all build test vet fmt fmt-check check clean THIRD_PARTY_NOTICES

all: check build

# crannog runs on macOS on Apple Silicon only (ADR 0006).
build:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/ ./...

test:
	go test ./...

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
