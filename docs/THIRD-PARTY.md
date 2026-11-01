# 第三方依赖与许可证

SVNAdmin4 自身以 **GNU General Public License v3.0 or later（GPL-3.0-or-later）** 发布
（见根目录 `LICENSE`）。本文件列出本发行物所包含的第三方开源组件及其原始许可证。

**兼容性结论**：下表所有依赖均为宽松许可证（MIT / Apache-2.0 / BSD-3-Clause /
MPL-2.0 / ISC），均为 GPL-3.0 兼容许可，可随本项目以 GPL-3.0 整体分发。
其中 MPL-2.0 依赖（`go-sql-driver/mysql`）按 MPL-2.0 第 3.3 条以
"次级许可（Secondary License）"方式与 GPL 组合，其源码仍按 MPL-2.0 单独授权。

许可证全文随各依赖自身的分发物提供：Go 依赖见 `server/go.mod`（模块缓存中
`$GOMODCACHE/<module>@<version>/LICENSE`），前端依赖见 `web/node_modules/<pkg>/LICENSE`。

## 一、前端（web/package.json）

| 组件 | 版本 | 许可证 |
| --- | --- | --- |
| vue | 3.5.x | MIT |
| vue-router | 4.6.x | MIT |
| pinia | 2.3.x | MIT |
| element-plus | 2.14.x | MIT |
| @element-plus/icons-vue | 2.3.x | MIT |
| axios | 1.21.x | MIT |
| vite（dev） | 5.4.x | MIT |
| @vitejs/plugin-vue（dev） | 5.2.x | MIT |
| vue-tsc（dev） | 2.2.x | MIT |
| typescript（dev） | 5.9.x | Apache-2.0 |
| @types/node（dev） | 20.x | MIT |

## 二、后端（server/go.mod）

### MIT

| 组件 | 版本 |
| --- | --- |
| github.com/gin-gonic/gin | v1.10.0 |
| github.com/gin-contrib/sse | v0.1.0 |
| github.com/glebarez/sqlite | v1.11.0 |
| gorm.io/gorm | v1.25.12 |
| gorm.io/driver/mysql | v1.5.7 |
| github.com/jinzhu/inflection | v1.0.0 |
| github.com/jinzhu/now | v1.1.5 |
| github.com/go-ldap/ldap/v3 | v3.4.6 |
| github.com/go-asn1-ber/asn1-ber | v1.5.5 |
| github.com/go-playground/validator/v10 | v10.20.0 |
| github.com/go-playground/locales | v0.14.1 |
| github.com/go-playground/universal-translator | v0.18.1 |
| github.com/leodido/go-urn | v1.4.0 |
| github.com/goccy/go-json | v0.10.2 |
| github.com/json-iterator/go | v1.1.12 |
| github.com/modern-go/reflect2 | v1.0.2 |
| github.com/mattn/go-isatty | v0.0.20 |
| github.com/klauspost/cpuid/v2 | v2.2.7 |
| github.com/gabriel-vasile/mimetype | v1.4.3 |
| github.com/pelletier/go-toml/v2 | v2.2.2 |
| github.com/ugorji/go/codec | v1.2.12 |
| github.com/Azure/go-ntlmssp | v0.0.0-2022… |
| github.com/dustin/go-humanize | v1.0.1 |
| github.com/robfig/cron/v3 | v3.0.1 |
| gopkg.in/yaml.v3 | v3.0.1（MIT / Apache-2.0 双许可） |

### Apache-2.0

| 组件 | 版本 |
| --- | --- |
| github.com/bytedance/sonic | v1.11.6 |
| github.com/bytedance/sonic/loader | v0.1.1 |
| github.com/cloudwego/base64x | v0.1.4 |
| github.com/cloudwego/iasm | v0.2.0 |
| github.com/modern-go/concurrent | v0.0.0-2018… |

### BSD-3-Clause

| 组件 | 版本 |
| --- | --- |
| golang.org/x/crypto | v0.26.0 |
| golang.org/x/net | v0.25.0 |
| golang.org/x/sys | v0.23.0 |
| golang.org/x/text | v0.17.0 |
| golang.org/x/mod | v0.21.0 |
| golang.org/x/arch | v0.8.0 |
| google.golang.org/protobuf | v1.34.1 |
| github.com/google/uuid | v1.3.1 |
| github.com/golang-jwt/jwt/v5 | v5.2.1 |
| github.com/twitchyliquid64/golang-asm | v0.15.1 |
| github.com/remyoudompheng/bigfft | v0.0.0-2023… |
| github.com/glebarez/go-sqlite | v1.21.2 |
| modernc.org/sqlite | v1.23.1 |
| modernc.org/libc | v1.22.5 |
| modernc.org/mathutil | v1.5.0 |
| modernc.org/memory | v1.5.0 |

### MPL-2.0

| 组件 | 版本 | 说明 |
| --- | --- | --- |
| github.com/go-sql-driver/mysql | v1.7.0 | 仅在使用 MySQL 驱动时链接；MPL-2.0 与 GPL-3.0 兼容 |

## 三、运行时外部程序

SVNAdmin4 通过子进程调用 Subversion 工具链，这些程序不属于本发行物，
其许可证由各自项目提供：

- `svn` / `svnadmin` / `svnlook` / `svnserve`（Apache Subversion，Apache-2.0）
- `saslauthd`（Cyrus SASL，BSD 风格）
- `httpd` + `mod_dav_svn`（Apache HTTP Server，Apache-2.0）— 仅启用 HTTP 协议时需要
- `htpasswd`（apr1 散列由本项目自行实现，不依赖外部 `htpasswd`）

## 四、上游参考

- [witersen/SvnAdminV2.0](https://github.com/witersen/SvnAdminV2.0)，GPL-3.0。
  本项目为功能设计参考后的独立重写，未复用其源码，因此其许可不向下传染；
  本项目仍主动沿用 GPL-3.0，与上游保持一致。
