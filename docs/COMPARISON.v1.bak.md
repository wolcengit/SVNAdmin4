# SVNAdmin4（Golang + Vue3） vs SVNAdmin 2.5.10（PHP + Vue2）功能逐项对比

> 对比基准
> - 原版：`witersen/SvnAdminV2.0`，`config/version.php` 版本号 **2.5.10**，PHP ≥ 5.5，Vue2 + iView + webpack
> - 重写版：SVNAdmin4 **4.0.0**，Go 1.22 + Gin + GORM，Vue3 + Vite + TS + Element Plus
> - 原版接口统计口径：`02.php/app/controller/*.php` 中的 `public function` 共 **117 个**（含非 HTTP 的内部方法）
> - SVNAdmin4 接口统计口径：`server/internal/router/router.go` 注册路由共 **77 个**（含 `/api/health`）
> - 图例：✅ 已实现（行为等价或更优）｜⚠️ 部分实现（能力缩减/交互不同）｜❌ 未实现｜➕ 重写版新增

---

## 一、总体架构对比

| 维度 | SVNAdmin 2.5.10 | SVNAdmin4 4.0.0 |
|---|---|---|
| 后端语言/框架 | PHP 5.5~8.2 + 自研 MVC（`api.php?c=&a=`）+ Medoo | Go 1.22 + Gin + GORM |
| 前端 | Vue2 + iView 3 + webpack | Vue3 + Vite + TypeScript + Element Plus + Pinia |
| 交付形态 | PHP 源文件 + Apache/Nginx + PHP-FPM + **svnadmind 守护进程** + 系统 crond/atd | **单个二进制**，内置静态资源托管与任务调度 |
| 数据库 | SQLite 或 MySQL（Medoo） | SQLite（glebarez/sqlite 纯 Go，无 CGO）或 MySQL |
| 后台任务 | PHP 写的 `svnadmind.php`（`pcntl_fork` + Unix Socket），轮询 `tasks` 表并执行 | Go 内置 goroutine 任务调度（`service/task.go`），进程内完成 |
| 定时任务 | **系统 crontab + at**：生成 `<sign>` bash 脚本后 `crontab -l/ crontab file` 增删行 | 进程内 `robfig/cron/v3`，写库即注册/注销，**不依赖系统 cron** |
| 鉴权 | 自研 token + `admin_users.admin_user_token` / `black_token` 表 | JWT（`X-Token` 头）+ 内存 token 黑名单（登出即失效） |
| 验证码 | 存储在 `verification_code` 表，带 start/end 时间 | 内存 map + TTL + 定时清理，**无表** |
| 路由 | 前端路由由**后端下发**（`Common/Login` 返回 `route`/`functions`，`router.addRoute` 动态注册） | 前端**静态路由**（`web/src/router/index.ts`），后端只做角色中间件 |
| 权限模型 | 角色 1 管理员 / 2 SVN 用户 / 3 子管理员；子管理员按 `subadmin_tree` 功能树精确到"控制器/方法" | 角色 admin / subadmin / svnuser；子管理员**仅按接口分组粗粒度授权**（`subadmin_functions`） |
| 协议栈 | svnserve + Apache httpd(mod_dav_svn/mod_authz_svn) + saslauthd + LDAP | svnserve（内置启停）；HTTP 仅生成检出地址与同步维护 `httpPasswd`，**不管理 Apache** |
| 主目录 | 固定 `/home/svnadmin/`（硬编码） | `svn.home_path` 可配置（默认 `./runtime/svnadmin/`），路径由 `config.SVNConfig` 推导 |

---

## 二、数据模型对比（13 张表）

| 表 | 原版 | SVNAdmin4 | 差异 |
|---|---|---|---|
| `admin_users` | 6 字段，密码**明文** | 同结构，密码存 **SHA-256**（`util.Hash`） | ➕ 安全性提升 |
| `subadmin` | 12 字段（含 `subadmin_tree`/`subadmin_functions`） | 字段一致 | ⚠️ `subadmin_tree` 有字段但**未实现权限树 UI** |
| `svn_users` | 8 字段 | 字段一致 + **唯一索引** `svn_user_name` | ✅ |
| `svn_groups` | 6 字段 | 一致 + 唯一索引 | ✅ |
| `svn_reps` | 6 字段 | 一致 + 唯一索引 | ✅ |
| `svn_user_pri_paths` | 7 字段（权限镜像表） | **不建表** | ➕ 设计变更：authz 文件为唯一数据源，不镜像权限 |
| `svn_second_pri` | 4 字段 | 建表但**扩展为 9 字段**（补 `second_pri_path`/`rep_name`/`perm`/`unique`/`add_time`） | ❌ 表在，业务逻辑未实现 |
| `crond` | 18 字段 | 一致 | ✅ |
| `tasks` | 11 字段 | 12 字段（➕ `task_message`） | ✅ |
| `logs` | 5 字段 | 一致 | ✅ |
| `options` | 4 字段 | 一致 | ✅ |
| `black_token` | 5 字段（token 黑名单表） | **不建表** | ➕ 改用进程内内存黑名单 |
| `verification_code` | 6 字段（验证码表） | **不建表** | ➕ 改用内存 + TTL |

**核心设计差异**：原版把 authz 权限镜像到 `svn_user_pri_paths` 表，存在"文件与库不一致"风险；SVNAdmin4 取消该表，所有权限判定直接解析 `authz` 文件（`internal/svn/authz.go`），写回时保留 `[groups]`、`[aliases]`、注释与顺序。

---

## 三、接口逐一对比

### 3.1 公共 / 登录（Common）

