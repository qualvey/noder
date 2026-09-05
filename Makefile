.PHONY: all build clean frontend backend run dev release help

# 默认目标：全量构建
all: build

help:
	@echo "可用构建命令:"
	@echo "  make build      - 构建前端与后端 (全量编译)"
	@echo "  make frontend   - 仅构建前端静态资源 (web/)"
	@echo "  make backend    - 仅编译 Go 后端二进制"
	@echo "  make dev        - 启动前端开发调试服务器"
	@echo "  make run        - 构建并启动后端服务"
	@echo "  make release    - 交叉编译各平台二进制 (Linux/macOS/Windows)"
	@echo "  make clean      - 清理构建产物与缓存"

# 前端构建
frontend:
	@echo "===> 构建前端..."
	cd web && pnpm build

# 后端编译
backend:
	@echo "===> 编译 Go 后端..."
	go build -ldflags "-s -w" -o noder ./cmd/server

# 全量构建
build: frontend backend
	@echo "===> 全量构建完成！"

# 开发模式 (前端)
dev:
	cd web && pnpm dev

# 构建并运行
run: build
	./noder

# 跨平台发布包构建
release:
	@echo "===> 交叉编译发布包..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o dist/noder-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-s -w" -o dist/noder-linux-arm64 ./cmd/server
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o dist/noder-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-s -w" -o dist/noder-darwin-arm64 ./cmd/server
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o dist/noder-windows-amd64.exe ./cmd/server
	@echo "===> 交叉编译完成，产物位于 dist/"

# 清理缓存
clean:
	rm -rf static/assets/* noder noder.exe dist
