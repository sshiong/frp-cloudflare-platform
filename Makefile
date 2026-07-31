# FRP Panel - Multi-User Cloud Tunnel Management Platform
# Build targets for Server Panel and Client Panel

.PHONY: all build-server build-client build-frontend-server build-frontend-client clean test lint

# Variables
GOFLAGS := -trimpath -ldflags="-s -w"
SERVER_BINARY := frp-panel-server
CLIENT_BINARY := frp-panel-client

# Default target
all: build-server build-client

# ========== Server Panel ==========

build-server: build-frontend-server
	@echo "Building Server Panel..."
	cd server-panel && go build $(GOFLAGS) -o bin/$(SERVER_BINARY) ./cmd/control/
	cd server-panel && go build $(GOFLAGS) -o bin/$(SERVER_BINARY)-router ./cmd/router/
	@echo "Server Panel built successfully!"

build-frontend-server:
	@echo "Building Server Panel Frontend..."
	cd server-panel/web-admin && npm install && npm run build
	@echo "Server Panel Frontend built!"

run-server-control:
	cd server-panel && go run ./cmd/control/

run-server-router:
	cd server-panel && go run ./cmd/router/

# ========== Client Panel ==========

build-client: build-frontend-client
	@echo "Building Client Panel..."
	cd client-panel && go build $(GOFLAGS) -o bin/$(CLIENT_BINARY) ./cmd/client/
	@echo "Client Panel built successfully!"

build-frontend-client:
	@echo "Building Client Panel Frontend..."
	cd client-panel/web-client && npm install && npm run build
	@echo "Client Panel Frontend built!"

run-client:
	cd client-panel && go run ./cmd/client/

# ========== Development ==========

dev-server:
	@echo "Starting Server Panel in dev mode..."
	cd server-panel/web-admin && npm run dev &
	cd server-panel && go run ./cmd/control/ -dev

dev-client:
	@echo "Starting Client Panel in dev mode..."
	cd client-panel/web-client && npm run dev &
	cd client-panel && go run ./cmd/client/ -dev

# ========== Testing ==========

test:
	@echo "Running tests..."
	cd server-panel && go test ./... -v -race -count=1
	cd client-panel && go test ./... -v -race -count=1

test-server:
	cd server-panel && go test ./... -v -race -count=1

test-client:
	cd client-panel && go test ./... -v -race -count=1

test-integration:
	cd server-panel && go test ./tests/... -v -race -count=1 -tags=integration

# ========== Linting ==========

lint:
	@echo "Running linters..."
	cd server-panel && golangci-lint run ./...
	cd client-panel && golangci-lint run ./...

# ========== Clean ==========

clean:
	rm -rf server-panel/bin/ server-panel/web-admin/dist/ server-panel/web-admin/node_modules/
	rm -rf client-panel/bin/ client-panel/web-client/dist/ client-panel/web-client/node_modules/
	@echo "Cleaned build artifacts."

# ========== Cross-compilation ==========

build-all-platforms:
	@echo "Building for all platforms..."
	# Linux amd64
	cd server-panel && GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -o bin/$(SERVER_BINARY)-linux-amd64 ./cmd/control/
	cd server-panel && GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -o bin/$(SERVER_BINARY)-router-linux-amd64 ./cmd/router/
	cd client-panel && GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -o bin/$(CLIENT_BINARY)-linux-amd64 ./cmd/client/
	# Linux arm64
	cd server-panel && GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -o bin/$(SERVER_BINARY)-linux-arm64 ./cmd/control/
	cd server-panel && GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -o bin/$(SERVER_BINARY)-router-linux-arm64 ./cmd/router/
	cd client-panel && GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -o bin/$(CLIENT_BINARY)-linux-arm64 ./cmd/client/
	# Windows amd64
	cd server-panel && GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o bin/$(SERVER_BINARY)-windows-amd64.exe ./cmd/control/
	cd server-panel && GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o bin/$(SERVER_BINARY)-router-windows-amd64.exe ./cmd/router/
	cd client-panel && GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o bin/$(CLIENT_BINARY)-windows-amd64.exe ./cmd/client/
	# macOS amd64
	cd server-panel && GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) -o bin/$(SERVER_BINARY)-darwin-amd64 ./cmd/control/
	cd client-panel && GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) -o bin/$(CLIENT_BINARY)-darwin-amd64 ./cmd/client/
	# macOS arm64
	cd server-panel && GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -o bin/$(SERVER_BINARY)-darwin-arm64 ./cmd/control/
	cd client-panel && GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -o bin/$(CLIENT_BINARY)-darwin-arm64 ./cmd/client/
	@echo "All platform builds complete!"

# ========== Docker ==========

docker-build-server:
	docker build -t frp-panel-server:latest -f server-panel/Dockerfile server-panel/

docker-build-client:
	docker build -t frp-panel-client:latest -f client-panel/Dockerfile client-panel/

docker-compose-up:
	docker-compose up -d

docker-compose-down:
	docker-compose down

# ========== Database ==========

migrate-up:
	cd server-panel && go run ./cmd/control/ -migrate-up

migrate-down:
	cd server-panel && go run ./cmd/control/ -migrate-down

# ========== Help ==========

help:
	@echo "FRP Panel Build System"
	@echo ""
	@echo "Targets:"
	@echo "  all                  - Build both Server and Client panels"
	@echo "  build-server         - Build Server Panel (with frontend)"
	@echo "  build-client         - Build Client Panel (with frontend)"
	@echo "  build-frontend-server - Build Server Panel frontend only"
	@echo "  build-frontend-client - Build Client Panel frontend only"
	@echo "  run-server-control   - Run Server Control process"
	@echo "  run-server-router    - Run Server Router process"
	@echo "  run-client           - Run Client Panel"
	@echo "  dev-server           - Start Server in dev mode (hot reload)"
	@echo "  dev-client           - Start Client in dev mode (hot reload)"
	@echo "  test                 - Run all tests"
	@echo "  test-server          - Run Server tests"
	@echo "  test-client          - Run Client tests"
	@echo "  lint                 - Run linters"
	@echo "  clean                - Clean build artifacts"
	@echo "  build-all-platforms  - Cross-compile for all platforms"
	@echo "  docker-build-server  - Build Server Docker image"
	@echo "  docker-build-client  - Build Client Docker image"
	@echo "  help                 - Show this help"
