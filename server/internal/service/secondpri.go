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
	"svnadmin/internal/util"
)

// 二次授权的对象类型
const (
	SecondObjectUser   = "user"
	SecondObjectGroup  = "group"
	SecondObjectAliase = "aliase"
)

// secondPriUnique 生成"某用户某仓库某路径"这条权限的唯一标识
func secondPriUnique(owner, repName, path string) string {
	return util.Unique(owner, repName, svnCleanPath(path))
}

func svnCleanPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
		if p == "" {
			p = "/"
		}
	}
	return p
}

// ---------------------------------------------------------------------------
// 开关
// ---------------------------------------------------------------------------

// SecondPriEnabled 判断某条用户路径权限是否开启了二次授权
func SecondPriEnabled(owner, repName, path string) bool {
	unique := secondPriUnique(owner, repName, path)
	var r model.SvnSecondPri
	err := store.DB().Where("second_pri_unique = ? AND svn_object_type = ? AND svn_object_name = ?",
		unique, "", "").First(&r).Error
	if err != nil {
		return false
	}
	return r.SecondPri == "1"
}

// UpdSecondPri 开启或关闭二次授权；关闭时同时清除白名单
func UpdSecondPri(owner, repName, path string, enable bool) error {
	if owner == "" || repName == "" {
		return errors.New("参数不完整")
	}
	path = svnCleanPath(path)
	unique := secondPriUnique(owner, repName, path)

	var r model.SvnSecondPri
	err := store.DB().Where("second_pri_unique = ? AND svn_object_type = ? AND svn_object_name = ?",
		unique, "", "").First(&r).Error
	val := "0"
	if enable {
		val = "1"
	}
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		r = model.SvnSecondPri{
			SecondPriOwner:   owner,
			SecondPriRepName: repName,
			SecondPriPath:    path,
			SecondPri:        val,
			SecondPriUnique:  unique,
			SecondPriAddTime: model.Now(),
		}
		if err := store.DB().Create(&r).Error; err != nil {
			return err
		}
	} else {
		if err := store.DB().Model(&r).Update("second_pri", val).Error; err != nil {
			return err
		}
	}
	if !enable {
		// 关闭时删除该权限下的全部白名单对象
		if err := store.DB().Where("second_pri_unique = ? AND NOT (svn_object_type = ? AND svn_object_name = ?)",
			unique, "", "").Delete(&model.SvnSecondPri{}).Error; err != nil {
			return err
		}
	}
	action := "关闭"
	if enable {
		action = "开启"
	}
	AddLog("二次授权", fmt.Sprintf("%s用户 [%s] 在 [%s%s] 的二次授权", action, owner, repName, path), "system")
	return nil
}

// ---------------------------------------------------------------------------
// 白名单对象
// ---------------------------------------------------------------------------

// SecondPriObjectList 查询白名单对象列表
func SecondPriObjectList(owner, repName, path, keyword string) ([]model.SvnSecondPri, error) {
	unique := secondPriUnique(owner, repName, path)
	q := store.DB().Where("second_pri_unique = ? AND NOT (svn_object_type = ? AND svn_object_name = ?)", unique, "", "")
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("svn_object_name LIKE ?", kw)
	}
	var list []model.SvnSecondPri
	err := q.Order("svn_second_pri_id").Find(&list).Error
	return list, err
}