| 原版接口 | SVNAdmin4 | 状态 | 说明 |
|---|---|---|---|
| `Common/Login` | `POST /api/common/login` | ✅ | 返回 JWT；另返回角色 `admin/subadmin/svnuser` |
| `Common/Logout` | `POST /api/common/logout` | ✅ | token 进内存黑名单 |
| `Common/GetVerifyCode` | `GET /api/common/verifyCode` | ✅ | 原版返回 base64 PNG；重写版返回 **SVG**（前端可直接内联，无需 base64 解码） |
| `Setting/GetVerifyOption` | `GET /api/common/verifyOption` | ✅ | 由 `security.enable_verify_code` 决定 |
| （无） | `GET /api/health` | ➕ | 健康检查 |
| （无） | `GET /api/common/userInfo` | ➕ | 当前登录人信息 |
| — | `Common/Login` 返回的 `route`/`functions` 动态路由 | ❌ | 重写版改为前端静态路由，后端不再下发路由表 |

### 3.2 仓库管理（Svnrep，原版 34 个 → 重写版 13 个）

| 原版接口 | SVNAdmin4 | 状态 | 说明 |
|---|---|---|---|
| `Svnrep/GetRepList` | `GET /api/rep/list` | ✅ | 分页 + 关键字 |
| `Svnrep/CreateRep` | `POST /api/rep/create` | ✅ | 原版 `rep_type` 只有"空仓库 / 模板01"；重写版 `initStruct` 支持 `""` / `01`，结构可扩展 |
| `Svnrep/DelRep` | `POST /api/rep/del` | ✅ | |
| `Svnrep/UpdRepName` | `POST /api/rep/updName` | ✅ | |
| `Svnrep/UpdRepNote` | `POST /api/rep/updNote` | ✅ | |
| `Svnrep/GetRepDetail` | `GET /api/rep/detail` | ⚠️ | 原版用 `svnadmin info` 输出全部键值（Subversion ≥1.10）；重写版返回结构化 `repName/repRev/repSize/repSizeHr/repUuid/repNote` + hooks 概览 |
| `Svnrep/SetUUID` | `POST /api/rep/setUUID` | ✅ | 留空自动生成 |
| `Svnrep/SyncRepSize` | `POST /api/rep/syncSize` | ✅ | |
| `Svnrep/SyncRepRev` | `POST /api/rep/syncRev` | ✅ | |
| `Svnrep/GetRepTree` | `GET /api/rep/tree` | ✅ | 管理员视角，`svnlook tree` 直读 |
| `Svnrep/CreateRepFolder` | `POST /api/rep/createFolder` | ✅ | `svn mkdir` |
| `Svnrep/CheckAuthz` | `GET /api/rep/checkAuthz` | ✅ | 调用 `svnauthz-validate` |
| `Svnrep/GetSvnUserRepList` | `GET /api/rep/userList` | ✅ | SVN 用户可见仓库 |
| `Svnrep/GetSvnUserRepList2` | `GET /api/rep/userList2` | ✅ | 管理员查看某用户的仓库权限列表 |
| `Svnrep/GetRepCon` | `GET /api/rep/con` | ✅ | 管理员浏览仓库内容（面包屑 + 文件/目录） |
| `Svnrep/GetUserRepCon` | — | ❌ | **未实现**：SVN 用户视角、经 `svn list` 真实鉴权的内容浏览（含单文件授权处理） |
| `Svnrep/GetRepTree2` | — | ❌ | **未实现**：二次授权视角的目录树（`svn list --username/--password` 走真实鉴权 + `first` 单路径树） |
| `Svnrep/GetCheckout` | `GET /api/rep/checkout` | ✅ | 返回 `protocal` + `prefix`；支持 svn/http 双协议 |
| `Svnrep/GetSvnserveStatus` | `GET /api/common/svnserveStatus` | ✅ | |
| `Svnrep/GetRepPathAllPri` | `GET /api/pri/list` | ✅ | |
| `Svnrep/CreateRepPathPri` | `POST /api/pri/create` | ✅ | |
| `Svnrep/UpdRepPathPri` | `POST /api/pri/update` | ✅ | |
| `Svnrep/DelRepPathPri` | `POST /api/pri/delete` | ✅ | |
| `Svnrep/GetRepHooks` | `GET /api/hook/list` | ✅ | 9 类钩子 |
| `Svnrep/UpdRepHook` | `POST /api/hook/update` | ✅ | 写文件 + `chmod +x` |
| `Svnrep/DelRepHook` | `POST /api/hook/del` | ✅ | |
| `Svnrep/GetRecommendHooks` | `GET /api/hook/recommend` | ✅ | 扫描推荐钩子目录 |
| `Svnrep/GetBackupList` | `GET /api/backup/list` | ✅ | |
| `Svnrep/SvnadminDump` | `POST /api/backup/dump` | ➕**增强** | 原版只支持全量；重写版支持 `revRange` 版本范围与 `incremental` 增量 |
| `Svnrep/DelRepBackup` | `POST /api/backup/del` | ✅ | |
| `Svnrep/DownloadRepBackup` | `GET /api/backup/download` | ⚠️ | 原版用 `hash_hmac` 生成 `fileToken` 校验；重写版走登录态鉴权，安全性等价但机制不同 |
| `Svnrep/UploadBackup` | `POST /api/backup/upload` | ⚠️ | 原版为**分片上传**（进度/暂停/分片清理）；重写版为整文件上传 |
| `Svnrep/GetUploadInfo` | — | ❌ | 分片上传所需的上传能力探测接口随分片上传一并未实现 |
| `Svnrep/SvnadminLoad` | `POST /api/backup/load` | ✅ | ➕ 重写版会在仓库不存在时自动创建 |
| — | `GET /api/rep/all` | ➕ | 仓库全量下拉（不分页） |

