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
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gorm.io/gorm"
	"svnadmin/internal/config"
	"svnadmin/internal/model"
	"svnadmin/internal/store"
	"svnadmin/internal/svn"
	"svnadmin/internal/util"
)

// ---------------------------------------------------------------------------
// 仓库列表与同步
// ---------------------------------------------------------------------------

// SyncReps 扫描磁盘目录，将仓库同步到数据库
func SyncReps() error {
	names, err := svn.ListRepDirs()
	if err != nil {
		return err
	}
	for _, name := range names {
		var r model.SvnRep
		if err := store.DB().Where("rep_name = ?", name).First(&r).Error; err != nil {
			store.DB().Create(&model.SvnRep{
				RepName: name,
				RepSize: svn.RepSize(name),
				RepRev:  svn.RepYoungest(name),
				RepUUID: svn.RepUUID(name),
			})
		}
	}
	// 清理磁盘上已不存在的记录
	var all []model.SvnRep
	_ = store.DB().Find(&all).Error
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	for _, r := range all {
		if !set[r.RepName] {
			store.DB().Delete(&r)
		}
	}
	// 物理仓库 ↔ authz 双向同步（对齐原版 SyncRep2Authz）：
	// 物理存在但 authz 无对应段 → 补写 [rep:/]；authz 有段但物理仓库已不存在 → 清理该段
	SyncRep2Authz(names)
	return nil
}

// SyncRep2Authz 保证 authz 中的仓库段与磁盘上的物理仓库一致
func SyncRep2Authz(names []string) {
	az, err := svn.LoadAuthz()
	if err != nil {
		return
	}
	changed := false
	// 1. 补齐缺失段
	for _, name := range names {
		if az.FindSection(name, "/") == nil {
			az.EnsureSection(name, "/")
			changed = true
		}
	}
	// 2. 清理失效段
	exist := map[string]bool{}
	for _, n := range names {
		exist[n] = true
	}
	seen := map[string]bool{}
	for _, sec := range az.Sections {
		if sec.Repo == "" || exist[sec.Repo] || seen[sec.Repo] {
			continue
		}
		seen[sec.Repo] = true
		az.DropRepo(sec.Repo)
		changed = true
	}
	if changed {
		_ = az.Save()
	}
}

// refreshRep 刷新单个仓库的体积与版本数
func refreshRep(name string) {
	store.DB().Model(&model.SvnRep{}).Where("rep_name = ?", name).Updates(map[string]interface{}{
		"rep_size": svn.RepSize(name),
		"rep_rev":  svn.RepYoungest(name),
		"rep_uuid": svn.RepUUID(name),
	})
}

