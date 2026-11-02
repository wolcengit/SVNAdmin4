# 全局构建参数（镜像元数据与二进制版本注入）
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

# ---------- 前端构建 ----------
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package.json ./
RUN npm install --registry=https://registry.npmmirror.com
COPY web/ ./
RUN npm run build

# ---------- 后端构建 ----------
FROM golang:1.22-alpine AS server
WORKDIR /go/src/svnadmin
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build \
    -ldflags "-X 'svnadmin/internal/version.Version=${VERSION}' -X 'svnadmin/internal/version.GitCommit=${COMMIT}' -X 'svnadmin/internal/version.BuildTime=${BUILD_TIME}'" \
    -o /out/svnadmin4 .

# ---------- 运行镜像 ----------
FROM alpine:3.20
RUN apk add --no-cache subversion subversion-tools ca-certificates tzdata \
    && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
WORKDIR /opt/svnadmin4

COPY --from=server /out/svnadmin4 ./build/svnadmin4
COPY --from=web /web/dist ./web/dist
COPY server/configs ./server/configs
COPY deploy/docker-entrypoint.sh ./entrypoint.sh
# GPL-3.0：二进制分发须随附许可证与版权声明
COPY LICENSE NOTICE ./
# OCI 镜像元数据 + GPL-3.0 合规标签
LABEL org.opencontainers.image.title="SVNAdmin4" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_TIME}" \
      org.opencontainers.image.licenses="GPL-3.0-or-later" \
      org.opencontainers.image.source="https://github.com/wolcengit/SVNAdmin4" \
      org.opencontainers.image.description="基于 Web 的 Subversion 服务端管理系统（Golang + Vue3）"
RUN echo "SVNAdmin4 ${VERSION} (${COMMIT}, ${BUILD_TIME})" > /opt/svnadmin4/VERSION
RUN chmod +x ./entrypoint.sh

ENV SVN_HOME=/home/svnadmin/
EXPOSE 8080 3690

VOLUME ["/home/svnadmin"]
ENTRYPOINT ["./entrypoint.sh"]