### 3.3 SVN 用户（Svnuser）

| 原版接口 | SVNAdmin4 | 状态 | 说明 |
|---|---|---|---|
| `Svnuser/GetUserList` | `GET /api/user/list` | ✅ | 分页 + 关键字 |
| `Svnuser/CreateUser` | `POST /api/user/create` | ✅ | ➕ `enable_http=true` 时同步写 `httpPasswd`（apr1） |
| `Svnuser/DelUser` | `POST /api/user/del` | ✅ | 同步清理 authz 引用 |
| `Svnuser/UpdUserPass` | `POST /api/user/updPass` | ✅ | |
| `Svnuser/UpdUserStatus` | `POST /api/user/updStatus` | ✅ | 禁用时从 `passwd` 删除行、启用时从库恢复 |
| `Svnuser/UpdUserNote` | `POST /api/user/updNote` | ✅ | |
| `Svnuser/UserScan` | `GET /api/user/scan` | ⚠️ | 原版为 POST 提交 passwd/httpPasswd 文件**正文**；重写版直接读服务端 passwd 文件 |
| `Svnuser/UserImport` | `POST /api/user/import` | ✅ | 批量导入，逐条返回 `{userName,status,reason}` |
| — | `GET /api/user/all` | ➕ | 不分页下拉 |
| — | `POST /api/user/updMail` | ➕ | 维护 `svn_user_mail`（为将来邮件通知预留） |

### 3.4 SVN 分组（Svngroup）

| 原版接口 | SVNAdmin4 | 状态 | 说明 |
|---|---|---|---|
| `Svngroup/GetGroupList` | `GET /api/group/list` | ✅ | |
| `Svngroup/CreateGroup` | `POST /api/group/create` | ✅ | |
| `Svngroup/DelGroup` | `POST /api/group/del` | ✅ | 清理 authz 中引用 |
| `Svngroup/UpdGroupName` | `POST /api/group/updName` | ✅ | |
| `Svngroup/UpdGroupNote` | `POST /api/group/updNote` | ✅ | |
| `Svngroup/GetGroupMember` | `GET /api/group/member` | ✅ | 三种成员：user / group / aliase |
| `Svngroup/UpdGroupMember` | `POST /api/group/updMember` | ⚠️ | 原版为"单个对象 add/delete"；重写版为**整组覆盖式**提交 `users[]`；循环嵌套检测已实现 |
| — | `GET /api/group/all` | ➕ | 不分页下拉 |

### 3.5 别名（Svnaliase）

| 原版接口 | SVNAdmin4 | 状态 | 说明 |
|---|---|---|---|
| `Svnaliase/GetAliaseList` | `GET /api/aliase/list` | ✅ | 读 authz `[aliases]` 段 |
| （原版本身也没有） | 别名增删改 | — | 双方一致：别名只能在 authz 文件手工维护 |

### 3.6 权限模型细节对比

| 能力 | 原版 | SVNAdmin4 | 状态 |
|---|---|---|---|
| 授权对象类型 | user / group / aliase / `*` / `$authenticated` / `$anonymous` | 同 6 种 | ✅ |
| 权限值 | rw / r / no（空） | rw / r / no | ✅ |
| 反转（`invert`，即 `~user`） | 支持 | ❌ 未实现 | ❌ |
| 分组嵌套继承 | 支持 | 支持（`inherits()`） | ✅ |
| 路径最长前缀匹配 | 支持 | 支持 | ✅ |
| 删除用户/分组时清理 authz 引用 | 支持 | 支持 | ✅ |
| 二次授权（`second_pri`） | 支持 | ❌ 未实现 | ❌ |

### 3.7 备份与恢复

| 能力 | 原版 | SVNAdmin4 | 状态 |
|---|---|---|---|
| 全量 dump | ✅ | ✅ | ✅ |
| 增量 dump（`--deltas`） | 仅定时任务 type=2 支持，手动备份不支持 | 手动备份也支持（`incremental`） | ➕ 增强 |
| 版本范围 `-r` | ❌ | ✅ | ➕ 增强 |
| 定时任务保存份数 `save_count` | ✅（按前缀 rsort 后删除旧文件） | ❌ 未实现 | ❌ |
| hotcopy 全量/增量 | 定义了 type 3/4 但**函数体为空** | 未实现 | — （原版本身未实现） |
| 备份文件命名 | 手动 `{rep}_{YmdHis}_{uniqid}.dump`；定时 `rep_{rep}_[deltas_]{YmdHis}.dump` | `{rep}_{YmdHis}.dump` | ⚠️ 简化 |
| 上传备份 | 分片上传 + 进度 + 暂停 | 整文件上传 | ⚠️ 缩减 |
| 下载备份 | token 校验流式输出 | 登录态校验 | ⚠️ 机制不同 |
| load 后刷新 `rep_rev` | ❌（源码注释 `//更新仓库的版本 todo`） | ✅（`refreshRep`） | ➕ 修复原版遗留问题 |

### 3.8 后台任务（Tasks）

| 原版接口 | SVNAdmin4 | 状态 | 说明 |
|---|---|---|---|
| `Tasks/GetTaskRun` | `GET /api/task/list` | ⚠️ | 重写版合并为单一列表 + `status` 过滤，未单独提供"运行中任务 + 实时日志" |
| `Tasks/GetTaskQueue` | `GET /api/task/list` | ⚠️ | 同上 |
| `Tasks/GetTaskHistory` | `GET /api/task/list` | ⚠️ | 同上 |
| `Tasks/GetTaskHistoryLog` | `GET /api/task/detail` | ✅ | |
| `Tasks/DelTaskHistory` | — | ❌ | 未提供删除历史任务接口（原版还有 bug：`task_lof_file` 拼写错误导致日志没删） |
| `Tasks/UpdTaskStop` | — | ❌ | **未实现**：中断/取消任务 |
| — | — | ⚠️ | 任务状态编码不同：原版 1待执行/2执行中/3已完成/4已停止/5历史；重写版 0等待/1运行中/2完成/3失败/4取消 |

