// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package util

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 状态码
const (
	CodeSuccess = 200
	CodeError   = 401
	CodeNoLogin = 402
	CodeNoPerm  = 403
)

// Result 统一响应结构，与前端约定保持一致
type Result struct {
	Code    int         `json:"code"`
	Status  bool        `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// Page 分页响应
type Page struct {
	Total int64       `json:"total"`
	List  interface{} `json:"list"`
}

// OK 成功响应
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Result{Code: CodeSuccess, Status: true, Message: "success", Data: data})
}

// OkMsg 成功响应带提示
func OkMsg(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, Result{Code: CodeSuccess, Status: true, Message: msg, Data: data})
}

// Fail 失败响应
func Fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Result{Code: code, Status: false, Message: msg, Data: nil})
}

// FailErr 失败响应（服务端错误）
func FailErr(c *gin.Context, err error) {
	Fail(c, CodeError, errMsg(err))
}

// FailMsg 失败响应（业务错误）
func FailMsg(c *gin.Context, msg string) {
	Fail(c, CodeError, msg)
}

// BadRequest 参数错误
func BadRequest(c *gin.Context, msg string) {
	Fail(c, CodeError, msg)
}

func errMsg(err error) string {
	if err == nil {
		return "未知错误"
	}
	return err.Error()
}

// PageResult 构造分页数据
func PageResult(total int64, list interface{}) Page {
	return Page{Total: total, List: list}
}
