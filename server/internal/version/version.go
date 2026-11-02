// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

// Package version 是 SVNAdmin4 产品版本信息的唯一权威源（SSOT）。
//
// 编译期通过 -ldflags "-X 'svnadmin/internal/version.Version=...'" 注入版本号、
// Git 提交与构建时间；若未注入，则自动回退到 Go 的 debug.ReadBuildInfo()
// （vcs.revision / vcs.time），最终兜底为 "dev"。上游致谢（witersen/SvnAdminV2.0）
// 只出现在 README / NOTICE，不进入运行时版本接口。
package version

import (
	"runtime"
	"runtime/debug"
)

// 以下变量由编译命令注入，默认值仅用于本地开发 / 未注入场景。
var (
	// Version 产品版本号（语义化版本），release 构建时被 -ldflags 覆盖。
	Version = "4.1.0"
	// GitCommit 构建时所在 Git 提交短哈希，为空时从 build info 取。
	GitCommit = ""
	// BuildTime 构建时间（RFC3339 UTC），为空时从 build info 取。
	BuildTime = ""
)

const (
	// Name 产品名
	Name = "SVNAdmin4"
	// License 开源许可证
	License = "GPL-3.0-or-later"
	// LicenseURL 许可证全文地址
	LicenseURL = "https://www.gnu.org/licenses/gpl-3.0.html"
	// Homepage 项目主页
	Homepage = "https://github.com/wolcengit/SVNAdmin4"
	// Repository 源码仓库地址
	Repository = "https://github.com/wolcengit/SVNAdmin4"
)

// Info 结构化版本信息（不含运行环境相关的 SVN 版本，由 service 层补充）。
type Info struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	GitCommit  string `json:"gitCommit"`
	BuildTime  string `json:"buildTime"`
	GoVersion  string `json:"goVersion"`
	Platform   string `json:"platform"`
	License    string `json:"license"`
	LicenseURL string `json:"licenseUrl"`
	Homepage   string `json:"homepage"`
	Repository string `json:"repository"`
}

// Get 返回填充完毕的版本信息，按 ldflags → ReadBuildInfo → 默认值 三级降级。
func Get() Info {
	commit := GitCommit
	built := BuildTime
	if commit == "" || built == "" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			for _, s := range bi.Settings {
				switch s.Key {
				case "vcs.revision":
					if commit == "" {
						commit = s.Value
					}
				case "vcs.time":
					if built == "" {
						built = s.Value
					}
				}
			}
		}
	}
	if commit == "" {
		commit = "dev"
	}
	if built == "" {
		built = "dev"
	}
	return Info{
		Name:       Name,
		Version:    Version,
		GitCommit:  commit,
		BuildTime:  built,
		GoVersion:  runtime.Version(),
		Platform:   runtime.GOOS + "/" + runtime.GOARCH,
		License:    License,
		LicenseURL: LicenseURL,
		Homepage:   Homepage,
		Repository: Repository,
	}
}

// Short 返回单行版本描述，用于 -v 与启动日志。
// 形如：SVNAdmin4 4.1.0 (abc1234, 2026-11-02, go1.22.x, GPL-3.0-or-later)
func Short() string {
	info := Get()
	return Name + " " + info.Version + " (" + ShortCommit() + ", " + info.BuildTime + ", " + info.GoVersion + ", " + License + ")"
}

// ShortCommit 返回 7 位短哈希，用于日志与关于页展示。
func ShortCommit() string {
	c := Get().GitCommit
	if len(c) > 7 {
		return c[:7]
	}
	return c
}