### 3.9 定时任务（Crond）

| 原版接口 | SVNAdmin4 | 状态 | 说明 |
|---|---|---|---|
| `Crond/GetCrontabList` | `GET /api/crond/list` | ✅ | |
| `Crond/CreateCrontab` | `POST /api/crond/create` | ✅ | |
| `Crond/UpdCrontab` | `POST /api/crond/update` | ✅ | |
| `Crond/DelCrontab` | `POST /api/crond/del` | ✅ | |
| `Crond/UpdCrontabStatus` | `POST /api/crond/update` | ✅ | 合并进 update |
| `Crond/GetRepList` | `GET /api/rep/all` | ✅ | 复用仓库下拉 |
| `Crond/TriggerCrontab` | — | ❌ | **未实现**：手动立即触发（原版依赖 `at -f ... now`） |
| `Crond/GetCrontabLog` | — | ❌ | **未实现**：任务日志查看 |
| `Crond/GetCronStatus` | — | ➕不需要 | 重写版用进程内 cron，无需检测系统 crond/atd |

**任务类型对比（关键差距）**

| type | 原版 | SVNAdmin4 |
|---|---|---|
| 1 仓库备份 dump 全量 | ✅ | ✅（`CrondTypeDump`） |
| 2 仓库备份 dump 增量 deltas | ✅ | ❌ |
| 3 hotcopy 全量 | ⚠️ 空实现 | ❌ |
| 4 hotcopy 增量 | ⚠️ 空实现 | ❌ |
| 5 仓库检查（`svnadmin verify`） | ✅ | ❌ |
| 6 shell 脚本 | ✅ | ✅（`CrondTypeShell`） |
| 7 同步 SVN 用户 | ✅ | ❌ |
| 8 同步 SVN 分组 | ✅ | ❌ |
| 9 同步 SVN 仓库 | ✅ | ❌ |

**执行周期对比**

| cycle_type | 原版 | SVNAdmin4 |
|---|---|---|
| `minute` 每分钟 | ✅ | ✅ |
| `minute_n` 每隔 N 分钟 | ✅ | ✅（复用 `minute` + `minute` 字段） |
| `hour` 每小时 | ✅ | ✅ |
| `hour_n` 每隔 N 小时 | ✅ | ❌ |
| `day` 每天 | ✅ | ✅ |
| `day_n` 每隔 N 天 | ✅ | ❌ |
| `week` 每周 | ✅ | ✅ |
| `month` 每月 | ✅ | ✅ |

**消息通知**：原版 `notice` 0无/1成功/2失败/3全部 + 邮件发送；SVNAdmin4 **未实现**（依赖邮件模块）。

### 3.10 系统日志（Logs）

| 原版接口 | SVNAdmin4 | 状态 | 说明 |
|---|---|---|---|
| `Logs/GetLogList` | `GET /api/log/list` | ✅ | 分页 + 关键字 |
| `Logs/DelLogs` | `POST /api/log/clear` | ✅ | 清空 |
| — | `GET /api/log/types` | ➕ | 日志类型下拉 |
| 前端"导出日志"（Excel） | ✅ `vue-json-excel` | ❌ | 重写版前端未提供导出 |

### 3.11 统计（Statistics）

| 原版接口 | SVNAdmin4 | 状态 | 说明 |
|---|---|---|---|
| `Statistics/GetLoadInfo` | `GET /api/common/statistics` | ⚠️ | 重写版合并为一个接口，返回计数类统计；**负载/CPU/内存实时仪表盘未实现** |
| `Statistics/GetDiskInfo` | — | ❌ | 磁盘挂载点用量未实现 |
| `Statistics/GetStatisticsInfo` | `GET /api/common/statistics` | ⚠️ | 计数类统计已实现（仓库数/体积/备份数/备份体积/日志数/管理员数/子管理员数/用户数/分组数/别名数） |
| — | `GET /api/common/statistics` 内含 `repSizeRank` | ➕ | 仓库体积排行 |

### 3.12 子管理员（Subadmin）

| 原版接口 | SVNAdmin4 | 状态 | 说明 |
|---|---|---|---|
| `Subadmin/GetSubadminList` | `GET /api/subadmin/list` | ✅ | |
| `Subadmin/CreateSubadmin` | `POST /api/subadmin/create` | ✅ | |
| `Subadmin/DelSubadmin` | `POST /api/subadmin/del` | ✅ | |
| `Subadmin/UpdSubadminPass` | `POST /api/subadmin/updPass` | ✅ | |
| `Subadmin/UpdSubadminStatus` | `POST /api/subadmin/updStatus` | ✅ | |
| `Subadmin/UpdSubadminNote` | — | ❌ | 未提供改备注接口 |
| `Subadmin/GetSubadminTree` | — | ❌ | **未实现**：功能权限树读取（含 `needUpdateTree` 版本升级检测） |
| `Subadmin/UpdSubadminTree` | `POST /api/subadmin/updFunctions` | ⚠️ | 重写版为**扁平功能列表**授权，非树形勾选 |

### 3.13 个人中心（Personal）

