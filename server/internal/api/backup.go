// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"svnadmin/internal/service"
)

// BackupList 备份文件列表
func BackupList(c *gin.Context) {
	list, err := service.BackupList(c.Query("repName"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// BackupDump 立即备份
func BackupDump(c *gin.Context) {
	var req struct {
		RepName     string `json:"repName"`
		RevRange    string `json:"revRange"`
		Incremental bool   `json:"incremental"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	task, err := service.StartDump(req.RepName, req.RevRange, req.Incremental)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, task)
}

// BackupDel 删除备份
func BackupDel(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.DelBackup(req.Name, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// BackupDownload 下载备份文件
func BackupDownload(c *gin.Context) {
	name := c.Query("name")
	p, err := service.BackupPath(name)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	f, err := os.Open(p)
	if err != nil {
		failMsg(c, "备份文件不存在")
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(p))
	c.Header("Content-Type", "application/octet-stream")
	http.ServeContent(c.Writer, c.Request, filepath.Base(p), stat.ModTime(), f)
}

// BackupUpload 上传备份文件（整文件，兼容小文件场景）
func BackupUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		failMsg(c, "未获取到上传文件")
		return
	}
	name, err := service.SaveBackupFile(file)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, gin.H{"name": name})
}

// BackupUploadInfo 上传能力信息（分片大小 / 允许后缀）
func BackupUploadInfo(c *gin.Context) {
	ok(c, service.UploadInfo())
}

// BackupUploadInit 初始化分片上传
func BackupUploadInit(c *gin.Context) {
	var req struct {
		FileName string `json:"fileName"`
		Md5      string `json:"md5"`
		Total    int    `json:"numBlobTotal"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	s, err := service.UploadInit(req.FileName, req.Md5, req.Total)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, s)
}

// BackupUploadPart 上传单个分片
func BackupUploadPart(c *gin.Context) {
	id := c.PostForm("uploadId")
	if id == "" {
		id = c.Query("uploadId")
	}
	idxStr := c.PostForm("index")
	if idxStr == "" {
		idxStr = c.Query("index")
	}
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		failMsg(c, "分片序号不合法")
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		failMsg(c, "未获取到分片数据")
		return
	}
	f, err := fh.Open()
	if err != nil {
		fail(c, err)
		return
	}
	defer f.Close()
	s, err := service.UploadChunk(id, idx, f)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, gin.H{"uploadId": s.ID, "received": len(s.Received), "total": s.Total})
}

// BackupUploadMerge 合并分片
func BackupUploadMerge(c *gin.Context) {
	var req struct {
		UploadId      string `json:"uploadId"`
		DeleteOnMerge bool   `json:"deleteOnMerge"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	name, err := service.UploadMerge(req.UploadId, req.DeleteOnMerge)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, gin.H{"name": name})
}

// BackupUploadAbort 取消上传并清理临时分片
func BackupUploadAbort(c *gin.Context) {
	id := c.Query("uploadId")
	if id == "" {
		failMsg(c, "参数 uploadId 缺失")
		return
	}
	service.UploadAbort(id)
	ok(c, nil)
}

// BackupUploadStatus 查询上传进度
func BackupUploadStatus(c *gin.Context) {
	s, found := service.UploadSessionGet(c.Query("uploadId"))
	if !found {
		failMsg(c, "上传会话不存在")
		return
	}
	ok(c, s)
}

// BackupLoad 从备份导入
func BackupLoad(c *gin.Context) {
	var req struct {
		RepName  string `json:"repName"`
		FileName string `json:"fileName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	task, err := service.StartLoad(req.RepName, req.FileName)
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, task)
}

// ---------------------------------------------------------------------------
// 钩子
// ---------------------------------------------------------------------------

// HookList 仓库钩子列表
func HookList(c *gin.Context) {
	list, err := service.RepHooks(c.Query("repName"))
	if err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, list)
}

// HookUpdate 修改钩子
func HookUpdate(c *gin.Context) {
	var req struct {
		RepName  string `json:"repName"`
		HookName string `json:"hookName"`
		Content  string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.UpdRepHook(req.RepName, req.HookName, req.Content, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// HookDel 删除钩子
func HookDel(c *gin.Context) {
	var req struct {
		RepName  string `json:"repName"`
		HookName string `json:"hookName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, err)
		return
	}
	if err := service.DelRepHook(req.RepName, req.HookName, operator(c)); err != nil {
		failMsg(c, err.Error())
		return
	}
	ok(c, nil)
}

// HookRecommend 推荐钩子
func HookRecommend(c *gin.Context) {
	ok(c, service.RecommendHooks())
}
