# 贡献指南

感谢你愿意为 SVNAdmin4 提交代码。本项目以 **GPL-3.0-or-later** 授权发布，
**你提交的所有贡献都将被视为在同一许可证下提供**，这一点在提交 PR 时即视为你已同意。

## 1. 许可与版权

- 项目许可：**GNU General Public License v3.0 or later**（见根目录 `LICENSE`）。
- 版权声明：`Copyright (C) 2026 wolcen@msn.com`。
- 新增源文件请在文件头部加入与现有文件一致的许可头：

  Go：

  ```go
  // Copyright (C) 2026 wolcen@msn.com
  // SPDX-License-Identifier: GPL-3.0-or-later
  //
  // SVNAdmin4 is free software: you can redistribute it and/or modify it under
  // the terms of the GNU General Public License as published by the Free Software
  // Foundation, either version 3 of the License, or (at your option) any later
  // version. See the LICENSE file in the project root for the full license text.
  ```

  TypeScript / JavaScript 使用 `/*! ... */` 块，Vue SFC 使用 `<!-- ... -->` 块。

- 引入第三方依赖时，必须确认其许可证与 GPL-3.0 兼容（MIT / BSD / Apache-2.0 /
  ISC / MPL-2.0 可以；GPL-2.0-only、AGPL-only 与专有许可不可以），
  并同步更新 `docs/THIRD-PARTY.md` 与 `NOTICE`。

## 2. 开发环境

```bash
# 后端
cd server && go run . -c ./configs/config.yaml -web ""

# 前端（vite 已代理 /api 到 8080）
cd web && npm install && npm run dev

# 全量构建
make build
```

要求：Go 1.22+，Node.js 18+，Subversion 1.8+（`svn` / `svnadmin` / `svnlook` /
`svnserve` 在 PATH 中）。

## 3. 提交前检查

```bash
make fmt          # gofmt
make vet          # go vet
cd web && npm run type-check
make smoke        # 接口冒烟（需服务已在 8080 运行）
```

新增或修改接口时，请在 `scripts/smoke*.sh` 中补充对应断言，保持四套冒烟脚本
（smoke / smoke2 / smoke3 / smoke4）的覆盖不被削弱。

## 4. 提交约定（本项目强制）

1. **增量式、单一目的提交**：一个逻辑改动一个提交，不要把重构、格式化、修 bug
   混在同一个提交里。
2. **附带修改（incidental）必须分离**：与本次目的无关的文件变更（如 IDE 生成物、
   顺手改的注释）单独成提交，并在描述里标注 `incidental:` 前缀，便于单独审查。
3. 提交信息格式：`类型(模块): 简述`，类型取 `feat` / `fix` / `refactor` /
   `docs` / `test` / `chore`。示例：

   ```
   feat(svn/authz): 支持权限反转 ~name 的解析与写回
   fix(api/rep): 目录树懒加载在仓库名含空格时 500
   ```

4. 行为变更必须同步更新 `README.md` 与 `CHANGELOG.md`。

## 5. 架构约定（改动前请先阅读）

- **authz 文件是权限的唯一数据源**，不要新增把权限镜像进数据库表的逻辑。
- SVN 用户密码在 `svn_users` 表明文留存（禁用/启用需在 `passwd` 中增删行，
  启用时从库恢复）；管理员与子管理员密码存 SHA-256。
- 统一响应体 `{ code, status, message, data }`，分页 `data = { total, list }`，
  鉴权头 `X-Token`。
- 路由风格为 RESTful `/api/模块/动作`，不再沿用原版 `api.php?c=Xxx&a=Yyy`。
- 仓库数据目录由 `svn.home_path` 推导，新增路径需同步扩展 `config.SVNConfig`
  的路径方法，不要散落字符串拼接。

## 6. Pull Request

- 从 `main` 拉分支，命名 `feat/xxx`、`fix/xxx`、`refactor/xxx`。
- PR 描述中说明：动机、改动范围、测试方式、是否涉及破坏性变更。
- 涉及安全问题的修复，请先按 `SECURITY.md` 私下报告，不要在公开 issue 中披露。

## 7. 行为准则

请保持就事论事的技术讨论。人身攻击、歧视性言论与无关营销一律不受欢迎，
维护者有权关闭相关 issue/PR。
