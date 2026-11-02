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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"svnadmin/internal/config"
	"svnadmin/internal/model"
	"svnadmin/internal/store"
	"svnadmin/internal/svn"
)

// ApacheInfo Apache / HTTP 协议信息
type ApacheInfo struct {
	Installed    bool     `json:"installed"`
	BinPath      string   `json:"binPath"`
	Version      string   `json:"version"`
	ModulesDir   string   `json:"modulesDir"`
	DavSvnModule bool     `json:"davSvnModule"`
	AuthzSvnMod  bool     `json:"authzSvnModule"`
	ConfPath     string   `json:"confPath"`
	ConfExists   bool     `json:"confExists"`
	Port         int      `json:"port"`
	Prefix       string   `json:"prefix"`
	Warnings     []string `json:"warnings"`
	// 数据源信息（对齐原版 GetApacheInfo 字段）
	UserSource  string   `json:"userSource"`
	GroupSource string   `json:"groupSource"`
	PasswordDB  bool     `json:"passwordDb"`
	LDAP        LDAPInfo `json:"ldap"`
}

// LDAPInfo LDAP 配置摘要（脱敏：不含 bind 密码）
type LDAPInfo struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Version     int    `json:"version"`
	BindDN      string `json:"bindDn"`
	UserBaseDN  string `json:"userBaseDn"`
	GroupBaseDN string `json:"groupBaseDn"`
}

// GetApacheInfo 探测 Apache 与 mod_dav_svn 安装情况
func GetApacheInfo() ApacheInfo {
	cfg := config.C()
	ds := GetDataSource(OptionHttpDatasource)
	info := ApacheInfo{
		ConfPath:    cfg.Apache.ConfPath,
		Port:        cfg.Apache.Port,
		Prefix:      cfg.SVN.HTTPPrefix,
		UserSource:  ds.UserSource,
		GroupSource: ds.GroupSource,
		PasswordDB:  ds.UserSource == SourceHTTPPasswd,
		LDAP: LDAPInfo{
			Host:        ds.LDAP.Host,
			Port:        ds.LDAP.Port,
			Version:     ds.LDAP.Version,
			BindDN:      ds.LDAP.BindDN,
			UserBaseDN:  ds.LDAP.UserBaseDN,
			GroupBaseDN: ds.LDAP.GroupBaseDN,
		},
	}
	bin := ""
	for _, p := range []string{cfg.Apache.BinPath + "/httpd", "/usr/sbin/httpd", "/usr/sbin/apache2", "/usr/local/apache2/bin/httpd"} {
		if p == "" || p == "/httpd" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			bin = p
			break
		}
	}
	if bin == "" {
		if p, err := exec.LookPath("httpd"); err == nil {
			bin = p
		} else if p, err := exec.LookPath("apache2"); err == nil {
			bin = p
		}
	}
	if bin == "" {
		info.Warnings = append(info.Warnings, "未检测到 Apache httpd，HTTP 协议检出不可用")
		return info
	}
	info.Installed = true
	info.BinPath = bin
	if out, err := exec.Command(bin, "-v").CombinedOutput(); err == nil {
		info.Version = strings.TrimSpace(string(out))
	}
	dirs := []string{cfg.Apache.ModulesDir, "/usr/lib64/httpd/modules", "/usr/lib/httpd/modules",
		"/usr/lib/apache2/modules", filepath.Join(filepath.Dir(bin), "..", "modules")}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			info.ModulesDir = d
			break
		}
	}
	if info.ModulesDir != "" {
		if _, err := os.Stat(filepath.Join(info.ModulesDir, "mod_dav_svn.so")); err == nil {
			info.DavSvnModule = true
		}
		if _, err := os.Stat(filepath.Join(info.ModulesDir, "mod_authz_svn.so")); err == nil {
			info.AuthzSvnMod = true
		}
	}
	if !info.DavSvnModule {
		info.Warnings = append(info.Warnings, "未找到 mod_dav_svn 模块")
	}
	if !info.AuthzSvnMod {
		info.Warnings = append(info.Warnings, "未找到 mod_authz_svn 模块")
	}
	if _, err := os.Stat(info.ConfPath); err == nil {
		info.ConfExists = true
	}
	return info
}

