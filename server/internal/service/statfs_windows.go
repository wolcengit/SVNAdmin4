// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

//go:build windows

package service

type fsStat struct {
	total uint64
	avail uint64
}

// statFS Windows 平台暂不支持磁盘容量读取
func statFS(path string) (fsStat, bool) { return fsStat{}, false }
