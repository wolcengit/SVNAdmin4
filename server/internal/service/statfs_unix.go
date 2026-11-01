// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

//go:build !windows

package service

import "syscall"

type fsStat struct {
	total uint64
	avail uint64
}

// statFS 读取文件系统容量
func statFS(path string) (fsStat, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return fsStat{}, false
	}
	bsize := uint64(st.Bsize)
	return fsStat{
		total: st.Blocks * bsize,
		avail: st.Bavail * bsize,
	}, true
}
