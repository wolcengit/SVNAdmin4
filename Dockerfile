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
RUN CGO_ENABLED=0 go build -o /out/svnadmin4 .

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
RUN chmod +x ./entrypoint.sh

ENV SVN_HOME=/home/svnadmin/
EXPOSE 8080 3690

VOLUME ["/home/svnadmin"]
ENTRYPOINT ["./entrypoint.sh"]
