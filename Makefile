.PHONY: all server web build run dev-web smoke clean fmt vet license-check

ROOT := $(shell pwd)
BIN  := $(ROOT)/build/svnadmin4
NPM  := npm
GO   := go

all: server web

## 编译后端（含前端静态资源打包进产物目录同级）
server:
	cd server && $(GO) build -o $(BIN) .

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

clean:
	rm -rf build web/dist
