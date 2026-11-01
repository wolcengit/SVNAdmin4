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
	"strings"

	"gorm.io/gorm"
	"svnadmin/internal/model"
	"svnadmin/internal/store"
	"svnadmin/internal/svn"
)

// GroupMemberResult 分组成员
type GroupMemberResult struct {
	Users   []string `json:"users"`
	Groups  []string `json:"groups"`
	Aliases []string `json:"aliases"`
}

// groupCounts 统计分组中各类成员数量
func groupCounts(members []string) (uc, gc, ac int) {
	for _, m := range members {
		switch {
		case strings.HasPrefix(m, "@"):
			gc++
		case strings.HasPrefix(m, "&"):
			ac++
		default:
			uc++
		}
	}
	return
}

// SyncGroups 将 authz 中的分组同步到数据库，保证列表数据一致
func SyncGroups() error {
	authz, err := svn.LoadAuthz()
	if err != nil {
		return err
	}
	for _, name := range authz.GroupOrder {
		members := authz.Groups[name]
		uc, gc, ac := groupCounts(members)
		var g model.SvnGroup
		if err := store.DB().Where("svn_group_name = ?", name).First(&g).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				store.DB().Create(&model.SvnGroup{
					SvnGroupName:       name,
					IncludeUserCount:   uc,
					IncludeGroupCount:  gc,
					IncludeAliaseCount: ac,
				})
			}
			continue
		}
		store.DB().Model(&model.SvnGroup{}).Where("svn_group_name = ?", name).Updates(map[string]interface{}{
			"include_user_count":   uc,
			"include_group_count":  gc,
			"include_aliase_count": ac,
		})
	}
	// 清理已不存在的分组
	var all []model.SvnGroup
	_ = store.DB().Find(&all).Error
	for _, g := range all {
		if !authz.HasGroup(g.SvnGroupName) {
			store.DB().Delete(&g)
		}
	}
	return nil
}

