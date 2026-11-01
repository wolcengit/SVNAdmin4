// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package service

import (
	"errors"
	"os"
	"path/filepath"

	"svnadmin/internal/config"
)

// EnsureTemplates 生成默认的仓库初始化结构模板（trunk / branches / tags）
func EnsureTemplates() error {
	dir := filepath.Join(config.C().SVN.TempleteInitPath(), "01")
	if err := os.MkdirAll(filepath.Join(dir, "trunk"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "branches"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "tags"), 0755); err != nil {
		return err
	}
	// 内置推荐钩子样例（对齐原版安装包自带的 hooks/01、hooks/02），
	// 目录为空时自动生成，保证"推荐钩子"功能开箱可用
	return EnsureRecommendHooks()
}

// EnsureRecommendHooks 在推荐钩子目录为空时写入两个内置样例
func EnsureRecommendHooks() error {
	base := config.C().SVN.RecommendHookPath()
	if entries, err := os.ReadDir(base); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				return nil // 已有自定义/内置内容，不覆盖
			}
		}
	}
	samples := []struct {
		dir, hook, name, desc, content string
	}{
		{
			dir: "01", hook: "pre-commit",
			name: "pre-commit",
			desc: "提交前校验日志长度不少于 10 个字符，不满足则拒绝提交",
			content: `#!/bin/sh
# 推荐钩子：提交前校验日志长度
REPOS="$1"
TXN="$2"
SVNLOOK=/usr/bin/svnlook
LOGMSG=$($SVNLOOK log -t "$TXN" "$REPOS" | grep "[a-zA-Z0-9]" | wc -m)
if [ "$LOGMSG" -lt 10 ]; then
  echo "提交日志至少需要 10 个字符" 1>&2
  exit 1
fi
exit 0
`,
		},
		{
			dir: "02", hook: "post-commit",
			name: "post-commit",
			desc: "提交后输出提交信息与作者，可作为自动部署的接入点",
			content: `#!/bin/sh
# 推荐钩子：提交后通知（可改为自动部署脚本）
REPOS="$1"
REV="$2"
SVNLOOK=/usr/bin/svnlook
AUTHOR=$($SVNLOOK author -r "$REV" "$REPOS")
CHANGED=$($SVNLOOK changed -r "$REV" "$REPOS")
echo "仓库：${REPOS}"
echo "版本：${REV} 作者：${AUTHOR}"
echo "变更："
echo "${CHANGED}"
exit 0
`,
		},
	}
	for _, s := range samples {
		dir := filepath.Join(base, s.dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		_ = writeFile(filepath.Join(dir, "hookName"), s.name, 0644)
		_ = writeFile(filepath.Join(dir, "hookDescription"), s.desc, 0644)
		_ = writeFile(filepath.Join(dir, s.hook), s.content, 0755)
	}
	return nil
}

// statFile 读取文件信息
func statFile(p string) (os.FileInfo, error) { return os.Stat(p) }

// writeFile 写入文件（自动创建父目录）
func writeFile(path, content string, perm os.FileMode) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(content), perm)
}

// writeFileIfAbsent 文件不存在时写入
func writeFileIfAbsent(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeFile(path, content, 0644)
}
