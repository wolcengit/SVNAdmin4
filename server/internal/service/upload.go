// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package service

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"svnadmin/internal/config"
	"svnadmin/internal/util"
)

// 允许上传的备份文件后缀
var allowBackupExt = []string{".dump"}

// UploadSession 分片上传会话
type UploadSession struct {
	ID         string    `json:"uploadId"`
	FileName   string    `json:"fileName"`
	MD5        string    `json:"md5"`
	Total      int       `json:"numBlobTotal"`
	Received   []int     `json:"numBlobReceived"`
	Size       int64     `json:"size"`
	Dir        string    `json:"-"`
	CreateTime time.Time `json:"createTime"`
}

var (
	uploadMu      sync.Mutex
	uploadSession = map[string]*UploadSession{}
)

// SaveBackupFile 保存整文件上传的备份文件
func SaveBackupFile(fh *multipart.FileHeader) (string, error) {
	if !validBackupExt(fh.Filename) {
		return "", errors.New("仅支持上传 .dump 备份文件")
	}
	name := dedupName(safeFileName(fh.Filename))
	target, err := BackupPath(name)
	if err != nil {
		return "", err
	}
	src, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()
	dst, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(target)
		return "", err
	}
	return name, nil
}

// UploadInfo 返回上传能力信息
func UploadInfo() map[string]interface{} {
	return map[string]interface{}{
		"chunkSize":    8 << 20, // 前端建议分片大小 8MB
		"allowExt":     allowBackupExt,
		"supportChunk": true,
		"maxSize":      0, // 0 表示不限制
	}
}

// UploadInit 初始化分片上传
func UploadInit(fileName, md5sum string, total int) (*UploadSession, error) {
	if !validBackupExt(fileName) {
		return nil, errors.New("仅支持上传 .dump 备份文件")
	}
	if total <= 0 {
		return nil, errors.New("分片数量不合法")
	}
	if total > 20000 {
		return nil, errors.New("分片数量过多")
	}
	id := util.RandomHex(16)
	dir := filepath.Join(config.C().SVN.TempPath(), "upload", id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	s := &UploadSession{
		ID:         id,
		FileName:   safeFileName(fileName),
		MD5:        strings.ToLower(strings.TrimSpace(md5sum)),
		Total:      total,
		Received:   []int{},
		Dir:        dir,
		CreateTime: time.Now(),
	}
	uploadMu.Lock()
	uploadSession[id] = s
	uploadMu.Unlock()
	return s, nil
}

// UploadChunk 写入一个分片
func UploadChunk(id string, index int, src io.Reader) (*UploadSession, error) {
	uploadMu.Lock()
	s, ok := uploadSession[id]
	uploadMu.Unlock()
	if !ok {
		return nil, errors.New("上传会话不存在或已过期，请重新上传")
	}
	if index < 0 || index >= s.Total {
		return nil, errors.New("分片序号不合法")
	}
	dst := filepath.Join(s.Dir, fmt.Sprintf("part_%06d", index))
	f, err := os.Create(dst)
	if err != nil {
		return nil, err
	}
	n, err := io.Copy(f, src)
	f.Close()
	if err != nil {
		_ = os.Remove(dst)
		return nil, err
	}
	uploadMu.Lock()
	s.Size += n
	exist := false
	for _, i := range s.Received {
		if i == index {
			exist = true
			break
		}
	}
	if !exist {
		s.Received = append(s.Received, index)
		sort.Ints(s.Received)
	}
	uploadMu.Unlock()
	return s, nil
}

// UploadMerge 合并分片并校验 MD5
func UploadMerge(id string, deleteOnMerge bool) (string, error) {
	uploadMu.Lock()
	s, ok := uploadSession[id]
	uploadMu.Unlock()
	if !ok {
		return "", errors.New("上传会话不存在或已过期")
	}
	if len(s.Received) < s.Total {
		return "", fmt.Errorf("分片未上传完整：已收到 %d / %d", len(s.Received), s.Total)
	}
	target, err := BackupPath(dedupName(s.FileName))
	if err != nil {
		return "", err
	}
	out, err := os.Create(target)
	if err != nil {
		return "", err
	}
	hash := md5.New()
	w := io.MultiWriter(out, hash)
	for i := 0; i < s.Total; i++ {
		part := filepath.Join(s.Dir, fmt.Sprintf("part_%06d", i))
		f, err := os.Open(part)
		if err != nil {
			out.Close()
			_ = os.Remove(target)
			return "", fmt.Errorf("缺少第 %d 个分片", i+1)
		}
		if _, err := io.Copy(w, f); err != nil {
			f.Close()
			out.Close()
			_ = os.Remove(target)
			return "", err
		}
		f.Close()
	}
	out.Close()

	if s.MD5 != "" {
		got := hex.EncodeToString(hash.Sum(nil))
		if !strings.EqualFold(got, s.MD5) {
			_ = os.Remove(target)
			return "", fmt.Errorf("文件校验失败，MD5 不一致（期望 %s，实际 %s）", s.MD5, got)
		}
	}
	if deleteOnMerge {
		_ = os.RemoveAll(s.Dir)
	}
	uploadMu.Lock()
	delete(uploadSession, id)
	uploadMu.Unlock()
	return filepath.Base(target), nil
}

// UploadAbort 取消上传并清理临时文件
func UploadAbort(id string) {
	uploadMu.Lock()
	s, ok := uploadSession[id]
	delete(uploadSession, id)
	uploadMu.Unlock()
	if ok {
		_ = os.RemoveAll(s.Dir)
	}
}

// UploadSessionGet 查询上传进度
func UploadSessionGet(id string) (*UploadSession, bool) {
	uploadMu.Lock()
	defer uploadMu.Unlock()
	s, ok := uploadSession[id]
	return s, ok
}

// CleanUploadSessions 清理超过 12 小时的残留会话（对外暴露供定时调用）
func CleanUploadSessions() {
	uploadMu.Lock()
	var expired []*UploadSession
	for id, s := range uploadSession {
		if time.Since(s.CreateTime) > 12*time.Hour {
			expired = append(expired, s)
			delete(uploadSession, id)
		}
	}
	uploadMu.Unlock()
	for _, s := range expired {
		_ = os.RemoveAll(s.Dir)
	}
}

func validBackupExt(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range allowBackupExt {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func safeFileName(name string) string {
	name = strings.ReplaceAll(filepath.Base(name), "\\", "/")
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		name = "upload_" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".dump"
	}
	return name
}

// dedupName 目标文件已存在时追加序号
func dedupName(name string) string {
	dir := config.C().SVN.BackupBasePath()
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		return name
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; i < 1000; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		if _, err := os.Stat(filepath.Join(dir, candidate)); err != nil {
			return candidate
		}
	}
	return fmt.Sprintf("%s_%d%s", base, time.Now().UnixNano(), ext)
}
