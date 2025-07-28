# Hyperliquid WebSocket Proxy Makefile

# Variables
APP_NAME = hyperliquid-ws-proxy
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT = $(shell git rev-parse --short HEAD)
BUILD_TIME = $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS = -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME) -w -s"

# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod

# Build targets
PLATFORMS = linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
BINARY_DIR = bin

.PHONY: all build clean test deps tidy run docker docker-build docker-run compose-up compose-down compose-build compose-logs compose-restart deploy undeploy test-proxy-local test-proxy-remote test-quick test-full help

# Default target
all: clean deps test build

# Build for current platform
build:
	@echo "Building $(APP_NAME) for current platform..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DIR)/$(APP_NAME) .
	@echo "Build complete: $(BINARY_DIR)/$(APP_NAME)"

# Build for all platforms
build-all: clean
	@echo "Building $(APP_NAME) for all platforms..."
	@mkdir -p $(BINARY_DIR)
	@for platform in $(PLATFORMS); do \
		OS=$$(echo $$platform | cut -d'/' -f1); \
		ARCH=$$(echo $$platform | cut -d'/' -f2); \
		echo "Building for $$OS/$$ARCH..."; \
		CGO_ENABLED=0 GOOS=$$OS GOARCH=$$ARCH $(GOBUILD) $(LDFLAGS) \
			-o $(BINARY_DIR)/$(APP_NAME)-$$OS-$$ARCH .; \
		if [ $$OS = "windows" ]; then \
			mv $(BINARY_DIR)/$(APP_NAME)-$$OS-$$ARCH $(BINARY_DIR)/$(APP_NAME)-$$OS-$$ARCH.exe; \
		fi \
	done
	@echo "All builds complete!"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	$(GOGET) -v ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Run the application
run: build
	@echo "Running $(APP_NAME)..."
	./$(BINARY_DIR)/$(APP_NAME)

# Run with config
run-config: build
	@echo "Running $(APP_NAME) with config..."
	./$(BINARY_DIR)/$(APP_NAME) -config config.yaml

# Run with debug logging
run-debug: build
	@echo "Running $(APP_NAME) with debug logging..."
	./$(BINARY_DIR)/$(APP_NAME) -log-level debug

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BINARY_DIR)
	rm -f coverage.out coverage.html

# Docker targets
docker: docker-build

docker-build:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):$(VERSION) .
	docker tag $(APP_NAME):$(VERSION) $(APP_NAME):latest

# Legacy Docker run (deprecated - use docker-compose instead)
docker-run:
	@echo "Running Docker container..."
	docker run -d \
		--name $(APP_NAME) \
		-p 8080:8080 \
		-v $(PWD)/config.yaml:/config.yaml \
		$(APP_NAME):latest \
		-config /config.yaml

docker-stop:
	@echo "Stopping Docker container..."
	docker stop $(APP_NAME) || true
	docker rm $(APP_NAME) || true

docker-logs:
	@echo "Showing Docker logs..."
	docker logs -f $(APP_NAME)

# Docker Compose targets (recommended)
compose-up:
	@echo "Starting services with Docker Compose..."
	docker-compose up -d

compose-down:
	@echo "Stopping services with Docker Compose..."
	docker-compose down

compose-build:
	@echo "Building services with Docker Compose..."
	docker-compose build

compose-logs:
	@echo "Showing logs from Docker Compose..."
	docker-compose logs -f

compose-restart:
	@echo "Restarting services with Docker Compose..."
	docker-compose restart

# Combined commands
deploy: compose-build compose-up
	@echo "Deployment complete! WebSocket available at: ws://localhost:8080/ws"

undeploy: compose-down
	@echo "Services stopped"

# Development targets
dev: clean deps
	@echo "Starting development mode..."
	$(GOCMD) run . -log-level debug

# Test targets
test-proxy-local:
	@echo "Testing local proxy with node data..."
	@command -v node >/dev/null 2>&1 || { echo "❌ Node.js is required for tests"; exit 1; }
	@npm list ws >/dev/null 2>&1 || npm install ws
	node test-local-proxy.js

test-proxy-remote:
	@echo "Testing proxy vs official Hyperliquid API..."
	@command -v node >/dev/null 2>&1 || { echo "❌ Node.js is required for tests"; exit 1; }
	@npm list ws >/dev/null 2>&1 || npm install ws
	node test-websocket-proxy.js

