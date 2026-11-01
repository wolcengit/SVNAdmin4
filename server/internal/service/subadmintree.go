// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"svnadmin/internal/model"
	"svnadmin/internal/store"
)

// FuncNode 子管理员功能权限树节点
type FuncNode struct {
	Title             string     `json:"title"`
	Expand            bool       `json:"expand"`
	Checked           bool       `json:"checked"`
	Disabled          bool       `json:"disabled"`
	RouterName        string     `json:"routerName,omitempty"`
	NecessaryFunction []string   `json:"necessaryFunctions"`
	Children          []FuncNode `json:"children,omitempty"`
}

// DefaultSubadminTree 内置的功能权限树定义
// necessaryFunctions 中每一项对应一个后端接口，子管理员鉴权时按此白名单放行
func DefaultSubadminTree() []FuncNode {
	return []FuncNode{
		{
			Title: "后台任务", Expand: true, Checked: true, Disabled: true, RouterName: "task",
			NecessaryFunction: []string{"task/list", "task/detail", "task/run", "task/log", "task/stop", "task/del", "task/clear"},
		},
		{
			Title: "信息统计", Expand: true, Checked: false, RouterName: "dashboard",
			NecessaryFunction: []string{"common/statistics", "common/loadInfo", "common/diskInfo"},
		},
		{
			Title: "SVN 仓库", Expand: true, Checked: false, RouterName: "repository",
			NecessaryFunction: []string{"rep/list", "rep/all", "rep/detail"},
			Children: []FuncNode{
				{Title: "新建仓库", NecessaryFunction: []string{"rep/create"}},
				{Title: "authz 检测", NecessaryFunction: []string{"rep/checkAuthz"}},
				{Title: "修改备注", NecessaryFunction: []string{"rep/updNote"}},
				{Title: "修改名称", NecessaryFunction: []string{"rep/updName"}},
				{Title: "删除仓库", NecessaryFunction: []string{"rep/del"}},
				{Title: "内容浏览", NecessaryFunction: []string{"rep/con", "rep/tree", "rep/checkout"}},
				{Title: "目录管理", NecessaryFunction: []string{"rep/createFolder"}},
				{Title: "权限管理", NecessaryFunction: []string{"pri/list", "pri/create", "pri/update", "pri/delete"}},
				{Title: "备份管理", NecessaryFunction: []string{"backup/list", "backup/dump", "backup/load", "backup/del", "backup/download", "backup/upload"}},
				{Title: "钩子管理", NecessaryFunction: []string{"hook/list", "hook/update", "hook/del", "hook/recommend"}},
				{Title: "同步信息", NecessaryFunction: []string{"rep/syncSize", "rep/syncRev", "rep/setUUID"}},
			},
		},
		{
			Title: "SVN 用户", Expand: true, Checked: false, RouterName: "user",
			NecessaryFunction: []string{"user/list", "user/all", "user/scan", "user/import", "user/updNote"},
			Children: []FuncNode{
				{Title: "新建用户", NecessaryFunction: []string{"user/create"}},
				{Title: "删除用户", NecessaryFunction: []string{"user/del"}},
				{Title: "修改密码", NecessaryFunction: []string{"user/updPass"}},
				{Title: "启用禁用", NecessaryFunction: []string{"user/updStatus"}},
			},
		},
		{
			Title: "SVN 分组", Expand: true, Checked: false, RouterName: "group",
			NecessaryFunction: []string{"group/list", "group/all", "group/member", "group/updNote"},
			Children: []FuncNode{
				{Title: "新建分组", NecessaryFunction: []string{"group/create"}},
				{Title: "删除分组", NecessaryFunction: []string{"group/del"}},
				{Title: "修改名称", NecessaryFunction: []string{"group/updName"}},
				{Title: "成员管理", NecessaryFunction: []string{"group/updMember"}},
			},
		},
		{
			Title: "系统日志", Expand: true, Checked: false, RouterName: "logs",
			NecessaryFunction: []string{"log/list", "log/types"},
			Children: []FuncNode{
				{Title: "清空日志", NecessaryFunction: []string{"log/clear"}},
			},
		},
		{
			Title: "任务计划", Expand: true, Checked: false, RouterName: "crond",
			NecessaryFunction: []string{"crond/list", "crond/dict"},
			Children: []FuncNode{
				{Title: "新建计划", NecessaryFunction: []string{"crond/create"}},
				{Title: "修改计划", NecessaryFunction: []string{"crond/update"}},
				{Title: "删除计划", NecessaryFunction: []string{"crond/del"}},
				{Title: "手动执行", NecessaryFunction: []string{"crond/trigger"}},
				{Title: "执行日志", NecessaryFunction: []string{"crond/log"}},
			},
		},
		{
			Title: "系统配置", Expand: true, Checked: false, RouterName: "setting",
			NecessaryFunction: []string{"setting/list", "setting/dirInfo", "setting/svnInfo"},
			Children: []FuncNode{
				{Title: "修改配置", NecessaryFunction: []string{"setting/update"}},
				{Title: "svnserve 启停", NecessaryFunction: []string{"setting/svnserveStart", "setting/svnserveStop"}},
			},
		},
	}
}