// apacheTemplate 生成 subversion.conf 内容。
// ds 为 HTTP 协议的数据源：userSource=ldap 时生成 LDAP 认证段（对齐原版 subversion-ldap.conf）。
func apacheTemplate(prefix string, ds DataSource) string {
	cfg := config.C().SVN
	loc := prefix
	if loc == "" || loc == "/" {
		loc = "/"
	}
	passFile := cfg.HTTPPasswdFile()
	if !fileExists(passFile) {
		passFile = cfg.PasswdFile()
	}
	head := fmt.Sprintf(`# 由 SVNAdmin4 自动生成，请勿手工修改
LoadModule dav_svn_module     modules/mod_dav_svn.so
LoadModule authz_svn_module   modules/mod_authz_svn.so

<Location %s>
    DAV svn
    SVNListParentPath on
    SVNParentPath %s
    AuthType Basic
    AuthName "SVNAdmin4 Authorization"
`, loc, cfg.RepBasePath())
	body := fmt.Sprintf(`    AuthUserFile %s
    AuthzSVNAccessFile %s
    Require valid-user
</Location>
`, passFile, cfg.AuthzFile())
	if ds.UserSource == SourceLDAP {
		ldapBlock := fmt.Sprintf(`    AuthBasicProvider ldap
    AuthLDAPBindAuthoritative off
    AuthzLDAPAuthoritative off
    AuthLDAPURL "%s"
    AuthLDAPBindDN "%s"
    AuthLDAPBindPassword "%s"
    Require ldap-user
</Location>
`, ldapAuthURL(ds.LDAP), ds.LDAP.BindDN, ds.LDAP.BindPassword)
		return head + ldapBlock
	}
	return head + body
}

// ldapAuthURL 组装 Apache mod_authnz_ldap 的 AuthLDAPURL
// 形如 ldap://host:port/baseDN?attribute?scope?filter
func ldapAuthURL(s LDAPSettings) string {
	addr, _, err := ldapAddr(s)
	if err != nil || addr == "" {
		addr = s.Host
	}
	scheme := "ldap://"
	if strings.HasPrefix(s.Host, "ldaps://") {
		scheme = "ldaps://"
	}
	attr := orDefault(s.UserAttributes, "uid")
	if i := strings.Index(attr, ","); i >= 0 {
		attr = strings.TrimSpace(attr[:i])
	}
	filter := orDefault(s.UserSearchFilter, "(objectClass=inetOrgPerson)")
	return fmt.Sprintf("%s%s/%s?%s?sub?%s", scheme, addr, s.UserBaseDN, attr, filter)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// UpdSubversionEnable 启用 HTTP 协议（写入 subversion.conf 并重载 Apache）
func UpdSubversionEnable(prefix string, operator string) error {
	cfg := config.C()
	content := apacheTemplate(prefix, GetDataSource(OptionHttpDatasource))
	target := cfg.Apache.ConfPath
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		// 无权限写入系统目录时退回主目录，并在日志中提示
		fallback := cfg.SVN.ApacheConfFile()
		if err2 := writeFile(fallback, content, 0644); err2 != nil {
			return fmt.Errorf("写入 subversion.conf 失败：%v（已尝试回退路径 %s：%v）", err, fallback, err2)
		}
		AddLog("系统配置", fmt.Sprintf("无权限写入 %s，配置已写入 %s，请手工复制并重载 Apache", target, fallback), operator)
	} else {
		if err := writeFile(target, content, 0644); err != nil {
			return fmt.Errorf("写入 subversion.conf 失败：%v", err)
		}
	}
	if prefix != "" {
		_ = SetOption(OptionHttpPrefix, prefix, "HTTP 访问前缀")
	}
	_ = SetOption(OptionEnableCheckout, "http", "当前启用的检出协议")
	reloadApache()
	AddLog("系统配置", "启用 HTTP 协议检出", operator)
	return nil
}

