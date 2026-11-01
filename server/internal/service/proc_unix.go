// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

//go:build linux || darwin || freebsd

package service

import (
	"os/exec"
	"syscall"
)

// setProcGroup 让子进程独立成组，便于整组终止（杀掉 svnadmin 及其派生进程）
func setProcGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = new(syscall.SysProcAttr)
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcGroup 终止整个进程组：先 SIGTERM 优雅退出，再 SIGKILL 强杀
func killProcGroup(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
