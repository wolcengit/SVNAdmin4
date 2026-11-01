// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

//go:build !(linux || darwin || freebsd)

package service

import (
	"os"
	"os/exec"
)

// setProcGroup 非 Unix 平台无法创建进程组，降级为默认行为
func setProcGroup(cmd *exec.Cmd) {}

// killProcGroup 非 Unix 平台只能终止主进程
func killProcGroup(pid int) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = p.Kill()
}