| 原版接口 | SVNAdmin4 | 状态 | 说明 |
|---|---|---|---|
| `Personal/EditAdminUserName` | — | ❌ | 管理员改账号名未实现 |
| `Personal/EditAdminUserPass` | `POST /api/common/updatePass` | ✅ | |
| `Personal/UpdSubadminUserPass` | `POST /api/common/updatePass` | ✅ | 合并为统一改密（按当前登录角色） |
| `Personal/EditSvnUserPass` | `POST /api/common/updatePass` | ✅ | 同上；SVN 用户改密同步 passwd |
| — | `POST /api/common/updatePass` | ➕ | 统一入口，一次实现覆盖三种角色 |

### 3.14 二次授权（Secondpri）—— 整块未实现

| 原版接口 | SVNAdmin4 | 状态 |
|---|---|---|
| `Secondpri/UpdSecondpri` | — | ❌ |
| `Secondpri/GetSecondpriObjectList` | — | ❌ |
| `Secondpri/CreateSecondpriObject` | — | ❌ |
| `Secondpri/DelSecondpriObject` | — | ❌ |

业务含义：SVN 用户可把自己某条路径权限**再下放**给白名单内的 user/group/aliase。SVNAdmin4 已建扩展后的 `svn_second_pri` 表，但服务层与前端均未实现。

### 3.15 系统设置（Setting，原版 26 个 → 重写版 5 个，差距最大）

| 原版接口 | SVNAdmin4 | 状态 | 说明 |
|---|---|---|---|
| `Setting/GetVerifyOption` | `GET /api/common/verifyOption` | ✅ | |
| `Setting/UpdSvnserveStatusStart` | `POST /api/setting/svnserveStart` | ✅ | |
| `Setting/UpdSvnserveStatusStop` | `POST /api/setting/svnserveStop` | ✅ | |
| `Setting/GetDcokerHostInfo` | yaml `svn.svnserve_host` | ⚠️ | 改为配置文件，不走数据库 option |
| `Setting/UpdDockerHostInfo` | 配置文件 | ⚠️ | 同上，需改 yaml 重启 |
| `Setting/UpdSvnservePort` | yaml `svn.svnserve_port` | ⚠️ | 同上 |
| `Setting/UpdSvnserveHost` | yaml `svn.svnserve_host` | ⚠️ | 同上 |
| `Setting/UpdHttpPrefix` | yaml `svn.http_prefix` | ⚠️ | 同上 |
| `Setting/UpdHttpPort` | yaml `svn.http_host` | ⚠️ | 同上 |
| `Setting/GetSvnInfo` | `GET /api/setting/list` | ⚠️ | 重写版 option 表为通用键值表，不含 LDAP/数据源结构 |
| `Setting/UpdSvnUsersource` | — | ❌ | **未实现**：切换 svn 用户/分组来源（passwd / httpPasswd / ldap） |
| `Setting/UpdHttpUsersource` | — | ❌ | **未实现** |
| `Setting/GetApacheInfo` | — | ❌ | **未实现**：Apache 模块/安装目录探测 |
| `Setting/UpdSubversionEnable` | — | ❌ | **未实现**：写 `/etc/httpd/conf.d/subversion.conf` 切 http 协议 |
| `Setting/UpdSvnEnable` | — | ❌ | **未实现**：反向切回 svn 协议 |
| `Setting/LdapTest` | — | ❌ | **未实现**：LDAP 连接/用户/分组测试 |
| `Setting/UpdSaslStatusStart` | — | ❌ | **未实现**：saslauthd 启停 |
| `Setting/UpdSaslStatusStop` | — | ❌ | **未实现** |
| `Setting/GetMailInfo` | — | ❌ | **未实现** |
| `Setting/UpdMailInfo` | — | ❌ | **未实现** |
| `Setting/SendMailTest` | — | ❌ | **未实现** |
| `Setting/GetMailPushInfo` / `UpdPushInfo` | — | ❌ | **未实现**：4 个推送 trigger（登录、改管理员账号、改管理员密码、改 SVN 用户密码） |
| `Setting/GetSafeInfo` / `UpdSafeInfo` | yaml `security.enable_verify_code` | ⚠️ | 仅登录验证码一项可配 |
| `Setting/CheckUpdate` | — | ❌ | **未实现**：版本更新检测 |
| `Setting/GetDirInfo` | — | ❌ | **未实现**：路径信息展示页 |
| — | `GET /api/setting/list` | ✅ | 通用 option 键值列表 |
| — | `POST /api/setting/update` | ✅ | 通用 option 写入 |

**原版 `options` 表 9 个键 → 重写版**

| option_name | 原版 | SVNAdmin4 |
|---|---|---|
| `24_docker_host`（宿主机 + svn 端口 + web 端口） | ✅ | ⚠️ 移到 yaml |
| `24_local_host`（本地 http 端口） | ✅ | ⚠️ 移到 yaml |
| `24_enable_checkout`（svn / http） | ✅ | ⚠️ 移到 yaml `svn.enable_http` |
| `24_svn_datasource`（user_source/group_source/ldap） | ✅ | ❌ |
| `24_http_datasource` | ✅ | ❌ |
| `24_http_prefix` | ✅ | ⚠️ 移到 yaml |
| `mail_smtp` | ✅ | ❌ |
| `message_push` | ✅ | ❌ |
| `safe_config` | ✅ | ⚠️ 仅保留登录验证码 |

---

## 四、前端页面对比

