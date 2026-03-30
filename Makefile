.PHONY: build dev test clean docker docker-cn

# 自动获取版本号（从 git tag 或 commit hash）
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# ── 一键构建 ──
build: build-server build-web

build-server:
	cd server && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/backupx ./cmd/backupx

build-web:
	cd web && npm run build

# ── 开发模式（分别在两个终端运行）──
dev-server:
	cd server && go run ./cmd/backupx

dev-web:
	cd web && npm run dev

# ── 测试 ──
test: test-server test-web

test-server:
	cd server && go test ./...

test-web:
	cd web && npm run test

# ── Docker 构建 ──
docker:
	docker build --build-arg VERSION=$(VERSION) -t backupx:$(VERSION) -t backupx:latest .

# 国内加速构建（使用国内镜像源）
docker-cn:
	docker build --build-arg VERSION=$(VERSION) --build-arg USE_CHINA_MIRROR=true -t backupx:$(VERSION) -t backupx:latest .

# ── 清理 ──
clean:
	rm -rf server/bin web/dist