// collectFunctions 递归收集被勾选节点的接口白名单。
// 与原版 GetPriFunctions 语义一致：子节点勾选时，父节点的 necessaryFunctions
// 也会一并加入白名单，保证页面基础功能可用（父子联动补齐）。
func collectFunctions(nodes []FuncNode) []string {
	var out []string
	seen := map[string]bool{}
	// walk 返回该层是否存在被勾选的节点
	var walk func([]FuncNode) bool
	walk = func(list []FuncNode) bool {
		anyChecked := false
		for _, n := range list {
			childChecked := false
			if len(n.Children) > 0 {
				childChecked = walk(n.Children)
			}
			// 自身勾选（disabled 节点视为强制勾选）或子节点有勾选 → 纳入自身函数
			if n.Checked || n.Disabled || childChecked {
				anyChecked = true
				for _, f := range n.NecessaryFunction {
					if f != "" && !seen[f] {
						seen[f] = true
						out = append(out, f)
					}
				}
			}
		}
		return anyChecked
	}
	walk(nodes)
	return out
}

// AlwaysFunctions 始终放行的接口：与权限树无关的基础能力，
// 子管理员登录后必须可用（否则无法登出、无法查看统计、无法使用后台任务页面）。
func AlwaysFunctions() []string {
	return []string{
		"common/logout", "common/userInfo", "common/version", "common/verifyOption",
		"common/svnserveStatus", "common/statistics", "common/monitor",
		"common/loadInfo", "common/diskInfo", "common/updatePass",
		"rep/all", "user/all", "group/all", "aliase/list", "rep/checkout",
		"task/list", "task/detail", "task/run", "task/log", "task/types",
		"crond/dict", "setting/dirInfo", "setting/svnInfo", "setting/version", "setting/checkUpdate",
	}
}

// FuncKey 由 gin 的完整路由推导接口键：/api/rep/list -> rep/list
func FuncKey(path string) string {
	p := strings.TrimPrefix(path, "/api/")
	p = strings.TrimPrefix(p, "/")
	return strings.TrimSuffix(p, "/")
}

// SubadminCanAccess 判定子管理员是否有权访问指定接口键
func SubadminCanAccess(id int64, key string) bool {
	if key == "" {
		return true
	}
	for _, f := range AlwaysFunctions() {
		if f == key {
			return true
		}
	}
	funcs := SubadminFunctions(id)
	if funcs == nil {
		return false
	}
	return funcs[key]
}

// applyChecked 将已保存的接口白名单映射回树节点的勾选状态
func applyChecked(nodes []FuncNode, funcs map[string]bool) []FuncNode {
	out := make([]FuncNode, len(nodes))
	copy(out, nodes)
	for i := range out {
		if out[i].Disabled {
			out[i].Checked = true
		} else {
			out[i].Checked = false
			for _, f := range out[i].NecessaryFunction {
				if funcs[f] {
					out[i].Checked = true
					break
				}
			}
		}
		if len(out[i].Children) > 0 {
			out[i].Children = applyChecked(out[i].Children, funcs)
		}
	}
	return out
}

// SubadminTree 返回指定子管理员的权限树（含是否需要更新的标记）
func SubadminTree(id int64) (map[string]interface{}, error) {
	var sub model.Subadmin
	if err := store.DB().Where("subadmin_id = ?", id).First(&sub).Error; err != nil {
		return nil, errors.New("子管理员不存在")
	}
	tree := DefaultSubadminTree()
	needUpdate := false
	funcs := splitFuncs(sub.SubadminFunctions)

	// treeOld：已保存权限树的只读快照（原版语义：结构变化时前端可对照旧勾选状态重新授权）
	var treeOld interface{}
	if strings.TrimSpace(sub.SubadminTree) != "" {
		var saved []FuncNode
		if err := json.Unmarshal([]byte(sub.SubadminTree), &saved); err == nil {
			treeOld = saved
		}
	}

	if strings.TrimSpace(sub.SubadminTree) == "" {
		needUpdate = true
	} else {
		var saved []FuncNode
		if err := json.Unmarshal([]byte(sub.SubadminTree), &saved); err != nil {
			needUpdate = true
		} else {
			// 与内置树比对，节点结构发生变化时提示重新授权
			if treeSignature(saved) != treeSignature(tree) {
				needUpdate = true
			}
			tree = applyChecked(tree, funcs)
		}
	}
	return map[string]interface{}{
		"tree":           tree,
		"treeOld":        treeOld,
		"needUpdateTree": needUpdate,
		"functions":      keysOf(funcs),
	}, nil
}

