// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package router

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"svnadmin/internal/api"
	"svnadmin/internal/auth"
	"svnadmin/internal/config"
	"svnadmin/internal/middleware"
)

// Register 注册全部路由
func Register(r *gin.Engine, webDir string) {
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": true, "code": 200, "message": "ok", "data": nil})
	})

	// 免鉴权接口
	pub := r.Group("/api/common")
	{
		pub.POST("/login", api.Login)
		pub.GET("/verifyCode", api.VerifyCode)
		pub.GET("/verifyOption", api.VerifyOption)
	}

	// 需登录（任意角色）
	any := r.Group("/api", middleware.Auth())
	{
		any.POST("/common/logout", api.Logout)
		any.GET("/common/userInfo", api.UserInfo)
		any.GET("/common/svnserveStatus", api.SvnserveStatus)
		any.GET("/common/statistics", api.Statistics)
		any.POST("/common/updatePass", api.UpdatePass)
		any.POST("/common/editAdminUserName", api.EditAdminUserName)
		any.GET("/common/monitor", api.MonitorInfo)
		any.GET("/common/loadInfo", api.LoadInfo)
		any.GET("/common/diskInfo", api.DiskInfo)
		any.GET("/common/version", api.VersionInfo)

		// SVN 用户可见的仓库
		any.GET("/rep/userList", api.SvnUserRepList)
		any.GET("/rep/all", api.RepAll)
		any.GET("/rep/checkout", api.RepCheckout)
		any.GET("/user/all", api.UserAll)
		any.GET("/group/all", api.GroupAll)
		any.GET("/aliase/list", api.AliaseList)
		any.GET("/pri/list", api.PriList)

		// 权限自助编辑（SVN 用户走二次授权链校验：写权限 + 开关 + 白名单；
		// 管理员/子管理员不受限——对齐原版二次授权的用户自服务闭环）
		any.POST("/pri/create", api.PriCreate)
		any.POST("/pri/update", api.PriUpdate)
		any.POST("/pri/delete", api.PriDelete)

		// 二次授权自助（SVN 用户强制 owner=当前登录用户）
		any.POST("/secondpri/update", api.SecondPriUpdate)
		any.GET("/secondpri/objectList", api.SecondPriObjectList)
		any.POST("/secondpri/objectCreate", api.SecondPriObjectCreate)
		any.POST("/secondpri/objectDel", api.SecondPriObjectDel)
		any.GET("/secondpri/userPaths", api.SecondPriUserPaths)

		any.GET("/task/list", api.TaskList)
		any.GET("/task/detail", api.TaskDetail)
		any.GET("/task/run", api.TaskRun)
		any.GET("/task/log", api.TaskLog)
		any.GET("/task/types", api.TaskTypes)
		any.GET("/crond/dict", api.CrondDict)
		any.GET("/crond/log", api.CrondLog)
		any.GET("/setting/dirInfo", api.DirInfo)
		any.GET("/setting/svnInfo", api.SvnInfo)
		any.GET("/setting/version", api.VersionInfo)
		any.GET("/setting/checkUpdate", api.CheckUpdate)
	}

	// 管理端（管理员 / 子管理员）
	adm := r.Group("/api", middleware.Auth(auth.RoleAdmin, auth.RoleSubadmin))
	{
		// 仓库
		adm.GET("/rep/list", api.RepList)
		adm.POST("/rep/create", api.RepCreate)
		adm.POST("/rep/del", api.RepDel)
		adm.POST("/rep/updName", api.RepUpdName)
		adm.POST("/rep/updNote", api.RepUpdNote)
		adm.GET("/rep/detail", api.RepDetail)
		adm.POST("/rep/setUUID", api.RepSetUUID)
		adm.POST("/rep/syncSize", api.RepSyncSize)
		adm.POST("/rep/syncRev", api.RepSyncRev)
		adm.POST("/rep/syncAll", api.SyncReps)
		adm.GET("/rep/tree", api.RepTree)
		adm.GET("/rep/tree2", api.RepTree2)
		adm.GET("/rep/con", api.RepCon)
		adm.GET("/rep/userCon", api.RepUserCon)
		adm.POST("/rep/createFolder", api.RepCreateFolder)
		adm.GET("/rep/checkAuthz", api.CheckAuthz)
		adm.GET("/rep/userList2", api.SvnUserRepList2)
		adm.POST("/rep/verify", api.RepVerify)
		adm.POST("/rep/hotcopy", api.RepHotcopy)

		// 用户
		adm.GET("/user/list", api.UserList)
		adm.POST("/user/create", api.UserCreate)
		adm.POST("/user/del", api.UserDel)
		adm.POST("/user/updPass", api.UserUpdPass)
		adm.POST("/user/updStatus", api.UserUpdStatus)
		adm.POST("/user/updNote", api.UserUpdNote)
		adm.POST("/user/updMail", api.UserUpdMail)
		adm.GET("/user/scan", api.UserScan)
		adm.POST("/user/import", api.UserImport)
		adm.POST("/user/sync", api.SyncUsers)

		// 分组
		adm.GET("/group/list", api.GroupList)
		adm.POST("/group/create", api.GroupCreate)
		adm.POST("/group/del", api.GroupDel)
		adm.POST("/group/updName", api.GroupUpdName)
		adm.POST("/group/updNote", api.GroupUpdNote)
		adm.GET("/group/member", api.GroupMember)
		adm.POST("/group/updMember", api.GroupUpdMember)
		adm.POST("/group/sync", api.SyncGroups)

		// 权限（管理端全量；SVN 用户走 any 组的二次授权链）
		// （pri/create / pri/update / pri/delete 已上移至 any 组）

		// 数据同步
		adm.POST("/sync/all", api.SyncAll)

		// 备份
		adm.GET("/backup/list", api.BackupList)
		adm.POST("/backup/dump", api.BackupDump)
		adm.POST("/backup/del", api.BackupDel)
		adm.POST("/backup/load", api.BackupLoad)
		adm.POST("/backup/upload", api.BackupUpload)
		adm.GET("/backup/uploadInfo", api.BackupUploadInfo)
		adm.POST("/backup/uploadInit", api.BackupUploadInit)
		adm.POST("/backup/uploadPart", api.BackupUploadPart)
		adm.POST("/backup/uploadMerge", api.BackupUploadMerge)
		adm.POST("/backup/uploadAbort", api.BackupUploadAbort)
		adm.GET("/backup/uploadStatus", api.BackupUploadStatus)
		r.GET("/api/backup/download", middleware.Auth(), api.BackupDownload)

		// 钩子
		adm.GET("/hook/list", api.HookList)
		adm.POST("/hook/update", api.HookUpdate)
		adm.POST("/hook/del", api.HookDel)
		adm.GET("/hook/recommend", api.HookRecommend)

		// 日志
		adm.GET("/log/list", api.LogList)
		adm.GET("/log/types", api.LogTypes)
		adm.POST("/log/clear", api.LogClear)

		// 设置
		adm.GET("/setting/list", api.OptionList)
		adm.POST("/setting/update", api.OptionUpdate)
		adm.POST("/setting/svnserveStart", api.SvnserveStart)
		adm.POST("/setting/svnserveStop", api.SvnserveStop)
		adm.GET("/setting/dockerHost", api.DockerHostInfo)
		adm.POST("/setting/dockerHost", api.DockerHostUpdate)
		adm.GET("/setting/apacheInfo", api.ApacheInfo)
		adm.POST("/setting/subversionEnable", api.SubversionEnable)
		adm.POST("/setting/svnEnable", api.SvnEnable)
		adm.POST("/setting/httpPrefix", api.HttpPrefixUpdate)
		adm.POST("/setting/httpPort", api.HttpPortUpdate)
		adm.POST("/setting/svnservePort", api.SvnservePortUpdate)
		adm.POST("/setting/svnserveHost", api.SvnserveHostUpdate)
		adm.POST("/setting/usersource", api.UsersourceUpdate)
		adm.GET("/setting/saslInfo", api.SaslInfo)
		adm.POST("/setting/saslStart", api.SaslStart)
		adm.POST("/setting/saslStop", api.SaslStop)

		// 邮件与推送
		adm.GET("/mail/info", api.MailInfo)
		adm.POST("/mail/update", api.MailUpdate)
		adm.POST("/mail/test", api.MailTest)
		adm.GET("/push/info", api.PushInfo)
		adm.POST("/push/update", api.PushUpdate)
		adm.GET("/safe/info", api.SafeInfo)
		adm.POST("/safe/update", api.SafeUpdate)

		// LDAP
		adm.POST("/ldap/test", api.LdapTest)
		adm.POST("/ldap/users", api.LdapUsers)
		adm.POST("/ldap/groups", api.LdapGroups)
		adm.POST("/ldap/sync", api.LdapSync)

		// 定时任务
		adm.GET("/crond/list", api.CrondList)
		adm.POST("/crond/create", api.CrondCreate)
		adm.POST("/crond/update", api.CrondUpdate)
		adm.POST("/crond/del", api.CrondDel)
		adm.POST("/crond/trigger", api.CrondTrigger)

		// 任务控制
		adm.POST("/task/stop", api.TaskStop)
		adm.POST("/task/del", api.TaskDel)
		adm.POST("/task/clear", api.TaskClear)
	}

	// 仅超级管理员
	root := r.Group("/api", middleware.Auth(auth.RoleAdmin))
	{
		root.GET("/subadmin/list", api.SubadminList)
		root.POST("/subadmin/create", api.SubadminCreate)
		root.POST("/subadmin/del", api.SubadminDel)
		root.POST("/subadmin/updStatus", api.SubadminUpdStatus)
		root.POST("/subadmin/updPass", api.SubadminUpdPass)
		root.POST("/subadmin/updNote", api.SubadminUpdNote)
		root.GET("/subadmin/tree", api.SubadminTree)
		root.POST("/subadmin/updTree", api.SubadminUpdTree)
	}

	// 前端静态资源
	if webDir != "" {
		registerSPA(r, webDir)
	}
}

// registerSPA 挂载单页应用，未命中的路径回退到 index.html
func registerSPA(r *gin.Engine, webDir string) {
	index := filepath.Join(webDir, "index.html")
	r.Static("/assets", filepath.Join(webDir, "assets"))
	r.StaticFile("/favicon.ico", filepath.Join(webDir, "favicon.ico"))

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "status": false, "message": "接口不存在", "data": nil})
			return
		}
		c.File(index)
	})
}

// Engine 创建 gin 引擎
func Engine(webDir string) *gin.Engine {
	mode := config.C().Server.Mode
	if mode == "" {
		mode = gin.ReleaseMode
	}
	gin.SetMode(mode)
	r := gin.New()
	Register(r, webDir)
	return r
}
