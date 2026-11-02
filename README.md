# SVNAdmin4

**仓库**：<https://github.com/wolcengit/SVNAdmin4>

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8)
![Vue](https://img.shields.io/badge/Vue-3-42b883)
![SVN](https://img.shields.io/badge/Subversion-1.8%2B-8099cf)
![Version](https://img.shields.io/badge/version-4.1.0-blue)

基于 Web 的 Subversion 服务端管理系统，参考 [witersen/SvnAdminV2.0](https://github.com/witersen/SvnAdminV2.0) 的功能设计，使用 **Golang + Vue3** 重新实现。

> **开源许可**：本项目与上游 SvnAdminV2.0 保持一致，采用 **GNU GPL v3.0 or later**
> 授权发布（见 [`LICENSE`](./LICENSE) 与 [`NOTICE`](./NOTICE)）。
> 你可以自由使用、修改与分发，但任何分发（含二进制与 Docker 镜像）都必须
> 以同一许可证开放完整源码。参与贡献请阅读 [`CONTRIBUTING.md`](./CONTRIBUTING.md)。

原项目为 `PHP + Vue2 + iView` 技术栈，本项目在保留其核心管理能力（仓库、用户、分组、authz 授权、备份恢复、钩子、任务计划、日志）的基础上，完成了全栈重写：

| 维度 | SvnAdminV2.0（原版） | SVNAdmin4（本项目） |
| --- | --- | --- |
| 后端 | PHP 5.5~8.2 + Medoo | Go 1.22 + Gin + GORM |
| 前端 | Vue2 + webpack + iView | Vue3 + Vite + TypeScript + Element Plus |
| 数据库 | SQLite / MySQL | SQLite / MySQL（GORM 自动迁移，零手工建表） |
| 部署 | Apache + PHP-FPM + 独立守护进程 svnadmind | 单二进制，内置静态资源托管与任务调度 |
| 鉴权 | 自研 token + 数据库 | JWT + 内存 token 黑名单 |
| 通信 | `api.php?c=Xxx&a=Yyy` | RESTful `/api/模块/动作` |

---

## 1. 功能清单

### 仓库管理
- 仓库列表（体积 / 版本数 / UUID / 备注），自动扫描磁盘同步
- 新建仓库（可选套用初始化结构模板）、删除、改名、修改备注
- 仓库属性明细（库格式、FS 类型、物理路径）
- 重设 UUID、同步体积、同步版本数
- 目录树**按需懒加载**、右键菜单（新建子目录 / 复制路径 / 设为权限路径 / 查看目录条目）
- 以**指定 SVN 用户视角**浏览目录（走 svn 协议真实鉴权，仅可见有权限内容）
- 检出地址生成（svn:// 与 http:// 双协议）并支持一键复制
- 仓库检查（`svnadmin verify`）与**热拷贝备份**（`svnadmin hotcopy`，支持增量）

### 权限管理（authz）
- 完整解析 / 写回 Subversion `authz` 文件（保留 `[groups]`、`[aliases]`、注释与顺序）
- 按路径展示权限项，自动区分当前路径权限与继承权限
- 授权对象支持：用户、分组（`@g`）、别名（`&a`）、`*`、`$authenticated`、`$anonymous`
- **权限反转（`~name`）**：表示"除该对象以外的所有人"
- 权限判定支持分组嵌套继承与路径最长前缀匹配
- 删除用户 / 分组时自动清理分组引用与授权项

### 二次授权（Secondpri）
- 允许某个 SVN 用户以自身身份登录后管理**指定白名单内**的用户 / 分组 / 别名
- 以「owner + 仓库 + 路径」为唯一键，开关记录与白名单记录分离
- 关闭二次授权时自动清空白名单

### 用户与分组
- SVN 用户增删改查、启停、改密、备注、邮箱
- 启停用户时同步 `passwd`（关闭即从 passwd 移除，开启时从库恢复）
- 扫描 `passwd` 文件批量导入历史用户；**用户迁入 3 步向导**（粘贴 passwd 正文 → 逐条移除 → 查看导入结果）
- 分组增删改查、改名、备注，成员可包含用户 / 分组 / 别名
- HTTP 协议开启时同步维护 `httpPasswd`（Apache apr1 散列）
- 一键同步用户 / 分组 / 仓库数据

### 备份与恢复
- 立即备份（`svnadmin dump`），支持版本范围与增量备份
- **分片上传**（8MB 分片、MD5 校验、进度展示、可取消）
- 备份文件列表、下载、删除
- 从 dump 文件导入仓库（`svnadmin load`）
- 备份/导入以异步任务方式执行，前端可查看任务状态与实时日志

### 钩子
- 内置 9 类钩子模板（pre-commit / post-commit / pre-lock 等）
- 在线查看与编辑钩子内容、清空钩子
- 推荐钩子目录自动发现，支持**一键套用**到仓库

### 后台任务
- 3 个视图：当前任务（**实时日志 2 秒轮询** + 中断）、排队任务（取消）、历史任务（状态筛选 / 日志 / 删除 / 清空）
- 任务以**独立进程组**启动，中断时可级联终止子进程（`svnadmin` 等）
- 任务类型与状态字典由后端下发

### 定时任务
- **9 种任务类型**：dump 全量 / dump 增量 / hotcopy 全量 / hotcopy 增量 / 仓库检查 / shell 脚本 / 同步用户 / 同步分组 / 同步仓库
- **8 种周期**：每分钟、每隔 N 分钟、每小时、每隔 N 小时、每天、每隔 N 天、每周、每月
- 保留份数 `save_count` 自动清理旧备份
- 手动立即执行、查看执行日志、消息通知（关闭 / 仅成功 / 仅失败 / 全部）
- **进程内调度**（`robfig/cron`），不依赖系统 crond/atd

### 系统监控
- 系统负载（1/5/15 分钟）、CPU 使用率、内存占用、各挂载点磁盘使用率
- 主机信息（操作系统、平台、架构、主机名、运行时、运行时长）
- 首页仪表盘支持 3 秒自动刷新

### 邮件与消息推送
- SMTP 配置（SSL / TLS / STARTTLS，PLAIN → LOGIN 认证回退）、测试邮件
- 6 个推送触发点：用户登录、管理员改名、管理员改密、SVN 用户改密、**任务成功**、**任务失败**

### 外部集成
- **LDAP**：连接 / 用户 / 分组测试、用户与分组同步、LDAP 鉴权、分组写回 authz
- **SASL**：saslauthd 启停与运行状态、`svn.conf` 维护
- **Apache / HTTP 协议**：`subversion.conf` 生成与 graceful 重载、访问前缀与端口配置、协议双向切换、HTTP 数据源切换

### 系统
- 操作日志（分页、按类型筛选、关键字检索、清空、**Excel 导出**）
- 系统配置项读写
- 路径信息总览（主目录 / authz / passwd / 备份 / 日志 / 模板 / 钩子 / 数据库等）
- svnserve 启停（进程内托管）与运行状态展示
- 子管理员：**功能权限树**（接口级白名单授权，节点变更时提示重新授权）、备注编辑、启停、改密
- 个人信息与改密（改密后强制登出）、**管理员修改账号名**
- 三种角色：管理员 `admin`、子管理员 `subadmin`、SVN 用户 `svnuser`；登录时可选身份
- **版本更新检测**（语义化版本比较 + 修复/新增/移除清单）

---

## 2. 目录结构

```
SVNAdmin4/
├── server/                       # Go 后端
│   ├── main.go
│   ├── configs/config.yaml       # 服务配置
│   └── internal/
│       ├── api/                  # HTTP 处理层
│       ├── auth/                 # JWT 签发/解析/黑名单
│       ├── config/               # 配置加载与 SVN 路径推导
│       ├── middleware/           # 鉴权、CORS
│       ├── model/                # GORM 实体
│       ├── router/               # 路由注册 + SPA 托管
│       ├── service/              # 业务逻辑
│       ├── store/                # 数据库初始化与迁移
│       ├── svn/                  # SVN 底层库（命令/authz/passwd/仓库）
│       └── util/                 # 通用工具
├── web/                          # Vue3 前端
│   └── src/
│       ├── api/                  # 接口定义
│       ├── layouts/              # 主框架布局
│       ├── router/
│       ├── stores/               # Pinia（用户态）
│       └── views/                # 各功能页面
├── deploy/
│   ├── svnadmin4.service         # systemd 单元
│   └── docker-entrypoint.sh
├── scripts/smoke*.sh             # 四套接口冒烟测试（190 断言）
├── docs/
│   ├── COMPARISON.md             # 与原版的逐项对照
│   └── THIRD-PARTY.md            # 第三方依赖与许可证清单
├── LICENSE                       # GNU GPL v3.0（与上游一致）
├── NOTICE                        # 版权、上游与源码提供声明
├── CONTRIBUTING.md               # 贡献指南（含提交约定）
├── CHANGELOG.md
├── SECURITY.md
└── Makefile
```

---

## 3. 快速开始

### 3.1 环境要求
- Go 1.22+
- Node.js 18+
- Subversion 1.8+（`svn` / `svnadmin` / `svnlook` / `svnserve` 需在 PATH 中）

### 3.2 编译与运行

```bash
# 构建后端 + 前端
make build

# 启动（默认 http://127.0.0.1:8080）
make run
```

默认管理员账号：`admin` / `admin`（首次启动自动写入数据库）。

### 3.3 开发模式

```bash
# 终端 1：后端
cd server && go run . -c ./configs/config.yaml -web ""

# 终端 2：前端（vite 已配置 /api 代理到 8080）
cd web && npm run dev
```

### 3.4 冒烟测试

```bash
make smoke
```

覆盖登录、用户、分组、仓库、在线建目录、授权、备份、钩子、日志、统计等 42 项接口断言（`scripts/smoke.sh`）。

另有三套补充冒烟：`scripts/smoke2.sh`（65 项：二次授权、取反权限、多模板、备份上传、定时任务、协议切换等）、`scripts/smoke3.sh`（37 项：子管理员逐接口鉴权、备份下载 URL token、authz 双向同步、任务重启清理、安全配置、`#disabled#` 兼容等）与 `scripts/smoke4.sh`（46 项：LDAP 写拦截、用户 online、black_token 持久化、二次授权自助闭环、hotcopy 入列、指定 UUID、crond 分页、日志时间范围等）。四套合计 **190 断言**。

---

## 4. 配置项说明

`server/configs/config.yaml`：

```yaml
server:
  host: 0.0.0.0
  port: 8080
  mode: release            # debug 时打印 SQL
  token_expire: 7200       # token 有效期（秒）

database:
  driver: sqlite           # sqlite | mysql
  sqlite_path: ./data/svnadmin.db
  mysql: { host, port, user, password, name }

svn:
  home_path: ./runtime/svnadmin/   # 数据主目录，其余路径自动推导
  bin_path: ""                     # svn 可执行文件目录，留空走 PATH
  svnserve_host: ""                # svn:// 检出地址主机，留空取请求 Host
  svnserve_port: 3690
  enable_http: false               # 开启后维护 httpPasswd 并展示 http:// 检出地址
  http_prefix: /svn

security:
  jwt_secret: svnadmin4-default-secret
  enable_verify_code: true

log:
  level: info
```

`home_path` 下自动生成的目录结构：

```
runtime/svnadmin/
├── rep/            # 仓库父目录
├── backup/         # 备份文件
├── crond/          # 任务计划
├── hooks/          # 推荐钩子
├── logs/           # 日志
├── templete/initStruct/01/   # 仓库初始化结构模板
├── authz           # 权限文件（主）
├── passwd          # svnserve 用户文件
├── httpPasswd      # Apache htpasswd（apr1）
├── svnserve.conf   # 统一管理的 svnserve 配置
└── svnserve.pid
```

---

## 5. 接口一览

统一响应：`{ code, status, message, data }`；分页接口 `data = { total, list }`。
鉴权：`X-Token` 请求头（`Authorization: Bearer <token>` 亦可）。

| 分组 | 方法 | 路径 | 说明 |
| --- | --- | --- | --- |
| 公共 | POST | `/api/common/login` | 登录（管理员/子管理员/SVN 用户） |
| 公共 | GET | `/api/common/verifyCode` | 获取 SVG 验证码 |
| 公共 | GET | `/api/common/verifyOption` | 登录页配置 |
| 公共 | POST | `/api/common/logout` | 注销（token 进黑名单） |
| 仓库 | GET/POST | `/api/rep/list`、`/api/rep/create` | 列表 / 新建 |
| 仓库 | POST | `/api/rep/del`、`/api/rep/updName`、`/api/rep/updNote` | 删除 / 改名 / 备注 |
| 仓库 | GET | `/api/rep/detail`、`/api/rep/tree`、`/api/rep/con` | 属性 / 目录树 / 目录内容 |
| 仓库 | POST | `/api/rep/createFolder`、`/api/rep/setUUID` | 建目录 / 重设 UUID |
| 权限 | GET | `/api/pri/list` | 路径权限（当前 + 继承） |
| 权限 | POST | `/api/pri/create`、`/api/pri/update`、`/api/pri/delete` | 增删改 |
| 用户 | GET | `/api/user/list`、`/api/user/scan` | 列表 / 扫描 passwd |
| 用户 | POST | `/api/user/create`、`del`、`updPass`、`updStatus`、`updNote`、`import` | 用户维护 |
| 分组 | GET | `/api/group/list`、`/api/group/member` | 列表 / 成员 |
| 分组 | POST | `/api/group/create`、`del`、`updName`、`updNote`、`updMember` | 分组维护 |
| 备份 | GET/POST | `/api/backup/list`、`dump`、`load`、`del`、`upload` | 备份恢复 |
| 备份 | GET | `/api/backup/download?name=` | 下载备份 |
| 钩子 | GET/POST | `/api/hook/list`、`update`、`del`、`recommend` | 钩子维护 |
| 系统 | GET | `/api/log/list`、`/api/setting/list`、`/api/crond/list`、`/api/task/list` | 查询类 |
| 系统 | POST | `/api/log/clear`、`/api/setting/update`、`/api/setting/svnserveStart` | 操作类 |
| 管理员 | GET/POST | `/api/subadmin/*` | 子管理员（仅 admin） |

---

## 6. 生产部署

```bash
make build
mkdir -p /opt/svnadmin4
cp -r build server/configs web/dist /opt/svnadmin4/
cp deploy/svnadmin4.service /usr/lib/systemd/system/
systemctl daemon-reload
systemctl enable --now svnadmin4
```

生产环境建议：
- 将 `svn.home_path` 指向独立数据盘（如 `/home/svnadmin/`）
- MySQL 场景将 `database.driver` 改为 `mysql` 并填写连接信息，表结构自动创建
- 需要 HTTP 协议检出时配置 Apache + `mod_dav_svn`，并把 `svn.enable_http` 置为 `true`
- 修改 `security.jwt_secret`

---

## 7. 与原版的差异说明

1. **去掉 IPC 守护进程**：原版需要单独运行 `svnadmind.php` 处理耗时任务，本项目在 Go 进程内以 goroutine + 任务表实现，无需额外进程。
2. **authz 为唯一数据源**：原版将权限镜像到 `svn_user_pri_paths` 表，本项目直接以 authz 文件为准并实时推导，避免双写不一致。
3. **验证码**：改用服务端生成 SVG，不依赖 GD 扩展。
4. **邮件通知**：以 Go 标准库 `net/smtp` 实现，未引入 PHPMailer 之类组件。
5. **Windows**：与原版一致，仅支持 Linux 部署。

---

## 8. 许可（License）

```
Copyright (C) 2026 wolcen@msn.com

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY
WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
PARTICULAR PURPOSE. See the GNU General Public License for more details.
```

- 许可证：**GNU General Public License v3.0 or later（GPL-3.0-or-later）**，
  全文见 [`LICENSE`](./LICENSE)，与上游
  [witersen/SvnAdminV2.0](https://github.com/witersen/SvnAdminV2.0) 保持一致。
- 版权与第三方声明：[`NOTICE`](./NOTICE)；
  依赖许可证清单：[`docs/THIRD-PARTY.md`](./docs/THIRD-PARTY.md)。
- 每个源文件头部均带有版权行与 `SPDX-License-Identifier: GPL-3.0-or-later`。
- **源码提供义务**：以二进制或镜像形式分发本程序时，须同时提供完整可构建源码
  （GPL-3.0 第 6 条）。Docker 镜像已在 `/opt/svnadmin4` 下随附 `LICENSE` 与 `NOTICE`。
- 上游项目为 PHP + Vue2 实现，本项目是 Golang + Vue3 的**独立重写**，未复用其源码；
  之所以沿用 GPL-3.0，是为了与上游生态的授权方式保持一致。
- 软件按"原样"提供，作者不承担任何担保责任；请勿将本系统直接暴露于公网。

安全漏洞报告方式见 [`SECURITY.md`](./SECURITY.md)，请勿在公开 issue 中披露。
