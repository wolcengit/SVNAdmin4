// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package service

import (
	"svnadmin/internal/model"
	"svnadmin/internal/store"
)

// AddLog 记录一条操作日志
func AddLog(typeName, content, userName string) {
	store.DB().Create(&model.Log{
		LogTypeName:    typeName,
		LogContent:     content,
		LogAddUserName: userName,
		LogAddTime:     model.Now(),
	})
}

// LogList 分页查询日志
func LogList(page, pageSize int, typeName, keyword, beginTime, endTime string) (int64, []model.Log, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 20
	}
	q := store.DB().Model(&model.Log{})
	if typeName != "" {
		q = q.Where("log_type_name = ?", typeName)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("log_content LIKE ? OR log_add_user_name LIKE ?", like, like)
	}
	// 时间范围（log_add_time 为 "2006-01-02 15:04:05" 字符串，字典序即时间序）
	if beginTime != "" {
		q = q.Where("log_add_time >= ?", beginTime)
	}
	if endTime != "" {
		q = q.Where("log_add_time <= ?", endTime)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	var list []model.Log
	if err := q.Order("log_id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return 0, nil, err
	}
	return total, list, nil
}

// LogTypes 返回全部日志类型
func LogTypes() ([]string, error) {
	var types []string
	err := store.DB().Model(&model.Log{}).Distinct().Pluck("log_type_name", &types).Error
	return types, err
}

// LogClear 清空日志
func LogClear() error {
	return store.DB().Where("1 = 1").Delete(&model.Log{}).Error
}
