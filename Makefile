BINARY=tokfence
CMD=./cmd/tokfence

.PHONY: build test install lint

build:
	@mkdir -p bin
	go build -o bin/$(BINARY) $(CMD)

test:
	go test ./...

install:
	go install $(CMD)

lint:
	@command -v golangci-lint >/dev/null 2>&1 || (echo "golangci-lint not found" && exit 1)
	golangci-lint run
