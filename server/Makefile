APP_NAME=backupx
BUILD_DIR=./bin

.PHONY: build run test

build:
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/backupx

run:
	go run ./cmd/backupx

test:
	go test ./...
