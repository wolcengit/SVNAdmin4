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
	"strings"

	"svnadmin/internal/config"
)

// 系统配置项键名（保持与原版同名，便于迁移）
const (
	OptionDockerHost     = "24_docker_host"
	OptionLocalHost      = "24_local_host"
	OptionEnableCheckout = "24_enable_checkout"
	OptionSvnDatasource  = "24_svn_datasource"
	OptionHttpDatasource = "24_http_datasource"
	OptionHttpPrefix     = "24_http_prefix"
	OptionMailSMTP       = "mail_smtp"
	OptionMessagePush    = "message_push"
	OptionSafeConfig     = "safe_config"
	OptionSystemVersion  = "system_version"
	OptionSvnservePort   = "svnserve_port"
	OptionSvnserveHost   = "svnserve_host"
	OptionHttpPort       = "http_port"
	OptionSvnserveStatus = "svnserve_status"
	OptionSaslStatus     = "sasl_status"
)

// 用户/分组来源
const (
	SourcePasswd     = "passwd"
	SourceHTTPPasswd = "httpPasswd"
	SourceLDAP       = "ldap"
	GroupSourceAuthz = "authz"
	GroupSourceLDAP  = "ldap"
)

// LDAPSettings LDAP 连接与查询参数
type LDAPSettings struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Version      int    `json:"version"`
	BindDN       string `json:"bindDn"`
	BindPassword string `json:"bindPassword"`
	Enabled      bool   `json:"enabled"`

	UserBaseDN       string `json:"userBaseDn"`
	UserSearchFilter string `json:"userSearchFilter"`
	UserAttributes   string `json:"userAttributes"`

	GroupBaseDN       string `json:"groupBaseDn"`
	GroupSearchFilter string `json:"groupSearchFilter"`
	GroupAttributes   string `json:"groupAttributes"`
	GroupsToUserAttr  string `json:"groupsToUserAttribute"`
	GroupsToUserValue string `json:"groupsToUserAttributeValue"`
}

// DataSource 用户/分组来源配置
type DataSource struct {
	UserSource  string       `json:"userSource"`
	GroupSource string       `json:"groupSource"`
	LDAP        LDAPSettings `json:"ldap"`
}

// SafeConfig 安全配置项
type SafeConfig struct {
	Name   string `json:"name"`
	Note   string `json:"note"`
	Enable bool   `json:"enable"`
}

// DefaultSafeConfig 默认安全配置
func DefaultSafeConfig() []SafeConfig {
	return []SafeConfig{
		{Name: "login_verify_code", Note: "登录验证码", Enable: config.C().Security.EnableVerifyCode},
		{Name: "login_fail_lock", Note: "登录失败锁定（连续失败达阈值后临时锁定账号）", Enable: config.C().Security.LoginFailLimit > 0},
		{Name: "svn_user_pass_check", Note: "SVN 用户密码强度校验（至少 4 位且非纯空格）", Enable: true},
	}
}

// GetSafeConfig 读取安全配置
func GetSafeConfig() []SafeConfig {
	def := DefaultSafeConfig()
	raw := GetOption(OptionSafeConfig, "")
	if raw == "" {
		return def
	}
	var saved []SafeConfig
	if err := json.Unmarshal([]byte(raw), &saved); err != nil || len(saved) == 0 {
		return def
	}
	idx := map[string]SafeConfig{}
	for _, s := range saved {
		idx[s.Name] = s
	}
	for i := range def {
		if s, ok := idx[def[i].Name]; ok {
			def[i].Enable = s.Enable
		}
	}
	return def
}

// SaveSafeConfig 保存安全配置
func SaveSafeConfig(list []SafeConfig) error {
	data, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return SetOption(OptionSafeConfig, string(data), "安全配置")
}

// GetDataSource 读取指定协议的数据源配置
func GetDataSource(key string) DataSource {
	def := DataSource{UserSource: SourcePasswd, GroupSource: GroupSourceAuthz}
	if key == OptionHttpDatasource {
		def.UserSource = SourceHTTPPasswd
	}
	raw := GetOption(key, "")
	if raw == "" {
		return def
	}
	var d DataSource
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return def
	}
	if d.UserSource == "" {
		d.UserSource = def.UserSource
	}
	if d.GroupSource == "" {
		d.GroupSource = def.GroupSource
	}
	return d
}

// userSourceIsLDAP 判断当前生效的用户来源是否为 LDAP
// （svn 协议与 http 协议任一启用 LDAP 用户来源即视为 LDAP，对齐原版各写接口的拦截口径）
func userSourceIsLDAP() bool {
	if GetDataSource(OptionSvnDatasource).UserSource == SourceLDAP {
		return true
	}
	return GetDataSource(OptionHttpDatasource).UserSource == SourceLDAP
}

// groupSourceIsLDAP 判断当前生效的分组来源是否为 LDAP
func groupSourceIsLDAP() bool {
	if GetDataSource(OptionSvnDatasource).GroupSource == GroupSourceLDAP {
		return true
	}
	return GetDataSource(OptionHttpDatasource).GroupSource == GroupSourceLDAP
}