// SubadminUpdTree 保存权限树并同步刷新接口白名单
func SubadminUpdTree(id int64, rawTree []interface{}, operator string) error {
	var sub model.Subadmin
	if err := store.DB().Where("subadmin_id = ?", id).First(&sub).Error; err != nil {
		return errors.New("子管理员不存在")
	}
	nodes, err := decodeTree(rawTree)
	if err != nil {
		return err
	}
	funcs := collectFunctions(nodes)
	data, err := json.Marshal(nodes)
	if err != nil {
		return err
	}
	if err := store.DB().Model(&model.Subadmin{}).Where("subadmin_id = ?", id).Updates(map[string]interface{}{
		"subadmin_tree":      string(data),
		"subadmin_functions": joinList(funcs),
	}).Error; err != nil {
		return err
	}
	InvalidateFuncCache(id)
	AddLog("子管理员", fmt.Sprintf("修改子管理员 ID=%d 的功能权限，共 %d 项", id, len(funcs)), operator)
	return nil
}

// decodeTree 将前端提交的通用结构转换为权限树
func decodeTree(raw []interface{}) ([]FuncNode, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var nodes []FuncNode
	if err := json.Unmarshal(data, &nodes); err != nil {
		return nil, errors.New("权限树格式不合法")
	}
	return nodes, nil
}

// treeSignature 生成树结构签名，用于判断内置树是否发生变化
func treeSignature(nodes []FuncNode) string {
	var b strings.Builder
	var walk func([]FuncNode)
	walk = func(list []FuncNode) {
		for _, n := range list {
			b.WriteString(n.Title + "|")
			b.WriteString(strings.Join(n.NecessaryFunction, "&") + ";")
			walk(n.Children)
		}
	}
	walk(nodes)
	return b.String()
}

func splitFuncs(s string) map[string]bool {
	out := map[string]bool{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out[p] = true
		}
	}
	return out
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// 白名单缓存，避免每次请求都查库（权限变更后由 InvalidateFuncCache 清除）
var (
	funcCacheMu   sync.RWMutex
	funcCache     = map[int64]funcCacheItem{}
	funcCacheTTL  = 30 * time.Second
	lastCacheScan = time.Now()
)

type funcCacheItem struct {
	funcs map[string]bool
	exp   time.Time
}

// SubadminFunctions 返回子管理员的接口白名单（供中间件使用）
func SubadminFunctions(id int64) map[string]bool {
	now := time.Now()
	funcCacheMu.RLock()
	it, ok := funcCache[id]
	funcCacheMu.RUnlock()
	if ok && now.Before(it.exp) {
		return it.funcs
	}
	funcs := loadSubadminFunctions(id)
	funcCacheMu.Lock()
	funcCache[id] = funcCacheItem{funcs: funcs, exp: now.Add(funcCacheTTL)}
	// 顺带清理过期项
	if now.Sub(lastCacheScan) > time.Minute {
		for k, v := range funcCache {
			if now.After(v.exp) {
				delete(funcCache, k)
			}
		}
		lastCacheScan = now
	}
	funcCacheMu.Unlock()
	return funcs
}

// InvalidateFuncCache 权限变更后清除缓存
func InvalidateFuncCache(id int64) {
	funcCacheMu.Lock()
	delete(funcCache, id)
	funcCacheMu.Unlock()
}

func loadSubadminFunctions(id int64) map[string]bool {
	var sub model.Subadmin
	if err := store.DB().Where("subadmin_id = ?", id).First(&sub).Error; err != nil {
		return nil
	}
	if strings.TrimSpace(sub.SubadminFunctions) == "" {
		// 未配置时按内置树的默认勾选（后台任务）
		return splitFuncs(joinList(collectFunctions(DefaultSubadminTree())))
	}
	return splitFuncs(sub.SubadminFunctions)
}

// KeysOfFuncs 返回子管理员白名单的键列表（供 UserInfo 接口使用）
func KeysOfFuncs(id int64) []string { return keysOf(SubadminFunctions(id)) }