// GroupList 分页查询分组
func GroupList(page, pageSize int, keyword, sortName, sortType string) (int64, []model.SvnGroup, error) {
	if err := SyncGroups(); err != nil {
		return 0, nil, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 1000 {
		pageSize = 20
	}
	q := store.DB().Model(&model.SvnGroup{})
	if keyword != "" {
		q = q.Where("svn_group_name LIKE ? OR svn_group_note LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	var list []model.SvnGroup
	order := orderClause(sortName, sortType, map[string]string{
		"svn_group_id":   "svn_group_id",
		"svnGroupId":     "svn_group_id",
		"svn_group_name": "svn_group_name",
		"svnGroupName":   "svn_group_name",
	}, "svn_group_id DESC")
	if err := q.Order(order).Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return 0, nil, err
	}
	return total, list, nil
}

// GroupAll 返回全部分组名
func GroupAll() ([]string, error) {
	authz, err := svn.LoadAuthz()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(authz.GroupOrder))
	out = append(out, authz.GroupOrder...)
	return out, nil
}

// CreateGroup 新建分组
func CreateGroup(name, note, operator string) error {
	if groupSourceIsLDAP() {
		return errors.New("分组来源为 LDAP 时禁止在本地创建分组")
	}
	if err := validName(name); err != nil {
		return err
	}
	authz, err := svn.LoadAuthz()
	if err != nil {
		return err
	}
	if authz.HasGroup(name) {
		return fmt.Errorf("分组 [%s] 已存在", name)
	}
	authz.AddGroup(name, []string{})
	if err := authz.Save(); err != nil {
		return err
	}
	store.DB().Create(&model.SvnGroup{SvnGroupName: name, SvnGroupNote: note})
	AddLog("新建分组", fmt.Sprintf("新建 SVN 分组 [%s]", name), operator)
	return nil
}

// DelGroup 删除分组
func DelGroup(name, operator string) error {
	authz, err := svn.LoadAuthz()
	if err != nil {
		return err
	}
	if !authz.HasGroup(name) {
		return fmt.Errorf("分组 [%s] 不存在", name)
	}
	authz.DelGroup(name)
	if err := authz.Save(); err != nil {
		return err
	}
	store.DB().Where("svn_group_name = ?", name).Delete(&model.SvnGroup{})
	AddLog("删除分组", fmt.Sprintf("删除 SVN 分组 [%s]", name), operator)
	return nil
}

// UpdGroupName 修改分组名称
func UpdGroupName(oldName, newName, operator string) error {
	if err := validName(newName); err != nil {
		return err
	}
	authz, err := svn.LoadAuthz()
	if err != nil {
		return err
	}
	if !authz.HasGroup(oldName) {
		return fmt.Errorf("分组 [%s] 不存在", oldName)
	}
	if authz.HasGroup(newName) {
		return fmt.Errorf("分组 [%s] 已存在", newName)
	}
	authz.RenameGroup(oldName, newName)
	if err := authz.Save(); err != nil {
		return err
	}
	store.DB().Model(&model.SvnGroup{}).Where("svn_group_name = ?", oldName).Update("svn_group_name", newName)
	AddLog("修改分组", fmt.Sprintf("分组 [%s] 更名为 [%s]", oldName, newName), operator)
	return nil
}

// UpdGroupNote 修改分组备注
func UpdGroupNote(name, note, operator string) error {
	authz, err := svn.LoadAuthz()
	if err != nil {
		return err
	}
	if !authz.HasGroup(name) {
		return fmt.Errorf("分组 [%s] 不存在", name)
	}
	var g model.SvnGroup
	if err := store.DB().Where("svn_group_name = ?", name).First(&g).Error; err == nil {
		store.DB().Model(&g).Update("svn_group_note", note)
	} else {
		store.DB().Create(&model.SvnGroup{SvnGroupName: name, SvnGroupNote: note})
	}
	AddLog("修改分组", fmt.Sprintf("修改分组 [%s] 的备注", name), operator)
	return nil
}

// GetGroupMember 获取分组成员（searchKeyword 非空时按成员名模糊过滤）
func GetGroupMember(name, searchKeyword string) (*GroupMemberResult, error) {
	authz, err := svn.LoadAuthz()
	if err != nil {
		return nil, err
	}
	res := &GroupMemberResult{Users: []string{}, Groups: []string{}, Aliases: []string{}}
	kw := strings.TrimSpace(searchKeyword)
	match := func(m string) bool {
		return kw == "" || strings.Contains(strings.TrimPrefix(strings.TrimPrefix(m, "@"), "&"), kw)
	}
	for _, m := range authz.GroupMembers(name) {
		if !match(m) {
			continue
		}
		switch {
		case strings.HasPrefix(m, "@"):
			res.Groups = append(res.Groups, strings.TrimPrefix(m, "@"))
		case strings.HasPrefix(m, "&"):
			res.Aliases = append(res.Aliases, strings.TrimPrefix(m, "&"))
		default:
			res.Users = append(res.Users, m)
		}
	}
	return res, nil
}

// UpdGroupMember 设置分组成员（全量覆盖）
func UpdGroupMember(name string, users, groups, aliases []string, operator string) error {
	if groupSourceIsLDAP() {
		return errors.New("分组来源为 LDAP 时禁止在本地编辑分组成员")
	}
	authz, err := svn.LoadAuthz()
	if err != nil {
		return err
	}
	if !authz.HasGroup(name) {
		return fmt.Errorf("分组 [%s] 不存在", name)
	}
	members := []string{}
	for _, u := range users {
		if u != "" {
			members = append(members, u)
		}
	}
	for _, g := range groups {
		if g != "" && g != name {
			members = append(members, "@"+g)
		}
	}
	for _, a := range aliases {
		if a != "" {
			members = append(members, "&"+a)
		}
	}
	// 循环嵌套检测：A 包含 B、B 又包含 A 会让权限求值陷入死循环（原版 GetSvnGroupAllGroupList 语义）
	if err := checkGroupCycle(authz, name, groups); err != nil {
		return err
	}
	authz.AddGroup(name, members)
	if err := authz.Save(); err != nil {
		return err
	}
	uc, gc, ac := groupCounts(members)
	store.DB().Model(&model.SvnGroup{}).Where("svn_group_name = ?", name).Updates(map[string]interface{}{
		"include_user_count":   uc,
		"include_group_count":  gc,
		"include_aliase_count": ac,
	})
	AddLog("修改分组", fmt.Sprintf("修改分组 [%s] 的成员", name), operator)
	return nil
}

// ---------------------------------------------------------------------------
// 别名
// ---------------------------------------------------------------------------

// AliaseList 返回用户别名（searchKeyword 非空时按别名前缀模糊过滤）
func AliaseList(searchKeyword string) (map[string]string, error) {
	authz, err := svn.LoadAuthz()
	if err != nil {
		return nil, err
	}
	if searchKeyword = strings.TrimSpace(searchKeyword); searchKeyword == "" {
		return authz.Aliases, nil
	}
	out := map[string]string{}
	for k, v := range authz.Aliases {
		if strings.Contains(k, searchKeyword) || strings.Contains(v, searchKeyword) {
			out[k] = v
		}
	}
	return out, nil
}

// checkGroupCycle 检测把 groups 加入 name 后是否出现分组循环嵌套。
// 从 name 出发沿"分组→其包含的分组"做深度遍历，若回到自身即存在环。
func checkGroupCycle(az *svn.Authz, name string, groups []string) error {
	// 先做一次快速判断：直接互相包含
	for _, g := range groups {
		if g == name {
			continue
		}
		if hasGroupMember(az, g, name) {
			return fmt.Errorf("存在分组循环嵌套的情况：[%s] 与 [%s] 互相包含", name, g)
		}
	}
	for _, g := range groups {
		if g == "" || g == name {
			continue
		}
		if hasCycleDFS(az, g, name, map[string]bool{}) {
			return fmt.Errorf("存在分组循环嵌套的情况：[%s] 已直接或间接包含 [%s]", name, g)
		}
	}
	return nil
}

// hasGroupMember 判断 group 是否直接包含 target
func hasGroupMember(az *svn.Authz, group, target string) bool {
	for _, m := range az.Groups[group] {
		if strings.TrimPrefix(m, "@") == target {
			return true
		}
	}
	return false
}

// hasCycleDFS 从 cur 出发向下遍历，判断能否到达 target
func hasCycleDFS(az *svn.Authz, cur, target string, visited map[string]bool) bool {
	if visited[cur] {
		return false
	}
	visited[cur] = true
	for _, m := range az.Groups[cur] {
		if !strings.HasPrefix(m, "@") {
			continue
		}
		sub := strings.TrimPrefix(m, "@")
		if sub == target {
			return true
		}
		if hasCycleDFS(az, sub, target, visited) {
			return true
		}
	}
	return false
}
