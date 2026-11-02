// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ServerConfig HTTP 服务相关配置
type ServerConfig struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	Mode        string `yaml:"mode"`
	TokenExpire int    `yaml:"token_expire"`
}

// MySQLConfig MySQL 连接配置
type MySQLConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

// DatabaseConfig 数据库配置，支持 sqlite / mysql
type DatabaseConfig struct {
	Driver     string      `yaml:"driver"`
	SQLitePath string      `yaml:"sqlite_path"`
	MySQL      MySQLConfig `yaml:"mysql"`
}

// SVNConfig Subversion 相关路径配置
type SVNConfig struct {
	// HomePath 数据主目录，其余路径均基于此目录推导
	HomePath string `yaml:"home_path"`
	// BinPath svn/svnadmin 可执行文件所在目录，留空则从 PATH 中查找
	BinPath string `yaml:"bin_path"`
	// SvnserveHost svn:// 检出地址使用的主机，留空则使用请求 Host
	SvnserveHost string `yaml:"svnserve_host"`
	// SvnservePort svnserve 监听端口
	SvnservePort int `yaml:"svnserve_port"`
	// HTTPHost http:// 检出地址使用的主机
	HTTPHost string `yaml:"http_host"`
	// HTTPPrefix HTTP 检出路径前缀，如 /svn
	HTTPPrefix string `yaml:"http_prefix"`
	// EnableHTTP 是否启用 HTTP 协议检出（需要 Apache + mod_dav_svn）
	EnableHTTP bool `yaml:"enable_http"`
}

