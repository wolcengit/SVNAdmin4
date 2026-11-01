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

// UserList 用户列表
func UserList(c *gin.Context) {
	p := bindPage(c)
	total, list, err := service.UserList(p.Page, p.PageSize, p.Keyword, p.Status, c.Query("sortName"), c.Query("sortType"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, pageData(total, list))
}

// UserAll 全部用户
func UserAll(c *gin.Context) {
	list, err := service.UserAll()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// UserCreate 新建用户
func UserCreate(c *gin.Context) {
	var req struct {
		SvnUserName string `json:"svnUserName"`
		SvnUserPass string `json:"svnUserPass"`
		SvnUserNote string `json:"svnUserNote"`
		SvnUserMail string `json:"svnUserMail"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.CreateUser(req.SvnUserName, req.SvnUserPass, req.SvnUserNote, req.SvnUserMail, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// UserDel 删除用户
func UserDel(c *gin.Context) {
	var req struct {
		SvnUserName string `json:"svnUserName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.DelUser(req.SvnUserName, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// UserUpdPass 修改密码
func UserUpdPass(c *gin.Context) {
	var req struct {
		SvnUserName string `json:"svnUserName"`
		SvnUserPass string `json:"svnUserPass"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdUserPass(req.SvnUserName, req.SvnUserPass, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// UserUpdStatus 启用/禁用
func UserUpdStatus(c *gin.Context) {
	var req struct {
		SvnUserName   string `json:"svnUserName"`
		SvnUserStatus int    `json:"svnUserStatus"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdUserStatus(req.SvnUserName, req.SvnUserStatus, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// UserUpdNote 修改备注
func UserUpdNote(c *gin.Context) {
	var req struct {
		SvnUserName string `json:"svnUserName"`
		SvnUserNote string `json:"svnUserNote"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdUserNote(req.SvnUserName, req.SvnUserNote, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// UserUpdMail 修改用户邮箱
func UserUpdMail(c *gin.Context) {
	var req struct {
		SvnUserName string `json:"svnUserName"`
		SvnUserMail string `json:"svnUserMail"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdUserMail(req.SvnUserName, req.SvnUserMail); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// UserScan 扫描 passwd 文件中未管理的用户
func UserScan(c *gin.Context) {
	list, err := service.UserScan()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// UserImport 批量导入
func UserImport(c *gin.Context) {
	var req struct {
		Names []string `json:"names"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	n, err := service.UserImport(req.Names, operator(c))
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, gin.H{"count": n})
}

// ---------------------------------------------------------------------------
// 分组
// ---------------------------------------------------------------------------

// GroupList 分组列表
func GroupList(c *gin.Context) {
	p := bindPage(c)
	total, list, err := service.GroupList(p.Page, p.PageSize, p.Keyword, c.Query("sortName"), c.Query("sortType"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, pageData(total, list))
}

// GroupAll 全部分组名
func GroupAll(c *gin.Context) {
	list, err := service.GroupAll()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// GroupCreate 新建分组
func GroupCreate(c *gin.Context) {
	var req struct {
		SvnGroupName string `json:"svnGroupName"`
		SvnGroupNote string `json:"svnGroupNote"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.CreateGroup(req.SvnGroupName, req.SvnGroupNote, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// GroupDel 删除分组
func GroupDel(c *gin.Context) {
	var req struct {
		SvnGroupName string `json:"svnGroupName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.DelGroup(req.SvnGroupName, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// GroupUpdName 修改分组名
func GroupUpdName(c *gin.Context) {
	var req struct {
		OldName string `json:"oldName"`
		NewName string `json:"newName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdGroupName(req.OldName, req.NewName, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// GroupUpdNote 修改分组备注
func GroupUpdNote(c *gin.Context) {
	var req struct {
		SvnGroupName string `json:"svnGroupName"`
		SvnGroupNote string `json:"svnGroupNote"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdGroupNote(req.SvnGroupName, req.SvnGroupNote, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// GroupMember 分组成员
func GroupMember(c *gin.Context) {
	res, err := service.GetGroupMember(c.Query("svnGroupName"), c.Query("searchKeyword"))
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, res)
}

// GroupUpdMember 更新分组成员
func GroupUpdMember(c *gin.Context) {
	var req struct {
		SvnGroupName string   `json:"svnGroupName"`
		Users        []string `json:"users"`
		Groups       []string `json:"groups"`
		Aliases      []string `json:"aliases"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdGroupMember(req.SvnGroupName, req.Users, req.Groups, req.Aliases, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// ---------------------------------------------------------------------------
// 别名
// ---------------------------------------------------------------------------

// AliaseList 别名列表（支持 searchKeyword 过滤）
func AliaseList(c *gin.Context) {
	res, err := service.AliaseList(c.Query("searchKeyword"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}
