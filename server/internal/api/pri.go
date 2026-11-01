// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package api

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"svnadmin/internal/auth"
	"svnadmin/internal/middleware"
	"svnadmin/internal/service"
	"svnadmin/internal/svn"
)

// PriList 路径权限列表
func PriList(c *gin.Context) {
	repName := c.Query("repName")
	priPath := c.Query("priPath")
	// 权限口径收敛：SVN 用户只能查看自己拥有权限的仓库路径（对齐原版的路径归属校验），
	// 管理员与子管理员不做限制。
	if u := middleware.Current(c); u != nil && u.Role == auth.RoleSvnUser {
		az, err := svn.LoadAuthz()
		if err != nil {
			fail(c, err)
			return
		}
		if svn.PermLevel(az.EffectivePerm(repName, priPath, u.Name)) <= 0 {
			fail(c, errors.New("无权限查看该路径的权限配置"))
			return
		}
	}
	res, err := service.GetRepPathAllPri(repName, priPath)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}

// priDelegateCheck SVN 用户通过二次授权链自助授权的校验：
// 该用户需对 (rep, path) 拥有写权限、该路径已开启二次授权、目标对象在白名单内。
// 管理员/子管理员直接放行。
func priDelegateCheck(c *gin.Context, repName, priPath, objectName string) bool {
	u := middleware.Current(c)
	if u == nil || u.Role != auth.RoleSvnUser {
		return true
	}
	az, err := svn.LoadAuthz()
	if err != nil {
		fail(c, err)
		return false
	}
	if svn.PermLevel(az.EffectivePerm(repName, priPath, u.Name)) < 2 {
		failMsg(c, "您对该路径没有写权限，无法授权")
		return false
	}
	if err := service.SecondPriDelegatable(u.Name, repName, priPath, objectName); err != nil {
		failMsg(c, err.Error())
		return false
	}
	return true
}

// PriCreate 新增权限；invert 为 true 时写回为 ~对象（除该对象外的所有人）
func PriCreate(c *gin.Context) {
	var req struct {
		RepName    string `json:"repName"`
		PriPath    string `json:"priPath"`
		ObjectType string `json:"objectType"`
		ObjectName string `json:"objectName"`
		Perm       string `json:"perm"`
		Invert     bool   `json:"invert"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if !priDelegateCheck(c, req.RepName, req.PriPath, req.ObjectName) {
		return
	}
	if err := service.CreateRepPathPri(req.RepName, req.PriPath, req.ObjectType, req.ObjectName,
		req.Perm, operator(c), req.Invert); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// PriUpdate 修改权限（含取反开关）
func PriUpdate(c *gin.Context) {
	var req struct {
		RepName    string `json:"repName"`
		PriPath    string `json:"priPath"`
		ObjectName string `json:"objectName"`
		Perm       string `json:"perm"`
		Invert     bool   `json:"invert"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if !priDelegateCheck(c, req.RepName, req.PriPath, req.ObjectName) {
		return
	}
	if err := service.UpdRepPathPri(req.RepName, req.PriPath, req.ObjectName, req.Perm, operator(c), req.Invert); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// PriDelete 删除权限
func PriDelete(c *gin.Context) {
	var req struct {
		RepName    string `json:"repName"`
		PriPath    string `json:"priPath"`
		ObjectName string `json:"objectName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if !priDelegateCheck(c, req.RepName, req.PriPath, req.ObjectName) {
		return
	}
	if err := service.DelRepPathPri(req.RepName, req.PriPath, req.ObjectName, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// ---------------------------------------------------------------------------
// 二次授权
// ---------------------------------------------------------------------------

// secondPriOwnerCheck SVN 用户角色时强制 owner 为当前登录用户（自助闭环）；
// 管理员/子管理员不受限。返回 (生效的 owner, 是否放行)。
func secondPriOwnerCheck(c *gin.Context, svnUserName string) (string, bool) {
	u := middleware.Current(c)
	if u == nil || u.Role != auth.RoleSvnUser {
		return svnUserName, true
	}
	if svnUserName != "" && svnUserName != u.Name {
		failMsg(c, "只能操作自己的二次授权配置")
		return "", false
	}
	if svnUserName == "" {
		return u.Name, true
	}
	return svnUserName, true
}

// SecondPriUpdate 开启 / 关闭二次授权
func SecondPriUpdate(c *gin.Context) {
	var req struct {
		SvnUserName string `json:"svnUserName"`
		RepName     string `json:"repName"`
		PriPath     string `json:"priPath"`
		Enable      bool   `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	owner, good := secondPriOwnerCheck(c, req.SvnUserName)
	if !good {
		return
	}
	if err := service.UpdSecondPri(owner, req.RepName, req.PriPath, req.Enable); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SecondPriObjectList 二次授权白名单列表
func SecondPriObjectList(c *gin.Context) {
	owner, good := secondPriOwnerCheck(c, c.Query("svnUserName"))
	if !good {
		return
	}
	list, err := service.SecondPriObjectList(owner, c.Query("repName"),
		c.Query("priPath"), c.Query("keyword"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// SecondPriObjectCreate 新增可下放对象
func SecondPriObjectCreate(c *gin.Context) {
	var req struct {
		SvnUserName string `json:"svnUserName"`
		RepName     string `json:"repName"`
		PriPath     string `json:"priPath"`
		ObjectType  string `json:"objectType"`
		ObjectName  string `json:"objectName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	owner, good := secondPriOwnerCheck(c, req.SvnUserName)
	if !good {
		return
	}
	if err := service.CreateSecondPriObject(owner, req.RepName, req.PriPath,
		req.ObjectType, req.ObjectName); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SecondPriObjectDel 删除可下放对象（兼容 query 与 body 两种传参）
func SecondPriObjectDel(c *gin.Context) {
	var req struct {
		SecondPriID int64 `json:"secondPriId"`
	}
	_ = c.ShouldBindJSON(&req)
	id := req.SecondPriID
	if id == 0 {
		id, _ = strconv.ParseInt(c.Query("secondPriId"), 10, 64)
	}
	if id == 0 {
		failMsg(c, "参数 secondPriId 缺失")
		return
	}
	// SVN 用户只能删除自己名下的白名单记录
	if u := middleware.Current(c); u != nil && u.Role == auth.RoleSvnUser {
		owner, err := service.SecondPriObjectOwner(id)
		if err != nil {
			failMsg(c, err.Error())
			return
		}
		if owner != u.Name {
			failMsg(c, "只能操作自己的二次授权白名单")
			return
		}
	}
	if err := service.DelSecondPriObject(id); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SecondPriUserPaths 查询用户已开启二次授权的路径
func SecondPriUserPaths(c *gin.Context) {
	owner, good := secondPriOwnerCheck(c, c.Query("svnUserName"))
	if !good {
		return
	}
	ok(c, service.SecondPriUserPaths(owner))
}
