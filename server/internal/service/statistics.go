// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package service

import (
	"os"

	"svnadmin/internal/config"
	"svnadmin/internal/model"
	"svnadmin/internal/store"
	"svnadmin/internal/svn"
	"svnadmin/internal/util"
)

// Statistics 系统总览统计
func Statistics() (map[string]interface{}, error) {
	var repCount, userCount, userEnabled, logCount int64
	store.DB().Model(&model.SvnRep{}).Count(&repCount)
	store.DB().Model(&model.SvnUser{}).Count(&userCount)
	store.DB().Model(&model.SvnUser{}).Where("svn_user_status = 1").Count(&userEnabled)
	store.DB().Model(&model.Log{}).Count(&logCount)

	groupCount := 0
	if authz, err := svn.LoadAuthz(); err == nil {
		groupCount = len(authz.GroupOrder)
	}

	var totalSize int64
	var reps []model.SvnRep
	_ = store.DB().Find(&reps).Error
	for _, r := range reps {
		totalSize += r.RepSize
	}

	// 备份文件数量与占用（对齐原版 GetStatisticsInfo 的 backupCount / backupSize）
	backupCount := int64(0)
	var backupSize int64
	if entries, err := os.ReadDir(config.C().SVN.BackupBasePath()); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			backupCount++
			backupSize += info.Size()
		}
	}

	// 管理员 / 子管理员 / 别名数量（原版 GetStatisticsInfo 维度）
	var adminCount, subadminCount int64
	store.DB().Model(&model.AdminUser{}).Count(&adminCount)
	store.DB().Model(&model.Subadmin{}).Count(&subadminCount)
	aliaseCount := 0
	if authz, err := svn.LoadAuthz(); err == nil {
		aliaseCount = len(authz.Aliases)
	}

	return map[string]interface{}{
		"os":            osName(),
		"repCount":      repCount,
		"repSize":       util.FmtSize(totalSize),
		"userCount":     userCount,
		"userEnabled":   userEnabled,
		"groupCount":    groupCount,
		"aliaseCount":   aliaseCount,
		"adminCount":    adminCount,
		"subadminCount": subadminCount,
		"logCount":      logCount,
		"backupCount":   backupCount,
		"backupSize":    util.FmtSize(backupSize),
		"totalSize":     totalSize,
		"totalSizeHr":   util.FmtSize(totalSize),
		"svnVersion":    svn.Version(),
		"svnserve":      SvnserveStatus(),
	}, nil
}

// RepSizeRank 仓库体积排行
func RepSizeRank(limit int) ([]model.SvnRep, error) {
	if limit <= 0 {
		limit = 10
	}
	var list []model.SvnRep
	err := store.DB().Order("rep_size DESC").Limit(limit).Find(&list).Error
	return list, err
}
