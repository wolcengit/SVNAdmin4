## 改动目的

<!-- 一句话说明本次 PR 解决什么问题 -->

## 改动范围

<!-- 涉及模块 / 文件 -->

## 测试方式

- [ ] `make fmt` / `make vet` 通过
- [ ] `cd web && npm run type-check` 通过
- [ ] 已在 `scripts/smoke*.sh` 补充或更新断言
- [ ] 本地 `make smoke` 通过

## 提交规范确认

- [ ] 单一目的提交，未混杂无关改动；附带修改（incidental）已单独成提交
- [ ] 新增源文件已加 GPL-3.0 许可头（`make license-check` 通过）
- [ ] 行为变更已同步 `README.md` 与 `CHANGELOG.md`

## 许可声明

- [ ] 我确认本 PR 的贡献以 **GPL-3.0-or-later** 授权提交，并同意并入本项目仓库
- [ ] 新增第三方依赖均为 GPL-3.0 兼容许可，且已更新 `docs/THIRD-PARTY.md`
