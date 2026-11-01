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
	"strings"

	"github.com/gin-gonic/gin"
	"svnadmin/internal/util"
)

// ok 成功响应
func ok(c *gin.Context, data interface{}) { util.OK(c, data) }

// fail 服务端错误响应
func fail(c *gin.Context, err error) { util.FailErr(c, err) }

// failMsg 业务错误响应
func failMsg(c *gin.Context, msg string) { util.FailMsg(c, msg) }

// page 分页参数
type page struct {
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"pageSize" json:"pageSize"`
	Keyword  string `form:"keyword" json:"keyword"`
	Status   int    `form:"status" json:"status"`
}

// bindPage 解析分页参数
func bindPage(c *gin.Context) page {
	var p page
	_ = c.ShouldBind(&p)
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 20
	}
	return p
}

// pageData 构造分页响应
func pageData(total int64, list interface{}) gin.H {
	return gin.H{"total": total, "list": list}
}

// intList 解析形如 "1,2,3" 或重复参数的状态列表
func intList(c *gin.Context, key string) []int {
	var out []int
	for _, v := range c.QueryArray(key) {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if n, err := strconv.Atoi(p); err == nil {
				out = append(out, n)
			}
		}
	}
	return out
}

// queryID 解析路径或查询参数中的整型 ID
func queryID(c *gin.Context, key string) (int64, error) {
	v := c.Query(key)
	if v == "" {
		v = c.Param(key)
	}
	if v == "" {
		return 0, errors.New("参数 " + key + " 缺失")
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, errors.New("参数 " + key + " 不合法")
	}
	return id, nil
}

// mustID 解析 ID，失败时直接返回错误响应并置 ok=false
func mustID(c *gin.Context, key string) (int64, bool) {
	id, err := queryID(c, key)
	if err != nil {
		failMsg(c, err.Error())
		return 0, false
	}
	return id, true
}
