// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package svn

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"svnadmin/internal/config"
)

// Entry authz 中的一条权限项，Name 形如 user / @group / &alias / * / $authenticated
// Invert 为 true 时表示取反（写回为 ~name），即"除该对象之外的所有人"
type Entry struct {
	Name   string `json:"name"`
	Perm   string `json:"perm"`
	Invert bool   `json:"invert"`
}

// RawName 返回写入 authz 文件时的原始名称（含取反前缀）
func (e *Entry) RawName() string {
	if e.Invert {
		return "~" + e.Name
	}
	return e.Name
}

// CanInvert 判断该对象类型是否支持取反（* 取反无意义）
func (e *Entry) CanInvert() bool { return e.Name != "*" }

// Section authz 中的一个路径段
type Section struct {
	Header  string   `json:"header"`  // 原始头部，如 rep:/trunk
	Repo    string   `json:"repo"`    // 仓库名，全局段为空
	Path    string   `json:"path"`    // 路径，始终以 / 开头
	Entries []*Entry `json:"entries"` // 权限项列表
	Comment []string `json:"-"`       // 段前注释
}

// Authz authz 文件的内存表示
type Authz struct {
	Path       string              `json:"-"`
	Head       []string            `json:"-"` // 文件头注释
	Aliases    map[string]string   `json:"aliases"`
	AliasOrder []string            `json:"-"`
	Groups     map[string][]string `json:"groups"`
	GroupOrder []string            `json:"-"`
	Sections   []*Section          `json:"sections"`

	index map[string]*Section
	mu    sync.Mutex
}

// LoadAuthz 读取并解析 authz 文件，文件不存在时返回空结构
func LoadAuthz() (*Authz, error) {
	p := config.C().SVN.AuthzFile()
	a := &Authz{
		Path:       p,
		Aliases:    map[string]string{},
		Groups:     map[string][]string{},
		index:      map[string]*Section{},
		AliasOrder: []string{},
		GroupOrder: []string{},
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return a, nil
		}
		return nil, err
	}
	a.parse(string(data))
	return a, nil
}

func (a *Authz) parse(content string) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var pending []string
	var cur *Section
	special := ""

	flushPending := func() {
		pending = nil
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			pending = append(pending, raw)
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			header := strings.TrimSpace(line[1 : len(line)-1])
			lower := strings.ToLower(header)
			switch lower {
			case "groups":
				special = "groups"
				cur = nil
				flushPending()
				continue
			case "aliases":
				special = "aliases"
				cur = nil
				flushPending()
				continue
			}
			special = ""
			sec := &Section{Header: header, Comment: pending}
			sec.Repo, sec.Path = splitHeader(header)
			a.Sections = append(a.Sections, sec)
			a.index[strings.ToLower(header)] = sec
			cur = sec
			flushPending()
			continue
		}

		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" {
			continue
		}
		switch special {
		case "groups":
			if _, ok := a.Groups[key]; !ok {
				a.GroupOrder = append(a.GroupOrder, key)
			}
			a.Groups[key] = splitList(val)
			flushPending()
		case "aliases":
			if _, ok := a.Aliases[key]; !ok {
				a.AliasOrder = append(a.AliasOrder, key)
			}
			a.Aliases[key] = val
			flushPending()
		default:
			if cur != nil {
				ent := &Entry{Name: key, Perm: normalizePerm(val)}
				if strings.HasPrefix(ent.Name, "~") {
					ent.Invert = true
					ent.Name = strings.TrimSpace(strings.TrimPrefix(ent.Name, "~"))
				}
				cur.Entries = append(cur.Entries, ent)
			}
			flushPending()
		}
	}
	if len(pending) > 0 && len(a.Sections) == 0 {
		a.Head = pending
	}
}

