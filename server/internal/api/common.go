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
	"svnadmin/internal/auth"
	"svnadmin/internal/middleware"
	"svnadmin/internal/service"
)

// operator 取当前操作者名称，用于写日志
func operator(c *gin.Context) string {
	if u := middleware.Current(c); u != nil {
		return u.Name
	}
	return "system"
}

// requireAdmin 校验当前用户具备管理权限
func requireAdmin(c *gin.Context) bool {
	u := middleware.Current(c)
	if u == nil || !u.CanManage() {
		return false
	}
	return true
}

type loginReq struct {
	UserName string `json:"userName"`
	Password string `json:"password"`
	Code     string `json:"code"`
	UUID     string `json:"uuid"`
	Role     string `json:"role"` // 可选：admin / subadmin / svnuser，为空时自动识别
}

// Login 登录
func Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	res, err := service.Login(req.UserName, req.Password, req.Code, req.UUID, req.Role)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, res)
}

// Logout 注销
func Logout(c *gin.Context) {
	if err := service.Logout(middleware.Token(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// VerifyCode 获取验证码
func VerifyCode(c *gin.Context) {
	uuid, svg, err := service.NewVerifyCode()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"uuid": uuid, "svg": svg})
}

// VerifyOption 登录页配置
func VerifyOption(c *gin.Context) {
	ok(c, service.VerifyOption())
}

// UserInfo 当前登录用户信息
func UserInfo(c *gin.Context) {
	u := middleware.Current(c)
	if u == nil {
		failMsg(c, "未登录")
		return
	}
	res := gin.H{"userId": u.ID, "userName": u.Name, "role": u.Role}
	if u.Role == auth.RoleSubadmin {
		res["functions"] = service.KeysOfFuncs(u.ID)
	}
	ok(c, res)
}

// SvnserveStatus svnserve 运行状态
func SvnserveStatus(c *gin.Context) {
	ok(c, gin.H{"status": service.SvnserveStatus()})
}

// Statistics 首页统计
func Statistics(c *gin.Context) {
	res, err := service.Statistics()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}

// UpdatePass 修改自身密码
func UpdatePass(c *gin.Context) {
	var req struct {
		OldPass string `json:"oldPass"`
		NewPass string `json:"newPass"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.AdminUpdPass(operator(c), req.OldPass, req.NewPass); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}
