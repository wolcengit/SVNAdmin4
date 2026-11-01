// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"svnadmin/internal/model"
	"svnadmin/internal/service"
)

// OptionList 配置项列表
func OptionList(c *gin.Context) {
	list, err := service.OptionList()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// OptionUpdate 修改配置
func OptionUpdate(c *gin.Context) {
	var req struct {
		OptionName  string `json:"optionName"`
		OptionValue string `json:"optionValue"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.SetOption(req.OptionName, req.OptionValue, ""); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SvnserveStart 启动 svnserve
func SvnserveStart(c *gin.Context) {
	if err := service.SvnserveStart(operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SvnserveStop 停止 svnserve
func SvnserveStop(c *gin.Context) {
	if err := service.SvnserveStop(operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// ---------------------------------------------------------------------------
// 日志
// ---------------------------------------------------------------------------

// LogList 日志列表（支持 typeName 精确过滤 + keyword 模糊 + beginTime/endTime 时间范围）
func LogList(c *gin.Context) {
	p := bindPage(c)
	total, list, err := service.LogList(p.Page, p.PageSize, c.Query("typeName"), p.Keyword, c.Query("beginTime"), c.Query("endTime"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, pageData(total, list))
}

// LogTypes 日志类型
func LogTypes(c *gin.Context) {
	types, err := service.LogTypes()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, types)
}

// LogClear 清空日志
func LogClear(c *gin.Context) {
	if err := service.LogClear(); err != nil {
		fail(c, err)
		return
	}
	ok(c, nil)
}

// ---------------------------------------------------------------------------
// 任务
// ---------------------------------------------------------------------------

// TaskList 任务列表；status 可传多次或用逗号分隔，为空表示全部
func TaskList(c *gin.Context) {
	p := bindPage(c)
	total, list, err := service.TaskList(p.Page, p.PageSize, intList(c, "status"), p.Keyword)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, pageData(total, list))
}

// TaskDetail 任务详情
func TaskDetail(c *gin.Context) {
	t, err := service.TaskByUnique(c.Query("taskUnique"))
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, t)
}

// TaskRun 当前运行任务 + 实时日志 + 排队情况
func TaskRun(c *gin.Context) {
	info, err := service.TaskRunning()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, info)
}

// TaskLog 任务日志
func TaskLog(c *gin.Context) {
	id, good := mustID(c, "taskId")
	if !good {
		return
	}
	log, trimmed, err := service.TaskLog(id)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, gin.H{"log": log, "trimed": trimmed})
}

// TaskStop 中断运行中的任务 / 取消排队任务（兼容 query 与 body 两种传参）
func TaskStop(c *gin.Context) {
	id := taskIDFrom(c)
	if id == 0 {
		failMsg(c, "参数 taskId 缺失")
		return
	}
	if err := service.TaskStop(id); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// TaskDel 删除任务记录（兼容 query 与 body 两种传参）
func TaskDel(c *gin.Context) {
	id := taskIDFrom(c)
	if id == 0 {
		failMsg(c, "参数 taskId 缺失")
		return
	}
	if err := service.TaskDel(id); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// taskIDFrom 从 query、param 或 JSON body 中提取 taskId
func taskIDFrom(c *gin.Context) int64 {
	var req struct {
		TaskID int64 `json:"taskId"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.TaskID != 0 {
		return req.TaskID
	}
	id, _ := strconv.ParseInt(c.Query("taskId"), 10, 64)
	return id
}

// TaskClear 清空历史任务
func TaskClear(c *gin.Context) {
	if err := service.TaskClear(); err != nil {
		fail(c, err)
		return
	}
	ok(c, nil)
}

// TaskTypes 任务类型与状态字典
func TaskTypes(c *gin.Context) {
	ok(c, gin.H{"types": service.TaskTypes(), "status": service.TaskStatusMap()})
}

// ---------------------------------------------------------------------------
// 定时任务
// ---------------------------------------------------------------------------

// CrondList 定时任务列表
func CrondList(c *gin.Context) {
	p := bindPage(c)
	// 兼容两种消费方式：显式携带 page/pageSize 时走服务端分页（对齐原版 GetCrontabList）；
	// 无分页参数时返回全量数组（现有前端全量拉取）
	if c.Query("page") == "" && c.Query("pageSize") == "" {
		p.Page, p.PageSize = 0, 0
	}
	list, err := service.CrondList(p.Page, p.PageSize, p.Keyword)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// CrondCreate 新建定时任务
func CrondCreate(c *gin.Context) {
	var cr model.Crond
	if err := c.ShouldBindJSON(&cr); err != nil {
		fail(c, err)
		return
	}
	if err := service.CrondCreate(&cr); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, cr)
}

// CrondUpdate 修改定时任务
func CrondUpdate(c *gin.Context) {
	var cr model.Crond
	if err := c.ShouldBindJSON(&cr); err != nil {
		fail(c, err)
		return
	}
	if cr.CrondID == 0 {
		failMsg(c, "参数 crondId 缺失")
		return
	}
	if err := service.CrondUpdate(&cr); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, cr)
}

// CrondDel 删除定时任务（兼容 query 与 body 两种传参）
func CrondDel(c *gin.Context) {
	var req struct {
		CrondID int64 `json:"crondId"`
	}
	_ = c.ShouldBindJSON(&req)
	id := req.CrondID
	if id == 0 {
		id, _ = strconv.ParseInt(c.Query("crondId"), 10, 64)
	}
	if id == 0 {
		failMsg(c, "参数 crondId 缺失")
		return
	}
	if err := service.CrondDel(id); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// CrondTrigger 手动立即执行一次（兼容 query 与 body 两种传参）
func CrondTrigger(c *gin.Context) {
	var req struct {
		CrondID int64 `json:"crondId"`
	}
	_ = c.ShouldBindJSON(&req)
	id := req.CrondID
	if id == 0 {
		id, _ = strconv.ParseInt(c.Query("crondId"), 10, 64)
	}
	if id == 0 {
		failMsg(c, "参数 crondId 缺失")
		return
	}
	t, err := service.CrondTrigger(id)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, t)
}

// CrondLog 定时任务执行日志
func CrondLog(c *gin.Context) {
	id, good := mustID(c, "crondId")
	if !good {
		return
	}
	log, err := service.CrondLog(id)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, gin.H{"log": log})
}

