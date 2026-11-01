// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package api

import (
	"github.com/gin-gonic/gin"
	"svnadmin/internal/service"
)

// MonitorInfo 系统监控（负载 / CPU / 内存 / 磁盘 / 主机）
func MonitorInfo(c *gin.Context) {
	ok(c, service.GetMonitorInfo())
}

// LoadInfo 系统负载
func LoadInfo(c *gin.Context) {
	ok(c, service.GetLoadInfo())
}

// DiskInfo 磁盘使用情况
func DiskInfo(c *gin.Context) {
	ok(c, service.GetDiskInfo())
}

// ---------------------------------------------------------------------------
// 邮件与消息推送
// ---------------------------------------------------------------------------

// MailInfo 获取邮件配置
func MailInfo(c *gin.Context) {
	ok(c, service.GetSMTP())
}

// MailUpdate 保存邮件配置
func MailUpdate(c *gin.Context) {
	var cfg service.SMTPConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		fail(c, err)
		return
	}
	if err := service.SaveSMTP(cfg); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// MailTest 发送测试邮件
func MailTest(c *gin.Context) {
	// 支持携带页面上尚未保存的 SMTP 参数直接测试（先测后存），字段为空时回落已保存配置
	var req struct {
		To string `json:"to"`
		service.SMTPConfig
	}
	_ = c.ShouldBindJSON(&req)
	cfg := req.SMTPConfig
	if err := service.SendMailTest(cfg, req.To); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// PushInfo 获取消息推送开关
func PushInfo(c *gin.Context) {
	ok(c, service.GetPushTriggers())
}

// PushUpdate 保存消息推送开关
func PushUpdate(c *gin.Context) {
	var list []service.PushTrigger
	if err := c.ShouldBindJSON(&list); err != nil {
		fail(c, err)
		return
	}
	if err := service.SavePushTriggers(list); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// ---------------------------------------------------------------------------
// 安全配置
// ---------------------------------------------------------------------------

// SafeInfo 获取安全配置
func SafeInfo(c *gin.Context) {
	ok(c, service.GetSafeConfig())
}

// SafeUpdate 保存安全配置
func SafeUpdate(c *gin.Context) {
	var list []service.SafeConfig
	if err := c.ShouldBindJSON(&list); err != nil {
		fail(c, err)
		return
	}
	if err := service.SaveSafeConfig(list); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// ---------------------------------------------------------------------------
// 主机与协议
// ---------------------------------------------------------------------------

// DockerHostInfo 获取宿主机信息
func DockerHostInfo(c *gin.Context) {
	ok(c, service.GetDockerHost())
}

// DockerHostUpdate 保存宿主机信息
func DockerHostUpdate(c *gin.Context) {
	var req struct {
		Host     string `json:"host"`
		SvnPort  int    `json:"svnPort"`
		HTTPPort int    `json:"httpPort"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.SaveDockerHost(req.Host, req.SvnPort, req.HTTPPort); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// DirInfo 路径信息
func DirInfo(c *gin.Context) {
	ok(c, service.DirInfo())
}

// SvnInfo svn 协议信息
func SvnInfo(c *gin.Context) {
	ok(c, service.SvnInfo())
}

// ApacheInfo Apache / HTTP 协议信息
func ApacheInfo(c *gin.Context) {
	ok(c, service.GetApacheInfo())
}

// SubversionEnable 启用 HTTP 协议
func SubversionEnable(c *gin.Context) {
	var req struct {
		Prefix string `json:"prefix"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := service.UpdSubversionEnable(req.Prefix, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SvnEnable 切换回 svn 协议
func SvnEnable(c *gin.Context) {
	if err := service.UpdSvnEnable(operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// HttpPrefixUpdate 修改 HTTP 访问前缀
func HttpPrefixUpdate(c *gin.Context) {
	var req struct {
		Prefix string `json:"prefix"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdHttpPrefix(req.Prefix, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// HttpPortUpdate 修改 HTTP 端口
func HttpPortUpdate(c *gin.Context) {
	var req struct {
		Port int `json:"port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdHttpPort(req.Port, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SvnservePortUpdate 修改 svnserve 端口
func SvnservePortUpdate(c *gin.Context) {
	var req struct {
		Port int `json:"port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdSvnservePort(req.Port, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SvnserveHostUpdate 修改 svnserve 监听地址
func SvnserveHostUpdate(c *gin.Context) {
	var req struct {
		Host string `json:"host"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdSvnserveHost(req.Host, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// UsersourceUpdate 修改用户与分组来源
func UsersourceUpdate(c *gin.Context) {
	var req struct {
		Protocol string             `json:"protocol"` // svn | http
		Source   service.DataSource `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	var err error
	if req.Protocol == "http" {
		err = service.UpdHttpUsersource(req.Source, operator(c))
	} else {
		err = service.UpdSvnUsersource(req.Source, operator(c))
	}
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// ---------------------------------------------------------------------------
// SASL
// ---------------------------------------------------------------------------

// SaslInfo saslauthd 状态
func SaslInfo(c *gin.Context) {
	ok(c, service.GetSaslInfo())
}

// SaslStart 启动 saslauthd
func SaslStart(c *gin.Context) {
	if err := service.SaslStart(operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SaslStop 停止 saslauthd
func SaslStop(c *gin.Context) {
	if err := service.SaslStop(operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// ---------------------------------------------------------------------------
// LDAP
// ---------------------------------------------------------------------------

// LdapTest LDAP 连接 / 用户 / 分组测试
func LdapTest(c *gin.Context) {
	var req struct {
		Type   string               `json:"type"`
		Config service.LDAPSettings `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	res, err := service.LDAPTest(req.Type, req.Config)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, res)
}

// LdapUsers LDAP 用户列表
func LdapUsers(c *gin.Context) {
	var cfg service.LDAPSettings
	if err := c.ShouldBindJSON(&cfg); err != nil {
		fail(c, err)
		return
	}
	users, err := service.LDAPUsers(cfg)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, users)
}

// LdapGroups LDAP 分组列表
func LdapGroups(c *gin.Context) {
	var cfg service.LDAPSettings
	if err := c.ShouldBindJSON(&cfg); err != nil {
		fail(c, err)
		return
	}
	groups, err := service.LDAPGroups(cfg)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, groups)
}

// LdapSync 同步 LDAP 用户 / 分组
func LdapSync(c *gin.Context) {
	var req struct {
		Kind string `json:"kind"` // user | group | all
	}
	_ = c.ShouldBindJSON(&req)
	op := operator(c)
	switch req.Kind {
	case "group":
		n, err := service.SyncLDAPGroupsToAuthz(op)
		if err != nil {
			failMsg(c, err.Error())
			return
		}
		ok(c, gin.H{"groupCount": n})
	case "all":
		u, err := service.SyncLDAPToDB(op)
		if err != nil {
			failMsg(c, err.Error())
			return
		}
		g, err := service.SyncLDAPGroupsToAuthz(op)
		if err != nil {
			failMsg(c, err.Error())
			return
		}
		ok(c, gin.H{"userCount": u, "groupCount": g})
	default:
		n, err := service.SyncLDAPToDB(op)
		if err != nil {
			failMsg(c, err.Error())
			return
		}
		ok(c, gin.H{"userCount": n})
	}
}

// ---------------------------------------------------------------------------
// 版本更新
// ---------------------------------------------------------------------------

// CheckUpdate 检测版本更新
func CheckUpdate(c *gin.Context) {
	info, err := service.CheckUpdate()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, info)
}

// VersionInfo 版本信息
func VersionInfo(c *gin.Context) {
	ok(c, service.VersionInfo())
}

// ---------------------------------------------------------------------------
// 数据同步
// ---------------------------------------------------------------------------

// SyncAll 同步用户 / 分组 / 仓库
func SyncAll(c *gin.Context) {
	if err := service.SyncAll(operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SyncUsers 同步 SVN 用户
func SyncUsers(c *gin.Context) {
	if err := service.SyncUsers(); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SyncGroups 同步 SVN 分组
func SyncGroups(c *gin.Context) {
	if err := service.SyncGroups(); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SyncReps 同步 SVN 仓库
func SyncReps(c *gin.Context) {
	if err := service.SyncReps(); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// ---------------------------------------------------------------------------
// 个人信息
// ---------------------------------------------------------------------------

// EditAdminUserName 管理员修改账号名
func EditAdminUserName(c *gin.Context) {
	var req struct {
		OldName string `json:"oldName"`
		NewName string `json:"newName"`
		Pass    string `json:"pass"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.AdminUpdName(req.OldName, req.NewName, req.Pass); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}
