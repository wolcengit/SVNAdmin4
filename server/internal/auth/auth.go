// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package auth

import (
	"errors"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"svnadmin/internal/config"
)

// 角色定义
const (
	RoleAdmin   = "admin"
	RoleSubadmin = "subadmin"
	RoleSvnUser = "svnuser"
)

// User 登录用户上下文
type User struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Token string `json:"token"`
}

// IsAdmin 是否为超级管理员
func (u *User) IsAdmin() bool { return u != nil && u.Role == RoleAdmin }

// CanManage 是否具备管理权限（超级管理员或子管理员）
func (u *User) CanManage() bool {
	return u != nil && (u.Role == RoleAdmin || u.Role == RoleSubadmin)
}

type claims struct {
	UID   int64  `json:"uid"`
	Name  string `json:"uname"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

// Generate 签发 token
func Generate(u *User) (string, error) {
	now := time.Now()
	exp := now.Add(time.Duration(config.C().Server.TokenExpire) * time.Second)
	c := claims{
		UID:   u.ID,
		Name:  u.Name,
		Role:  u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "svnadmin",
			Subject:   u.Name,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        u.Token,
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(config.C().Security.JWTSecret))
}

// Parse 解析 token
func Parse(tokenStr string) (*User, error) {
	if tokenStr == "" {
		return nil, errors.New("token 为空")
	}
	var c claims
	t, err := jwt.ParseWithClaims(tokenStr, &c, func(*jwt.Token) (interface{}, error) {
		return []byte(config.C().Security.JWTSecret), nil
	})
	if err != nil {
		return nil, errors.New("token 无效或已过期")
	}
	if !t.Valid {
		return nil, errors.New("token 无效")
	}
	return &User{ID: c.UID, Name: c.Name, Role: c.Role, Token: tokenStr}, nil
}

// ---------------------------------------------------------------------------
// 注销黑名单：保存尚未过期但已主动注销的 token
// ---------------------------------------------------------------------------

var (
	blackMu sync.RWMutex
	black   = map[string]time.Time{}
)

// Blacklist 将 token 加入黑名单
func Blacklist(token string, exp time.Time) {
	blackMu.Lock()
	defer blackMu.Unlock()
	black[token] = exp
	cleanupLocked()
}

// IsBlacklisted 判断 token 是否已被注销
func IsBlacklisted(token string) bool {
	blackMu.RLock()
	_, ok := black[token]
	blackMu.RUnlock()
	return ok
}

// cleanupLocked 清理已过期的黑名单项，调用方需持有写锁
func cleanupLocked() {
	now := time.Now()
	for k, v := range black {
		if v.Before(now) {
			delete(black, k)
		}
	}
}