// CreateSecondPriObject 添加可下放的对象
func CreateSecondPriObject(owner, repName, path, objectType, objectName string) error {
	switch objectType {
	case SecondObjectUser, SecondObjectGroup, SecondObjectAliase:
	default:
		return errors.New("对象类型不合法，仅支持 user / group / aliase")
	}
	objectName = strings.TrimSpace(objectName)
	if objectName == "" {
		return errors.New("对象名称不能为空")
	}
	if objectName == owner {
		return errors.New("可管理对象不能包括本身")
	}
	path = svnCleanPath(path)
	unique := secondPriUnique(owner, repName, path)

	var exist model.SvnSecondPri
	err := store.DB().Where("second_pri_unique = ? AND svn_object_type = ? AND svn_object_name = ?",
		unique, objectType, objectName).First(&exist).Error
	if err == nil {
		return fmt.Errorf("对象 [%s] 已存在", objectName)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	r := model.SvnSecondPri{
		SecondPriOwner:   owner,
		SecondPriRepName: repName,
		SecondPriPath:    path,
		SvnObjectType:    objectType,
		SvnObjectName:    objectName,
		SecondPri:        "1",
		SecondPriUnique:  unique,
		SecondPriAddTime: model.Now(),
	}
	if err := store.DB().Create(&r).Error; err != nil {
		return err
	}
	AddLog("二次授权", fmt.Sprintf("用户 [%s] 新增可管理对象 [%s]（%s）", owner, objectName, objectType), "system")
	return nil
}

// DelSecondPriObject 删除白名单对象
func DelSecondPriObject(id int64) error {
	res := store.DB().Where("svn_second_pri_id = ?", id).Delete(&model.SvnSecondPri{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("记录不存在")
	}
	return nil
}

// ---------------------------------------------------------------------------
// 可见对象过滤
// ---------------------------------------------------------------------------

// SecondPriObjects 返回用户可管理的对象集合，按类型分组
func SecondPriObjects(owner string) map[string][]string {
	out := map[string][]string{}
	var list []model.SvnSecondPri
	store.DB().Where("second_pri_owner = ? AND svn_object_type != ?", owner, "").Find(&list)
	for _, r := range list {
		out[r.SvnObjectType] = append(out[r.SvnObjectType], r.SvnObjectName)
	}
	return out
}

// FilterBySecondPri 按二次授权白名单过滤对象列表
// 未开启二次授权的用户返回全部；开启了则返回白名单范围内的对象
func FilterBySecondPri(role string, userName string, objectType string, names []string) []string {
	if role != "svnuser" || userName == "" {
		return names
	}
	enabled := int64(0)
	store.DB().Model(&model.SvnSecondPri{}).
		Where("second_pri_owner = ? AND NOT (svn_object_type = ? AND svn_object_name = ?)", userName, "", "").
		Count(&enabled)
	if enabled == 0 {
		return names
	}
	allow := map[string]bool{}
	for _, n := range SecondPriObjects(userName)[objectType] {
		allow[n] = true
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if allow[n] {
			out = append(out, n)
		}
	}
	return out
}

// SecondPriUserPaths 返回用户开启了二次授权的全部路径（用于前端标注）
func SecondPriUserPaths(owner string) []map[string]string {
	var list []model.SvnSecondPri
	store.DB().Where("second_pri_owner = ? AND svn_object_type = ? AND svn_object_name = ? AND second_pri = ?",
		owner, "", "", "1").Find(&list)
	out := make([]map[string]string, 0, len(list))
	for _, r := range list {
		out = append(out, map[string]string{
			"repName": r.SecondPriRepName,
			"priPath": r.SecondPriPath,
		})
	}
	return out
}

// SecondPriObjectOwner 返回白名单记录的归属人（SVN 用户自助删除时校验归属）
func SecondPriObjectOwner(id int64) (string, error) {
	var r model.SvnSecondPri
	if err := store.DB().Where("svn_second_pri_id = ?", id).First(&r).Error; err != nil {
		return "", errors.New("记录不存在")
	}
	return r.SecondPriOwner, nil
}

// SecondPriDelegatable 校验 SVN 用户是否可以把 (repName, path) 的权限下放给 objectName。
// 完整链路校验（对齐原版二次授权的自助语义）：
//  1. 该用户在该路径开启了二次授权；
//  2. 目标对象在白名单中（且不得是自身）。
//
// 管理员/子管理员调用时直接放行。
func SecondPriDelegatable(owner, repName, path, objectName string) error {
	if objectName == owner {
		return errors.New("不能下放权限给自己")
	}
	unique := secondPriUnique(owner, repName, path)
	// 1. 二次授权开关已开启
	var sw model.SvnSecondPri
	if err := store.DB().Where("second_pri_unique = ? AND svn_object_type = ? AND svn_object_name = ?",
		unique, "", "").First(&sw).Error; err != nil || sw.SecondPri != "1" {
		return errors.New("该路径未开启二次授权，无法下放权限")
	}
	// 2. 目标对象在白名单中（类型不限：user/group/aliase 任一记录命中即可）
	var cnt int64
	store.DB().Model(&model.SvnSecondPri{}).
		Where("second_pri_unique = ? AND svn_object_name = ?", unique, objectName).
		Count(&cnt)
	if cnt == 0 {
		return fmt.Errorf("对象 [%s] 不在二次授权白名单中", objectName)
	}
	return nil
}

// DelSecondPriByObject 删除用户/分组时清理二次授权白名单中的引用
func DelSecondPriByObject(objectType, objectName string) error {
	return store.DB().Where("svn_object_type = ? AND svn_object_name = ?", objectType, objectName).
		Delete(&model.SvnSecondPri{}).Error
}

// DelSecondPriByOwner 删除用户时清理其名下全部二次授权记录
func DelSecondPriByOwner(owner string) error {
	return store.DB().Where("second_pri_owner = ?", owner).Delete(&model.SvnSecondPri{}).Error
}