// RepList 分页查询仓库
func RepList(page, pageSize int, keyword, sortName, sortType string) (int64, []model.SvnRep, error) {
	if err := SyncReps(); err != nil {
		return 0, nil, err
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 1000 {
		pageSize = 20
	}
	q := store.DB().Model(&model.SvnRep{})
	if keyword != "" {
		q = q.Where("rep_name LIKE ? OR rep_note LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	var list []model.SvnRep
	order := orderClause(sortName, sortType, map[string]string{
		"rep_id":   "rep_id",
		"repId":    "rep_id",
		"rep_name": "rep_name",
		"repName":  "rep_name",
		"rep_size": "rep_size",
		"repSize":  "rep_size",
		"rep_rev":  "rep_rev",
		"repRev":   "rep_rev",
		"rep_note": "rep_note",
	}, "rep_id DESC")
	if err := q.Order(order).Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return 0, nil, err
	}
	return total, list, nil
}

// RepAll 返回全部仓库
func RepAll() ([]model.SvnRep, error) {
	if err := SyncReps(); err != nil {
		return nil, err
	}
	var list []model.SvnRep
	err := store.DB().Order("rep_name").Find(&list).Error
	return list, err
}

// ---------------------------------------------------------------------------
// 仓库增删改
// ---------------------------------------------------------------------------

func validRepName(name string) error {
	if name == "" {
		return errors.New("仓库名不能为空")
	}
	for _, c := range name {
		switch c {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ', '#', ';', '[', ']':
			return errors.New("仓库名不能包含空格或 / \\ : * ? \" < > | # ; [ ] 等字符")
		}
	}
	return nil
}

// CreateRep 新建仓库
func CreateRep(name, note, initStruct, operator string) error {
	if err := validRepName(name); err != nil {
		return err
	}
	if svn.RepExists(name) {
		return fmt.Errorf("仓库 [%s] 已存在", name)
	}
	// 未指定模板时不套用（对齐原版 rep_type=1 空仓库）；配置了 rep_init_default 时按其值兜底
	if initStruct == "" {
		initStruct = GetOption("rep_init_default", "")
	}
	if err := EnsureAuthzFile(); err != nil {
		return err
	}
	if err := svn.CreateRep(name, initStruct); err != nil {
		return err
	}
	store.DB().Create(&model.SvnRep{
		RepName: name,
		RepNote: note,
		RepSize: svn.RepSize(name),
		RepRev:  svn.RepYoungest(name),
		RepUUID: svn.RepUUID(name),
	})
	// 对齐原版：新建仓库即在 authz 中写入 [rep:/] 段
	if az, err := svn.LoadAuthz(); err == nil {
		if az.FindSection(name, "/") == nil {
			az.EnsureSection(name, "/")
			_ = az.Save()
		}
	}
	AddLog("新建仓库", fmt.Sprintf("新建仓库 [%s]", name), operator)
	return nil
}

// DelRep 删除仓库
func DelRep(name, operator string) error {
	if !svn.RepExists(name) {
		return fmt.Errorf("仓库 [%s] 不存在", name)
	}
	if err := svn.DelRep(name); err != nil {
		return err
	}
	authz, err := svn.LoadAuthz()
	if err == nil {
		authz.DropRepo(name)
		_ = authz.Save()
	}
	store.DB().Where("rep_name = ?", name).Delete(&model.SvnRep{})
	AddLog("删除仓库", fmt.Sprintf("删除仓库 [%s]", name), operator)
	return nil
}

// UpdRepName 修改仓库名称
func UpdRepName(oldName, newName, operator string) error {
	if err := validRepName(newName); err != nil {
		return err
	}
	if !svn.RepExists(oldName) {
		return fmt.Errorf("仓库 [%s] 不存在", oldName)
	}
	if svn.RepExists(newName) {
		return fmt.Errorf("仓库 [%s] 已存在", newName)
	}
	if err := svn.RenameRep(oldName, newName); err != nil {
		return err
	}
	authz, err := svn.LoadAuthz()
	if err == nil {
		authz.RenameRepo(oldName, newName)
		_ = authz.Save()
	}
	store.DB().Model(&model.SvnRep{}).Where("rep_name = ?", oldName).Update("rep_name", newName)
	AddLog("修改仓库", fmt.Sprintf("仓库 [%s] 更名为 [%s]", oldName, newName), operator)
	return nil
}

// UpdRepNote 修改仓库备注
func UpdRepNote(name, note, operator string) error {
	res := store.DB().Model(&model.SvnRep{}).Where("rep_name = ?", name).Update("rep_note", note)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		store.DB().Create(&model.SvnRep{RepName: name, RepNote: note})
	}
	AddLog("修改仓库", fmt.Sprintf("修改仓库 [%s] 的备注", name), operator)
	return nil
}

// GetRepDetail 获取仓库属性明细
func GetRepDetail(name string) (map[string]interface{}, error) {
	if !svn.RepExists(name) {
		return nil, fmt.Errorf("仓库 [%s] 不存在", name)
	}
	detail := svn.RepDetail(name)
	var r model.SvnRep
	if err := store.DB().Where("rep_name = ?", name).First(&r).Error; err == nil {
		detail["repNote"] = r.RepNote
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return detail, nil
}

// ResetUUID 重设仓库 UUID（newUUID 为空时随机生成，非空时设置为指定值）
func ResetUUID(name, newUUID, operator string) (string, error) {
	if !svn.RepExists(name) {
		return "", fmt.Errorf("仓库 [%s] 不存在", name)
	}
	uuid, err := svn.SetUUID(name, newUUID)
	if err != nil {
		return "", err
	}
	store.DB().Model(&model.SvnRep{}).Where("rep_name = ?", name).Update("rep_uuid", uuid)
	AddLog("修改仓库", fmt.Sprintf("重设仓库 [%s] 的 UUID", name), operator)
	return uuid, nil
}

// SyncRepSize 同步仓库体积
func SyncRepSize(name string) (int64, error) {
	if !svn.RepExists(name) {
		return 0, fmt.Errorf("仓库 [%s] 不存在", name)
	}
	size := svn.RepSize(name)
	store.DB().Model(&model.SvnRep{}).Where("rep_name = ?", name).Update("rep_size", size)
	return size, nil
}

// SyncRepRev 同步仓库版本数
func SyncRepRev(name string) (int64, error) {
	if !svn.RepExists(name) {
		return 0, fmt.Errorf("仓库 [%s] 不存在", name)
	}
	rev := svn.RepYoungest(name)
	store.DB().Model(&model.SvnRep{}).Where("rep_name = ?", name).Update("rep_rev", rev)
	return rev, nil
}

// ---------------------------------------------------------------------------
// 目录树与在线操作
// ---------------------------------------------------------------------------

// GetRepTree 获取仓库目录树
func GetRepTree(name, path string) ([]*svn.TreeNode, error) {
	if !svn.RepExists(name) {
		return nil, fmt.Errorf("仓库 [%s] 不存在", name)
	}
	return svn.RepTree(name, path)
}

// GetRepCon 获取路径下的文件与目录
func GetRepCon(name, path string) ([]*svn.TreeNode, error) {
	if !svn.RepExists(name) {
		return nil, fmt.Errorf("仓库 [%s] 不存在", name)
	}
	return svn.RepCon(name, path)
}

// GetUserRepCon SVN 用户浏览仓库内容：走 svn 协议真实鉴权，只能看到有权限的路径
func GetUserRepCon(repName, path, userName string) (map[string]interface{}, error) {
	if !svn.RepExists(repName) {
		return nil, fmt.Errorf("仓库 [%s] 不存在", repName)
	}
	authz, err := svn.LoadAuthz()
	if err != nil {
		return nil, err
	}
	if !authz.UserCanAccess(repName, userName) {
		return nil, fmt.Errorf("用户 [%s] 无权访问仓库 [%s]", userName, repName)
	}
	pass, err := svnUserPassword(userName)
	if err != nil {
		return nil, err
	}
	nodes, err := svn.SvnListAuth(repName, cleanPathSvc(path), userName, pass)
	if err != nil {
		if isAuthError(err) {
			return nil, fmt.Errorf("用户 [%s] 无权访问路径 [%s]", userName, path)
		}
		return nil, err
	}
	// 单文件授权时回退到上层目录
	breadPath := cleanPathSvc(path)
	if len(nodes) == 1 && nodes[0].Type == "file" && nodes[0].Name != "" {
		idx := strings.LastIndex(breadPath, "/")
		if idx > 0 {
			breadPath = breadPath[:idx]
		} else {
			breadPath = "/"
		}
	}
	return map[string]interface{}{
		"data": nodes,
		"bread": map[string]string{
			"path": breadPath,
			"name": pathBase(breadPath),
		},
	}, nil
}

// GetRepTree2 SVN 用户视角的目录树（二次授权 / 用户自助授权场景）
// first=true 时只返回到目标路径的单路径树
func GetRepTree2(repName, path, userName string, first bool) ([]*svn.TreeNode, error) {
	if !svn.RepExists(repName) {
		return nil, fmt.Errorf("仓库 [%s] 不存在", repName)
	}
	authz, err := svn.LoadAuthz()
	if err != nil {
		return nil, err
	}
	if !authz.UserCanAccess(repName, userName) {
		return nil, fmt.Errorf("用户 [%s] 无权访问仓库 [%s]", userName, repName)
	}
	pass, err := svnUserPassword(userName)
	if err != nil {
		return nil, err
	}
	p := cleanPathSvc(path)
	if first {
		nodes, err := svn.SvnListAuth(repName, p, userName, pass)
		if err != nil {
			return []*svn.TreeNode{}, nil
		}
		return nodes, nil
	}
	// 逐级展开到目标路径
	segments := []string{}
	for _, s := range strings.Split(strings.Trim(p, "/"), "/") {
		if s != "" {
			segments = append(segments, s)
		}
	}
	cur := "/"
	roots, err := svn.SvnListAuth(repName, cur, userName, pass)
	if err != nil {
		return nil, err
	}
	if len(segments) == 0 {
		return roots, nil
	}
	target := roots
	for _, seg := range segments {
		cur = cur + seg + "/"
		children, err := svn.SvnListAuth(repName, cur, userName, pass)
		if err != nil {
			break
		}
		for _, n := range target {
			if n.Name == seg {
				n.Children = children
				target = children
				break
			}
		}
	}
	return roots, nil
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	for _, code := range []string{"E170001", "E220003", "E170013", "W160013", "E215004", "Authorization failed", "认证失败"} {
		if strings.Contains(s, code) {
			return true
		}
	}
	return false
}

func pathBase(p string) string {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return "/"
	}
	idx := strings.LastIndex(p, "/")
	if idx >= 0 {
		return p[idx+1:]
	}
	return p
}

// svnUserPassword 读取 SVN 用户密码（用于 svn 协议鉴权）
func svnUserPassword(userName string) (string, error) {
	var u model.SvnUser
	if err := store.DB().Where("svn_user_name = ?", userName).First(&u).Error; err != nil {
		return "", fmt.Errorf("用户 [%s] 不存在", userName)
	}
	return u.SvnUserPass, nil
}

// CreateRepFolder 在线创建目录
func CreateRepFolder(name, path, message, operator string) error {
	if !svn.RepExists(name) {
		return fmt.Errorf("仓库 [%s] 不存在", name)
	}
	if err := svn.CreateFolder(name, path, message, operator); err != nil {
		return err
	}
	AddLog("创建目录", fmt.Sprintf("在仓库 [%s] 创建目录 [%s]", name, path), operator)
	return nil
}

// CheckAuthz 校验 authz 文件是否可被正常解析
func CheckAuthz() error {
	authz, err := svn.LoadAuthz()
	if err != nil {
		return err
	}
	if len(authz.Sections) == 0 {
		return errors.New("authz 文件中未配置任何仓库路径权限")
	}
	return nil
}

// ---------------------------------------------------------------------------
// 路径权限
// ---------------------------------------------------------------------------

// PriItem 权限项展示结构
type PriItem struct {
	RepName     string `json:"repName"`
	PriPath     string `json:"priPath"`
	ObjectType  string `json:"objectType"`
	ObjectName  string `json:"objectName"`
	Perm        string `json:"perm"`
	Unique      string `json:"unique"`
	InheritPath string `json:"inheritPath"`
	Invert      bool   `json:"invert"`
}

func objectType(name string) string {
	switch {
	case name == "*":
		return "all"
	case name == "$authenticated":
		return "authenticated"
	case name == "$anonymous":
		return "anonymous"
	case strings.HasPrefix(name, "@"):
		return "group"
	case strings.HasPrefix(name, "&"):
		return "aliase"
	}
	return "user"
}

// GetRepPathAllPri 获取某仓库某路径下的权限列表（含继承来源）
func GetRepPathAllPri(repName, path string) (map[string]interface{}, error) {
	authz, err := svn.LoadAuthz()
	if err != nil {
		return nil, err
	}
	items := []PriItem{}
	inherit := []PriItem{}
	for _, sec := range authz.Sections {
		if sec.Repo != repName {
			continue
		}
		for _, e := range sec.Entries {
			it := PriItem{
				RepName:    repName,
				PriPath:    sec.Path,
				ObjectType: objectType(e.Name),
				ObjectName: e.Name,
				Perm:       e.Perm,
				Unique:     util.Unique(repName, sec.Path, e.Name),
				Invert:     e.Invert,
			}
			if sec.Path == cleanPathSvc(path) {
				items = append(items, it)
			} else if inherits(sec.Path, cleanPathSvc(path)) {
				it.InheritPath = sec.Path
				inherit = append(inherit, it)
			}
		}
	}
	return map[string]interface{}{
		"current": items,
		"inherit": inherit,
	}, nil
}

func inherits(parent, child string) bool {
	if parent == "/" {
		return child != "/"
	}
	return strings.HasPrefix(child, parent+"/")
}

func cleanPathSvc(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}

// CreateRepPathPri 新增路径权限；invert 表示取反（除该对象之外的所有人）
func CreateRepPathPri(repName, path, objType, objName, perm, operator string, invert bool) error {
	authz, err := svn.LoadAuthz()
	if err != nil {
		return err
	}
	name := formatObject(objType, objName)
	if name == "*" {
		invert = false
	}
	authz.SetPathPri(repName, cleanPathSvc(path), name, perm, invert)
	if err := authz.Save(); err != nil {
		return err
	}
	AddLog("权限管理", fmt.Sprintf("为仓库 [%s] 路径 [%s] 增加 [%s] 的权限 [%s]", repName, path, name, perm), operator)
	return nil
}

// UpdRepPathPri 修改路径权限；invert 表示取反
func UpdRepPathPri(repName, path, objName, perm, operator string, invert bool) error {
	authz, err := svn.LoadAuthz()
	if err != nil {
		return err
	}
	if objName == "*" {
		invert = false
	}
	authz.SetPathPri(repName, cleanPathSvc(path), objName, perm, invert)
	if err := authz.Save(); err != nil {
		return err
	}
	AddLog("权限管理", fmt.Sprintf("修改仓库 [%s] 路径 [%s] 中 [%s] 的权限为 [%s]", repName, path, objName, perm), operator)
	return nil
}

// DelRepPathPri 删除路径权限
func DelRepPathPri(repName, path, objName, operator string) error {
	authz, err := svn.LoadAuthz()
	if err != nil {
		return err
	}
	authz.DelPathPri(repName, cleanPathSvc(path), objName)
	if err := authz.Save(); err != nil {
		return err
	}
	AddLog("权限管理", fmt.Sprintf("删除仓库 [%s] 路径 [%s] 中 [%s] 的权限", repName, path, objName), operator)
	return nil
}

func formatObject(objType, objName string) string {
	switch objType {
	case "group":
		return "@" + objName
	case "aliase":
		return "&" + objName
	case "all":
		return "*"
	case "authenticated":
		return "$authenticated"
	case "anonymous":
		return "$anonymous"
	}
	return objName
}

// GetSvnUserRepList SVN 用户获取自己有权限的仓库
func GetSvnUserRepList(userName string) ([]map[string]interface{}, error) {
	authz, err := svn.LoadAuthz()
	if err != nil {
		return nil, err
	}
	reps, err := RepAll()
	if err != nil {
		return nil, err
	}
	out := []map[string]interface{}{}
	for _, r := range reps {
		perm := authz.EffectivePerm(r.RepName, "/", userName)
		if svn.PermLevel(perm) == 0 {
			continue
		}
		paths := authz.UserPermPaths(r.RepName, userName)
		sort.Strings(paths)
		out = append(out, map[string]interface{}{
			"repName": r.RepName,
			"repNote": r.RepNote,
			"repRev":  r.RepRev,
			"repSize": r.RepSize,
			"perm":    perm,
			"paths":   paths,
		})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// 备份
// ---------------------------------------------------------------------------

// BackupFile 备份文件信息
type BackupFile struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	SizeHr  string `json:"sizeHr"`
	ModTime string `json:"modTime"`
	RepName string `json:"repName"`
	// Type 备份产物类型：dump（svnadmin dump 文件）或 hotcopy（svnadmin hotcopy 目录）
	Type string `json:"type"`
}

// BackupList 列出备份目录下的备份产物（dump 文件 + hotcopy 目录）
func BackupList(repName string) ([]BackupFile, error) {
	dir := configBackupDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupFile{}, nil
		}
		return nil, err
	}
	out := []BackupFile{}
	for _, e := range entries {
		name := e.Name()
		if repName != "" && !strings.HasPrefix(name, repName+"_") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		owner := ""
		if i := strings.Index(name, "_"); i > 0 {
			owner = name[:i]
		}
		bf := BackupFile{
			Name:    name,
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
			RepName: owner,
			Type:    "dump",
		}
		if e.IsDir() {
			// hotcopy 产物为目录，体积按递归累加
			bf.Type = "hotcopy"
			var total int64
			_ = filepath.Walk(filepath.Join(dir, name), func(_ string, fi os.FileInfo, err error) error {
				if err == nil && fi != nil && !fi.IsDir() {
					total += fi.Size()
				}
				return nil
			})
			bf.Size = total
		} else {
			bf.Size = info.Size()
		}
		bf.SizeHr = util.FmtSize(bf.Size)
		out = append(out, bf)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime > out[j].ModTime })
	return out, nil
}