test-assets:
	@echo "Testing asset fetcher and /assets endpoint..."
	@command -v node >/dev/null 2>&1 || { echo "❌ Node.js is required for tests"; exit 1; }
	@npm list axios >/dev/null 2>&1 || npm install axios
	node test-asset-fetcher.js

test-quick:
	@echo "Quick health check..."
	@curl -s http://localhost:8080/health || echo "❌ Service not responding"
	@curl -s http://localhost:8080/stats || echo "❌ Stats not available"

# Comprehensive test (build, deploy, test, cleanup)
test-full: compose-build compose-up
	@echo "Waiting for service to start..."
	@sleep 10
	@make test-quick
	@make test-assets
	@make test-proxy-local
	@make compose-down

# Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

# Lint code (requires golangci-lint)
lint:
	@echo "Linting code..."
	golangci-lint run

# Security scan (requires gosec)
security:
	@echo "Running security scan..."
	gosec ./...

# Generate mocks (requires mockgen)
mocks:
	@echo "Generating mocks..."
	go generate ./...

# Benchmark tests
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# Profile the application
profile: build
	@echo "Profiling application..."
	./$(BINARY_DIR)/$(APP_NAME) -cpuprofile cpu.prof -memprofile mem.prof

# Install the binary to GOPATH/bin
install: build
	@echo "Installing $(APP_NAME) to GOPATH/bin..."
	cp $(BINARY_DIR)/$(APP_NAME) $(GOPATH)/bin/

# Create release archives
release: build-all
	@echo "Creating release archives..."
	@mkdir -p release
	@for platform in $(PLATFORMS); do \
		OS=$$(echo $$platform | cut -d'/' -f1); \
		ARCH=$$(echo $$platform | cut -d'/' -f2); \
		BINARY_NAME=$(APP_NAME)-$$OS-$$ARCH; \
		if [ $$OS = "windows" ]; then \
			BINARY_NAME=$$BINARY_NAME.exe; \
		fi; \
		ARCHIVE_NAME=$(APP_NAME)-$(VERSION)-$$OS-$$ARCH; \
		if [ $$OS = "windows" ]; then \
			zip -j release/$$ARCHIVE_NAME.zip $(BINARY_DIR)/$$BINARY_NAME README.md config.yaml; \
		else \
			tar -czf release/$$ARCHIVE_NAME.tar.gz -C $(BINARY_DIR) $$BINARY_NAME -C .. README.md config.yaml; \
		fi; \
		echo "Created release/$$ARCHIVE_NAME"; \
	done

# Help target
help:
	@echo "Available targets:"
	@echo "  build         - Build for current platform"
	@echo "  build-all     - Build for all platforms"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Install dependencies"
	@echo "  tidy          - Tidy dependencies"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  run           - Build and run the application"
	@echo "  run-config    - Run with config file"
	@echo "  run-debug     - Run with debug logging"
	@echo "  dev           - Start development mode"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code (requires golangci-lint)"
	@echo "  security      - Security scan (requires gosec)"
	@echo "  mocks         - Generate mocks"
	@echo "  bench         - Run benchmarks"
	@echo "  profile       - Profile the application"
	@echo "  install       - Install binary to GOPATH/bin"
	@echo "  release       - Create release archives"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container (legacy)"
	@echo "  docker-stop   - Stop Docker container"
	@echo "  docker-logs   - Show Docker logs"
	@echo ""
	@echo "Docker Compose (recommended):"
	@echo "  compose-build - Build services with Docker Compose"
	@echo "  compose-up    - Start services with Docker Compose"
	@echo "  compose-down  - Stop services with Docker Compose"
	@echo "  compose-logs  - Show logs from Docker Compose"
	@echo "  compose-restart - Restart services"
	@echo "  deploy        - Build and start services (compose-build + compose-up)"
	@echo "  undeploy      - Stop services (compose-down)"
	@echo ""
	@echo "Test targets:"
	@echo "  test-quick        - Quick health check"
	@echo "  test-assets       - Test asset fetcher and /assets endpoint"
	@echo "  test-proxy-local  - Test local proxy with node data"
	@echo "  test-proxy-remote - Test proxy vs official API"
	@echo "  test-full         - Full test suite (build + deploy + test + cleanup)"
	@echo ""
	@echo "  help          - Show this help message" 