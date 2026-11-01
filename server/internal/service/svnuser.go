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
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"svnadmin/internal/model"
	"svnadmin/internal/store"
	"svnadmin/internal/svn"
)

// validName 校验 SVN 用户名合法性
func validName(name string) error {
	if name == "" {
		return errors.New("名称不能为空")
	}
	for _, c := range name {
		switch c {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ',', '@', '&', '=', '[', ']', '#', ';':
			return errors.New("名称不能包含 / \\ : * ? \" < > | , @ & = [ ] # ; 等特殊字符")
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 用户列表
// ---------------------------------------------------------------------------

// UserList 分页查询 SVN 用户
func UserList(page, pageSize int, keyword string, status int, sortName, sortType string) (int64, []model.SvnUser, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 1000 {
		pageSize = 20
	}
	q := store.DB().Model(&model.SvnUser{})
	if keyword != "" {
		q = q.Where("svn_user_name LIKE ? OR svn_user_note LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if status > 0 {
		q = q.Where("svn_user_status = ?", status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	var list []model.SvnUser
	order := orderClause(sortName, sortType, map[string]string{
		"svn_user_id":        "svn_user_id",
		"svnUserId":          "svn_user_id",
		"svn_user_name":      "svn_user_name",
		"svnUserName":        "svn_user_name",
		"svn_user_status":    "svn_user_status",
		"svnUserStatus":      "svn_user_status",
		"svn_user_last_login": "svn_user_last_login",
	}, "svn_user_id DESC")
	if err := q.Order(order).Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return 0, nil, err
	}
	// 在线态：svn_user_token 记录登录 token 的到期时间（Unix 秒），未过期即在线
	now := time.Now().Unix()
	for i := range list {
		list[i].Online = list[i].SvnUserStatus == 1 && list[i].SvnUserToken != "" && list[i].SvnUserToken != "-"
		if list[i].Online {
			if exp, err := strconv.ParseInt(list[i].SvnUserToken, 10, 64); err != nil || exp < now {
				list[i].Online = false
			}
		}
	}
	return total, list, nil
}

// UserAll 返回全部用户（不分页）
func UserAll() ([]model.SvnUser, error) {
	var list []model.SvnUser
	err := store.DB().Order("svn_user_name").Find(&list).Error
	return list, err
}

// ---------------------------------------------------------------------------
// 用户增删改
// ---------------------------------------------------------------------------

// ensureSvnUser 保证用户存在于库中（LDAP / 外部来源同步时使用，密码留空）
func ensureSvnUser(name, note string) error {
	if err := validName(name); err != nil {
		return err
	}
	var exist model.SvnUser
	if err := store.DB().Where("svn_user_name = ?", name).First(&exist).Error; err == nil {
		if note != "" && exist.SvnUserNote == "" {
			_ = store.DB().Model(&exist).Update("svn_user_note", note)
		}
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return store.DB().Create(&model.SvnUser{
		SvnUserName:   name,
		SvnUserPass:   "",
		SvnUserStatus: 1,
		SvnUserNote:   note,
	}).Error
}

// CreateUser 新建 SVN 用户
func CreateUser(name, pass, note, mail, operator string) error {
	if userSourceIsLDAP() {
		return errors.New("用户来源为 LDAP 时禁止在本地创建用户")
	}
	if err := validName(name); err != nil {
		return err
	}
	if err := CheckUserPass(pass); err != nil {
		return err
	}
	var exist model.SvnUser
	if err := store.DB().Where("svn_user_name = ?", name).First(&exist).Error; err == nil {
		return fmt.Errorf("用户 [%s] 已存在", name)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	user := model.SvnUser{
		SvnUserName:   name,
		SvnUserPass:   pass,
		SvnUserStatus: 1,
		SvnUserNote:   note,
		SvnUserMail:   mail,
	}
	if err := store.DB().Create(&user).Error; err != nil {
		return err
	}

	pw, err := svn.LoadPasswd()
	if err != nil {
		return err
	}
	pw.Set(name, pass)
	if err := pw.Save(); err != nil {
		return err
	}
	if configHTTPEnabled() {
		hp, err := svn.LoadHTTPPasswd()
		if err == nil {
			_ = hp.Set(name, pass)
			_ = hp.Save()
		}
	}
	AddLog("新建用户", fmt.Sprintf("新建 SVN 用户 [%s]", name), operator)
	return nil
}

// DelUser 删除 SVN 用户
func DelUser(name, operator string) error {
	var user model.SvnUser
	if err := store.DB().Where("svn_user_name = ?", name).First(&user).Error; err != nil {
		return fmt.Errorf("用户 [%s] 不存在", name)
	}
	if err := store.DB().Delete(&user).Error; err != nil {
		return err
	}

	pw, err := svn.LoadPasswd()
	if err == nil {
		pw.Del(name)
		_ = pw.Save()
	}
	if configHTTPEnabled() {
		hp, err := svn.LoadHTTPPasswd()
		if err == nil {
			hp.Del(name)
			_ = hp.Save()
		}
	}

	authz, err := svn.LoadAuthz()
	if err == nil {
		// 从分组中移除
		for g, members := range authz.Groups {
			var keep []string
			for _, m := range members {
				if m != name {
					keep = append(keep, m)
				}
			}
			authz.Groups[g] = keep
		}
		// 移除授权项
		authz.DelObjectPri(name)
		_ = authz.Save()
	}
	AddLog("删除用户", fmt.Sprintf("删除 SVN 用户 [%s]", name), operator)
	return nil
}

// UpdUserPass 修改 SVN 用户密码
func UpdUserPass(name, pass, operator string) error {
	if userSourceIsLDAP() {
		return errors.New("用户来源为 LDAP 时密码由 LDAP 管理，禁止本地修改")
	}
	if err := CheckUserPass(pass); err != nil {
		return err
	}
	var user model.SvnUser
	if err := store.DB().Where("svn_user_name = ?", name).First(&user).Error; err != nil {
		return fmt.Errorf("用户 [%s] 不存在", name)
	}
	if err := store.DB().Model(&model.SvnUser{}).Where("svn_user_name = ?", name).Update("svn_user_pass", pass).Error; err != nil {
		return err
	}
	if user.SvnUserStatus == 1 {
		pw, err := svn.LoadPasswd()
		if err == nil {
			pw.Set(name, pass)
			_ = pw.Save()
		}
		if configHTTPEnabled() {
			hp, err := svn.LoadHTTPPasswd()
			if err == nil {
				_ = hp.Set(name, pass)
				_ = hp.Save()
			}
		}
	}
	AddLog("修改密码", fmt.Sprintf("修改 SVN 用户 [%s] 的密码", name), operator)
	return nil
}

// UpdUserStatus 启用或禁用 SVN 用户
func UpdUserStatus(name string, status int, operator string) error {
	if userSourceIsLDAP() {
		return errors.New("用户来源为 LDAP 时禁止在本地启用/禁用用户")
	}
	var user model.SvnUser
	if err := store.DB().Where("svn_user_name = ?", name).First(&user).Error; err != nil {
		return fmt.Errorf("用户 [%s] 不存在", name)
	}
	if err := store.DB().Model(&model.SvnUser{}).Where("svn_user_name = ?", name).Update("svn_user_status", status).Error; err != nil {
		return err
	}
	pw, err := svn.LoadPasswd()
	if err != nil {
		return err
	}
	if status == 1 {
		pw.Set(name, user.SvnUserPass)
	} else {
		pw.Del(name)
	}
	if err := pw.Save(); err != nil {
		return err
	}
	if configHTTPEnabled() {
		hp, err := svn.LoadHTTPPasswd()
		if err == nil {
			if status == 1 {
				_ = hp.Set(name, user.SvnUserPass)
			} else {
				hp.Del(name)
			}
			_ = hp.Save()
		}
	}
	action := "禁用"
	if status == 1 {
		action = "启用"
	}
	AddLog("用户状态", fmt.Sprintf("%s SVN 用户 [%s]", action, name), operator)
	return nil
}

// UpdUserNote 修改用户备注
func UpdUserNote(name, note, operator string) error {
	res := store.DB().Model(&model.SvnUser{}).Where("svn_user_name = ?", name).Update("svn_user_note", note)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("用户 [%s] 不存在", name)
	}
	AddLog("修改备注", fmt.Sprintf("修改 SVN 用户 [%s] 的备注", name), operator)
	return nil
}

// UpdUserMail 修改用户邮箱
func UpdUserMail(name, mail string) error {
	res := store.DB().Model(&model.SvnUser{}).Where("svn_user_name = ?", name).Update("svn_user_mail", mail)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("用户 [%s] 不存在", name)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 扫描与导入 / 同步
// ---------------------------------------------------------------------------

// SyncUsers 将 passwd（或 httpPasswd）文件中的用户同步到数据库
func SyncUsers() error {
	ds := GetDataSource(OptionSvnDatasource)
	if ds.UserSource == SourceLDAP {
		// LDAP 来源时从 LDAP 拉取
		if _, err := SyncLDAPToDB("system"); err != nil {
			return err
		}
		return nil
	}
	var names []string
	var getPass func(string) (string, bool)
	if ds.UserSource == SourceHTTPPasswd {
		hp, err := svn.LoadHTTPPasswd()
		if err != nil {
			return err
		}
		names = hp.Users
		getPass = func(n string) (string, bool) { return hp.Hash[n], true }
	} else {
		pw, err := svn.LoadPasswd()
		if err != nil {
			return err
		}
		names = pw.Names()
		getPass = pw.Get
	}
	for _, name := range names {
		pass, _ := getPass(name)
		var u model.SvnUser
		if err := store.DB().Where("svn_user_name = ?", name).First(&u).Error; err == nil {
			if pass != "" && u.SvnUserPass != pass {
				_ = store.DB().Model(&u).Update("svn_user_pass", pass)
			}
			continue
		}
		_ = store.DB().Create(&model.SvnUser{
			SvnUserName:   name,
			SvnUserPass:   pass,
			SvnUserStatus: 1,
			SvnUserNote:   "数据同步自动创建",
		}).Error
	}
	// 清理文件中已不存在的用户
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	var all []model.SvnUser
	_ = store.DB().Find(&all).Error
	for _, u := range all {
		if !set[u.SvnUserName] {
			_ = store.DB().Delete(&u).Error
		}
	}
	AddLog("数据同步", fmt.Sprintf("同步 SVN 用户，共 %d 个", len(names)), "system")
	return nil
}

// UserScan 扫描 passwd 文件，返回其中尚未纳入系统管理的用户
func UserScan() ([]string, error) {
	if userSourceIsLDAP() {
		return nil, errors.New("用户来源为 LDAP 时无需扫描本地 passwd 文件")
	}
	pw, err := svn.LoadPasswd()
	if err != nil {
		return nil, err
	}
	exist := map[string]bool{}
	var all []model.SvnUser
	_ = store.DB().Find(&all).Error
	for _, u := range all {
		exist[u.SvnUserName] = true
	}
	var out []string
	for _, name := range pw.Names() {
		if !exist[name] {
			out = append(out, name)
		}
	}
	return out, nil
}

// UserImport 批量导入 passwd 文件中的用户
func UserImport(names []string, operator string) (int, error) {
	if userSourceIsLDAP() {
		return 0, errors.New("用户来源为 LDAP 时禁止导入本地 passwd 用户")
	}
	pw, err := svn.LoadPasswd()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		pass, ok := pw.Get(name)
		if !ok {
			continue
		}
		var exist model.SvnUser
		if err := store.DB().Where("svn_user_name = ?", name).First(&exist).Error; err == nil {
			continue
		}
		// 兼容原版 #disabled# 标记：passwd 中被标记禁用的用户导入后保持禁用状态
		status := 1
		if pw.Disabled[name] {
			status = 0
		}
		// 注意：SvnUserStatus 带 gorm default:1，struct Create 会跳过零值字段导致
		// 禁用状态(0)被数据库默认值覆盖，因此这里用 map 形式强制写入全部字段
		if err := store.DB().Model(&model.SvnUser{}).Create(map[string]interface{}{
			"svn_user_name":   name,
			"svn_user_pass":   pass,
			"svn_user_status": status,
			"svn_user_note":   "由 passwd 文件扫描导入",
		}).Error; err != nil {
			continue
		}
		count++
	}
	if count > 0 {
		AddLog("导入用户", fmt.Sprintf("从 passwd 文件导入 %d 个 SVN 用户", count), operator)
	}
	return count, nil
}
