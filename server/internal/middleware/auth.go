// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"svnadmin/internal/auth"
	"svnadmin/internal/service"
	"svnadmin/internal/util"
)

// CurrentUserKey context 中存放当前用户的键
const CurrentUserKey = "currentUser"

// Token 从请求头中提取 token，兼容 Authorization / X-Token 两种写法
func Token(c *gin.Context) string {
	if t := c.GetHeader("X-Token"); t != "" {
		return t
	}
	h := c.GetHeader("Authorization")
	if h == "" {
		if t, err := c.Cookie("token"); err == nil && t != "" {
			return t
		}
		// 兜底：URL 查询参数（用于 window.open 下载等无法自定义请求头的场景）
		if t := c.Query("token"); t != "" {
			return t
		}
		return ""
	}
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return h
}

// Current 获取当前登录用户
func Current(c *gin.Context) *auth.User {
	v, ok := c.Get(CurrentUserKey)
	if !ok || v == nil {
		return nil
	}
	u, ok := v.(*auth.User)
	if !ok {
		return nil
	}
	return u
}

// Auth 鉴权中间件，roles 为空表示只要登录即可
func Auth(roles ...string) gin.HandlerFunc {
	allow := map[string]bool{}
	for _, r := range roles {
		allow[r] = true
	}
	return func(c *gin.Context) {
		token := Token(c)
		u, err := auth.Parse(token)
		if err != nil {
			util.Fail(c, util.CodeNoLogin, "登录状态已失效，请重新登录")
			c.Abort()
			return
		}
		if service.TokenBlacklisted(token) {
			util.Fail(c, util.CodeNoLogin, "登录状态已失效，请重新登录")
			c.Abort()
			return
		}
		if len(allow) > 0 && !allow[u.Role] {
			util.Fail(c, util.CodeNoPerm, "当前角色无权访问该接口")
			c.Abort()
			return
		}
		// 子管理员按功能权限树逐接口鉴权（原版 Base.php 的 functions 白名单语义）
		if u.Role == auth.RoleSubadmin {
			service.TouchOnline(u.ID)
			key := service.FuncKey(c.FullPath())
			if !service.SubadminCanAccess(u.ID, key) {
				util.Fail(c, util.CodeNoPerm, "当前账号未被授予该功能权限")
				c.Abort()
				return
			}
		}
		c.Set(CurrentUserKey, u)
		c.Next()
	}
}

// CORS 跨域中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", c.GetHeader("Origin"))
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,X-Token,Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
