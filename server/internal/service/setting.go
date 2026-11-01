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
	"fmt"
	"strconv"
	"strings"

	"svnadmin/internal/config"
	"svnadmin/internal/model"
	"svnadmin/internal/store"
	"svnadmin/internal/svn"
)

// GetOption 读取系统配置，不存在时返回默认值
func GetOption(name, def string) string {
	var o model.Option
	if err := store.DB().Where("option_name = ?", name).First(&o).Error; err != nil {
		return def
	}
	return o.OptionValue
}

// SetOption 写入系统配置
func SetOption(name, value, desc string) error {
	var o model.Option
	if err := store.DB().Where("option_name = ?", name).First(&o).Error; err != nil {
		return store.DB().Create(&model.Option{OptionName: name, OptionValue: value, OptionDescription: desc}).Error
	}
	upd := map[string]interface{}{"option_value": value}
	if desc != "" {
		upd["option_description"] = desc
	}
	return store.DB().Model(&o).Updates(upd).Error
}

// OptionList 返回全部配置项
func OptionList() ([]model.Option, error) {
	var list []model.Option
	err := store.DB().Order("option_id").Find(&list).Error
	return list, err
}

// configHTTPEnabled 是否启用 HTTP 协议支持
func configHTTPEnabled() bool { return config.C().SVN.EnableHTTP }

// SetOptionBool 写入布尔型配置
func SetOptionBool(name string, v bool) error {
	val := "0"
	if v {
		val = "1"
	}
	return SetOption(name, val, "")
}

// GetOptionBool 读取布尔型配置
func GetOptionBool(name string, def bool) bool {
	v := GetOption(name, "")
	if v == "1" || strings.EqualFold(v, "true") {
		return true
	}
	if v == "0" || strings.EqualFold(v, "false") {
		return false
	}
	return def
}

// VersionText 返回 Subversion 版本描述
func VersionText() string { return svn.Version() }

// VerifyOption 登录页需要读取的配置
func VerifyOption() map[string]interface{} {
	return map[string]interface{}{
		"enableVerifyCode": VerifyEnabled(),
		"enableHTTP":       configHTTPEnabled(),
		"svnVersion":       svn.Version(),
		"svnserveStatus":   svn.SvnserveStatus(),
		"version":          GetOption("system_version", "4.0.0"),
	}
}

// ---------------------------------------------------------------------------
// 服务启停
// ---------------------------------------------------------------------------

// SvnservePort 生效的 svnserve 端口：优先取数据库中保存的配置，回落 yaml
func SvnservePort() int {
	raw := strings.TrimSpace(GetOption(OptionSvnservePort, ""))
	if raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 65535 {
			return n
		}
	}
	return config.C().SVN.SvnservePort
}

// SvnserveHost 生效的 svnserve 监听地址：优先取数据库中保存的配置，回落 yaml
func SvnserveHost() string {
	raw := strings.TrimSpace(GetOption(OptionSvnserveHost, ""))
	if raw != "" {
		return raw
	}
	return config.C().SVN.SvnserveHost
}

// SvnserveStart 启动 svnserve
func SvnserveStart(operator string) error {
	if SvnserveStatus() {
		return errors.New("svnserve 已在运行")
	}
	if err := EnsureSvnserveConf(useSASL()); err != nil {
		return err
	}
	if err := svn.StartSvnserveWith(SvnservePort(), SvnserveHost()); err != nil {
		return err
	}
	_ = SetOptionBool("svnserve_status", true)
	AddLog("服务管理", fmt.Sprintf("启动 svnserve 服务（%s:%d）", SvnserveHost(), SvnservePort()), operator)
	return nil
}

// useSASL 当前 svn 数据源是否要求启用 SASL
func useSASL() bool {
	return GetDataSource(OptionSvnDatasource).UserSource == SourceLDAP
}

// SvnserveStop 停止 svnserve
func SvnserveStop(operator string) error {
	if err := svn.StopSvnserve(); err != nil {
		return err
	}
	_ = SetOptionBool("svnserve_status", false)
	AddLog("服务管理", "停止 svnserve 服务", operator)
	return nil
}

// SvnserveStatus 返回 svnserve 运行状态
func SvnserveStatus() bool { return svn.SvnserveStatus() }

// EnsureSvnserveConf 写入统一管理的 svnserve.conf，useSASL 时开启 [sasl] use-sasl
func EnsureSvnserveConf(useSASL bool) error {
	cfg := config.C().SVN
	sasl := "false"
	if useSASL {
		sasl = "true"
	}
	content := fmt.Sprintf(`[general]
anon-access = none
auth-access = write
password-db = %s
authz-db = %s
realm = %s

[sasl]
use-sasl = %s
`, cfg.PasswdFile(), cfg.AuthzFile(), cfg.RepBasePath(), sasl)
	return writeFile(cfg.SvnConfFile(), content, 0644)
}

// EnsureSvnserveConfDefault 兼容旧调用：按当前数据源推导 SASL 开关
func EnsureSvnserveConfDefault() error { return EnsureSvnserveConf(useSASL()) }

// EnsureAuthzFile 确保 authz / passwd 文件存在
func EnsureAuthzFile() error {
	cfg := config.C().SVN
	if err := writeFileIfAbsent(cfg.AuthzFile(), "[aliases]\n\n[groups]\n"); err != nil {
		return err
	}
	return writeFileIfAbsent(cfg.PasswdFile(), "[users]\n")
}

// ---------------------------------------------------------------------------
// 检出地址
// ---------------------------------------------------------------------------

// Checkout 返回仓库的检出地址
func Checkout(repName, host string) map[string]string {
	cfg := config.C().SVN
	svnHost := cfg.SvnserveHost
	if svnHost == "" {
		svnHost = host
	}
	httpHost := cfg.HTTPHost
	if httpHost == "" {
		httpHost = host
	}
	prefix := cfg.HTTPPrefix
	if prefix != "" && !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	res := map[string]string{
		"svn":  fmt.Sprintf("svn://%s:%d/%s", svnHost, cfg.SvnservePort, repName),
		"http": "",
	}
	if configHTTPEnabled() {
		res["http"] = fmt.Sprintf("http://%s%s/%s", httpHost, prefix, repName)
	}
	return res
}