| 原版页面（路由） | SVNAdmin4 页面 | 状态 | 差异 |
|---|---|---|---|
| `/login` | `views/login/index.vue` | ⚠️ | 原版有**角色下拉**（管理人员/子管理员/SVN用户，记 `localStorage`）；重写版按后端返回角色自动分流，**无角色选择** |
| `/index` 信息统计 | `views/dashboard/index.vue` | ⚠️ | 原版：负载/CPU/内存/磁盘环形仪表盘（3 秒轮询）+ 10 个数字卡片；重写版只保留计数卡片与体积排行，**无实时监控仪表盘** |
| `/repositoryInfo` SVN仓库 | `views/repository/index.vue` | ➕**重构** | 原版全部是**弹窗**；重写版改为**页面级 5 Tab**：仓库属性 / 目录结构 / 路径权限 / 备份与恢复 / 仓库钩子 |
| ↳ 仓库浏览弹窗（全屏） | 目录结构 Tab | ✅ | |
| ↳ 仓库钩子弹窗（仓库钩子/常用钩子 2 Tab） | 仓库钩子 Tab | ⚠️ | 重写版保留 9 类钩子编辑；常用钩子（推荐钩子）**未做查看入口** |
| ↳ 高级弹窗（仓库属性/仓库备份 2 Tab） | 拆为「仓库属性」「备份与恢复」两个 Tab | ✅ | |
| ↳ 权限弹窗 `modalRepPri` | 路径权限 Tab | ⚠️ | 重写版为表单式路径授权；原版为**左树（懒加载 + 右键新建文件夹）+ 右权限表**；重写版**缺反转（invert）开关** |
| ↳ 备份分片上传（进度/暂停） | 备份与恢复 Tab | ⚠️ | 简化为整文件上传 |
| `/repositoryUser` SVN用户 | `views/user/index.vue` | ⚠️ | 原版有**用户迁入 3 步向导**（粘贴 passwd 正文 → 逐行移除 → 导入结果）、**权限路径弹窗 + 二次授权 Switch + 二次授权对象弹窗**；重写版保留列表/增删改/启停/改密/备注/扫描导入/权限查看，**缺二次授权全部 UI** |
| `/repositoryGroup` SVN分组 | `views/group/index.vue` | ✅ | 成员管理（user/group/aliase）已实现 |
| `/subadmin` 子管理员 | `views/subadmin/index.vue` | ⚠️ | 原版有**双权限树勾选**（旧树 + 新树，`needUpdateTree` 提示）；重写版为扁平功能勾选 |
| `/setting` 系统配置（8 Tab） | `views/setting/index.vue` | ❌**大幅缩减** | 仅保留 svnserve 状态卡 + option 键值表；原版 8 Tab（主机配置/路径信息/svn协议检出/http协议检出/邮件服务/消息推送/安全配置/关于更新）全部缺失 |
| `/crond` 任务计划 | `views/crond/index.vue` | ⚠️ | 任务类型仅 2 种（备份/shell）；周期 5 种；**无手动执行、无任务日志、无保存份数、无消息通知** |
| `/logs` 系统日志 | `views/logs/index.vue` | ⚠️ | 缺 Excel 导出 |
| `/personal` 个人中心 | `views/personal/index.vue` | ⚠️ | 统一改密；**缺"修改管理员账号名"**；改密后不强制登出 |
| （Header 后台任务弹窗，3 Tab） | `views/task/index.vue` | ⚠️ | 原版为 Header 弹窗（当前/排队/历史 + 实时日志 + 中断/取消）；重写版为独立页面，**缺中断任务、缺删除历史、缺实时日志轮询** |
| `/distribute` | — | — | 原版本身是**死文件**（与 subadmin 逐字节相同、未注册路由），无需迁移 |
| 组件 `modalSvnObject`（6 Tab 选择授权对象） | 路径权限 Tab 内下拉 | ⚠️ | 重写版未做独立对象选择弹窗（含分组"成员"查看） |

**通用交互能力对比**

| 能力 | 原版 | SVNAdmin4 |
|---|---|---|
| 分页 | ✅ | ✅ |
| 服务端排序 | ✅ | ❌（前端本地排序） |
| 行内编辑备注 | ✅ | ✅ |
| 行内 Switch 启停 | ✅ | ✅ |
| 目录懒加载树 + 右键新建文件夹 | ✅ | ❌ |
| 导入向导（多步） | ✅ | ⚠️ 简化为一步 |
| 日志 Excel 导出 | ✅ | ❌ |
| 分片上传 + 进度 + 暂停 | ✅ | ❌ |
| 复制到剪贴板（检出地址/仓库属性） | ✅（`vue-clipboard2`） | ❌ |
| 首页 3 秒轮询 | ✅ | ❌ |
| 全屏弹窗 / 可拖拽弹窗 | ✅ | ⚠️ 部分 |
| 后端驱动的动态路由 + 菜单分组 | ✅ | ❌（改为前端静态路由） |

---

## 五、部署与运维对比

| 维度 | 原版 | SVNAdmin4 |
|---|---|---|
| 安装 | `server/install.php` 初始化目录/配置/服务单元；需 WANdisco 安装脚本装 Subversion | 无需安装，下载二进制直接运行 |
| 进程数 | php-fpm + httpd + svnserve + saslauthd + svnadmind + crond + atd（**7 类**） | **1 个** Go 进程 + 可选 svnserve |
| systemd 单元 | `svnadmind.service`、`svnserve.service` | `deploy/svnadmin4.service`（单个） |
| 静态资源 | Apache/Nginx 托管 | 内置 `-web ./web/dist`，SPA 回落 |
| 定时任务可靠性 | 依赖系统 crond/atd；重装后需重建 crontab 行 | 进程内调度，随程序启停 |
| 数据库迁移 | SQL 文件 + install.php | GORM AutoMigrate（启动自动建表/加索引） |
| 容器化 | 无官方 Dockerfile | ➕ 提供 `Dockerfile` + `deploy/docker-entrypoint.sh` |
| 跨平台 | 需 Linux + PHP 环境 | ➕ 单二进制可交叉编译（Windows/macOS 亦可，只要能装 svn 命令行） |

---

## 六、结论：差距清单

