// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package svn

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"svnadmin/internal/config"
	"svnadmin/internal/util"
)

// ---------------------------------------------------------------------------
// 仓库基础操作
// ---------------------------------------------------------------------------

// ListRepDirs 扫描仓库父目录，返回全部合法仓库名
func ListRepDirs() ([]string, error) {
	base := config.C().SVN.RepBasePath()
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(base, e.Name(), "format")); err != nil {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

// RepExists 判断仓库目录是否存在
func RepExists(repName string) bool {
	_, err := os.Stat(filepath.Join(RepPath(repName), "format"))
	return err == nil
}

// CreateRep 创建仓库，initStruct 为空表示不套用结构模板
func CreateRep(repName, initStruct string) error {
	if repName == "" {
		return errors.New("仓库名不能为空")
	}
	if RepExists(repName) {
		return fmt.Errorf("仓库 [%s] 已存在", repName)
	}
	if _, err := Run(BinSVNAdmin, "create", RepPath(repName)); err != nil {
		return err
	}
	if initStruct != "" {
		tpl := filepath.Join(config.C().SVN.TempleteInitPath(), initStruct)
		if info, err := os.Stat(tpl); err == nil && info.IsDir() {
			empty, _ := dirEmpty(tpl)
			if !empty {
				if _, err := Run(BinSVN, "import", tpl, RepURL(repName), "-m", "初始化仓库结构"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func dirEmpty(p string) (bool, error) {
	f, err := os.Open(p)
	if err != nil {
		return true, err
	}
	defer f.Close()
	names, err := f.Readdirnames(1)
	if err != nil && len(names) == 0 {
		return true, err
	}
	return len(names) == 0, nil
}

// DelRep 删除仓库（连带目录）
func DelRep(repName string) error {
	if !RepExists(repName) {
		return fmt.Errorf("仓库 [%s] 不存在", repName)
	}
	return os.RemoveAll(RepPath(repName))
}

// RenameRep 重命名仓库
func RenameRep(oldName, newName string) error {
	if !RepExists(oldName) {
		return fmt.Errorf("仓库 [%s] 不存在", oldName)
	}
	if RepExists(newName) {
		return fmt.Errorf("仓库 [%s] 已存在", newName)
	}
	return os.Rename(RepPath(oldName), RepPath(newName))
}

// RepSize 统计仓库占用字节数
func RepSize(repName string) int64 {
	var size int64
	_ = filepath.Walk(RepPath(repName), func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// RepYoungest 返回仓库最新版本号
func RepYoungest(repName string) int64 {
	out, err := Run(BinSVNLook, "youngest", RepPath(repName))
	if err != nil {
		return readCurrent(repName)
	}
	return parseInt64(strings.TrimSpace(out))
}

func readCurrent(repName string) int64 {
	data, err := os.ReadFile(filepath.Join(RepPath(repName), "db", "current"))
	if err != nil {
		return 0
	}
	return parseInt64(strings.TrimSpace(string(data)))
}

func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return v
}

// RepUUID 读取仓库 UUID
func RepUUID(repName string) string {
	out, err := Run(BinSVNLook, "uuid", RepPath(repName))
	if err != nil {
		data, _ := os.ReadFile(filepath.Join(RepPath(repName), "db", "uuid"))
		return strings.TrimSpace(string(data))
	}
	return strings.TrimSpace(out)
}

// SetUUID 重设仓库 UUID（uuid 为空时随机生成；非空时设置为指定值）
func SetUUID(repName string, uuid ...string) (string, error) {
	args := []string{"setuuid", RepPath(repName)}
	if len(uuid) > 0 && strings.TrimSpace(uuid[0]) != "" {
		args = append(args, strings.TrimSpace(uuid[0]))
	}
	if _, err := Run(BinSVNAdmin, args...); err != nil {
		return "", err
	}
	return RepUUID(repName), nil
}

// RepDetail 返回仓库属性明细
func RepDetail(repName string) map[string]interface{} {
	root := RepPath(repName)
	detail := map[string]interface{}{
		"repName":    repName,
		"repPath":    root,
		"repSize":    RepSize(repName),
		"repSizeHr":  util.FmtSize(RepSize(repName)),
		"repRev":     RepYoungest(repName),
		"repUuid":    RepUUID(repName),
		"svnVersion": Version(),
	}
	read := func(rel string) string {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}
	detail["format"] = read("format")
	detail["dbFormat"] = read(filepath.Join("db", "format"))
	detail["fsType"] = read(filepath.Join("db", "fs-type"))
	detail["fsfsConf"] = read(filepath.Join("db", "fsfs.conf"))
	detail["hooks"] = RepHooks(repName)
	return detail
}

// ---------------------------------------------------------------------------
// 目录树与文件浏览
// ---------------------------------------------------------------------------

type listEntry struct {
	Kind   string     `xml:"kind,attr"`
	Size   string     `xml:"size"`
	Name   string     `xml:"name"`
	Commit listCommit `xml:"commit"`
}

type listCommit struct {
	Revision string `xml:"revision,attr"`
	Author   string `xml:"author"`
	Date     string `xml:"date"`
}

type listRoot struct {
	XMLName xml.Name    `xml:"lists"`
	List    []xmlListEl `xml:"list"`
}

type xmlListEl struct {
	Path    string      `xml:"path,attr"`
	Entries []listEntry `xml:"entry"`
}

// TreeNode 目录树节点
type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     string      `json:"type"` // dir | file
	Size     int64       `json:"size"`
	Rev      int64       `json:"rev"`
	Author   string      `json:"author"`
	Date     string      `json:"date"`
	Children []*TreeNode `json:"children,omitempty"`
}

func svnList(repName, path string, recursive bool) ([]listEntry, error) {
	target := RepURL(repName) + cleanPath(path)
	args := []string{"list", "--xml"}
	if recursive {
		args = append(args, "-R")
	}
	args = append(args, target)
	out, err := Run(BinSVN, args...)
	if err != nil {
		return nil, err
	}
	var root listRoot
	if err := xml.Unmarshal([]byte(out), &root); err != nil {
		return nil, fmt.Errorf("解析 svn list 输出失败: %w", err)
	}
	var all []listEntry
	for _, l := range root.List {
		all = append(all, l.Entries...)
	}
	return all, nil
}

// SvnListAuth 以指定用户身份执行 svn list，走真实鉴权（用于 SVN 用户视角浏览）
func SvnListAuth(repName, path, user, pass string) ([]*TreeNode, error) {
	target := RepURL(repName) + cleanPath(path)
	args := []string{"list", "--xml", "--non-interactive", "--no-auth-cache"}
	if user != "" {
		args = append(args, "--username", user)
		if pass != "" {
			args = append(args, "--password", pass)
		}
	}
	args = append(args, target)
	out, err := Run(BinSVN, args...)
	if err != nil {
		return nil, err
	}
	var root listRoot
	if err := xml.Unmarshal([]byte(out), &root); err != nil {
		return nil, fmt.Errorf("解析 svn list 输出失败: %w", err)
	}
	base := cleanPath(path)
	if base == "/" {
		base = ""
	}
	outNodes := make([]*TreeNode, 0)
	for _, l := range root.List {
		for _, e := range l.Entries {
			outNodes = append(outNodes, &TreeNode{
				Name:   e.Name,
				Path:   base + "/" + e.Name,
				Type:   e.Kind,
				Size:   parseInt64(e.Size),
				Rev:    parseInt64(e.Commit.Revision),
				Author: e.Commit.Author,
				Date:   e.Commit.Date,
			})
		}
	}
	// 目录排在文件之前
	sort.SliceStable(outNodes, func(i, j int) bool {
		return outNodes[i].Type == "dir" && outNodes[j].Type != "dir"
	})
	return outNodes, nil
}

// RepCon 列出某个路径下的直接子项
func RepCon(repName, path string) ([]*TreeNode, error) {
	entries, err := svnList(repName, path, false)
	if err != nil {
		return nil, err
	}
	base := cleanPath(path)
	if base == "/" {
		base = ""
	}
	out := make([]*TreeNode, 0, len(entries))
	for _, e := range entries {
		out = append(out, &TreeNode{
			Name:   e.Name,
			Path:   base + "/" + e.Name,
			Type:   e.Kind,
			Size:   parseInt64(e.Size),
			Rev:    parseInt64(e.Commit.Revision),
			Author: e.Commit.Author,
			Date:   e.Commit.Date,
		})
	}
	return out, nil
}

// RepTree 递归获取某个路径下的目录树（仅目录）
func RepTree(repName, path string) ([]*TreeNode, error) {
	entries, err := svnList(repName, path, true)
	if err != nil {
		return nil, err
	}
	base := cleanPath(path)
	if base == "/" {
		base = ""
	}

	root := &TreeNode{Name: repName, Path: base, Type: "dir", Children: []*TreeNode{}}
	index := map[string]*TreeNode{"": root}

	// 按路径深度排序，保证父节点先建立
	type item struct {
		full string
		e    listEntry
	}
	var items []item
	for _, e := range entries {
		items = append(items, item{full: base + "/" + e.Name, e: e})
	}
	sort.Slice(items, func(i, j int) bool { return len(items[i].full) < len(items[j].full) })

	for _, it := range items {
		node := &TreeNode{
			Name:   filepath.Base(it.full),
			Path:   it.full,
			Type:   it.e.Kind,
			Size:   parseInt64(it.e.Size),
			Rev:    parseInt64(it.e.Commit.Revision),
			Author: it.e.Commit.Author,
			Date:   it.e.Commit.Date,
		}
		parentPath := filepath.ToSlash(filepath.Dir(it.full))
		if parentPath == "/" {
			parentPath = ""
		}
		if parentPath == base {
			parentPath = ""
		}
		parent, ok := index[parentPath]
		if !ok {
			parent = root
		}
		if node.Type == "dir" {
			node.Children = []*TreeNode{}
			index[it.full] = node
		}
		parent.Children = append(parent.Children, node)
	}
	return root.Children, nil
}

// CreateFolder 在线创建目录（直接提交到仓库）
func CreateFolder(repName, path, message, author string) error {
	target := RepURL(repName) + cleanPath(path)
	if message == "" {
		message = "在线创建目录"
	}
	args := []string{"mkdir", "-m", message}
	if author != "" {
		args = append(args, "--with-revprop", "author="+author)
	}
	args = append(args, "--parents", target)
	_, err := Run(BinSVN, args...)
	return err
}

// ---------------------------------------------------------------------------
// 备份与恢复
// ---------------------------------------------------------------------------

// Dump 导出仓库到 dump 文件
func Dump(repName, outFile, revRange string, incremental bool) error {
	args := []string{"dump", RepPath(repName)}
	if revRange != "" {
		args = append(args, "-r", revRange)
	}
	if incremental {
		args = append(args, "--incremental")
	}
	if err := os.MkdirAll(dirOf(outFile), 0755); err != nil {
		return err
	}
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = New(BinSVNAdmin, args...).WithStdout(f).Run()
	return err
}

// Load 从 dump 文件导入仓库
func Load(repName, inFile string) error {
	f, err := os.Open(inFile)
	if err != nil {
		return err
	}
	defer f.Close()

	bin, err := Resolve(BinSVNAdmin)
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, "load", RepPath(repName))
	cmd.Stdin = f
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return errors.New(msg)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 钩子
// ---------------------------------------------------------------------------

// Hook 仓库钩子
type Hook struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Tmpl        string `json:"tmpl"`
	Exists      bool   `json:"exists"`
}

// HookTemplates 各钩子的官方样例模板（对应原版 hooks/<name>.tmpl，供 UI 的"介绍/查看模板"使用）
var HookTemplates = map[string]string{
	"start-commit": `#!/bin/sh
# start-commit 样例：提交事务开始之前触发
REPOS="$1"
USER="$2"
CAPABILITIES="$3"
TXN_NAME="$4"
exit 0
`,
	"pre-commit": `#!/bin/sh
# pre-commit 样例：提交事务完成之前触发（常用于校验日志长度）
REPOS="$1"
TXN="$2"
SVNLOOK=/usr/bin/svnlook
LOGMSG=$($SVNLOOK log -t "$TXN" "$REPOS" | grep "[a-zA-Z0-9]" | wc -m)
if [ "$LOGMSG" -lt 10 ]; then
  echo "提交日志至少需要 10 个字符" 1>&2
  exit 1
fi
exit 0
`,
	"post-commit": `#!/bin/sh
# post-commit 样例：提交事务完成之后触发（常用于自动部署）
REPOS="$1"
REV="$2"
SVNLOOK=/usr/bin/svnlook
AUTHOR=$($SVNLOOK author -r "$REV" "$REPOS")
echo "[$REV] 由 $AUTHOR 提交"
exit 0
`,
	"pre-revprop-change": `#!/bin/sh
# pre-revprop-change 样例：修改版本属性之前触发
REPOS="$1"
REV="$2"
USER="$3"
PROPNAME="$4"
ACTION="$5"
if [ "$ACTION" = "M" -a "$PROPNAME" = "svn:log" ]; then exit 0; fi
exit 1
`,
	"post-revprop-change": `#!/bin/sh
# post-revprop-change 样例：修改版本属性之后触发
REPOS="$1"
REV="$2"
USER="$3"
PROPNAME="$4"
ACTION="$5"
exit 0
`,
	"pre-lock": `#!/bin/sh
# pre-lock 样例：加锁之前触发
REPOS="$1"
PATH="$2"
USER="$3"
exit 0
`,
	"post-lock": `#!/bin/sh
# post-lock 样例：加锁之后触发
REPOS="$1"
PATH="$2"
USER="$3"
exit 0
`,
	"pre-unlock": `#!/bin/sh
# pre-unlock 样例：解锁之前触发
REPOS="$1"
PATH="$2"
USER="$3"
exit 0
`,
	"post-unlock": `#!/bin/sh
# post-unlock 样例：解锁之后触发
REPOS="$1"
PATH="$2"
USER="$3"
exit 0
`,
}

// RepHooks 列出仓库的钩子及其内容（tmpl 为官方样例模板，对齐原版 .tmpl 文件）
func RepHooks(repName string) []*Hook {
	dir := filepath.Join(RepPath(repName), "hooks")
	out := make([]*Hook, 0, len(defaultHooks))
	for _, h := range defaultHooks {
		content := ""
		exists := false
		data, err := os.ReadFile(filepath.Join(dir, h.Name))
		if err == nil {
			content = string(data)
			exists = true
		}
		tmpl := HookTemplates[h.Name]
		// 仓库自带的 .tmpl 优先（通常为 subversion 安装时生成的官方样例）
		if data, err := os.ReadFile(filepath.Join(dir, h.Name+".tmpl")); err == nil && strings.TrimSpace(string(data)) != "" {
			tmpl = string(data)
		}
		out = append(out, &Hook{Name: h.Name, Type: h.Name, Description: h.Desc, Content: content, Tmpl: tmpl, Exists: exists})
	}
	return out
}

var defaultHooks = []struct{ Name, Desc string }{
	{"start-commit", "提交事务开始之前触发"},
	{"pre-commit", "提交事务完成之前触发，可用于校验日志长度"},
	{"post-commit", "提交事务完成之后触发，可用于自动部署"},
	{"pre-revprop-change", "修改版本属性之前触发"},
	{"post-revprop-change", "修改版本属性之后触发"},
	{"pre-lock", "加锁之前触发"},
	{"post-lock", "加锁之后触发"},
	{"pre-unlock", "解锁之前触发"},
	{"post-unlock", "解锁之后触发"},
}

// UpdRepHook 写入仓库钩子内容
func UpdRepHook(repName, hookName, content string) error {
	dir := filepath.Join(RepPath(repName), "hooks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, filepath.Base(hookName)), []byte(content), 0755)
}

// DelRepHook 删除（清空）仓库钩子
func DelRepHook(repName, hookName string) error {
	p := filepath.Join(RepPath(repName), "hooks", filepath.Base(hookName))
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// RecommendHooks 列出推荐钩子目录
func RecommendHooks() []*Hook {
	base := config.C().SVN.RecommendHookPath()
	entries, err := os.ReadDir(base)
	if err != nil {
		return []*Hook{}
	}
	out := []*Hook{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		h := &Hook{Name: e.Name()}
		if data, err := os.ReadFile(filepath.Join(base, e.Name(), "hookName")); err == nil {
			h.Type = strings.TrimSpace(string(data))
		}
		if data, err := os.ReadFile(filepath.Join(base, e.Name(), "hookDescription")); err == nil {
			h.Description = strings.TrimSpace(string(data))
		}
		if h.Type != "" {
			if data, err := os.ReadFile(filepath.Join(base, e.Name(), h.Type)); err == nil {
				h.Content = string(data)
			}
		}
		out = append(out, h)
	}
	return out
}
