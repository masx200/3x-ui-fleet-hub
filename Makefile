.PHONY: help build clean frontend

help:
	@echo "Available targets:"
	@echo "  make build      - Build production release (frontend + Go binary)"
	@echo "  make frontend   - Build frontend only"
	@echo "  make clean      - Clean build artifacts"

# 安装前端依赖
web/node_modules:
	@echo "→ Installing frontend dependencies..."
	cd web && npm install

# 前端构建
frontend: web/node_modules
	@echo "→ Building frontend..."
	cd web && npm run build

# Go 构建
go-build:
	@echo "→ Building Go binary..."
	go build -o bin/3x-ui.exe main.go

# 完整构建
build: frontend go-build
	@echo "\n✓ Production build completed!"
	@echo "  Binary: bin/3x-ui.exe"

# 清理
clean:
	@echo "→ Cleaning build artifacts..."
	rm -rf web/build
	rm -rf web/node_modules
	rm -rf bin
