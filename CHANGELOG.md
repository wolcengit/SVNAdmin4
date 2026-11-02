# 更新日志

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。
许可证：GPL-3.0-or-later。

## [Unreleased]

待合并的改动记录在 PR 中，合并时归入下一版本。

## [4.1.0] - 2026-11-02

### 新增

- **产品版本信息（SSOT）**：新增 `server/internal/version` 作为唯一版本源；`svnadmin4 -v`
  输出单行版本（版本号 / Git 提交 / 构建时间 / Go 运行时 / 许可证）；`/api/common/version`
  与 `/api/setting/version` 返回结构化版本信息（版本号、Git 提交、构建时间、Go 运行时、
  平台、Subversion 版本、许可证与仓库地址）。
- **构建与发布流水线**：Makefile 通过 `-ldflags` 注入版本 / 提交 / 构建时间，新增
  `make version-check` 一致性门禁；Docker 镜像附带 OCI 标签与 `/opt/svnadmin4/VERSION`
  文件；新增 `.github/workflows/release.yml`，打 `v*` 标签时自动构建 `linux/amd64 + arm64`
  二进制与多架构镜像并推送至 GHCR（`ghcr.io/wolcengit/svnadmin4`）。
- **关于页增强**：展示 Git 提交短哈希、构建时间、许可证（指向 GPL-3.0 全文）与项目仓库地址。

## [4.0.0] - 2026-11-01

参考 [witersen/SvnAdminV2.0](https://github.com/witersen/SvnAdminV2.0) 的功能设计，
使用 Golang + Vue3 完成全栈重写后的首个版本。

### 新增

- **后端**：Go 1.22 + Gin + GORM（SQLite / MySQL，自动迁移）+ JWT + 进程内 cron 调度，
  单二进制交付，内置静态资源托管，无需额外守护进程。
- **前端**：Vue3 + Vite + TypeScript + Element Plus + Pinia，hash 路由。
- **仓库管理**：列表（体积/版本数/UUID/备注）、新建（可套用初始化结构模板）、
  删除、改名、备注、重设 UUID、目录树懒加载、指定 SVN 用户视角浏览、
  检出地址生成、`svnadmin verify` 检查、`svnadmin hotcopy` 热拷贝。
- **权限管理**：完整解析与写回 Subversion `authz`（保留 `[groups]`、`[aliases]`、
  注释与顺序），支持用户 / 分组 / 别名 / `*` / `$authenticated` / `$anonymous`
  与权限反转（`~name`），支持分组嵌套继承与路径最长前缀匹配。
- **二次授权（Secondpri）**：以「owner + 仓库 + 路径」为唯一键的白名单授权。
- **用户与分组**：增删改查、启停（同步 `passwd`）、改密、扫描导入、迁入向导、
  HTTP 协议下同步维护 `httpPasswd`。
- **备份与恢复**：`svnadmin dump`（全量/增量/版本范围）、分片上传（8MB 分片 + MD5
  校验 + 进度）、下载、删除、`svnadmin load` 导入，以异步任务执行。
- **钩子**：9 类内置模板，在线查看/编辑/清空，推荐钩子一键套用。
- **后台任务**：当前/排队/历史三视图，实时日志轮询、中断、进程组级联终止。
- **定时任务**：9 种任务类型 × 8 种周期，`save_count` 自动清理，手动执行与执行日志。
- **系统监控**：负载、CPU、内存、磁盘、主机信息，首页 3 秒自动刷新。
- **邮件与消息推送**：SMTP（SSL/TLS/STARTTLS，PLAIN→LOGIN 回退）与 6 个推送触发点。
- **外部集成**：LDAP（连接/用户/分组测试、同步、鉴权、分组写回 authz）、
  SASL（saslauthd 启停、`svn.conf`）、Apache（`subversion.conf` 生成与 graceful
  重载、协议双向切换）。
- **系统**：操作日志（筛选/检索/清空/Excel 导出）、配置项读写、路径信息总览、
  svnserve 启停、子管理员接口级功能权限树、个人信息与改密、管理员改名、
  三种角色登录、版本更新检测。

### 变更（相对原版 SvnAdminV2.0）

- 去掉独立 IPC 守护进程 `svnadmind.php`，耗时任务在 Go 进程内以 goroutine + 任务表实现。
- `authz` 为唯一数据源，不再把权限镜像到 `svn_user_pri_paths` 表，避免双写不一致。
- 鉴权改为 JWT + 内存 token 黑名单；验证码改为服务端生成 SVG，不依赖 GD 扩展。
- 通信协议改为 RESTful `/api/模块/动作`，不再使用 `api.php?c=Xxx&a=Yyy`。
- 许可由原版的 GPL-3.0 沿用至本项目（见 `LICENSE` 与 `NOTICE`）。

### 已知未实现

- 多仓库初始化模板的选择 UI（模板文件已就位，前端未暴露入口）。
- 二次授权 `svn_second_pri` 的完整业务流程（数据表已建，Web 侧尚未闭环）。

[Unreleased]: https://github.com/wolcengit/SVNAdmin4/compare/v4.1.0...HEAD
[4.1.0]: https://github.com/wolcengit/SVNAdmin4/releases/tag/v4.1.0
[4.0.0]: https://github.com/wolcengit/SVNAdmin4/releases/tag/v4.0.0
