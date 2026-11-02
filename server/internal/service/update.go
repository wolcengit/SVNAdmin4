// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package service

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"golang.org/x/mod/semver"
	"svnadmin/internal/config"
	"svnadmin/internal/version"
)

// UpdateInfo 版本更新信息
type UpdateInfo struct {
	Checked     bool     `json:"checked"`
	Current     string   `json:"current"`
	Latest      string   `json:"latest"`
	HasUpdate   bool     `json:"hasUpdate"`
	PublishedAt string   `json:"publishedAt"`
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Fixed       []string `json:"fixed"`
	Added       []string `json:"added"`
	Removed     []string `json:"removed"`
	Download    string   `json:"download"`
	Message     string   `json:"message"`
}

// githubRelease GitHub Release 的最小结构
type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// CheckUpdate 检测版本更新
func CheckUpdate() (*UpdateInfo, error) {
	info := &UpdateInfo{Checked: true, Current: version.Version}
	servers := configUpdateServers()
	if len(servers) == 0 {
		info.Message = "未配置更新检测服务器"
		return info, nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error
	for _, srv := range servers {
		req, err := http.NewRequest(http.MethodGet, srv, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "SVNAdmin4/"+version.Version)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		var rel githubRelease
		err = json.NewDecoder(resp.Body).Decode(&rel)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		fillUpdateInfo(info, &rel)
		return info, nil
	}
	if lastErr != nil {
		info.Message = "更新检测失败：" + lastErr.Error()
	} else {
		info.Message = "未能获取版本信息"
	}
	return info, nil
}

func configUpdateServers() []string {
	c := config.C().Update
	if !c.Enable {
		return nil
	}
	out := []string{}
	for _, s := range c.Server {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func fillUpdateInfo(info *UpdateInfo, rel *githubRelease) {
	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	info.Latest = latest
	info.Title = rel.Name
	info.Body = rel.Body
	info.PublishedAt = rel.PublishedAt
	info.URL = rel.HTMLURL
	info.HasUpdate = versionGreater(latest, version.Version)
	// 从 Release 说明中解析 "修复 / 新增 / 移除" 三个区块
	info.Fixed = parseSection(rel.Body, "修复", "fix", "bug")
	info.Added = parseSection(rel.Body, "新增", "add", "feat")
	info.Removed = parseSection(rel.Body, "移除", "remove")
	// 资源包
	var full, patch string
	for _, a := range rel.Assets {
		lower := strings.ToLower(a.Name)
		switch {
		case strings.Contains(lower, "patch") || strings.Contains(lower, "update"):
			if patch == "" {
				patch = a.BrowserDownloadURL
			}
		case strings.Contains(lower, ".tar.gz") || strings.Contains(lower, ".zip") ||
			strings.Contains(lower, "linux") || strings.Contains(lower, "full"):
			if full == "" {
				full = a.BrowserDownloadURL
			}
		}
	}
	if full == "" && len(rel.Assets) > 0 {
		full = rel.Assets[0].BrowserDownloadURL
	}
	info.Download = full
	if info.Download == "" {
		info.Download = patch
	}
	if !info.HasUpdate {
		info.Message = "当前已是最新版本"
	}
}

// parseSection 从 Markdown 说明中提取指定小节下的条目
func parseSection(body string, keys ...string) []string {
	var out []string
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	in := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(trimmed, "#") {
			in = false
			for _, k := range keys {
				if strings.Contains(lower, strings.ToLower(k)) {
					in = true
					break
				}
			}
			continue
		}
		if !in {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "* ")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	sort.Strings(out)
	return out
}

// versionGreater 比较版本号，v1 比 v2 新时返回 true
func versionGreater(v1, v2 string) bool {
	n1, n2 := normalizeVersion(v1), normalizeVersion(v2)
	if !semver.IsValid(n1) || !semver.IsValid(n2) {
		return v1 != "" && v1 != v2
	}
	return semver.Compare(n1, n2) > 0
}

func normalizeVersion(v string) string {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(v, ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	for i := range parts {
		parts[i] = digitsOnly(parts[i])
		if parts[i] == "" {
			parts[i] = "0"
		}
	}
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return "v" + strings.Join(parts, ".")
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			break
		}
	}
	return b.String()
}

// VersionResp 版本信息（含运行环境），供 /api/common/version 与 /api/setting/version 透出。
// 不含上游参考项目字段；上游致谢见 README / NOTICE。
type VersionResp struct {
	version.Info
	SvnVersion string `json:"svnVersion"`
}

// VersionInfo 返回结构化版本信息。
func VersionInfo() VersionResp {
	vi := VersionResp{Info: version.Get()}
	vi.SvnVersion = VersionText()
	return vi
}
