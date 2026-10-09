.PHONY: all build test clean install

BINARY_NAME=prv

all: test build

build:
	go build -o bin/$(BINARY_NAME) cmd/prv/main.go

test:
	go test -v ./...

clean:
	rm -rf bin/

install:
	go install cmd/prv/main.go
