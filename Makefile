.PHONY: build dev test clean

# 一次性构建前后端
build: build-server build-web

build-server:
	cd server && go build -o bin/backupx ./cmd/backupx

build-web:
	cd web && npm run build

# 开发模式（分别在两个终端运行）
dev-server:
	cd server && go run ./cmd/backupx

dev-web:
	cd web && npm run dev

# 运行所有测试
test: test-server test-web

test-server:
	cd server && go test ./...

test-web:
	cd web && npm run test

# 清理构建产物
clean:
	rm -rf server/bin web/dist
