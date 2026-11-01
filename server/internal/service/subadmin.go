// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package service

import (
	"errors"
	"fmt"
	"regexp"

	"gorm.io/gorm"
	"svnadmin/internal/model"
	"svnadmin/internal/store"
	"svnadmin/internal/util"
	"sync"
	"time"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)

func usernameValid(name string) bool { return usernamePattern.MatchString(name) }

// SubadminList 查询子管理员
func SubadminList(page, pageSize int, keyword, sortName, sortType string) (int64, []model.Subadmin, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 1000 {
		pageSize = 20
	}
	q := store.DB().Model(&model.Subadmin{})
	if keyword != "" {
		q = q.Where("subadmin_name LIKE ? OR subadmin_note LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	order := orderClause(sortName, sortType, map[string]string{
		"subadmin_id":         "subadmin_id",
		"subadminId":          "subadmin_id",
		"subadmin_name":       "subadmin_name",
		"subadminName":        "subadmin_name",
		"subadmin_status":     "subadmin_status",
		"subadminStatus":      "subadmin_status",
		"subadmin_last_login": "subadmin_last_login",
	}, "subadmin_id DESC")
	var list []model.Subadmin
	if err := q.Order(order).Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return 0, nil, err
	}
	for i := range list {
		list[i].Online = IsOnline(list[i].SubadminID)
	}
	return total, list, nil
}

// ---------------------------------------------------------------------------
// 在线状态（内存态最近活跃判定，对齐原版 token 过期在线语义）
// ---------------------------------------------------------------------------

var (
	onlineMu   sync.RWMutex
	onlineSeen = map[int64]time.Time{}
)

// onlineWindow 判定在线的活跃窗口
const onlineWindow = 10 * time.Minute

// TouchOnline 记录子管理员最近一次请求
func TouchOnline(id int64) {
	onlineMu.Lock()
	onlineSeen[id] = time.Now()
	onlineMu.Unlock()
}

// IsOnline 判断子管理员是否在线（最近 10 分钟内有请求）
func IsOnline(id int64) bool {
	onlineMu.RLock()
	t, ok := onlineSeen[id]
	onlineMu.RUnlock()
	return ok && time.Since(t) < onlineWindow
}

// SubadminCreate 新建子管理员
func SubadminCreate(name, pass, note, phone, email string, functions []string, operator string) error {
	if name == "" || pass == "" {
		return errors.New("用户名和密码不能为空")
	}
	var exist model.Subadmin
	if err := store.DB().Where("subadmin_name = ?", name).First(&exist).Error; err == nil {
		return fmt.Errorf("子管理员 [%s] 已存在", name)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	sub := model.Subadmin{
		SubadminName:       name,
		SubadminPassword:   util.Hash(pass),
		SubadminPhone:      phone,
		SubadminEmail:      email,
		SubadminStatus:     1,
		SubadminNote:       note,
		SubadminCreateTime: model.Now(),
		SubadminFunctions:  joinList(functions),
	}
	// 未指定权限时按内置权限树的默认勾选授权（后台任务），避免新建账号无任何可用功能
	if len(functions) == 0 {
		sub.SubadminFunctions = joinList(collectFunctions(DefaultSubadminTree()))
	}
	if err := store.DB().Create(&sub).Error; err != nil {
		return err
	}
	AddLog("子管理员", fmt.Sprintf("新建子管理员 [%s]", name), operator)
	return nil
}

// SubadminDel 删除子管理员
func SubadminDel(id int64, operator string) error {
	res := store.DB().Where("subadmin_id = ?", id).Delete(&model.Subadmin{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("子管理员不存在")
	}
	AddLog("子管理员", fmt.Sprintf("删除子管理员 ID=%d", id), operator)
	return nil
}

// SubadminUpdStatus 启用/禁用子管理员
func SubadminUpdStatus(id int64, status int, operator string) error {
	res := store.DB().Model(&model.Subadmin{}).Where("subadmin_id = ?", id).Update("subadmin_status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("子管理员不存在")
	}
	AddLog("子管理员", fmt.Sprintf("修改子管理员 ID=%d 的状态为 %d", id, status), operator)
	return nil
}

// SubadminUpdPass 修改子管理员密码
func SubadminUpdPass(id int64, pass, operator string) error {
	if pass == "" {
		return errors.New("密码不能为空")
	}
	res := store.DB().Model(&model.Subadmin{}).Where("subadmin_id = ?", id).Update("subadmin_password", util.Hash(pass))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("子管理员不存在")
	}
	AddLog("子管理员", fmt.Sprintf("修改子管理员 ID=%d 的密码", id), operator)
	return nil
}

// SubadminUpdNote 修改子管理员备注
func SubadminUpdNote(id int64, note, operator string) error {
	res := store.DB().Model(&model.Subadmin{}).Where("subadmin_id = ?", id).Update("subadmin_note", note)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("子管理员不存在")
	}
	AddLog("子管理员", fmt.Sprintf("修改子管理员 ID=%d 的备注", id), operator)
	return nil
}

