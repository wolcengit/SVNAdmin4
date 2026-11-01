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

// RepList 仓库列表（支持 sortName / sortType 排序）
func RepList(c *gin.Context) {
	p := bindPage(c)
	total, list, err := service.RepList(p.Page, p.PageSize, p.Keyword, c.Query("sortName"), c.Query("sortType"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, pageData(total, list))
}

// RepAll 全部仓库
func RepAll(c *gin.Context) {
	list, err := service.RepAll()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// RepCreate 新建仓库
func RepCreate(c *gin.Context) {
	var req struct {
		RepName    string `json:"repName"`
		RepNote    string `json:"repNote"`
		InitStruct string `json:"initStruct"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.CreateRep(req.RepName, req.RepNote, req.InitStruct, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// RepDel 删除仓库
func RepDel(c *gin.Context) {
	var req struct {
		RepName string `json:"repName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.DelRep(req.RepName, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// RepUpdName 修改仓库名
func RepUpdName(c *gin.Context) {
	var req struct {
		OldName string `json:"oldName"`
		NewName string `json:"newName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdRepName(req.OldName, req.NewName, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// RepUpdNote 修改仓库备注
func RepUpdNote(c *gin.Context) {
	var req struct {
		RepName string `json:"repName"`
		RepNote string `json:"repNote"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdRepNote(req.RepName, req.RepNote, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// RepDetail 仓库属性
func RepDetail(c *gin.Context) {
	res, err := service.GetRepDetail(c.Query("repName"))
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, res)
}

// RepSetUUID 重设 UUID（uuid 可选，为空则随机）
func RepSetUUID(c *gin.Context) {
	var req struct {
		RepName string `json:"repName"`
		UUID    string `json:"uuid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	uuid, err := service.ResetUUID(req.RepName, req.UUID, operator(c))
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, gin.H{"uuid": uuid})
}

// RepSyncSize 同步仓库体积
func RepSyncSize(c *gin.Context) {
	size, err := service.SyncRepSize(c.Query("repName"))
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, gin.H{"size": size})
}

// RepSyncRev 同步仓库版本数
func RepSyncRev(c *gin.Context) {
	rev, err := service.SyncRepRev(c.Query("repName"))
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, gin.H{"rev": rev})
}

// RepTree 仓库目录树
func RepTree(c *gin.Context) {
	tree, err := service.GetRepTree(c.Query("repName"), c.Query("path"))
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, tree)
}

// RepCon 目录内容
func RepCon(c *gin.Context) {
	list, err := service.GetRepCon(c.Query("repName"), c.Query("path"))
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, list)
}

// RepCreateFolder 在线创建目录
func RepCreateFolder(c *gin.Context) {
	var req struct {
		RepName string `json:"repName"`
		Path    string `json:"path"`
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.CreateRepFolder(req.RepName, req.Path, req.Message, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// RepCheckout 检出地址
func RepCheckout(c *gin.Context) {
	ok(c, service.Checkout(c.Query("repName"), c.Request.Host))
}

// CheckAuthz 校验 authz
func CheckAuthz(c *gin.Context) {
	if err := service.CheckAuthz(); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SvnUserRepList SVN 用户可访问的仓库
func SvnUserRepList(c *gin.Context) {
	list, err := service.GetSvnUserRepList(operator(c))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// SvnUserRepList2 管理员查看指定用户的仓库权限
func SvnUserRepList2(c *gin.Context) {
	list, err := service.GetSvnUserRepList(c.Query("svnUserName"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// ---------------------------------------------------------------------------
// SVN 用户视角浏览（走 svn 协议真实鉴权）
// ---------------------------------------------------------------------------

// RepUserCon SVN 用户浏览仓库内容，只能看到有权限的路径
func RepUserCon(c *gin.Context) {
	res, err := service.GetUserRepCon(c.Query("repName"), c.Query("path"), c.Query("svnUserName"))
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, res)
}

// RepTree2 SVN 用户视角的目录树
func RepTree2(c *gin.Context) {
	first := c.Query("first") == "1" || c.Query("first") == "true"
	tree, err := service.GetRepTree2(c.Query("repName"), c.Query("path"), c.Query("svnUserName"), first)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, tree)
}

// ---------------------------------------------------------------------------
// 仓库检查与热拷贝
// ---------------------------------------------------------------------------

// RepVerify 启动仓库检查任务
func RepVerify(c *gin.Context) {
	var req struct {
		RepName string `json:"repName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	task, err := service.StartVerify(req.RepName)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, task)
}

// RepHotcopy 启动仓库热拷贝任务
func RepHotcopy(c *gin.Context) {
	var req struct {
		RepName     string `json:"repName"`
		Incremental bool   `json:"incremental"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	task, err := service.StartHotcopy(req.RepName, req.Incremental)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, task)
}

// ShellTask 启动自定义脚本任务
func ShellTask(c *gin.Context) {
	var req struct {
		Name   string `json:"name"`
		Script string `json:"script"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	task, err := service.StartShell(req.Name, req.Script)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, task)
}