// SaveDataSource 保存数据源配置
func SaveDataSource(key string, d DataSource) error {
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return SetOption(key, string(data), "用户与分组来源配置")
}

// LDAPFromConfig 将 yaml 中的 LDAP 配置转换为结构化对象
func LDAPFromConfig() LDAPSettings {
	l := config.C().LDAP
	return LDAPSettings{
		Host:              l.Host,
		Port:              l.Port,
		Version:           l.Version,
		BindDN:            l.BindDN,
		BindPassword:      l.BindPassword,
		UserBaseDN:        l.UserBaseDN,
		UserSearchFilter:  l.UserSearchFilter,
		UserAttributes:    l.UserAttributes,
		GroupBaseDN:       l.GroupBaseDN,
		GroupSearchFilter: l.GroupSearchFilter,
		GroupAttributes:   l.GroupAttributes,
		GroupsToUserAttr:  l.GroupsToUserAttr,
		GroupsToUserValue: l.GroupsToUserValue,
	}
}

// DockerHost 宿主机信息（用于生成检出地址）
type DockerHost struct {
	Host     string `json:"host"`
	SvnPort  int    `json:"svnPort"`
	HTTPPort int    `json:"httpPort"`
}

// GetDockerHost 读取宿主机信息
func GetDockerHost() DockerHost {
	d := DockerHost{Host: "127.0.0.1", SvnPort: config.C().SVN.SvnservePort, HTTPPort: config.C().Apache.Port}
	raw := GetOption(OptionDockerHost, "")
	if raw == "" {
		return d
	}
	var v struct {
		DockerHost    string `json:"docker_host"`
		DockerSvnPort int    `json:"docker_svn_port"`
		DockerHTTPPor int    `json:"docker_http_port"`
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return d
	}
	if v.DockerHost != "" {
		d.Host = v.DockerHost
	}
	if v.DockerSvnPort != 0 {
		d.SvnPort = v.DockerSvnPort
	}
	if v.DockerHTTPPor != 0 {
		d.HTTPPort = v.DockerHTTPPor
	}
	return d
}

// SaveDockerHost 保存宿主机信息
func SaveDockerHost(host string, svnPort, httpPort int) error {
	if svnPort == 0 {
		svnPort = config.C().SVN.SvnservePort
	}
	if httpPort == 0 {
		httpPort = config.C().Apache.Port
	}
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return errInvalidHost
	}
	v := map[string]interface{}{
		"docker_host":      host,
		"docker_svn_port":  svnPort,
		"docker_http_port": httpPort,
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return SetOption(OptionDockerHost, string(data), "宿主机地址与端口")
}

// GetLocalHost 读取本地 HTTP 端口
func GetLocalHost() int {
	p := config.C().Apache.Port
	raw := GetOption(OptionLocalHost, "")
	if raw == "" {
		return p
	}
	var v struct {
		LocalHTTPPort int `json:"local_http_port"`
	}
	if err := json.Unmarshal([]byte(raw), &v); err == nil && v.LocalHTTPPort != 0 {
		return v.LocalHTTPPort
	}
	return p
}

// SaveLocalHost 保存本地 HTTP 端口
func SaveLocalHost(port int) error {
	if port == 0 {
		port = 80
	}
	data, err := json.Marshal(map[string]interface{}{"local_http_port": port})
	if err != nil {
		return err
	}
	return SetOption(OptionLocalHost, string(data), "本地 HTTP 端口")
}

// GetEnableCheckout 当前启用的检出协议
func GetEnableCheckout() string {
	// 优先取数据库中保存的协议选择（UpdSubversionEnable / UpdSvnEnable 写入）
	v := strings.ToLower(strings.TrimSpace(GetOption(OptionEnableCheckout, "")))
	if v == "http" || v == "svn" {
		return v
	}
	if config.C().SVN.EnableHTTP {
		return "http"
	}
	return "svn"
}

// InitOptions 初始化系统配置项
func InitOptions() {
	_ = SetOption(OptionSystemVersion, "4.0.0", "系统版本")
	if GetOption(OptionEnableCheckout, "") == "" {
		_ = SetOption(OptionEnableCheckout, GetEnableCheckout(), "当前启用的检出协议")
	}
	if GetOption(OptionHttpPrefix, "") == "" {
		_ = SetOption(OptionHttpPrefix, config.C().SVN.HTTPPrefix, "HTTP 访问前缀")
	}
	if GetOption(OptionSvnDatasource, "") == "" {
		_ = SaveDataSource(OptionSvnDatasource, DataSource{UserSource: SourcePasswd, GroupSource: GroupSourceAuthz, LDAP: LDAPFromConfig()})
	}
	if GetOption(OptionHttpDatasource, "") == "" {
		_ = SaveDataSource(OptionHttpDatasource, DataSource{UserSource: SourceHTTPPasswd, GroupSource: GroupSourceAuthz, LDAP: LDAPFromConfig()})
	}
	if GetOption(OptionSafeConfig, "") == "" {
		_ = SaveSafeConfig(DefaultSafeConfig())
	}
	if GetOption(OptionMessagePush, "") == "" {
		_ = SavePushTriggers(DefaultPushTriggers())
	}
}