// SecurityConfig 安全相关配置
type SecurityConfig struct {
	JWTSecret        string `yaml:"jwt_secret"`
	EnableVerifyCode bool   `yaml:"enable_verify_code"`
	// LoginFailLimit 登录失败次数限制，超过后锁定账号（0 表示不限制）
	LoginFailLimit int `yaml:"login_fail_limit"`
	// LoginLockSeconds 锁定时长（秒）
	LoginLockSeconds int `yaml:"login_lock_seconds"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level    string `yaml:"level"`
	SaveFile bool   `yaml:"save_file"`
}

// MailConfig 邮件服务配置（SMTP）
type MailConfig struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	Encryption string `yaml:"encryption"` // none | SSL | TLS
	AutoTLS    bool   `yaml:"auto_tls"`
	Auth       bool   `yaml:"auth"`
	User       string `yaml:"user"`
	Pass       string `yaml:"pass"`
	From       string `yaml:"from"`
	To         string `yaml:"to"` // 逗号分隔的收件人
	Timeout    int    `yaml:"timeout"`
	Status     bool   `yaml:"status"`
}

// LDAPConfig LDAP 对接配置
type LDAPConfig struct {
	Host         string `yaml:"host"` // ldap:// 或 ldaps://，不带端口
	Port         int    `yaml:"port"`
	Version      int    `yaml:"version"`
	BindDN       string `yaml:"bind_dn"`
	BindPassword string `yaml:"bind_password"`
	// 用户查询
	UserBaseDN       string `yaml:"user_base_dn"`
	UserSearchFilter string `yaml:"user_search_filter"`
	UserAttributes   string `yaml:"user_attributes"`
	// 分组查询
	GroupBaseDN       string `yaml:"group_base_dn"`
	GroupSearchFilter string `yaml:"group_search_filter"`
	GroupAttributes   string `yaml:"group_attributes"`
	GroupsToUserAttr  string `yaml:"groups_to_user_attribute"`
	GroupsToUserValue string `yaml:"groups_to_user_attribute_value"`
}

// ApacheConfig Apache / HTTP 协议配置
type ApacheConfig struct {
	// ConfPath subversion.conf 路径
	ConfPath string `yaml:"conf_path"`
	// BinPath httpd 可执行文件所在目录
	BinPath string `yaml:"bin_path"`
	// ModulesDir Apache 模块目录
	ModulesDir string `yaml:"modules_dir"`
	// Port HTTP 监听端口
	Port int `yaml:"port"`
}

// UpdateConfig 版本更新检测配置
type UpdateConfig struct {
	Enable bool     `yaml:"enable"`
	Server []string `yaml:"server"`
}

// Config 应用总配置
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	SVN      SVNConfig      `yaml:"svn"`
	Security SecurityConfig `yaml:"security"`
	Log      LogConfig      `yaml:"log"`
	Mail     MailConfig     `yaml:"mail"`
	LDAP     LDAPConfig     `yaml:"ldap"`
	Apache   ApacheConfig   `yaml:"apache"`
	Update   UpdateConfig   `yaml:"update"`

	path string `yaml:"-"`
}

var (
	global *Config
	mu     sync.RWMutex
)

// Load 从 yaml 文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &Config{path: path}
	c.applyDefault()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	c.normalize()
	mu.Lock()
	global = c
	mu.Unlock()
	return c, nil
}

// Default 返回一份内置默认配置并置为全局配置
func Default() *Config {
	c := &Config{}
	c.applyDefault()
	c.normalize()
	mu.Lock()
	global = c
	mu.Unlock()
	return c
}

// C 获取全局配置
func C() *Config {
	mu.RLock()
	defer mu.RUnlock()
	if global == nil {
		return (&Config{}).applyDefaultAndNormalize()
	}
	return global
}

// Path 返回配置文件所在路径
func (c *Config) Path() string { return c.path }

func (c *Config) applyDefaultAndNormalize() *Config {
	c.applyDefault()
	c.normalize()
	return c
}

func (c *Config) applyDefault() {
	c.Server.Host = "0.0.0.0"
	c.Server.Port = 8080
	c.Server.Mode = "release"
	c.Server.TokenExpire = 7200
	c.Database.Driver = "sqlite"
	c.Database.SQLitePath = "./data/svnadmin.db"
	c.SVN.HomePath = "/home/svnadmin/"
	c.SVN.SvnservePort = 3690
	c.SVN.HTTPPrefix = "/svn"
	c.Security.JWTSecret = "svnadmin4-default-secret"
	c.Security.EnableVerifyCode = true
	c.Security.LoginFailLimit = 5
	c.Security.LoginLockSeconds = 300
	c.Log.Level = "info"
	c.Mail.Port = 25
	c.Mail.Encryption = "none"
	c.Mail.Timeout = 5
	c.LDAP.Port = 389
	c.LDAP.Version = 3
	c.Apache.ConfPath = "/etc/httpd/conf.d/subversion.conf"
	c.Apache.BinPath = ""
	c.Apache.Port = 80
	c.Update.Enable = false
}

func (c *Config) normalize() {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.Mode == "" {
		c.Server.Mode = "release"
	}
	if c.Server.TokenExpire <= 0 {
		c.Server.TokenExpire = 7200
	}
	if c.Database.Driver == "" {
		c.Database.Driver = "sqlite"
	}
	if c.Database.SQLitePath == "" {
		c.Database.SQLitePath = "./data/svnadmin.db"
	}
	if c.SVN.HomePath == "" {
		c.SVN.HomePath = "/home/svnadmin/"
	}
	if !strings.HasSuffix(c.SVN.HomePath, "/") {
		c.SVN.HomePath += "/"
	}
	if c.SVN.SvnservePort == 0 {
		c.SVN.SvnservePort = 3690
	}
	if c.Security.JWTSecret == "" {
		c.Security.JWTSecret = "svnadmin4-default-secret"
	}
	if c.Security.LoginFailLimit < 0 {
		c.Security.LoginFailLimit = 0
	}
	if c.Security.LoginLockSeconds <= 0 {
		c.Security.LoginLockSeconds = 300
	}
	if c.Mail.Port == 0 {
		c.Mail.Port = 25
	}
	if c.Mail.Encryption == "" {
		c.Mail.Encryption = "none"
	}
	if c.Mail.Timeout <= 0 {
		c.Mail.Timeout = 5
	}
	if c.LDAP.Port == 0 {
		c.LDAP.Port = 389
	}
	if c.LDAP.Version == 0 {
		c.LDAP.Version = 3
	}
	if c.Apache.Port == 0 {
		c.Apache.Port = 80
	}
	if c.SVN.HTTPPrefix == "" {
		c.SVN.HTTPPrefix = "/svn"
	}
}

// ---------------------------------------------------------------------------
// SVN 路径推导
// ---------------------------------------------------------------------------

func (s SVNConfig) home(elem ...string) string {
	parts := append([]string{s.HomePath}, elem...)
	return filepath.Join(parts...)
}

// RepBasePath 仓库父目录
func (s SVNConfig) RepBasePath() string { return s.home("rep") + "/" }

// SvnserveEnvFile svnserve 环境变量文件
func (s SVNConfig) SvnserveEnvFile() string { return s.home("svnserve") }

// SvnConfFile svnserve.conf
func (s SVNConfig) SvnConfFile() string { return s.home("svnserve.conf") }

// AuthzFile authz 权限文件
func (s SVNConfig) AuthzFile() string { return s.home("authz") }

// PasswdFile passwd 用户文件
func (s SVNConfig) PasswdFile() string { return s.home("passwd") }

// HTTPPasswdFile httpPasswd 文件（htpasswd apr1 格式）
func (s SVNConfig) HTTPPasswdFile() string { return s.home("httpPasswd") }

// SvnservePidFile svnserve pid 文件
func (s SVNConfig) SvnservePidFile() string { return s.home("svnserve.pid") }

// RecommendHookPath 推荐钩子目录
func (s SVNConfig) RecommendHookPath() string { return s.home("hooks") + "/" }

// BackupBasePath 备份目录
func (s SVNConfig) BackupBasePath() string { return s.home("backup") + "/" }

// CrondBasePath 任务计划目录
func (s SVNConfig) CrondBasePath() string { return s.home("crond") + "/" }

// LogBasePath 日志目录
func (s SVNConfig) LogBasePath() string { return s.home("logs") + "/" }

// SvnserveLogFile svnserve 运行日志
func (s SVNConfig) SvnserveLogFile() string { return s.home("logs", "svnserve.log") }

// TempleteBasePath 模板根目录
func (s SVNConfig) TempleteBasePath() string { return s.home("templete") + "/" }

// TempleteInitPath 初始化仓库结构模板目录
func (s SVNConfig) TempleteInitPath() string { return s.home("templete", "initStruct") + "/" }

// SASLHome sasl 目录
func (s SVNConfig) SASLHome() string { return s.home("sasl") + "/" }

// SASLConfFile saslauthd 使用的 LDAP 配置
func (s SVNConfig) SASLConfFile() string { return s.home("sasl", "ldap", "saslauthd.conf") }

// SaslauthdPidFile saslauthd pid 文件
func (s SVNConfig) SaslauthdPidFile() string { return s.home("sasl", "saslauthd.pid") }

// SASL2ConfFile /etc/sasl2/svn.conf 的本地副本（容器内映射场景）
func (s SVNConfig) SASL2ConfFile() string { return s.home("sasl", "svn.conf") }

// TempPath 临时目录（分片上传等）
func (s SVNConfig) TempPath() string { return s.home("temp") + "/" }

// CrondLogFile 指定 sign 的定时任务日志
func (s SVNConfig) CrondLogFile(sign string) string {
	return s.home("crond", sign+".log")
}

// ApacheConfFile 本机维护的 subversion.conf 副本
func (s SVNConfig) ApacheConfFile() string { return s.home("subversion.conf") }

// EnsureDirs 初始化运行所需的全部目录
func (s SVNConfig) EnsureDirs() error {
	dirs := []string{
		s.HomePath,
		s.RepBasePath(),
		s.BackupBasePath(),
		s.CrondBasePath(),
		s.LogBasePath(),
		s.RecommendHookPath(),
		s.TempleteInitPath(),
		s.SASLHome(),
		s.TempPath(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}
