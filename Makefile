.PHONY: all build clean run deps test cross-compile

# Application name
APP_NAME = dicom-anonymizer
BUILD_DIR = build

# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOTEST = $(GOCMD) test
GOMOD = $(GOCMD) mod
GOCLEAN = $(GOCMD) clean

# Build flags for smaller binary
LDFLAGS = -ldflags="-s -w"

# Default target
all: deps build

# Install dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Build native binary
build:
	mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./cmd/anonymizer

# Run the application
run: build
	./$(BUILD_DIR)/$(APP_NAME)

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Run tests
test:
	$(GOTEST) -v ./...

# Cross-compile for all platforms (requires fyne-cross)
cross-compile: deps
	@echo "Cross-compiling for Windows..."
	fyne-cross windows -arch=amd64 ./cmd/anonymizer
	@echo "Cross-compiling for Linux..."
	fyne-cross linux -arch=amd64 ./cmd/anonymizer
	@echo "Cross-compiling for macOS..."
	fyne-cross darwin -arch=amd64,arm64 ./cmd/anonymizer

# Build for Windows only
build-windows:
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME).exe ./cmd/anonymizer

# Build for Linux only
build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux ./cmd/anonymizer

# Install fyne-cross for cross-compilation
install-fyne-cross:
	go install github.com/fyne-io/fyne-cross@latest

# Development build (no optimization)
dev:
	$(GOBUILD) -o $(BUILD_DIR)/$(APP_NAME) ./cmd/anonymizer

# Show binary size
size: build
	ls -lh $(BUILD_DIR)/$(APP_NAME)
