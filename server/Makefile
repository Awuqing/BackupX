APP_NAME=backupx
BUILD_DIR=./bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build run test

build:
	mkdir -p $(BUILD_DIR)
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(APP_NAME) ./cmd/backupx

run:
	go run -ldflags "-X main.version=$(VERSION)" ./cmd/backupx

test:
	go test ./...
