.PHONY: all build test test-coverage coverage-html lint fmt fmt-check clean install check

BINARY_NAME=prv

all: check build

build:
	go build -o bin/$(BINARY_NAME) cmd/prv/main.go

test:
	go test -v ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

coverage-html: test-coverage
	go tool cover -html=coverage.out -o coverage.html

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Unformatted files found. Run 'make fmt'" && exit 1)

lint:
	go vet ./...
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed; ran go vet only"; \
	fi

check: fmt-check lint test

clean:
	rm -rf bin/ coverage.out coverage.html

install:
	go install cmd/prv/main.go