// SubadminUpdFunctions 设置子管理员可管理的仓库树
func SubadminUpdFunctions(id int64, functions, tree []string, operator string) error {
	res := store.DB().Model(&model.Subadmin{}).Where("subadmin_id = ?", id).Updates(map[string]interface{}{
		"subadmin_functions": joinList(functions),
		"subadmin_tree":      joinList(tree),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("子管理员不存在")
	}
	AddLog("子管理员", fmt.Sprintf("修改子管理员 ID=%d 的权限范围", id), operator)
	return nil
}

func joinList(list []string) string {
	out := ""
	for i, v := range list {
		if i > 0 {
			out += ","
		}
		out += v
	}
	return out
}

// ---------------------------------------------------------------------------
// 个人信息
// ---------------------------------------------------------------------------

// AdminUpdName 管理员修改自身账号名
func AdminUpdName(oldName, newName, pass string) error {
	if newName == "" {
		return errors.New("新账号名不能为空")
	}
	if !usernameValid(newName) {
		return errors.New("账号名只能包含字母、数字、下划线、短横线与点，且长度不超过 32 位")
	}
	var admin model.AdminUser
	if err := store.DB().Where("admin_user_name = ?", oldName).First(&admin).Error; err == nil {
		if !util.SecureEqual(admin.AdminUserPassword, util.Hash(pass)) {
			return errors.New("密码错误")
		}
		var dup model.AdminUser
		if err := store.DB().Where("admin_user_name = ? AND admin_user_id != ?", newName, admin.AdminUserID).
			First(&dup).Error; err == nil {
			return fmt.Errorf("账号名 [%s] 已存在", newName)
		}
		if err := store.DB().Model(&model.AdminUser{}).Where("admin_user_id = ?", admin.AdminUserID).
			Update("admin_user_name", newName).Error; err != nil {
			return err
		}
		AddLog("修改账号", fmt.Sprintf("管理员账号名由 [%s] 修改为 [%s]", oldName, newName), newName)
		go SendMailByTrigger(MailTriggerAdminName, "SVNAdmin4 管理员账号名变更",
			fmt.Sprintf("管理员账号名由 [%s] 变更为 [%s]，时间：%s", oldName, newName, model.Now()))
		return nil
	}
	// 子管理员只能改密码，不支持改账号名
	var sub model.Subadmin
	if err := store.DB().Where("subadmin_name = ?", oldName).First(&sub).Error; err == nil {
		return errors.New("子管理员不支持修改账号名")
	}
	return errors.New("用户不存在")
}

// AdminUpdPass 管理员修改自身密码
func AdminUpdPass(userName, oldPass, newPass string) error {
	var admin model.AdminUser
	if err := store.DB().Where("admin_user_name = ?", userName).First(&admin).Error; err == nil {
		if !util.SecureEqual(admin.AdminUserPassword, util.Hash(oldPass)) {
			return errors.New("原密码错误")
		}
		if err := store.DB().Model(&model.AdminUser{}).Where("admin_user_name = ?", userName).
			Update("admin_user_password", util.Hash(newPass)).Error; err != nil {
			return err
		}
		AddLog("修改密码", "管理员修改自身密码", userName)
		go SendMailByTrigger(MailTriggerAdminPass, "SVNAdmin4 管理员密码变更",
			fmt.Sprintf("管理员 [%s] 于 %s 修改了登录密码。", userName, model.Now()))
		return nil
	}
	// SVN 用户
	var su model.SvnUser
	if err := store.DB().Where("svn_user_name = ?", userName).First(&su).Error; err == nil {
		if !util.SecureEqual(su.SvnUserPass, oldPass) {
			return errors.New("原密码错误")
		}
		if err := UpdUserPass(userName, newPass, userName); err != nil {
			return err
		}
		go SendMailByTrigger(MailTriggerSvnUserPas, "SVNAdmin4 SVN 用户密码变更",
			fmt.Sprintf("SVN 用户 [%s] 于 %s 修改了密码。", userName, model.Now()))
		return nil
	}
	// 子管理员
	var sub model.Subadmin
	if err := store.DB().Where("subadmin_name = ?", userName).First(&sub).Error; err == nil {
		if !util.SecureEqual(sub.SubadminPassword, util.Hash(oldPass)) {
			return errors.New("原密码错误")
		}
		return SubadminUpdPass(sub.SubadminID, newPass, userName)
	}
	return errors.New("用户不存在")
}