func configBackupDir() string { return config.C().SVN.BackupBasePath() }

// BackupPath 返回备份文件绝对路径（带目录穿越防护）
func BackupPath(name string) (string, error) {
	if strings.Contains(name, "/") || strings.Contains(name, "..") {
		return "", errors.New("非法的文件名")
	}
	return filepath.Join(configBackupDir(), name), nil
}

// DelBackup 删除备份文件
func DelBackup(name, operator string) error {
	p, err := BackupPath(name)
	if err != nil {
		return err
	}
	// hotcopy 备份为目录，需递归删除；dump 备份为单文件
	if err := os.RemoveAll(p); err != nil {
		return err
	}
	AddLog("删除备份", fmt.Sprintf("删除备份产物 [%s]", name), operator)
	return nil
}

// ---------------------------------------------------------------------------
// 钩子
// ---------------------------------------------------------------------------

// RepHooks 获取仓库钩子
func RepHooks(repName string) ([]*svn.Hook, error) {
	if !svn.RepExists(repName) {
		return nil, fmt.Errorf("仓库 [%s] 不存在", repName)
	}
	return svn.RepHooks(repName), nil
}

// UpdRepHook 修改仓库钩子
func UpdRepHook(repName, hookName, content, operator string) error {
	if err := svn.UpdRepHook(repName, hookName, content); err != nil {
		return err
	}
	AddLog("修改钩子", fmt.Sprintf("修改仓库 [%s] 的钩子 [%s]", repName, hookName), operator)
	return nil
}

// DelRepHook 删除仓库钩子
func DelRepHook(repName, hookName, operator string) error {
	if err := svn.DelRepHook(repName, hookName); err != nil {
		return err
	}
	AddLog("删除钩子", fmt.Sprintf("删除仓库 [%s] 的钩子 [%s]", repName, hookName), operator)
	return nil
}

// RecommendHooks 推荐钩子列表
func RecommendHooks() []*svn.Hook { return svn.RecommendHooks() }

// orderClause 生成安全的排序子句（对齐原版 sortName / sortType 能力）。
// 仅允许白名单内的字段，避免 SQL 注入；非法输入回落默认排序。
func orderClause(sortName, sortType string, allowed map[string]string, def string) string {
	col, ok := allowed[sortName]
	if !ok || col == "" {
		return def
	}
	dir := "ASC"
	if strings.EqualFold(sortType, "desc") {
		dir = "DESC"
	}
	return col + " " + dir
}
