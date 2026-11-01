# SVNAdmin4 与 SvnAdminV2.0 功能对比（现状快照）

| 项目 | 原版 | 重写版 |
|---|---|---|
| 名称 | [witersen/SvnAdminV2.0](https://github.com/witersen/SvnAdminV2.0) | 本仓库 SVNAdmin4 |
| 版本 | 2.5.10（commit `1c4305a`，2024-10-06） | 4.0.0 |
| 技术栈 | PHP 5.5+ / Vue2 + iView / Medoo | Go 1.22 + Gin + GORM / Vue3 + TS + Element Plus |
| 接口基数 | 13 个控制器、**116 个方法**（`Setting::UpdPushInfo` 未写 `public` 修饰符，仍可路由调用） | 9 个 api 文件、**141 条路由**（免登录 3 / 任意角色 36 / 管理端 92 / 超管 8 / 独立注册 2） |
| 数据表 | **13 张** | **11 张** |
| 前端页面 | 12 个视图目录 | 12 个视图目录（新增 `myrepo`、`task`，原 `repositoryUser` 与任务弹窗重组） |
| 许可证 | GPL-3.0 | GPL-3.0-or-later（与上游保持一致，见 `LICENSE` / `NOTICE`） |

对比口径：逐控制器方法核对入参、返回体、文件与数据库副作用、边界错误；所有结论均可落到两侧源码行。运行时行为以 `scripts/smoke.sh`(42) + `smoke2.sh`(65) + `smoke3.sh`(37) + `smoke4.sh`(46) 共 **190 条断言**在真实实例上核验。

图例：✅ 对等 · ⭐ 更强 · ⚠️ 实现不同但能力对等 · 🔻 弱于原版 · ❌ 缺失 · ➕ 重写版新增

---

## 一、模块覆盖总览

| 模块 | 原版接口 | 重写版入口 | 判定 |
|---|---|---|---|
| 公共 / 登录 | 3（Login、Logout、GetVerifyCode） | `/api/common/login`、`/logout`、`/verifyCode`、`/verifyOption` | ✅ 功能对等，多一个聚合开关接口 |
| 个人中心 | 4（EditAdminUserName、EditAdminUserPass、EditSvnUserPass、UpdSubadminUserPass） | `/api/common/editAdminUserName`、`/updatePass`（三种身份统一分支） | ✅ 4 → 2 合并，校验更严 |
| SVN 用户 | 8 | `/api/user/*`（list/all/scan/import/create/del/updPass/updStatus/updNote/updMail/sync） | ⭐ 增 3 个 |
| SVN 分组 | 7 | `/api/group/*`（list/all/create/del/updName/updNote/member/updMember/sync） | ⭐ 增 2 个 |
| 别名 | 1（GetAliaseList） | `/api/aliase/list` | ✅ |
| 仓库 | 34（含权限 4 个） | `/api/rep/*`（22）、`/api/pri/*`（4） | ✅ 一一对应，另增 8 个 |
| 后台任务 | 6 | `/api/task/*`（list/detail/run/log/types/stop/del/clear） | ⭐ 增 2 个 |
| 定时任务 | 9 | `/api/crond/*`（list/create/update/del/trigger/dict/log） | ⚠️ 9 → 7 合并 |
| 统计 | 3（GetLoadInfo、GetDiskInfo、GetStatisticsInfo） | `/api/common/*`（statistics/monitor/loadInfo/diskInfo） | ⭐ 增 1 个 |
| 日志 | 2（GetLogList、DelLogs） | `/api/log/*`（list/types/clear） | ⭐ 增 1 个 |
| 子管理员 | 8 | `/api/subadmin/*`（list/create/del/updStatus/updPass/updNote/tree/updTree） | ✅ 一一对应 |
| 二次授权 | 4 | `/api/secondpri/*`（update/objectList/objectCreate/objectDel/userPaths） | ⭐ 增 1 个 |
| 系统设置 | 27 | `/api/setting/*`、`/api/mail/*`、`/api/push/*`、`/api/safe/*`、`/api/ldap/*` | ✅ 拆分更细 |
| 备份与钩子 | （含在仓库 34 内） | `/api/backup/*`（12）、`/api/hook/*`（4） | ⭐ 上传会话化 |

---

## 二、架构与运行时

| 维度 | 原版 | 重写版 | 判定 |
|---|---|---|---|
| 分发 | `api.php?c=Xxx&a=Yyy` 反射分发 + 控制器/服务双层 | RESTful `/api/模块/动作` + handler/service 两层 | ⚠️ 风格差异 |
| 配置存储 | Medoo + PHP `serialize` | GORM + JSON | ⚠️ 值格式不互通 |
| 交付形态 | PHP 站点 + 独立守护进程 `svnadmind`（systemd + Unix Socket + 3s 轮询） | 单一二进制：内置静态资源（`-web`）与调度器 | ⭐ 少一个常驻进程 |
| 任务执行 | 守护进程 `passthru` 拉取 status=1 记录 | 进程内 goroutine 队列（缓冲 1024、并发 1、`Setpgid` 独立进程组） | ⭐ 免守护进程 |
| 定时调度 | 系统 `crond` + `atd`（生成脚本 + `crontab` 装载 + `at -f now`） | 进程内 `robfig/cron/v3`（6 段含秒） | ⭐ 免系统依赖；⚠️ 进程退出即停摆 |
| 鉴权 | 自定义 token：`role\|name\|start\|end\|HMAC-MD5`，落库，`black_token` 拉黑 | JWT HS256（`X-Token` 头或 `?token=` 兜底），黑名单落库 + 内存双缓存 | ⚠️ 机制不同；能力对等 |
| 逐接口授权 | `Base.php`：role=2 查 `svn_user_routers` 白名单；role=3 查 `subadmin_functions` | 中间件按路由分组 + `FuncKey`/`SubadminCanAccess` 白名单（30s TTL 缓存） | ✅ 语义对齐 |
| 密码 | 管理员/SVN 用户明文；子管理员 md5 | 管理员/子管理员 SHA-256 + 恒时比较；SVN 用户明文（passwd 机制所限） | ⭐ |
| 外部依赖 | php-cli、pcntl、crond、atd、htpasswd、可选 php-ldap/php-gd | svnadmin/svnlook/svn/svnserve，可选 saslauthd/httpd | ⭐ 依赖面小 |
| 平台 | 仅 Linux | Linux/macOS/Windows（syscall 按 build tag 拆分） | ⭐ |

---

## 三、数据模型（13 → 11）

| 原版表 | 重写版 | 差异 |
|---|---|---|
| admin_users | `admin_users` | 密码改 SHA-256；不再存 token | ⭐ |
| subadmin | `subadmin` | 密码 SHA-256；`subadmin_functions` 由 JSON 数组改为逗号串；新增 phone/email | ✅ |
| svn_users | `svn_users` | `svn_user_mail` 由死字段变为可读写；`svn_user_token` 存 token 到期戳用于在线判定；用户名唯一索引 | ⭐ |
| svn_groups | `svn_groups` | 字段同构，加唯一索引 | ✅ |
| svn_reps | `svn_reps` | 字段同构 | ✅ |
| svn_user_pri_paths | — | **取消镜像表**，权限以 authz 为唯一数据源；二次授权改用 `owner+rep+path` 组合键定位 | ⚠️ 设计变更（免去 sync 漂移） |
| svn_second_pri | `svn_second_pri` | 开关记录与白名单记录同表；`second_pri_unique(owner+rep+path)` 唯一键 | ⚠️ 结构扩展 |
| crond | `crond` | `rep_name` 支持多仓库 JSON 数组 | ⭐ |
| tasks | `tasks` | 新增 `task_message`（失败/取消原因）；状态语义见 4.7 | ⭐ |
| logs | `logs` | 同构 | ✅ |
| options | `options` | 值为 JSON；新增 `svnserve_status`/`sasl_status` 等运行时项 | ⚠️ 不兼容原版 serialize |
| black_token | `black_token` | 同款落库，另加内存缓存 | ✅ |
| verification_code | — | 取消落库，改进程内存（180s、SVG） | ⚠️ 多实例不共享 |

索引与约束：重写版对 `svn_users.svn_user_name`、`svn_groups.svn_group_name`、`svn_reps.rep_name` 建唯一索引，原版仅靠业务层查重。

---

## 四、接口逐一对比

### 4.1 公共 / 登录（3）

| 原版接口 | 原版行为 | 重写版 | 重写版行为 | 判定 |
|---|---|---|---|---|
| Login | `user_role` 必填（1/2/3）；管理员明文比对；SVN 用户依次尝试本地口令 → **LDAP 远程 bind** → **Apache 认证**；认证成功后把口令回写 passwd/htpasswd；返回 `{token, user_role_name, user_role_id, route, functions}` 供前端动态路由；成功发登录邮件；受 `safe_config` 登录失败锁定约束 | `POST /api/common/login` | `role` 可选（admin/subadmin/svnuser/中文别名），为空时按 管理员→子管理员→SVN 用户 顺序识别；返回 `{token, role, userName, userId, expire, functions}`；SVN 用户本地口令失败后按数据源走 `LDAPBindAuth` 并回写口令；登录前查 `loginLockLeft`，失败计数与锁定时长取自 `safe_config`；成功后异步 `notifyLogin` 发邮件；子管理员 `functions` 为实际白名单 | ✅ 对等；🔻 （Apache 认证分支未实现，见 7.1） |
| Logout | token 写 `black_token` 表（登录时清理过期记录）；各身份表 token 字段置 `-`；写日志 | `POST /api/common/logout` | token 写 `black_token` + 内存黑名单；SVN 用户 `svn_user_token` 置 `-`（在线态立即消失）；写日志 | ✅ |
| GetVerifyCode | PHP GD 生成 PNG（base64）+ 落库 60s；登录失败作废该 uuid 全部记录 | `GET /api/common/verifyCode` | 纯 Go 拼 SVG（前端 `v-html`）+ 内存 180s；字符集剔除 `0/1/I/O`；校验后即删 | ⚠️ 免 GD 依赖；⚠️ 内存态 |

### 4.2 个人中心（4）

| 原版接口 | 原版行为 | 重写版 | 重写版行为 | 判定 |
|---|---|---|---|---|
| EditAdminUserName | 改 `admin_users.user_name`；**不校验原密码**；不写日志；成功文案误写为"修改密码成功" | `POST /api/common/editAdminUserName` | `AdminUpdName(oldName,newName,pass)`：校验当前密码 + 用户名正则 + 查重 + 写日志 + 发通知邮件；前端改名成功后 800ms 强制登出 | ⭐ |
| EditAdminUserPass | 明文覆写；不校验原密码 | `POST /api/common/updatePass` | `AdminUpdPass`：SHA-256 + 原密码校验 + 强度校验（受 `safe_config` 控制）+ 日志 + 邮件 | ⭐ |
| EditSvnUserPass | LDAP 来源直接拒绝；svn 源改 passwd、http 源改 htpasswd | 同上（按用户名命中身份自动分支） | SVN 分支落 `UpdUserPass`：LDAP 源拒绝 → 强度校验 → 同步 DB 与 passwd/htpasswd（仅启用态回写文件） | ✅ |
| UpdSubadminUserPass | md5 覆写 | 同上 | `SubadminUpdPass`：原密码校验 + SHA-256 | ⭐ |

### 4.3 SVN 用户（8）

| 原版接口 | 原版行为 | 重写版 | 重写版行为 | 判定 |
|---|---|---|---|---|
| GetUserList | 入参 `sync` 时先做 passwd/htpasswd/LDAP→DB 差集同步；内存过滤 + 分页 + 4 字段排序（`svn_user_id/name/status/last_login`）；返回**明文密码**与 `online`（token 到期比较）；role=2 时按二次授权白名单裁剪对象 | `GET /api/user/list` | SQL LIKE（名称/备注）+ 分页 + status 过滤 + `orderClause` 白名单排序；`online` 由 `svn_user_token` 到期戳计算；密码字段 `json:"-"` 不下发；同步拆分为 `/api/user/sync` | ⭐ 不下发密码；⚠️ 同步解耦；🔻 role=2 对象裁剪缺失（见 7.1） |
| UserScan | 扫描**前端粘贴的任意 passwd 文本**；识别 `#disabled#` 前缀；返回 `{userName,userPass,disabled}[]`；LDAP 源拒绝 | `GET /api/user/scan` | 读服务器 passwd 文件，与 DB 差集返回**未纳管的用户名数组**；LDAP 源拒绝 | ⚠️ 语义改为"纳管本机文件"；`#disabled#` 已识别（`Passwd.Disabled`） |
| UserImport | 逐条写入 passwd/htpasswd + DB 删重插；返回逐条失败原因 | `POST /api/user/import` | 将 passwd 中已存在、DB 未登记的用户纳库（备注"由 passwd 文件扫描导入"），`#disabled#` 用户以 status=0 落库；返回 `{count}` | ⚠️ 语义变化；🔻 无逐条结果 |
| UpdUserStatus | 给密码行加/去 `#disabled#` 前缀保留原密码；LDAP 拒绝 | `POST /api/user/updStatus` | DB 状态切换 + passwd/htpasswd 整行增删（启用时用 DB 明文重建）；存在性校验 + 日志 + LDAP 源拦截 | ⚠️ 文件格式不同（行增删 vs 前缀）；⭐ 校验/日志 |
| UpdUserNote | 直接 UPDATE，不校验存在 | `POST /api/user/updNote` | 存在性校验 + 日志 | ⭐ |
| CreateUser | 先写文件再写 DB（DB 删后重建，可覆盖同名）；LDAP 拒绝 | `POST /api/user/create` | 先 DB（重名直接报错）再写 passwd + htpasswd（Apr1 纯 Go 实现，不调 `htpasswd`）；接收 `svnUserMail`；强度校验；日志 | ⭐ 不覆盖同名；⚠️ DB 先写存在毫秒级不一致窗口 |
| UpdUserPass | svn/http 二选一改文件；无条件 UPDATE DB | `POST /api/user/updPass` | UPDATE DB；仅启用态回写文件（避免禁用用户被动复活）；强度校验 + LDAP 拦截 + 日志 | ⭐ |
| DelUser | 删文件 → authz 清权限行与分组成员引用 → 删 DB → 日志 | `POST /api/user/del` | 同链路 + 存在性校验；`DelObjectPri` + 分组成员遍历清理 | ✅ |

### 4.4 SVN 分组（7）

| 原版接口 | 原版行为 | 重写版 | 重写版行为 | 判定 |
|---|---|---|---|---|
| GetGroupList | `sync` 开关触发 authz→DB 同步；内存分页 + 2 字段排序；role=2 白名单裁剪 | `GET /api/group/list` | 每次强制同步（代价：列表即写库）；SQL 分页 + 关键字 + 排序；另有 `/api/group/all` 供下拉 | ✅；⚠️ 每次列表写库 |
| UpdGroupNote | 直接 UPDATE | `POST /api/group/updNote` | authz 存在性校验 + DB 自动补记录 + 日志 | ⭐ |
| CreateGroup | 双 LDAP 源拒绝；DB 删重插 | `POST /api/group/create` | `HasGroup` 冲突报错 + 写 `[groups]` 段 + Save + DB Create；LDAP 源拦截 | ⚠️ DB Create 错误未检查（唯一键冲突时备注可能不落库） |
| DelGroup | authz 清组行 + 权限行 + 嵌套引用；删 DB | `POST /api/group/del` | `DelGroup` 原子处理（含 GroupOrder 维护与嵌套引用）+ 存在性校验 + 日志 | ✅ ⭐ |
| UpdGroupName | 正则改写 authz 组名；DB 靠两次 sync 差集迁移（**备注丢失**） | `POST /api/group/updName` | `RenameGroup` 结构化改写（key/Order/成员引用/权限项）+ 直接 UPDATE DB（备注保留）+ 日志 | ⭐ |
| GetGroupMember | 扁平 `{objectType,objectName}[]`；`searchKeyword`；分组不存在报 720；role=2 裁剪 | `GET /api/group/member` | 三段式 `{users[],groups[],aliases[]}`；`searchKeyword` 模糊过滤；分组不存在返回空集 | ⚠️ 结构变化；⚠️ 无 720 错误码 |
| UpdGroupMember | **单对象 add/delete**；重复 803 / 不存在 703；循环嵌套检测（A→B、B→A 拒绝）；LDAP 拒绝 | `POST /api/group/updMember` | **三数组全量覆盖**；即时更新 count；`checkGroupCycle`（直接互含 + DFS 查间接环）拒绝；LDAP 源拦截；日志 | ⚠️ 幂等性更强；⚠️ 成员顺序重排、行内注释丢失 |

### 4.5 别名（1）

| 原版接口 | 原版行为 | 重写版 | 重写版行为 | 判定 |
|---|---|---|---|---|
| GetAliaseList | 解析 `[aliases]` 段；`searchKeyword` 对别名与内容双向匹配；role=2 白名单裁剪 | `GET /api/aliase/list?searchKeyword=` | 解析 `[aliases]` 段，返回 `map[别名]内容`；`searchKeyword` 双向匹配 | ⚠️ 返回结构由数组变 map；🔻 role=2 裁剪缺失（见 7.1）。两侧均无别名**写**接口（原版 service 层为空实现） |

### 4.6 仓库（34）

| 原版接口 | 原版行为 | 重写版 | 重写版行为 | 判定 |
|---|---|---|---|---|
| CheckAuthz | 调外部 `svnauthz-validate`，回显官方错误 | `GET /api/rep/checkAuthz` | 自研解析器校验段与路径存在性，返回结构化错误 | ⚠️ 零依赖；规则覆盖略窄于官方工具 |
| GetCheckout | 返回 `{protocal, prefix}` 两段，端口 3690/80/443 省略 | `GET /api/rep/checkout` | 直接返回完整 `svn://` / `http(s)://` 地址 | ⭐ 前端可直接复制 |
| GetSvnserveStatus | 仅 svn 协议下检测，返回状态 + 中文说明 | `GET /api/common/svnserveStatus` | pid 文件 + `/proc` 判定 | ✅（无说明文案） |
| CreateRep | `rep_type` 二选一：1 空库 / 2 套唯一硬编码 `initStruct/01`；`svn import` 作者写死 `SVNAdmin`；执行 `setenforce 0`；写入 authz `[rep:/]` 段；仓库名白名单校验 | `POST /api/rep/create` | `initStruct` 作**模板名**（多模板并存，空则按 `rep_init_default` 兜底，不套用即空库）；作者=实际操作人；写 authz `[rep:/]` 段；仓库名黑名单校验 | ⭐ 多模板 + 真实操作人 |
| GetRepList | `sync` 时物理↔authz 双向同步 + DB 同步 + 可选重算 size/rev；4 字段排序；size 已格式化 | `GET /api/rep/list` | 每次扫描磁盘（`format` 文件判定合法性）同步 DB；`SyncRep2Authz` 双向同步（物理有 authz 无→补段；authz 有物理无→清段）；SQL 分页 + 排序；size 为原始字节 | ⭐ 自动发现；⚠️ 列表不刷新已有记录的 size/rev（需 syncSize/syncRev） |
| GetSvnUserRepList | `sync` 时把用户可访问路径写入 `svn_user_pri_paths` 镜像表；分页 + `searchKeyword`；http 协议附 `raw_url` | `GET /api/rep/userList` | 实时计算（`EffectivePerm` 全量求值，含嵌套分组/别名/取反），返回仓库元信息 + 有效权限 + 路径清单 | ⭐ 无脏数据；🔻 无分页/搜索 |
| SyncRepSize | `du -s`×1024 写库，无返回 | `POST /api/rep/syncSize` | `filepath.Walk` 累加；校验存在 + 返回 `{size}` | ⭐ |
| SyncRepRev | `svnlook youngest` 写库，无返回 | `POST /api/rep/syncRev` | 校验 + 返回 `{rev}`；失败回落读 `db/current` | ⭐ |
| GetSvnUserRepList2 | 重建 Service 后复用同一逻辑，查指定用户 | `GET /api/rep/userList2?svnUserName=` | 直接实时计算指定用户 | ✅ |
| UpdRepNote | 直接 UPDATE，不存在静默 | `POST /api/rep/updNote` | 存在性兜底补插 + 日志 | ⭐ |
| GetUserRepCon | SVN 用户视角；`svn list` 走 svn:// 或 http:// **真实网络鉴权**；错误码精细映射（E170001/E220003/E170013）；返回含 `revLog` 与多级面包屑 | `GET /api/rep/userCon` | `UserCanAccess` 先行判定 + `svn list --xml` 走 **file://**；返回单级面包屑 | ⚠️ 鉴权前置而非依赖网络层；🔻 无 revLog、面包屑单级 |
| GetRepCon | 管理员视角；`svnlook tree` 直读磁盘；SVN 用户被路由白名单拦截 | `GET /api/rep/con` | `svn list --xml`（一次拿到提交元信息）；路由置于管理端分组 | ⚠️ 性能更好，权限口径一致 |
| GetRepTree | `svnlook tree --non-recursive` 逐层懒加载；目录优先 | `GET /api/rep/tree` | `svn list --xml -R` 一次递归全量后重建树（含文件/大小/版本/作者） | ⚠️ 信息更丰富；⚠️ 前端逐节点展开会重复拉整棵子树 |
| GetRepTree2 | 用户视角树；`first` 参数一次拼出"根→目标"单路径 | `GET /api/rep/tree2` | `UserCanAccess` + `SvnListAuth`；`first=false` 时逐层展开 | ⚠️ 仅管理端可达（SVN 用户改走 `/rep/userCon`） |
| CreateRepFolder | 临时目录 + `svn import`（作者/提交信息写死）；可能残留临时目录；不支持多级 | `POST /api/rep/createFolder` | `svn mkdir --parents -m <自定义信息> --with-revprop author=<操作人>`；无临时文件；日志 | ⭐ 支持多级 |
| GetRepPathAllPri | 只读当前段权限项，无继承展示；`objectPri='no'` 表示空权限；role=2 做路径归属 + 真实鉴权 + 二次授权白名单三重校验；错误码 751/752 | `GET /api/pri/list` | 返回 `{current[], inherit[]}`（新增继承权限可视化）；objectType 规范化、路径清洗；SVN 用户以 `EffectivePerm>0` 作为门票 | ⭐ 继承展示；🔻 对象列表未按白名单裁剪（见 7.1）；⚠️ 无 751/752 错误码 |
| CreateRepPathPri | 段不存在自动建段后重试；对象已存在报 801；**新增时 invert 硬编码 false**；正则最小替换以保排版 | `POST /api/pri/create` | 自动建段；同名 upsert；**新增即可设 `~` 反转**；authz 全量重写 | ⭐ 反转；⚠️ 语义由报错改覆盖 |
| UpdRepPathPri | 支持 invert 切换；对象不存在报 701 | `POST /api/pri/update` | 不存在时退化为新增；匹配键为 rep+path+objectName | ⚠️ |
| DelRepPathPri | 不存在时报"已删除" | `POST /api/pri/delete` | 不存在也返回成功；`~` 前缀双向清理 | ⚠️ |
| UpdRepName | `mv` + 正则改写 authz 段头 + UPDATE DB | `POST /api/rep/updName` | `os.Rename` + `RenameRepo`（段头与索引）+ UPDATE + 日志；双向存在性校验 | ✅ ⭐ |
| DelRep | authz→DB→`rm -rf`；不校验存在；无备份 | `POST /api/rep/del` | 校验存在→`RemoveAll`→`DropRepo`→删 DB；前端二次确认 | ✅ |
| GetRepDetail | `svnadmin info` 输出逐行 key:value | `GET /api/rep/detail` | 结构化字段：路径/版本/format/dbFormat/fsType/fsfs.conf/9 个钩子内容/备注 | ⭐ |
| SetUUID | 可传指定 UUID，也可随机 | `POST /api/rep/setUUID` | `uuid` 可选：传入则用指定值，否则随机；返回新值 | ✅ ⭐ |
| GetBackupList | 列 backup/ 全部**文件**；逐文件算 HMAC-MD5 `fileToken` 并拼 `fileUrl` | `GET /api/backup/list` | 按仓库前缀过滤 + 归属识别 + 时间倒序；dump 文件与 **hotcopy 目录**均入列（`type` 字段区分，目录体积递归累加） | ⭐ 覆盖 hotcopy；⚠️ 无下载 token |
| SvnadminDump | `svnadmin dump --quiet`，无版本范围、无增量；守护进程执行；文件名含 task_unique | `POST /api/backup/dump` | 支持 `revRange`（`-r`）与增量；进程内队列执行；完成后校验非空；邮件通知；返回完整任务对象 | ⭐ |
| DelRepBackup | `rm -f`，无路径穿越防护、无日志 | `POST /api/backup/del` | 拒绝含 `/`、`..` 的名字；目录用 `RemoveAll`；存在性报错 + 日志 | ⭐ |
| DownloadRepBackup | **免登录白名单路由** + 文件级 HMAC token 鉴权；流式输出 | `GET /api/backup/download` | 需登录（`X-Token` 头或 `?token=` URL 参数，后者供 `window.open` 场景）；`http.ServeContent` 原生 Range | ⚠️ 鉴权模型不同，能力对等 |
| GetUploadInfo | 读 php.ini `file_uploads`；`chunkSize=1`（MB） | `GET /api/backup/uploadInfo` | 固定 `{chunkSize: 8MB, allowExt, supportChunk: true}` | ⚠️ 分片更大 |
| UploadBackup | 前端 1MB 分片；片按 md5 目录暂存；**最后一片到达自动合并**；限制 `.dump` 后缀 | `/api/backup/upload{Init,Part,Merge,Abort,Status}` | 会话化三阶段：`uploadInit`→`uploadPart`→`uploadMerge`，另有中止与进度查询；缺片报错；MD5 强校验（可为空跳过）；12h 孤儿清理；同名去重 | ⭐ |
| SvnadminLoad | 要求目标仓库已存在；守护进程执行；导入后**不刷新版本数/体积**（原版 TODO） | `POST /api/backup/load` | 仓库不存在则自动建库（备注"由备份文件导入"）；队列执行；成功后刷新 size/rev/uuid | ⭐ |
| GetRepHooks | 9 个钩子；额外返回 `.tmpl` 官方模板内容；hooks 目录缺失报错 | `GET /api/hook/list` | 9 个钩子 + 中文触发说明 + `tmpl` 字段（优先读仓库实际 `.tmpl`，回落内置 9 个官方样例）；目录缺失返回空 | ⭐ |
| DelRepHook | 文件不存在提示"已移除"；无路径防护 | `POST /api/hook/del` | 防穿越 + 日志 | ⭐ |
| UpdRepHook | 可写检查 + 写入 + chmod | `POST /api/hook/update` | 自动 `MkdirAll` + 0755 + 防穿越 + 日志 | ✅ ⭐ |
| GetRecommendHooks | 扫描 `$home/hooks/` 子目录（`hookName`+`hookDescription`）；缺文件即报错并指出路径；安装包自带 2 个样例 | `GET /api/hook/recommend` | 同格式扫描；目录缺失返回空数组；内置 2 个推荐样例（pre-commit 提交日志校验 / post-commit 通知）；前端支持一键套用到仓库 | ⭐ 一键套用 |

### 4.7 后台任务（6）

| 原版接口 | 原版行为 | 重写版 | 重写版行为 | 判定 |
|---|---|---|---|---|
| GetTaskRun | 取首条运行任务 + 日志（>10MB 拒读）+ 排队数 | `GET /api/task/run` | `{running, task, log, logTrimed, queue[], queueCount}`；日志 2MB 截断并标记 | ⭐ 队列带完整对象 |
| GetTaskQueue | 查 status IN(1,2)，无分页 | `GET /api/task/list` | 统一列表：分页 + `status[]` 多值 + keyword | ⭐ |
| GetTaskHistory | 分页查历史 | 同上（status 过滤等价） | 同上 | ✅ 状态编码不同（原 1-5 / 新 0-4） |
| GetTaskHistoryLog | 全文读取，无大小限制 | `GET /api/task/log` | 2MB 保护；排队态给出提示 | ⭐ |
| DelTaskHistory | 取日志路径时字段名拼写错误（`task_lof_file`）→ 日志文件永不删除 | `POST /api/task/del` | 拒绝删除运行中任务；删日志 + 删记录；另有 `/api/task/clear` 批量清空 | ⭐ 修正原版缺陷 |
| UpdTaskStop | `ps aux \| grep task_unique` 抓 PID 后 kill -15/-9（字符串匹配可能误杀）；执行完一律置"完成"，不判退出码 | `POST /api/task/stop` | 进程组终止（`kill(-pgid)` SIGTERM→SIGKILL，级联清理 svnadmin 子进程）+ 排队任务 cancel channel；`task_message` 记录原因；完成/失败/取消三态由退出码区分 | ⭐ |

状态语义对照：原版 1 排队 / 2 执行中 / 3 执行结束待清理 / 4 中断 / 5 完成（**无失败态**）；重写 0 排队 / 1 执行中 / 2 完成 / 3 失败 / 4 取消。服务启动时 `ClearStaleTasks` 把遗留的 0/1 态任务置为取消并写明"服务重启，任务已中断"（对应原版守护进程的清理职责）。

### 4.8 定时任务（9）

| 原版接口 | 原版行为 | 重写版 | 重写版行为 | 判定 |
|---|---|---|---|---|
| GetRepList | 专用下拉，含固定项"-1 所有仓库" | `GET /api/rep/all` | 通用仓库下拉，前端自行加"全部" | ⚠️ 无专用接口 |
| GetCrontabList | 内存分页/搜索/排序 + **字段美化**（save_count 占位、status→bool、notice→数组、rep_name 解码后丢弃） | `GET /api/crond/list` | 带 `page/pageSize/keyword` 时服务端分页返回 `{total,list}`，不带则返回全量数组（兼容前端）；字段美化在前端做 | ⚠️ 职责划分不同 |
| CreateCrontab | 生成 shell 脚本 + 写 `crontab` 行（依赖系统 crond） | `POST /api/crond/create` | 进程内 `robfig/cron` 热注册；周期参数夹取（N≥2、week 0-6、day 1-31）；`save_count` 夹 [1,100]；返回完整对象 | ⭐ |
| UpdCrontab | 重写脚本 + 替换 crontab 行；不改状态 | `POST /api/crond/update` | 按 ID unregister→update→（可一并改 status）→热重载 | ⭐ 合并启停 |
| UpdCrontabStatus | 独立启停接口（增删 crontab 行） | 并入 `crond/update` | 传完整对象即改状态 | ⚠️ 接口合并，功能可达 |
| DelCrontab | 删 crontab 行 + unlink 脚本与日志 + 删 DB | `POST /api/crond/del` | unregister + 删日志 + 删 DB + 日志 | ✅ ⭐ |
| GetCrontabLog | 读 `<sign>.log`，返回 `log_path` | `GET /api/crond/log` | 2MB 保护 + 空态文案（不返回路径） | ⭐ |
| TriggerCrontab | `at -f now` 异步投递，无返回（依赖 atd） | `POST /api/crond/trigger` | 同步创建可追踪任务（返回 taskId，可在任务页看状态与日志）+ 前置校验 + 日志 | ⭐ |
| GetCronStatus | `ps` 检测 crond / atd 是否安装运行 | 并入 `GET /api/crond/dict` | 返回 `{scheduler, jobCount, nextRun, status:{running}}`；`running` 取自调度器对象本身，非外部服务健康检查 | ⚠️ 免外部依赖 |

任务类型与周期：9 种类型编号一致（1 dump 全量 / 2 dump deltas / 3、4 hotcopy 全量/增量 / 5 verify / 6 shell / 7 同步用户 / 8 同步分组 / 9 同步仓库），其中 **3/4 在原版为仅落库不执行**，重写版完整实现；8 种周期（minute/minute_n/hour/hour_n/day/day_n/week/month）与 notice 0-3 语义一致；`save_count` 在多仓库任务下按仓库逐个清理历史备份。

### 4.9 统计（3）

| 原版接口 | 原版行为 | 重写版 | 重写版行为 | 判定 |
|---|---|---|---|---|
| GetLoadInfo | 返回 load + cpu 三段（物理核数、仅 idle 计空闲） | `GET /api/common/loadInfo`（负载）+ `/api/common/monitor`（全量） | monitor 含 host/os/uptime/goVersion/cpu/mem/load；cpuCore 取逻辑核、空闲含 iowait | ⚠️ 拆分重组，口径略有差异 |
| GetDiskInfo | 解析 `/proc/mounts`，按文件系统去重；人类可读字符串 | `GET /api/common/diskInfo` | 按**挂载点**去重；返回数值（GB float） | ⚠️ 单位口径不同 |
| GetStatisticsInfo | **11 项**：os / repCount / repSize / backupCount / backupSize / logCount / adminCount / subadminCount / userCount / groupCount / aliaseCount | `GET /api/common/statistics` | **16 项**：上述 11 项全覆盖（`groupCount` 改按 authz 计算），新增 userEnabled / totalSize / totalSizeHr / svnVersion / svnserve | ⭐ |

### 4.10 日志（2）

| 原版接口 | 原版行为 | 重写版 | 重写版行为 | 判定 |
|---|---|---|---|---|
| GetLogList | 4 字段模糊搜索（类型、内容、操作人、时间字符串） | `GET /api/log/list` | `typeName` 精确过滤 + keyword（内容/操作人）+ `beginTime`/`endTime` 时间范围 + 分页；另有 `/api/log/types` 动态枚举 + 前端 Excel 导出 | ⭐ |
| DelLogs | 清空整表 | `POST /api/log/clear` | 同 | ✅ |

重写版额外记录：任务计划增删改、二次授权开关与白名单变更、权限变更、账号改名等事件。

### 4.11 子管理员（8）

| 原版接口 | 原版行为 | 重写版 | 重写版行为 | 判定 |
|---|---|---|---|---|
| GetSubadminList | 分页/搜索/排序；`online` 由 token 到期判定 | `GET /api/subadmin/list` | 分页 + keyword + `orderClause` 排序 + `online`（中间件记录活跃时间，10 分钟窗口）；返回 phone/email | ✅ ⭐ |
| CreateSubadmin | 密码 md5；**自动按默认树授权** | `POST /api/subadmin/create` | SHA-256；`functions` 为空时自动按默认树授权；phone/email | ⭐ |
| DelSubadmin | 不校验存在 | `POST /api/subadmin/del` | 存在性校验 + 清缓存 | ⭐ |
| UpdSubadminPass | md5 | `POST /api/subadmin/updPass` | SHA-256 + 校验 | ⭐ |
| UpdSubadminStatus | bool 校验 | `POST /api/subadmin/updStatus` | int，非法值归 0 | ✅ |
| UpdSubadminNote | — | `POST /api/subadmin/updNote` | 校验 + 日志 | ✅ ⭐ |
| GetSubadminTree | 默认树 **10 个一级节点、最深 7 层**、117 条函数引用；另返回 `treeOld` 只读快照并做结构变更提示 | `GET /api/subadmin/tree` | 默认树 **8 个一级节点、2 层**；签名比对给出 `needUpdateTree`；按 functions 回填勾选；`treeOld` 返回已保存的原树快照 | ⚠️ 树粒度更粗（见 7.2） |
| UpdSubadminTree | 存 tree + functions（JSON 数组）；不写日志 | `POST /api/subadmin/updTree` | 存 tree + functions（逗号串）+ 日志；`collectFunctions` 做父子联动（子节点勾选时父节点 `necessaryFunctions` 一并入白名单，对齐原版 `GetPriFunctions`） | ✅ ⭐ |

逐接口鉴权链路：中间件对 `RoleSubadmin` 用 `FuncKey(c.FullPath())` 把 `/api/rep/list` 归一成 `rep/list`，再查 `SubadminCanAccess(id, key)`（白名单缓存 30s，权限变更即失效）；未授权返回 403"当前账号未被授予该功能权限"。`login`/`userInfo` 返回 `functions`，前端按白名单过滤菜单。

### 4.12 二次授权（4）

| 原版接口 | 原版行为 | 重写版 | 重写版行为 | 判定 |
|---|---|---|---|---|
| UpdSecondpri | type 0/1 改 `svn_user_pri_paths.second_pri`；关闭时清白名单 | `POST /api/secondpri/update` | 按 `owner+rep+path` 唯一键查/建开关记录；关闭清白名单；路径归一化；日志 | ✅ ⭐ |
| GetSecondpriObjectList | 按 `pri_path_id` 查白名单，内存过滤 | `GET /api/secondpri/objectList` | 按 `owner+rep+path` + SQL LIKE | ✅ ⭐ |
| CreateSecondpriObject | 仅 `objectType==user` 时禁止把自己加入 | `POST /api/secondpri/objectCreate` | 任何类型都禁止 `objectName==owner`；查重 + 日志 | ⭐ |
| DelSecondpriObject | 无校验无日志 | `POST /api/secondpri/objectDel` | 存在性校验 + 日志 | ⭐ |
| — | — | `GET /api/secondpri/userPaths` | 用户已开启二次授权的路径清单（前端标注用） | ➕ |

自助闭环：`secondpri` 5 个接口与 `pri/create|update|delete` 均挂在"任意角色"分组。SVN 用户调用时 owner 被强制为当前登录用户（操作他人配置返回"只能操作自己的二次授权配置"）；`pri` 写接口经 `priDelegateCheck` 三层校验——**对该路径有写权限（EffectivePerm≥rw）→ 该路径已开启二次授权 → 目标对象在白名单内**。管理员与子管理员不受该链路限制。前端"我的仓库"页提供二次授权抽屉（开关、白名单增删、已开启路径标注）。

### 4.13 系统设置（27）

| 原版接口 | 原版行为 | 重写版 | 重写版行为 | 判定 |
|---|---|---|---|---|
| GetDcokerHostInfo | 读 `24_docker_host`（serialize），缺省插入 | `GET /api/setting/dockerHost` | JSON 读写 | ✅（值格式不兼容） |
| UpdDockerHostInfo | 校验 + 写回 | `POST /api/setting/dockerHost` | 校验 + 写回（兼容旧 key 名） | ✅ |
| GetSvnInfo | 11 键：enable/version/status/listen_*/svnserve_log/password_db/sasl/user_source/group_source/ldap | `GET /api/setting/svnInfo` | 聚合视图：协议/状态/端口/主机/版本/sasl/数据源/http 前缀与端口/dockerHost | ⚠️ 字段重组 |
| UpdSvnUsersource | 重量级编排：LDAP 字段校验 → 删 `svn_users`（+`svn_groups`）→ 改 `svnserve.conf` `[sasl] use-sasl` → 写 `/etc/sasl2/svn.conf` → 生成 `saslauthd.conf`（含 `ldap_password_attr`，内容变化才重启）→ 写 option → **重启 svnserve** | `POST /api/setting/usersource` | 同链路：LDAP 字段校验 → `clearUsersForSourceSwitch`（删 svn_users + 可选清 authz groups/svn_groups）→ `EnsureSvnserveConf(useSASL)` → `writeSASL2Conf` → `writeSaslauthdConf`（含 `ldap_password_attr: userPassword`）→ 写 option → 重启 svnserve | ✅ ⭐ |
| UpdSvnserveStatusStart | `svnserve --daemon --pid-file -r --config-file --listen-port --listen-host` | `POST /api/setting/svnserveStart` | `EnsureSvnserveConf`（use-sasl 由数据源推导）→ `StartSvnserveWith(生效端口, host)`：先删旧 pid、`CombinedOutput` 捕获失败、等待 pid 写入最多 3s | ⭐ 启动可观测 |
| UpdSvnserveStatusStop | pid 文件 + `/proc` 判定 → kill -15 → 复查失败报错 | `POST /api/setting/svnserveStop` | 读 pid → kill -15 → 轮询 `/proc` → kill -9 → `killSvnserveByScan` 进程表兜底；失败明确报错 | ✅ ⭐ |
| UpdSvnservePort | stop → 重写 `$home/svnserve` 环境文件 → start | `POST /api/setting/svnservePort` | 写 option + `SvnservePort()` 优先读 option 回落 yaml + 保存后自动重启；同值报"端口未发生变化" | ✅ ⭐ |
| UpdSvnserveHost | 同端口流程 | `POST /api/setting/svnserveHost` | 同链路（`SvnserveHost()` option 优先 + 自动重启） | ✅ ⭐ |
| GetDirInfo | 数组 7 项 | `GET /api/setting/dirInfo` | 对象 15 键（含 configFile/database/binPath/saslPath 等） | ⭐ |
| CheckUpdate | 请求私有更新服务器 `update.witersen.com`，由服务端给结论 | `GET /api/setting/checkUpdate` | GitHub Releases API + 本地 semver 比较 + 变更清单解析 + 下载链接推断 + 10s 超时 | ⭐（默认 `update.enable=false`） |
| GetMailInfo | `from` 为 `{address,name}`，`to` 为对象数组 | `GET /api/mail/info` | `from` 字符串、`to` 字符串数组；yaml 初始化兜底 | ⚠️ 丢发件人显示名 |
| SendMailTest | **用页面上未保存的参数**直接发测试邮件 | `POST /api/mail/test` | 接收完整 SMTP 覆盖参数（非零字段覆盖已保存配置，全空回落库中值；零值 bool 不误覆盖）；显式测试视为开启邮件服务 | ✅ ⭐ |
| UpdMailInfo | 白名单写 11 字段 | `POST /api/mail/update` | 整体覆盖 | ✅ |
| GetMailPushInfo | 默认 4 个触发点 | `GET /api/push/info` | 默认 6 个触发点（新增任务成功/失败）+ 与默认清单自动合并 | ⭐ |
| UpdPushInfo | serialize 写入 | `POST /api/push/update` | JSON 整体覆盖 | ✅ |
| GetSafeInfo | 默认 1 项（登录验证码） | `GET /api/safe/info` | 默认 3 项（+登录失败锁定 +密码强度）；三项均实际生效：`VerifyEnabled()`/`PassCheckEnabled()` 优先读 `safe_config` 回落 yaml；登录前 `loginLockLeft`；创建/改密走 `CheckUserPass`（≥4 位） | ⭐ |
| UpdSafeInfo | serialize 写入 | `POST /api/safe/update` | JSON 写入，运行时即时生效 | ⭐ |
| GetVerifyOption | 单条返回（免登录白名单路由） | `GET /api/common/verifyOption` | 聚合 `{enableVerifyCode, enableHTTP, svnVersion, svnserveStatus, version}`，`enableVerifyCode` 与 `safe_config` 同源 | ⭐ |
| LdapTest | connection/user/group 三型；**LDAP 分页**（每页 100）；user 型查 `groups_to_user_attribute_value`；返回 count/success/fail | `POST /api/ldap/test` | 三型；无分页（单次全量）；默认值兜底 + TLS1.2+；名单截断 50 条 | ⚠️ 互有强弱 |
| UpdSaslStatusStart | `saslauthd -a ldap -O uniqid -O conf`；sleep 后 `ps aux \| grep uniqid` 抓 PID 写入 pid 文件并校验 | `POST /api/setting/saslStart` | 写 conf → 启动 → `readSaslauthdPid`（候选路径 + `ps` 扫描兜底）轮询 → 写 pid 文件；conf 含 `ldap_password_attr` | ✅ ⭐ |
| UpdSaslStatusStop | pid + `/proc` → kill → 复查 | `POST /api/setting/saslStop` | 读 pid → kill → alive 复查，失败报错 | ✅ |
| GetApacheInfo | enable/version/modules/modules_path/port/prefix/password_db/user_source/group_source/ldap | `GET /api/setting/apacheInfo` | 自动探测 httpd（4 候选路径 + LookPath）+ 模块存在性 + confExists + warnings；并返回 userSource/groupSource/passwordDb/ldap（脱敏摘要，不含 bind 密码） | ⭐ |
| UpdSubversionEnable | 按数据源选 passwd/LDAP 双模板写 conf → 清 `svn_users` → 切 option → **停 svnserve** → `httpd -k graceful`（写失败用 sudo 强制写） | `POST /api/setting/subversionEnable` | `apacheTemplate(prefix, ds)` 在 ldap 源下生成 `AuthBasicProvider ldap` / `AuthLDAPURL` / `AuthLDAPBindDN` / `AuthLDAPBindPassword` / `Require ldap-user` 段；写 option → reload（失败时回退写 `$home/subversion.conf` 并提示手工复制） | ⭐ 双模板；🔻 不停 svnserve、不清用户（切换不阻断 svn 协议访问） |
| UpdSvnEnable | 重启 svnserve → 清用户 → 删 conf（校验）→ graceful | `POST /api/setting/svnEnable` | svnserve 未运行时自动启动（失败仅告警）→ passwd 源清空 svn_users → 删 conf（失败明确报错）→ 切 option → graceful（失败记日志） | ✅ |
| UpdHttpPrefix | svn 协议态拒绝；`/` 开头校验；重写 conf + graceful | `POST /api/setting/httpPrefix` | 校验 + 写 option；仅当 conf 已存在才重生成 | ⚠️ 更宽松 |
| UpdHttpPort | svn 协议态拒绝；整数校验 | `POST /api/setting/httpPort` | 1-65535 校验 + 写 `24_local_host` | ✅ |
| UpdHttpUsersource | 校验 → ldap 分支清表 → 写 option → `ReloadDatasource` → 按 `user_source` 选模板重写 conf → graceful | `POST /api/setting/usersource`（http 分支） | 校验 → `clearUsersForSourceSwitch` → 写 option → 按数据源选 passwd/LDAP 模板重写 conf → reload | ✅ ⭐ |

除上表外，重写版把邮件/推送/安全/LDAP 从 `Setting` 拆到独立路由前缀（`/api/mail/*`、`/api/push/*`、`/api/safe/*`、`/api/ldap/*`），并新增 `ldap/users`、`ldap/groups`、`ldap/sync` 三个显式同步入口。

---

## 五、前端页面

| 页面 | 原版（Vue2 + iView） | 重写版（Vue3 + Element Plus） | 判定 |
|---|---|---|---|
| 登录 | 角色下拉（默认 SVN 用户，必选） | 角色下拉含"自动识别"（默认）+ SVG 验证码 | ⭐ |
| 仪表盘 | 简单统计 + 负载 | 9 张统计卡 + 负载/CPU/内存仪表 + 磁盘条 + 后台任务/定时任务状态 + 3s 自动刷新 | ⭐ |
| 仓库管理 | 全弹窗交互 | 页面级多 Tab 抽屉：属性 / 目录树（懒加载 + 右键菜单）/ 权限（继承 + 反转）/ 备份（分片上传进度）/ 钩子（推荐一键套用） | ⭐ |
| 我的仓库（repositoryUser） | SVN 用户自助：浏览、授权、二次授权闭环 | `/myrepo`：仓库列表 + 目录浏览（面包屑）+ 检出地址复制 + 二次授权抽屉；菜单仅 svnuser 可见 | ✅ |
| 分组 | 单对象增删成员 | 三段式全量编辑 | ⚠️ |
| 子管理员 | 权限树（7 层）+ treeOld 快照 | 权限树（2 层）+ needUpdateTree 提示 + 接口明细 | ⚠️ 粒度简化 |
| 定时任务 | 硬编码字典 | `/api/crond/dict` 字典驱动 + 手动执行可追踪 + 执行日志 | ⭐ |
| 任务中心 | Header 弹窗（当前/排队/历史） | 独立页面 3 Tab（2s 轮询实时日志/中断/删除/清空） | ⭐ |
| 日志 | 列表 | 列表 + 类型动态枚举 + Excel 导出（SpreadsheetML） | ⭐ |
| 设置 | 7 Tab（2743 行） | 10 Tab（+LDAP、+关于更新、+配置项） | ⭐ |
| 个人中心 | 改名 / 改密 | 改名（校验密码 + 强制重登）/ 改密 / 账号信息 | ⭐ |
| 用户管理 | 列表 | 列表 + 邮箱编辑 + 用户迁入 3 步向导 + 二次授权抽屉 + 权限查看 | ⭐ |

---

## 六、部署与运维

| 维度 | 原版 | 重写版 |
|---|---|---|
| 安装 | PHP 站点 + composer/扩展 + systemd 守护进程 + crond/atd | 单二进制 + `-c config.yaml -web ./web/dist` |
| 容器 | 官方镜像内多进程 | Dockerfile 单进程，健康检查 `/api/health` |
| 升级检测 | 私有更新服务器 | GitHub Releases（semver + 变更清单） |
| 多实例 | token 与验证码均落库，可共享 | 黑名单落库可共享；验证码与任务队列为进程内存，**不可横向扩展** |
| 服务重启 | 守护进程清理残留任务，crond 继续调度 | 启动时清理残留任务（置取消并记录原因）；**进程退出即调度停摆** |
| 日志排障 | PHP 错误日志 + `task_error.log` | 标准输出 + 任务日志文件（2MB 截断保护）+ 系统日志表 |

---

## 七、当前差异清单

### 7.1 弱于原版（2 项）

| # | 差异 | 影响 | 说明 |
|---|---|---|---|
| 1 | **SVN 用户的授权对象可见范围未裁剪** | 中（信息暴露） | 原版对 role=2 调用 GetUserList / GetGroupList / GetGroupMember / GetAliaseList / GetRepPathAllPri 时，会用 `svn_second_pri` 白名单过滤返回对象；重写版这 4 个查询接口（`/user/all`、`/group/all`、`/aliase/list`、`/pri/list`）对 svnuser 只做"有读权限"门槛，不裁剪对象列表 → SVN 用户可枚举全部用户名/分组名/别名，以及某路径下全部授权项。`service.FilterBySecondPri` 已实现但当前无调用点 |
| 2 | **HTTP 协议下的 Apache 认证登录分支缺失** | 低 | 原版登录支持 Apache 认证（依赖 httpd 已认证的 REMOTE_USER）；重写版仅实现本地口令与 LDAP bind 两条通路。svn 协议与 LDAP 场景不受影响 |

### 7.2 实现差异（能力对等，不影响使用）

| # | 差异点 | 原版 | 重写版 |
|---|---|---|---|
| 1 | 权限树粒度 | 10 个一级节点、最深 7 层、117 条函数引用 | 8 个一级节点、2 层；接口级授权保留，但同一分组内接口只能整组授予 |
| 2 | 配置值格式 | PHP `serialize` | JSON（跨版本迁移需转换） |
| 3 | 登录凭证 | 自定义 `role\|name\|start\|end\|HMAC` | JWT HS256（有效期可配，默认 2h） |
| 4 | 验证码 | GD 生成 PNG + 落库 60s | SVG + 进程内存 180s |
| 5 | 任务状态编码 | 1-5（无失败态） | 0-4（含失败态） |
| 6 | 用户扫描语义 | 解析前端粘贴的 passwd 文本 | 读本机 passwd 与 DB 差集 |
| 7 | 分组成员编辑 | 单对象 add/delete | 三数组全量覆盖 |
| 8 | 仓库树 | `svnlook tree --non-recursive` 懒加载 | `svn list -R` 一次递归 |
| 9 | 仓库内容 | `svnlook tree` 直读磁盘 | `svn list --xml`（含提交元信息） |
| 10 | 用户浏览仓库 | `svn list` 走 svn:// 或 http:// 网络鉴权 | `svn list` 走 file://，鉴权由 `EffectivePerm` 前置完成 |
| 11 | 上传分片 | 1MB，最后一片自动合并 | 8MB 会话化（init/part/merge/abort/status），MD5 强校验 |
| 12 | 备份下载鉴权 | 免登录路由 + 文件级 HMAC token | 登录态（头或 URL 参数）+ Range |
| 13 | 定时任务字段美化 | 服务端（status→bool、notice→数组） | 前端 |
| 14 | 统计项数 | 11 | 16 |
| 15 | authz 写回 | 正则最小替换，保留手工排版 | 结构体全量重写（排版规范化） |
| 16 | 备份文件名 | 含 task_unique 后缀 | 时间戳 + 仓库名（同秒同名理论冲突极低） |

### 7.3 原版自身缺陷（重写版未复刻）

| # | 缺陷 |
|---|---|
| 1 | `DelTaskHistory` 取列名拼写为 `task_lof_file` → 日志文件永远不会被删除 |
| 2 | `UpdTaskStop` 用 `ps aux \| grep task_unique` 字符串匹配抓 PID，可能误杀同名字符串的进程；执行结束不判断退出码，一律置"完成" |
| 3 | 定时任务类型 3/4（hotcopy）仅落库，无执行逻辑 |
| 4 | 前端存在调用不存在接口的死链（`Setting/UpdPort`） |
| 5 | 依赖 `svn_user_pri_paths` 镜像表，需靠 sync 保持与 authz 一致，长期会出现漂移 |
| 6 | 管理员与 SVN 用户密码明文存储；改密不校验原密码 |
| 7 | `EditAdminUserName` 成功文案误写为"修改密码成功"；`UpdSvnserveStatusStop` 只发 SIGTERM 后等 1s，进程未退出即报失败，不再补 SIGKILL |

---

## 八、重写版独有能力

| 类别 | 能力 |
|---|---|
| 任务系统 | 失败态与 `task_message`；`/task/types`、`/task/detail`、`/task/clear`；进程组级联终止；排队任务协作式取消；任务成功/失败邮件触发器 |
| 定时任务 | `/crond/dict` 四组字典（含 needRep 标记）；hotcopy 类型 3/4 完整实现；手动执行返回可追踪 taskId；多仓库任务；周期边界夹取 |
| 监控 | `/common/monitor` 全量指标（host/os/uptime/goVersion/cpu/mem/load）；仪表盘自动刷新 |
| 备份 | 版本范围导出与增量 dump；上传会话化（uploadId / 进度 / 中止 / MD5 校验 / 孤儿清理 / 重名去重）；导入自动建库并刷新元数据 |
| 权限 | 新增权限即可设 `~` 反转；继承权限可视化（`inherit[]`）；`EffectivePerm` 独立求值器（嵌套分组防环、深度上限） |
| 安全 | SHA-256 + 恒时比较；改密/改名强制原密码；密码不下发；登录失败锁定；密码强度策略；token 黑名单持久化；LDAP 数据源下禁止本地写操作 |
| 体验 | 检出地址一键复制；目录树右键菜单；推荐钩子一键套用；用户迁入向导；改名后强制重登；SMTP PLAIN→LOGIN 回退；SASL/httpd 自动探测与 warnings |
| 工程 | 单二进制交付；零 PHP/pcntl/crond/atd/GD/htpasswd 依赖；Linux/macOS/Windows 构建；`/api/health`；190 条冒烟断言 |

---

## 九、总体判定

| 模块 | 判定 | 一句话结论 |
|---|---|---|
| 仓库 / 权限 / 备份 / 钩子 | ⭐ 更强 | 多模板、继承可视化、新增即反转、版本范围导出、上传会话化、hotcopy 入列、.tmpl 模板与推荐样例齐备；差异集中在 authz 全量重写与对象可见范围 |
| 用户 / 分组 / 别名 | ⭐ 更强 | 双协议同步、纯 Go htpasswd、循环嵌套检测、`#disabled#` 兼容、在线态、邮箱字段、LDAP 写拦截；弱项为 SVN 用户对象裁剪 |
| 后台任务 | ⭐ 更强 | 修正原版 2 个缺陷；进程组终止、失败态、批量清空、重启残留清理 |
| 定时任务 | ⭐ 更强 | 摆脱 crond/atd；补全原版空壳 hotcopy；多仓库保留策略；手动执行可追踪 |
| 监控 / 统计 | ⭐ 更强 | monitor 维度更多；statistics 11 → 16 项 |
| 日志 | ⭐ 更强 | 类型枚举、时间范围、Excel 导出、事件覆盖更广 |
| 子管理员 | ✅ 略强 | 逐接口鉴权 + 动态菜单 + 分页/排序/在线态/treeOld；树粒度 2 层（原版 7 层） |
| 二次授权 | ⭐ 更强 | 表结构更完备、接口更全、自助闭环带三层校验；对象可见范围未裁剪 |
| 个人中心 | ⭐ 更强 | 哈希、原密码校验、日志、通知邮件 |
| 登录 | ✅ 略强 | LDAP 登录、失败锁定、登录邮件、动态菜单、角色自动识别；缺 Apache 认证分支 |
| 设置 / 集成 | ⭐ 更强 | svn/http × passwd/ldap 四路编排、svnserve 启停/端口/主机、SASL pid 闭环、Apache LDAP 模板、邮件先测后存 |

**结论**：原版 116 个接口全部有对应实现，无功能缺失；其中绝大多数为对等或更强，明确的弱项只有 2 处（SVN 用户授权对象可见范围、Apache 认证登录），其余差异属机制选择或原版缺陷修正。运行基线：`make build`、`go vet ./...`、`vue-tsc --noEmit`、`npm run build` 全部通过；四套冒烟脚本 190 条断言 0 失败。

---

*本文档为源码级现状快照（2026-10-30）：原版取自 `witersen/SvnAdminV2.0` commit `1c4305a`，重写版取自本仓库当前工作树；所有行为描述均对应该时点代码。*