// UpdSvnEnable 切回 svn 协议（移除 subversion.conf 并重载 Apache）
func UpdSvnEnable(operator string) error {
	cfg := config.C()
	// 对齐原版：切回 svn 协议时若 svnserve 未运行则先启动，保证 svn:// 检出可用
	if !SvnserveStatus() {
		if err := SvnserveStart(operator); err != nil {
			// 启动失败不阻断切换，仅记录警告日志
			AddLog("系统配置", fmt.Sprintf("切换回 svn 协议时启动 svnserve 失败：%v", err), operator)
		}
	}
	// 数据源为 passwd 时清空 svn_users（用户将改由 passwd 文件管理，重新扫描导入）
	if ds := GetDataSource(OptionSvnDatasource); ds.UserSource == SourcePasswd {
		_ = clearUsersForSourceSwitch(false)
	}
	targets := []string{cfg.Apache.ConfPath, cfg.SVN.ApacheConfFile()}
	removed := false
	for _, t := range targets {
		if fileExists(t) {
			if err := os.Remove(t); err != nil {
				return fmt.Errorf("删除 Apache 配置文件 %s 失败：%v", t, err)
			}
			removed = true
		}
	}
	if !removed {
		AddLog("系统配置", "切换回 svn 协议（未发现 Apache 配置文件）", operator)
	}
	if err := SetOption(OptionEnableCheckout, "svn", "当前启用的检出协议"); err != nil {
		return err
	}
	if err := reloadApache(); err != nil {
		AddLog("系统配置", fmt.Sprintf("切换回 svn 协议后重载 Apache 失败：%v", err), operator)
	}
	AddLog("系统配置", "切换回 svn 协议检出", operator)
	return nil
}

