BINARY=tokfence
CMD=./cmd/tokfence
DESKTOP_DIR=./apps/TokfenceDesktop

.PHONY: build test install lint desktop-generate desktop-build

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

desktop-generate:
	cd $(DESKTOP_DIR) && xcodegen generate

desktop-build:
	cd $(DESKTOP_DIR) && xcodebuild -project TokfenceDesktop.xcodeproj -scheme TokfenceDesktop -configuration Debug CODE_SIGNING_ALLOWED=NO build