### 6.1 完全未实现（原版有，SVNAdmin4 无）

1. **LDAP 对接**（`app/service/Ldap.php` 705 行）：数据源切换、LDAP 用户/分组同步、连接测试、`SyncLdapToAuthz`
2. **SASL**：saslauthd 启停、`/etc/sasl2/svn.conf` 维护
3. **Apache / HTTP 协议切换**：`subversion.conf` 生成、httpd graceful、http 数据源切换
4. **邮件与消息推送**：SMTP 配置、测试邮件、4 个推送 trigger、定时任务邮件通知
5. **二次授权（Secondpri）**：4 个接口 + 用户页全部相关 UI
6. **子管理员功能权限树**（`subadmin_tree`）：树形勾选与 `necessary_functions` 精确接口级授权
7. **定时任务类型 2/3/4/5/7/8/9**（增量备份、hotcopy、仓库检查、同步用户/分组/仓库）
8. **定时任务手动触发与任务日志**（`TriggerCrontab` / `GetCrontabLog`）
9. **后台任务中断/取消/删除历史**（`UpdTaskStop` / `DelTaskHistory`）
10. **系统监控仪表盘**：负载、CPU、内存、磁盘（`GetLoadInfo` / `GetDiskInfo`）
11. **备份保存份数 `save_count`** 与增量备份定时任务
12. **备份分片上传**与进度控制
13. **版本更新检测**（`CheckUpdate`）
14. **SVN 用户视角内容浏览**（`GetUserRepCon`）与二次授权目录树（`GetRepTree2`）
15. **权限反转（`invert`）**
16. **管理员账号名修改**
17. **日志 Excel 导出**
18. **管理员改密后强制登出**（原版行为）

### 6.2 重写版新增 / 优于原版

1. 单二进制交付，进程数从 7 类降到 1 个
2. 定时任务进程内调度，**不依赖系统 crond/atd**
3. 权限以 authz 文件为唯一数据源，取消 `svn_user_pri_paths` 镜像表，消除文件/库不一致风险
4. 管理员/子管理员密码 SHA-256 哈希（原版明文）
5. JWT + 内存黑名单（原版要查 `black_token` 表）
6. 手动备份支持 **增量 + 版本范围**
7. `svnadmin load` 后自动刷新 `rep_rev`（原版遗留 TODO 未做）
8. 数据主目录可配置（原版硬编码 `/home/svnadmin/`）
9. 提供 Dockerfile 与 systemd 单元
10. TypeScript + 组合式 API，前端类型安全
11. 仓库管理改为页面级 5 Tab（原版深嵌套弹窗）
12. 前端构建产物可直接由后端托管，`make build && make run` 一条链路

### 6.3 已知行为偏差

| 项 | 说明 |
|---|---|
| 任务状态编码 | 原版 1/2/3/4/5；SVNAdmin4 0/1/2/3/4，前端已适配新编码 |
| 分组成员更新 | 原版单对象 add/delete；SVNAdmin4 整组覆盖 |
| 用户扫描 | 原版提交文件正文；SVNAdmin4 读服务端文件 |
| 备份下载鉴权 | 原版 HMAC token；SVNAdmin4 登录态 |
| 验证码 | 原版 PNG base64；SVNAdmin4 SVG |
| 路由下发 | 原版后端下发；SVNAdmin4 前端静态 |
| 备份命名 | `{rep}_{YmdHis}.dump`（原版手动备份带 `uniqid` 后缀） |

---

## 七、覆盖率速算（补齐后）

| 模块 | 原版接口数 | SVNAdmin4 已覆盖 | 覆盖率 |
|---|---|---|---|
| 公共/登录 | 3 | 3 + 1（角色选择） | 100% |
| 仓库管理 | 34 | 34（verify/hotcopy/tree2/userCon 已补齐） | 100% |
| SVN 用户 | 9 | 9 + 2（updMail / sync） | 100% |
| SVN 分组 | 8 | 8 + 1（sync） | 100% |
| 别名 | 1 | 1 | 100% |
| 备份恢复 | 4 | 4 + 6（分片上传） | 100% |
| 钩子 | 4 | 4 | 100% |
| 后台任务 | 6 | 6 + 5（run/log/stop/del/clear/types） | 100% |
| 定时任务 | 9 | 9 + 3（dict/trigger/log） | 100% |
| 日志 | 2 | 2（+ 前端 Excel 导出） | 100% |
| 统计 | 3 | 3 + 4（monitor/loadInfo/diskInfo/version） | 100% |
| 子管理员 | 8 | 8 + 3（tree/updTree/updNote） | 100% |
| 个人中心 | 4 | 4 + 1（editAdminUserName） | 100% |
| 二次授权 | 4 | 4 | 100% |
| 系统设置 | 26 | 26（含 LDAP / SASL / Apache / 邮件 / 推送 / 安全） | 100% |
| **合计** | **117** | **117 + 约 60 项扩展** | **100%（并超出）** |

> 说明：原版 117 个接口已全部具备对等实现；其中原版为空壳或 TODO 的 `hotcopy`（定时任务类型 3/4）、`save_count` 保留份数清理、`svnadmin load` 后刷新版本号等，在 SVNAdmin4 中均为**完整实现**。此外新增约 60 个接口用于监控、任务控制、分片上传、LDAP、SASL、Apache、邮件推送、二次授权等能力。

---

## 八、后续补齐建议（按优先级）

> 以下 P0–P3 项在本次迭代中**已全部完成**，保留清单用于追溯原始差距。