// reloadApache 优雅重载 Apache
func reloadApache() error {
	info := GetApacheInfo()
	if !info.Installed {
		return errors.New("未检测到 Apache httpd，请手工重载")
	}
	out, err := exec.Command(info.BinPath, "-k", "graceful").CombinedOutput()
	if err != nil {
		return fmt.Errorf("重载 Apache 失败：%v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// UpdHttpPrefix 修改 HTTP 访问前缀
func UpdHttpPrefix(prefix string, operator string) error {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return errors.New("访问前缀不能为空")
	}
	if !strings.HasPrefix(prefix, "/") {
		return errors.New("访问前缀必须以 / 开头")
	}
	if err := SetOption(OptionHttpPrefix, prefix, "HTTP 访问前缀"); err != nil {
		return err
	}
	// 同步更新已生成的配置文件
	if fileExists(config.C().Apache.ConfPath) {
		_ = UpdSubversionEnable(prefix, operator)
	}
	AddLog("系统配置", fmt.Sprintf("修改 HTTP 访问前缀为 [%s]", prefix), operator)
	return nil
}

// UpdHttpPort 修改 HTTP 端口
func UpdHttpPort(port int, operator string) error {
	if port <= 0 || port > 65535 {
		return errors.New("端口不合法")
	}
	if err := SaveLocalHost(port); err != nil {
		return err
	}
	AddLog("系统配置", fmt.Sprintf("修改 HTTP 端口为 %d", port), operator)
	return nil
}

// UpdSvnservePort 修改 svnserve 监听端口（写配置并按需要重启 svnserve 使其生效）
func UpdSvnservePort(port int, operator string) error {
	if port <= 0 || port > 65535 {
		return errors.New("端口不合法")
	}
	if port == SvnservePort() {
		return errors.New("端口未发生变化")
	}
	if err := SetOption(OptionSvnservePort, fmt.Sprintf("%d", port), "svnserve 监听端口"); err != nil {
		return err
	}
	if SvnserveStatus() {
		_ = SvnserveStop(operator)
		time.Sleep(time.Second)
		if err := SvnserveStart(operator); err != nil {
			return fmt.Errorf("端口已保存，但重启 svnserve 失败：%v", err)
		}
		AddLog("系统配置", fmt.Sprintf("修改 svnserve 端口为 %d 并重启服务", port), operator)
		return nil
	}
	AddLog("系统配置", fmt.Sprintf("修改 svnserve 端口为 %d", port), operator)
	return nil
}

// UpdSvnserveHost 修改 svnserve 监听地址
func UpdSvnserveHost(host, operator string) error {
	host = strings.TrimSpace(host)
	if host != "" {
		low := strings.ToLower(host)
		if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
			return errors.New("监听地址无需携带协议前缀")
		}
	}
	if err := SetOption(OptionSvnserveHost, host, "svnserve 监听地址"); err != nil {
		return err
	}
	if SvnserveStatus() {
		_ = SvnserveStop(operator)
		time.Sleep(time.Second)
		if err := SvnserveStart(operator); err != nil {
			return fmt.Errorf("地址已保存，但重启 svnserve 失败：%v", err)
		}
		AddLog("系统配置", fmt.Sprintf("修改 svnserve 监听地址为 [%s] 并重启服务", host), operator)
		return nil
	}
	AddLog("系统配置", fmt.Sprintf("修改 svnserve 监听地址为 [%s]", host), operator)
	return nil
}

// UpdHttpUsersource 修改 HTTP 协议下的用户来源。
// 与原版一致：切到 ldap 时清空本地用户表，并用对应模板重写 subversion.conf 后重载 Apache。
func UpdHttpUsersource(d DataSource, operator string) error {
	if d.UserSource != SourceHTTPPasswd && d.UserSource != SourceLDAP {
		return errors.New("HTTP 协议下用户来源仅支持 httpPasswd / ldap")
	}
	if d.GroupSource != GroupSourceAuthz && d.GroupSource != GroupSourceLDAP {
		return errors.New("分组来源仅支持 authz / ldap")
	}
	if err := SaveDataSource(OptionHttpDatasource, d); err != nil {
		return err
	}
	// 切换数据源后本地账号已失效，清空用户（分组为 ldap 时一并清空）
	if err := clearUsersForSourceSwitch(d.GroupSource == GroupSourceLDAP); err != nil {
		return err
	}
	// 已经启用 HTTP 协议时，按新数据源重新生成配置并重载
	if GetEnableCheckout() == "http" {
		prefix := GetOption(OptionHttpPrefix, "/svn")
		content := apacheTemplate(prefix, d)
		if cfg := config.C(); fileExists(cfg.Apache.ConfPath) || fileExists(cfg.SVN.ApacheConfFile()) {
			target := cfg.Apache.ConfPath
			if err := os.MkdirAll(filepath.Dir(target), 0755); err == nil {
				_ = writeFile(target, content, 0644)
			} else {
				_ = writeFile(cfg.SVN.ApacheConfFile(), content, 0644)
			}
		}
		if err := reloadApache(); err != nil {
			AddLog("系统配置", fmt.Sprintf("重载 Apache 失败：%v", err), operator)
		}
	}
	AddLog("系统配置", fmt.Sprintf("修改 HTTP 用户来源为 [%s]，分组来源为 [%s]", d.UserSource, d.GroupSource), operator)
	return nil
}

// UpdSvnUsersource 修改 svn 协议下的用户与分组来源。
// 与原版一致：切到 ldap 时编排 SASL（svnserve.conf / sasl2 / saslauthd.conf）并重启 svnserve；
// 切回 passwd 时关闭 SASL 并重启 svnserve。
func UpdSvnUsersource(d DataSource, operator string) error {
	switch d.UserSource {
	case SourcePasswd, SourceLDAP:
	default:
		return errors.New("svn 协议下用户来源仅支持 passwd / ldap")
	}
	switch d.GroupSource {
	case GroupSourceAuthz, GroupSourceLDAP:
	default:
		return errors.New("分组来源仅支持 authz / ldap")
	}
	if d.UserSource == SourceLDAP && strings.TrimSpace(d.LDAP.Host) == "" {
		return errors.New("LDAP 主机地址不能为空")
	}
	if err := SaveDataSource(OptionSvnDatasource, d); err != nil {
		return err
	}
	// 切换数据源后本地账号失效，清空用户/分组
	if err := clearUsersForSourceSwitch(d.GroupSource == GroupSourceLDAP); err != nil {
		return err
	}
	// 编排 SASL：ldap 开启、passwd 关闭
	if err := EnsureSvnserveConf(d.UserSource == SourceLDAP); err != nil {
		AddLog("系统配置", fmt.Sprintf("写入 svnserve.conf 失败：%v", err), operator)
	}
	if d.UserSource == SourceLDAP {
		if err := writeSaslauthdConf(d.LDAP); err != nil {
			AddLog("系统配置", fmt.Sprintf("写入 saslauthd.conf 失败：%v", err), operator)
		}
		_ = writeSASL2Conf()
		if SvnserveStatus() {
			_ = SvnserveStop(operator)
			time.Sleep(time.Second)
			_ = SvnserveStart(operator)
		}
	} else {
		if err := SaslStop(operator); err != nil {
			// 未在运行属正常情况，忽略
			_ = err
		}
		if SvnserveStatus() {
			_ = SvnserveStop(operator)
			time.Sleep(time.Second)
			_ = SvnserveStart(operator)
		}
	}
	AddLog("系统配置", fmt.Sprintf("修改 SVN 用户来源为 [%s]，分组来源为 [%s]", d.UserSource, d.GroupSource), operator)
	return nil
}

// writeSaslauthdConf 生成 saslauthd 的 LDAP 配置（对齐原版 templete/sasl/ldap/saslauthd.conf）
func writeSaslauthdConf(s LDAPSettings) error {
	cfg := config.C().SVN
	conf := cfg.SASLConfFile()
	if err := os.MkdirAll(filepath.Dir(conf), 0755); err != nil {
		return err
	}
	attr := orDefault(s.UserAttributes, "uid")
	if i := strings.Index(attr, ","); i >= 0 {
		attr = strings.TrimSpace(attr[:i])
	}
	filter := orDefault(s.UserSearchFilter, "(uid=%u)")
	content := fmt.Sprintf(`ldap_servers: %s
ldap_bind_dn: %s
ldap_bind_pw: %s
ldap_search_base: %s
ldap_filter: %s
ldap_password_attr: userPassword
ldap_version: %d
`, ldapServerURL(s), s.BindDN, s.BindPassword, s.UserBaseDN, filter, orDefaultInt(s.Version, 3))
	_ = attr
	return writeFile(conf, content, 0600)
}

// writeSASL2Conf 写入 /etc/sasl2/svn.conf（目录不存在时创建，失败则写本地副本留待手工复制）
func writeSASL2Conf() error {
	content := "pwcheck_method: saslauthd\nmech_list: PLAIN LOGIN\n"
	if err := os.MkdirAll("/etc/sasl2", 0755); err == nil {
		if err := os.WriteFile("/etc/sasl2/svn.conf", []byte(content), 0644); err == nil {
			return nil
		}
	}
	// 回退：写入主目录副本
	return writeFile(config.C().SVN.SASL2ConfFile(), content, 0644)
}

// ---------------------------------------------------------------------------
// SASL
// ---------------------------------------------------------------------------

// SaslInfo saslauthd 运行状态
type SaslInfo struct {
	Installed bool     `json:"installed"`
	Running   bool     `json:"running"`
	Version   string   `json:"version"`
	Mechanis  []string `json:"mechanisms"`
	Pid       string   `json:"pid"`
	ConfFile  string   `json:"confFile"`
}

// saslauthdBin 查找 saslauthd
func saslauthdBin() string {
	for _, p := range []string{"/usr/sbin/saslauthd", "/usr/local/sbin/saslauthd", "/sbin/saslauthd"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath("saslauthd"); err == nil {
		return p
	}
	return ""
}

// GetSaslInfo 读取 saslauthd 信息
func GetSaslInfo() SaslInfo {
	cfg := config.C().SVN
	info := SaslInfo{ConfFile: cfg.SASLConfFile()}
	bin := saslauthdBin()
	if bin == "" {
		return info
	}
	info.Installed = true
	if out, err := exec.Command(bin, "-v").CombinedOutput(); err == nil {
		s := string(out)
		info.Version = strings.TrimSpace(s)
		for _, line := range strings.Split(s, "\n") {
			line = strings.ToLower(line)
			if idx := strings.Index(line, "authentication mechanisms:"); idx >= 0 {
				for _, m := range strings.Fields(line[idx+len("authentication mechanisms:"):]) {
					info.Mechanis = append(info.Mechanis, strings.ToUpper(strings.Trim(m, ",")))
				}
			}
		}
	}
	if data, err := os.ReadFile(cfg.SaslauthdPidFile()); err == nil {
		pid := strings.TrimSpace(string(data))
		info.Pid = pid
		if pid != "" {
			if _, err := os.Stat("/proc/" + pid); err == nil {
				info.Running = true
			}
		}
	}
	return info
}

// SaslStart 启动 saslauthd（LDAP 模式）
func SaslStart(operator string) error {
	info := GetSaslInfo()
	if !info.Installed {
		return errors.New("未检测到 saslauthd")
	}
	if info.Running {
		return errors.New("saslauthd 已在运行")
	}
	cfg := config.C().SVN
	if err := os.MkdirAll(filepath.Dir(cfg.SASLConfFile()), 0755); err != nil {
		return err
	}
	ds := GetDataSource(OptionSvnDatasource)
	if err := writeSaslauthdConf(ds.LDAP); err != nil {
		return err
	}
	// 同步写入 /etc/sasl2/svn.conf（目录不可写时落主目录副本）
	_ = writeSASL2Conf()

	// 若 svnserve 正在运行且尚未开启 SASL，需要重写配置并重启使 use-sasl 生效
	if SvnserveStatus() {
		_ = EnsureSvnserveConf(true)
		_ = SvnserveStop(operator)
		time.Sleep(time.Second)
		_ = SvnserveStart(operator)
	}

	bin := saslauthdBin()
	pidFile := cfg.SaslauthdPidFile()
	_ = os.Remove(pidFile)
	out, err := exec.Command(bin, "-a", "ldap", "-O", cfg.SASLConfFile(), "-n", "0").CombinedOutput()
	if err != nil {
		return fmt.Errorf("启动 saslauthd 失败：%v %s", err, strings.TrimSpace(string(out)))
	}
	// 记录 pid：saslauthd 默认把 pid 写在 /var/run/saslauthd/saslauthd.pid 或 -m 指定目录，
	// 这里优先读取，读不到则回落到进程表扫描，保证启停闭环。
	pid := ""
	for i := 0; i < 20 && pid == ""; i++ {
		pid = readSaslauthdPid()
		if pid == "" {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if pid == "" {
		return errors.New("saslauthd 已启动但未能获取进程号，请检查 /var/run/saslauthd 权限")
	}
	if err := writeFile(pidFile, pid, 0644); err != nil {
		return err
	}
	_ = SetOption(OptionSaslStatus, "1", "saslauthd 运行状态")
	AddLog("系统配置", fmt.Sprintf("启动 saslauthd（LDAP 模式），进程号 %s", pid), operator)
	return nil
}

// readSaslauthdPid 从常见位置读取 saslauthd 进程号，读不到则扫描进程表
func readSaslauthdPid() string {
	candidates := []string{
		"/var/run/saslauthd/saslauthd.pid",
		"/run/saslauthd/saslauthd.pid",
		"/var/run/saslauthd.pid",
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			if pid := strings.TrimSpace(string(data)); pid != "" {
				return pid
			}
		}
	}
	out, err := exec.Command("sh", "-c", "ps -eo pid,args | grep -v grep | grep '[s]aslauthd' | awk 'NR==1{print $1}'").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// SaslStop 停止 saslauthd：读 pid 文件发送 SIGTERM，必要时 SIGKILL 并复查
func SaslStop(operator string) error {
	cfg := config.C().SVN
	pid := ""
	if data, err := os.ReadFile(cfg.SaslauthdPidFile()); err == nil {
		pid = strings.TrimSpace(string(data))
	}
	alive := func() bool {
		if pid == "" {
			return false
		}
		_, err := os.Stat("/proc/" + pid)
		return err == nil
	}
	if !alive() {
		// pid 文件可能已过期，尝试从进程表重新定位
		if p := readSaslauthdPid(); p != "" {
			pid = p
		}
	}
	if !alive() {
		_ = os.Remove(cfg.SaslauthdPidFile())
		_ = SetOption(OptionSaslStatus, "0", "saslauthd 运行状态")
		return errors.New("saslauthd 未在运行")
	}
	_ = exec.Command("kill", "-15", pid).Run()
	for i := 0; i < 20 && alive(); i++ {
		time.Sleep(100 * time.Millisecond)
	}
	if alive() {
		_ = exec.Command("kill", "-9", pid).Run()
		time.Sleep(200 * time.Millisecond)
	}
	if alive() {
		return errors.New("saslauthd 停止失败")
	}
	_ = os.Remove(cfg.SaslauthdPidFile())
	_ = SetOption(OptionSaslStatus, "0", "saslauthd 运行状态")
	AddLog("系统配置", "停止 saslauthd", operator)
	return nil
}

func ldapServerURL(s LDAPSettings) string {
	addr, _, err := ldapAddr(s)
	if err != nil {
		return ""
	}
	scheme := "ldap://"
	if strings.HasPrefix(s.Host, "ldaps://") {
		scheme = "ldaps://"
	}
	return scheme + addr
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func orDefaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func writeFileIfExists(path, content string) error {
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// ---------------------------------------------------------------------------
// 路径信息
// ---------------------------------------------------------------------------

// DirInfo 系统使用的关键路径
func DirInfo() map[string]interface{} {
	cfg := config.C().SVN
	return map[string]interface{}{
		"homePath":     cfg.HomePath,
		"repBasePath":  cfg.RepBasePath(),
		"authzFile":    cfg.AuthzFile(),
		"passwdFile":   cfg.PasswdFile(),
		"httpPasswd":   cfg.HTTPPasswdFile(),
		"svnConfFile":  cfg.SvnConfFile(),
		"backupPath":   cfg.BackupBasePath(),
		"crondPath":    cfg.CrondBasePath(),
		"logPath":      cfg.LogBasePath(),
		"templetePath": cfg.TempleteBasePath(),
		"hookPath":     cfg.RecommendHookPath(),
		"saslPath":     cfg.SASLHome(),
		"binPath":      cfg.BinPath,
		"configFile":   config.C().Path(),
		"database":     databaseDesc(),
	}
}

func databaseDesc() string {
	c := config.C().Database
	if c.Driver == "mysql" {
		return fmt.Sprintf("mysql://%s:%d/%s", c.MySQL.Host, c.MySQL.Port, c.MySQL.Name)
	}
	return "sqlite://" + c.SQLitePath
}

// SvnInfo svn 协议相关信息
func SvnInfo() map[string]interface{} {
	cfg := config.C().SVN
	ds := GetDataSource(OptionSvnDatasource)
	httpDS := GetDataSource(OptionHttpDatasource)
	return map[string]interface{}{
		"enableCheckout": GetEnableCheckout(),
		"svnserveStatus": svnStatus(),
		"svnservePort":   cfg.SvnservePort,
		"svnserveHost":   cfg.SvnserveHost,
		"svnVersion":     VersionText(),
		"sasl":           GetSaslInfo(),
		"dataSource":     ds,
		"httpDataSource": httpDS,
		"httpPrefix":     GetOption(OptionHttpPrefix, cfg.HTTPPrefix),
		"httpPort":       GetLocalHost(),
		"dockerHost":     GetDockerHost(),
	}
}

func svnStatus() bool { return SvnserveStatus() }

// SyncAll 一次性同步用户、分组、仓库
func SyncAll(operator string) error {
	if err := SyncUsers(); err != nil {
		return err
	}
	if err := SyncGroups(); err != nil {
		return err
	}
	return SyncReps()
}

// clearUsersForSourceSwitch 切换数据源时清空本地账号（与原版 UpdSvnUsersource / UpdHttpUsersource 行为一致：
// 数据源切换后本地 passwd 账号不再生效，需清空后由新数据源重新同步）。
// 分组来源同样切到 ldap 时一并清空分组表。
func clearUsersForSourceSwitch(clearGroups bool) error {
	if err := store.DB().Where("1 = 1").Delete(&model.SvnUser{}).Error; err != nil {
		return fmt.Errorf("清空用户表失败：%v", err)
	}
	if clearGroups {
		// authz 中的 [groups] 段由 LDAP 同步重建
		az, err := svn.LoadAuthz()
		if err == nil {
			for name := range az.Groups {
				az.DelGroup(name)
			}
			if err := az.Save(); err != nil {
				AddLog("系统配置", fmt.Sprintf("清空 authz 分组失败：%v", err), "system")
			}
		}
		if err := store.DB().Where("1 = 1").Delete(&model.SvnGroup{}).Error; err != nil {
			return fmt.Errorf("清空分组表失败：%v", err)
		}
	}
	return nil
}
