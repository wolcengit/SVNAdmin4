// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package service

import (
	"os/exec"
)

// buildShellCmd 构造执行 shell 脚本的命令
// 优先使用 bash，不存在时回退到 sh
func buildShellCmd(script string) (*exec.Cmd, error) {
	if sh, err := exec.LookPath("bash"); err == nil {
		return exec.Command(sh, "-c", script), nil
	}
	if sh, err := exec.LookPath("sh"); err == nil {
		return exec.Command(sh, "-c", script), nil
	}
	return nil, exec.ErrNotFound
}

// runShell 执行 shell 脚本并返回输出
func runShell(script string) (string, error) {
	cmd, err := buildShellCmd(script)
	if err != nil {
		return "", err
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
