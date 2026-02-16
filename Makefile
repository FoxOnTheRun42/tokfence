BINARY=tokfence
CMD=./cmd/tokfence
DESKTOP_DIR=./apps/TokfenceDesktop

.PHONY: build test install lint desktop-generate desktop-build
.PHONY: smoke-e2e smoke-e2e-openai smoke-e2e-anthropic

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

smoke-e2e-openai:
	TOKFENCE_PROVIDER=openai TOKFENCE_SMOKE_KEEP_DAEMON=0 ./scripts/live-e2e.sh

smoke-e2e-anthropic:
	TOKFENCE_PROVIDER=anthropic TOKFENCE_SMOKE_KEEP_DAEMON=0 ./scripts/live-e2e.sh

smoke-e2e:
	@if [ -z "${TOKFENCE_PROVIDER:-}" ]; then \
		echo "Set TOKFENCE_PROVIDER=openai or TOKFENCE_PROVIDER=anthropic"; \
		exit 1; \
	fi
	./scripts/live-e2e.sh