// CrondDict 定时任务字典（类型 / 周期 / 通知方式）
func CrondDict(c *gin.Context) {
	ok(c, gin.H{
		"types":      service.CrondTypes(),
		"cycleTypes": service.CycleTypes(),
		"status":     service.CrondStatus(),
		"notices": []map[string]interface{}{
			{"value": service.NoticeNone, "label": "通知关闭"},
			{"value": service.NoticeSucc, "label": "仅成功"},
			{"value": service.NoticeFail, "label": "仅失败"},
			{"value": service.NoticeAll, "label": "成功+失败"},
		},
	})
}

// ---------------------------------------------------------------------------
// 子管理员
// ---------------------------------------------------------------------------

// SubadminList 子管理员列表
func SubadminList(c *gin.Context) {
	p := bindPage(c)
	total, list, err := service.SubadminList(p.Page, p.PageSize, p.Keyword, c.Query("sortName"), c.Query("sortType"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, pageData(total, list))
}

// SubadminCreate 新建子管理员
func SubadminCreate(c *gin.Context) {
	var req struct {
		SubadminName  string   `json:"subadminName"`
		SubadminPass  string   `json:"subadminPass"`
		SubadminNote  string   `json:"subadminNote"`
		SubadminPhone string   `json:"subadminPhone"`
		SubadminEmail string   `json:"subadminEmail"`
		Functions     []string `json:"functions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.SubadminCreate(req.SubadminName, req.SubadminPass, req.SubadminNote,
		req.SubadminPhone, req.SubadminEmail, req.Functions, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SubadminDel 删除子管理员
func SubadminDel(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Query("subadminId"), 10, 64)
	if err := service.SubadminDel(id, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SubadminUpdStatus 修改状态
func SubadminUpdStatus(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Query("subadminId"), 10, 64)
	status, _ := strconv.Atoi(c.Query("status"))
	if err := service.SubadminUpdStatus(id, status, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SubadminUpdPass 修改密码
func SubadminUpdPass(c *gin.Context) {
	var req struct {
		SubadminID   int64  `json:"subadminId"`
		SubadminPass string `json:"subadminPass"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.SubadminUpdPass(req.SubadminID, req.SubadminPass, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SubadminUpdNote 修改备注
func SubadminUpdNote(c *gin.Context) {
	var req struct {
		SubadminID   int64  `json:"subadminId"`
		SubadminNote string `json:"subadminNote"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.SubadminUpdNote(req.SubadminID, req.SubadminNote, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// SubadminTree 获取子管理员功能权限树
func SubadminTree(c *gin.Context) {
	id, good := mustID(c, "subadminId")
	if !good {
		return
	}
	tree, err := service.SubadminTree(id)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, tree)
}

// SubadminUpdTree 保存子管理员功能权限树
func SubadminUpdTree(c *gin.Context) {
	var req struct {
		SubadminID int64         `json:"subadminId"`
		Tree       []interface{} `json:"tree"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.SubadminUpdTree(req.SubadminID, req.Tree, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}