func splitHeader(header string) (repo, path string) {
	if i := strings.Index(header, ":"); i >= 0 {
		repo = header[:i]
		path = header[i+1:]
	} else {
		path = header
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimSpace(repo), strings.TrimSpace(path)
}

func splitList(v string) []string {
	var out []string
	for _, item := range strings.Split(v, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func normalizePerm(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, `"`)
	var r, w bool
	for _, ch := range v {
		switch ch {
		case 'r', 'R':
			r = true
		case 'w', 'W':
			w = true
		}
	}
	switch {
	case r && w:
		return "rw"
	case r:
		return "r"
	case w:
		return "w"
	}
	return ""
}

// Save 将内存结构写回 authz 文件
func (a *Authz) Save() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.Path == "" {
		a.Path = config.C().SVN.AuthzFile()
	}
	var sb strings.Builder
	for _, l := range a.Head {
		sb.WriteString(l + "\n")
	}
	if len(a.AliasOrder) > 0 {
		sb.WriteString("\n[aliases]\n")
		for _, k := range a.AliasOrder {
			sb.WriteString(fmt.Sprintf("%s = %s\n", k, a.Aliases[k]))
		}
	}
	sb.WriteString("\n[groups]\n")
	for _, k := range a.GroupOrder {
		sb.WriteString(fmt.Sprintf("%s = %s\n", k, strings.Join(a.Groups[k], ", ")))
	}
	for _, sec := range a.Sections {
		sb.WriteString("\n")
		for _, c := range sec.Comment {
			sb.WriteString(c + "\n")
		}
		sb.WriteString(fmt.Sprintf("[%s]\n", sec.Header))
		for _, e := range sec.Entries {
			sb.WriteString(fmt.Sprintf("%s = %s\n", e.RawName(), e.Perm))
		}
	}
	if err := os.MkdirAll(dirOf(a.Path), 0755); err != nil {
		return err
	}
	return os.WriteFile(a.Path, []byte(sb.String()), 0644)
}

func dirOf(p string) string {
	if i := strings.LastIndex(p, "/"); i > 0 {
		return p[:i]
	}
	return "."
}

// ---------------------------------------------------------------------------
// 分组操作
// ---------------------------------------------------------------------------

// HasGroup 判断分组是否存在
func (a *Authz) HasGroup(name string) bool {
	_, ok := a.Groups[name]
	return ok
}

// AddGroup 新增分组
func (a *Authz) AddGroup(name string, members []string) {
	if _, ok := a.Groups[name]; !ok {
		a.GroupOrder = append(a.GroupOrder, name)
	}
	a.Groups[name] = members
}

// DelGroup 删除分组，同时清理其它分组中的引用
func (a *Authz) DelGroup(name string) {
	delete(a.Groups, name)
	for i, g := range a.GroupOrder {
		if g == name {
			a.GroupOrder = append(a.GroupOrder[:i], a.GroupOrder[i+1:]...)
			break
		}
	}
	ref := "@" + name
	for g, members := range a.Groups {
		var keep []string
		for _, m := range members {
			if m != ref {
				keep = append(keep, m)
			}
		}
		a.Groups[g] = keep
	}
	for _, sec := range a.Sections {
		var keep []*Entry
		for _, e := range sec.Entries {
			if e.Name != ref {
				keep = append(keep, e)
			}
		}
		sec.Entries = keep
	}
}

// RenameGroup 重命名分组
func (a *Authz) RenameGroup(oldName, newName string) {
	members, ok := a.Groups[oldName]
	if !ok {
		return
	}
	delete(a.Groups, oldName)
	a.Groups[newName] = members
	for i, g := range a.GroupOrder {
		if g == oldName {
			a.GroupOrder[i] = newName
			break
		}
	}
	oldRef, newRef := "@"+oldName, "@"+newName
	for g, ms := range a.Groups {
		for i, m := range ms {
			if m == oldRef {
				ms[i] = newRef
			}
		}
		a.Groups[g] = ms
	}
	for _, sec := range a.Sections {
		for _, e := range sec.Entries {
			if e.Name == oldRef {
				e.Name = newRef
			}
		}
	}
}

// GroupMembers 返回分组的直接成员
func (a *Authz) GroupMembers(name string) []string {
	return a.Groups[name]
}

// ResolveUserGroups 递归求解用户所属的全部分组
func (a *Authz) ResolveUserGroups(userName string) map[string]bool {
	got := map[string]bool{}
	var walk func(g string, depth int)
	walk = func(g string, depth int) {
		if depth > 16 || got[g] {
			return
		}
		got[g] = true
		for _, m := range a.Groups[g] {
			if strings.HasPrefix(m, "@") {
				walk(strings.TrimPrefix(m, "@"), depth+1)
			}
		}
	}
	for g, members := range a.Groups {
		for _, m := range members {
			if m == userName {
				walk(g, 0)
			}
		}
	}
	return got
}

// ---------------------------------------------------------------------------
// 路径权限操作
// ---------------------------------------------------------------------------

// FindSection 查找指定仓库路径对应的段
func (a *Authz) FindSection(repName, path string) *Section {
	header := repName + ":" + cleanPath(path)
	if sec, ok := a.index[strings.ToLower(header)]; ok {
		return sec
	}
	return nil
}

// EnsureSection 查找或创建指定仓库路径的段
func (a *Authz) EnsureSection(repName, path string) *Section {
	if sec := a.FindSection(repName, path); sec != nil {
		return sec
	}
	header := repName + ":" + cleanPath(path)
	sec := &Section{Header: header, Repo: repName, Path: cleanPath(path)}
	a.Sections = append(a.Sections, sec)
	a.index[strings.ToLower(header)] = sec
	return sec
}

// GetPathAllPri 获取某个仓库路径下的全部权限项
func (a *Authz) GetPathAllPri(repName, path string) []*Entry {
	if sec := a.FindSection(repName, path); sec != nil {
		return sec.Entries
	}
	return nil
}

// SetPathPri 设置某个仓库路径下对象的权限；invert 表示取反（~）
func (a *Authz) SetPathPri(repName, path, name, perm string, invert bool) {
	if name == "*" {
		invert = false
	}
	sec := a.EnsureSection(repName, path)
	for _, e := range sec.Entries {
		if e.Name == name {
			e.Perm = normalizePerm(perm)
			e.Invert = invert
			return
		}
	}
	sec.Entries = append(sec.Entries, &Entry{Name: name, Perm: normalizePerm(perm), Invert: invert})
}

// DelPathPri 删除某个仓库路径下对象的权限
func (a *Authz) DelPathPri(repName, path, name string) {
	sec := a.FindSection(repName, path)
	if sec == nil {
		return
	}
	name = strings.TrimPrefix(name, "~")
	var keep []*Entry
	for _, e := range sec.Entries {
		if e.Name != name {
			keep = append(keep, e)
		}
	}
	sec.Entries = keep
}

// DelObjectPri 删除某对象在所有路径上的权限（删除用户/分组时调用）
func (a *Authz) DelObjectPri(name string) {
	name = strings.TrimPrefix(name, "~")
	for _, sec := range a.Sections {
		var keep []*Entry
		for _, e := range sec.Entries {
			if e.Name != name {
				keep = append(keep, e)
			}
		}
		sec.Entries = keep
	}
}

// DropRepo 删除仓库对应的全部路径段
func (a *Authz) DropRepo(repName string) {
	var keep []*Section
	for _, sec := range a.Sections {
		if sec.Repo == repName {
			delete(a.index, strings.ToLower(sec.Header))
			continue
		}
		keep = append(keep, sec)
	}
	a.Sections = keep
}

// RenameRepo 重命名仓库对应的路径段
func (a *Authz) RenameRepo(oldName, newName string) {
	for _, sec := range a.Sections {
		if sec.Repo == oldName {
			delete(a.index, strings.ToLower(sec.Header))
			sec.Repo = newName
			sec.Header = newName + ":" + sec.Path
			a.index[strings.ToLower(sec.Header)] = sec
		}
	}
}

func cleanPath(p string) string {
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
// 权限判定
// ---------------------------------------------------------------------------

// PermLevel 将权限字符串转换为可比较等级
func PermLevel(p string) int {
	switch p {
	case "rw":
		return 3
	case "w":
		return 2
	case "r":
		return 1
	}
	return 0
}

// EffectivePerm 计算某个用户在某个仓库路径上的有效权限
func (a *Authz) EffectivePerm(repName, path, userName string) string {
	path = cleanPath(path)
	groups := a.ResolveUserGroups(userName)
	matches := a.matchSections(repName, path)
	for _, sec := range matches {
		for _, e := range sec.Entries {
			hit := entryMatches(e, userName, groups, a.Aliases)
			if e.Invert {
				// 取反：命中该对象的用户不适用，其余用户适用
				if !hit {
					return e.Perm
				}
				continue
			}
			if hit {
				return e.Perm
			}
		}
	}
	return ""
}

// entryMatches 判断权限项是否命中指定用户
func entryMatches(e *Entry, userName string, groups map[string]bool, aliases map[string]string) bool {
	switch {
	case e.Name == "*":
		return true
	case e.Name == "$authenticated":
		return userName != ""
	case e.Name == "$anonymous":
		return userName == ""
	case e.Name == userName:
		return userName != ""
	case strings.HasPrefix(e.Name, "@"):
		return groups[strings.TrimPrefix(e.Name, "@")]
	case strings.HasPrefix(e.Name, "&"):
		return aliases[strings.TrimPrefix(e.Name, "&")] != ""
	}
	return false
}

// matchSections 按路径精确度从高到低返回匹配的段
func (a *Authz) matchSections(repName, path string) []*Section {
	type item struct {
		sec   *Section
		score int
	}
	var items []item
	for _, sec := range a.Sections {
		if sec.Repo == repName {
			if isPathPrefix(sec.Path, path) {
				items = append(items, item{sec, 1000 + len(sec.Path)})
			}
		} else if sec.Repo == "" {
			if isPathPrefix(sec.Path, path) {
				items = append(items, item{sec, len(sec.Path)})
			}
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].score > items[j].score })
	out := make([]*Section, 0, len(items))
	for _, it := range items {
		out = append(out, it.sec)
	}
	return out
}

func isPathPrefix(prefix, path string) bool {
	if prefix == "/" {
		return true
	}
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}

// UserPermPaths 返回用户在指定仓库中所有被显式授权（含继承）的路径
func (a *Authz) UserPermPaths(repName, userName string) []string {
	var out []string
	for _, sec := range a.Sections {
		if sec.Repo != repName {
			continue
		}
		p := a.EffectivePerm(repName, sec.Path, userName)
		if PermLevel(p) > 0 {
			out = append(out, sec.Path)
		}
	}
	if len(out) == 0 {
		if PermLevel(a.EffectivePerm(repName, "/", userName)) > 0 {
			out = append(out, "/")
		}
	}
	return out
}

// UserCanAccess 判断用户是否可以访问仓库
func (a *Authz) UserCanAccess(repName, userName string) bool {
	return PermLevel(a.EffectivePerm(repName, "/", userName)) > 0
}
