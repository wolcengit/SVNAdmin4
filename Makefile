.PHONY: all server web build run dev-web smoke clean fmt vet license-check version-check

ROOT := $(shell pwd)
BIN  := $(ROOT)/build/svnadmin4
NPM  := npm
GO   := go

# 版本信息：编译期注入；未打 tag 时回退到 git describe 生成 dev 版本号
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILT   ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X 'svnadmin/internal/version.Version=$(VERSION)' \
           -X 'svnadmin/internal/version.GitCommit=$(COMMIT)' \
           -X 'svnadmin/internal/version.BuildTime=$(BUILT)'

all: server web

## 编译后端（含前端静态资源打包进产物目录同级）
server:
	cd server && $(GO) build -ldflags "$(LDFLAGS)" -o $(BIN) .

## 构建前端
web:
	cd web && $(NPM) install && $(NPM) run build

## 一键构建
build: server web

## 启动服务（默认读取 server/configs/config.yaml，托管 web/dist）
run:
	cd server && $(BIN) -c ./configs/config.yaml -web ../web/dist

## 前端开发模式（需先启动后端，vite 已配置 /api 代理到 8080）
dev-web:
	cd web && $(NPM) run dev

## 接口冒烟测试
smoke: server
	bash scripts/smoke.sh 8080

fmt:
	cd server && $(GO) fmt ./...

vet:
	cd server && $(GO) vet ./...

## 校验源文件 GPL-3.0 许可头与开源元文件是否齐全
license-check:
	bash scripts/check-license.sh

## 校验 Go 版本常量与前端 package.json 版本号一致（release 前门禁）
version-check:
	@V_GO=$$(grep -m1 'Version = ' server/internal/version/version.go | sed -E 's/.*"([0-9][0-9.]*)".*/\1/'); \
	V_NPM=$$(grep -m1 '"version"' web/package.json | sed -E 's/.*"([0-9][0-9.]*)".*/\1/'); \
	echo "版本号核对：Go=$$V_GO  web=$$V_NPM"; \
	if [ "$$V_GO" != "$$V_NPM" ]; then echo "❌ 版本号不一致，请同步 server/internal/version/version.go 与 web/package.json"; exit 1; fi; \
	echo "✓ 版本号一致"

clean:
	rm -rf build web/dist