| 优先级 | 事项 | 状态 |
|---|---|---|
| P0 | 后台任务中断/取消/删除历史 + 实时日志 | ✅ 已完成 |
| P0 | 子管理员功能权限树（`subadmin_tree`） | ✅ 已完成 |
| P1 | 定时任务补齐类型 5/7/8/9 + `save_count` + 手动触发 + 任务日志 | ✅ 已完成 |
| P1 | 系统监控仪表盘（负载/CPU/内存/磁盘） | ✅ 已完成 |
| P1 | 二次授权（Secondpri）全链路 | ✅ 已完成 |
| P2 | 权限反转 `invert` + SVN 用户视角浏览（`GetUserRepCon`/`GetRepTree2`） | ✅ 已完成 |
| P2 | 设置页 8 Tab 重建（主机/路径/协议/安全） | ✅ 已完成 |
| P3 | 邮件通知与消息推送 | ✅ 已完成 |
| P3 | LDAP 对接 + SASL | ✅ 已完成 |
| P3 | Apache / HTTP 协议切换 | ✅ 已完成 |

---

## 九、补齐实现对照（本次迭代）

### 9.1 后端新增文件

| 文件 | 职责 |
|---|---|
| `server/internal/service/monitor.go` | 负载 / CPU / 内存 / 磁盘 / 主机信息采集（`/proc` 解析 + `Statfs`） |
| `server/internal/service/statfs_unix.go`、`statfs_windows.go` | 跨平台磁盘统计 |
| `server/internal/service/mail.go` | SMTP 配置、测试邮件、6 个推送触发点（含任务成功/失败两项扩展） |
| `server/internal/service/ldap.go` | LDAP 拨号/绑定/检索、用户与分组同步、LDAP 鉴权 |
| `server/internal/service/apache.go` | Apache + mod_dav_svn 探测、`subversion.conf` 生成与重载、SASL(saslauthd) 管理 |
| `server/internal/service/upload.go` | 分片上传会话（MD5 校验、12 小时过期清理） |
| `server/internal/service/secondpri.go` | 二次授权：开关 + 可下放对象白名单 + 过滤 |
| `server/internal/service/subadmintree.go` | 子管理员功能权限树（`necessaryFunctions` → 接口级白名单） |
| `server/internal/service/update.go` | 版本更新检测（语义化版本比较 + 变更清单解析） |
| `server/internal/service/options.go` | 配置项常量、数据源、安全配置 |
| `server/internal/service/proc_unix.go`、`proc_other.go` | 进程组创建与整组终止（任务可中断的关键） |

### 9.2 后端改造要点

| 项 | 实现 |
|---|---|
| 任务中断 | `exec.Cmd` 以 `Setpgid` 独立进程组启动，终止时 `kill(-pid)` 级联杀死子进程；排队任务通过 cancel channel 丢弃 |
| 任务状态 | 新增 `已取消(4)`；任务完成/失败触发邮件推送 |
| 定时任务 | 9 种任务类型（hotcopy 全量/增量、仓库检查、同步用户/分组/仓库均为完整实现）、8 种周期、保留份数自动清理、手动触发、执行日志 |
| 权限反转 | `authz` 解析器识别 `~` 前缀，`Entry.Invert` + `RawName()` 保证写回保真；`entryMatches()` 按取反语义匹配 |
| 二次授权 | 「owner + rep + path」唯一键，开关记录与白名单记录分离；关闭时清空白名单 |
| 登录角色 | `role` 可选参数（admin/subadmin/svnuser），指定时只查对应身份表；留空按序自动识别 |
| 分片上传 | 8MB 分片、MD5 校验、断点续传（已收分片可查询） |

### 9.3 前端补齐

| 页面 | 补齐内容 |
|---|---|
| `dashboard` | 负载/CPU/内存环形仪表盘 + 磁盘进度条 + 9 个统计卡片 + 3 秒自动刷新开关 + 后端任务/定时任务状态 |
| `task` | 3 Tab：当前任务（实时日志 2 秒轮询 + 中断）、排队任务（取消）、历史任务（状态筛选 / 日志 / 删除 / 清空） |
| `crond` | 字典驱动的 9 种任务类型与 8 种周期、多仓库或「全部仓库」、保留份数、消息通知、手动执行、执行日志 |
| `setting` | 9 Tab：主机配置 / 路径信息 / SVN 协议检出（含 SASL）/ HTTP 协议检出（Apache）/ LDAP / 邮件服务 / 消息推送 / 安全配置 / 关于与更新 + 配置项键值表 |
| `user` | 二次授权抽屉（逐路径开关 + 白名单对象管理）、邮箱编辑、用户迁入 3 步向导（粘贴 passwd → 移除条目 → 导入结果） |
| `subadmin` | 功能权限树（el-tree 勾选 + `needUpdateTree` 提示 + 接口明细展示）、备注编辑 |
| `repository` | 目录树懒加载 + 右键菜单（新建子目录 / 复制路径 / 设为权限路径 / 查看目录条目）、权限取反开关、分片上传（进度 + 取消）、检出地址一键复制、仓库检查与热拷贝、推荐钩子套用 |
| `logs` | Excel 导出（SpreadsheetML 2003，Excel/WPS 可直接打开） |
| `login` | 登录身份下拉（自动识别 / 管理人员 / 子管理员 / SVN 用户） |
| `personal` | 修改管理员账号名；改密后强制登出 |

### 9.4 回归验证

| 脚本 | 断言数 | 结果 |
|---|---|---|
| `scripts/smoke.sh`（核心 SVN 能力） | 42 | 42 通过 / 0 失败 |
| `scripts/smoke2.sh`（本次补齐能力） | 65 | 65 通过 / 0 失败 |
| `cd web && npm run build`（含 `vue-tsc --noEmit`） | — | 通过 |
| `cd server && go build ./... && go vet ./...` | — | 通过 |
