// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package svn

import (
	"os"
	"strings"
	"sync"

	"svnadmin/internal/config"
)

// Passwd svnserve 的 passwd 文件（明文 [users] 段）
type Passwd struct {
	Path     string
	Head     []string
	Comment  map[string][]string // 用户名 -> 其上方注释
	Users    []string            // 保持顺序的用户名
	Pass     map[string]string
	Disabled map[string]bool // 被 #disabled# 前缀禁用的用户（兼容原版写法）
	mu       sync.Mutex
}

// LoadPasswd 读取 passwd 文件
func LoadPasswd() (*Passwd, error) {
	p := config.C().SVN.PasswdFile()
	pw := &Passwd{Path: p, Pass: map[string]string{}, Comment: map[string][]string{}, Users: []string{}, Disabled: map[string]bool{}}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return pw, nil
		}
		return nil, err
	}
	pw.parse(string(data))
	return pw, nil
}

// disabledPrefix 原版用于标记禁用用户的前缀
const disabledPrefix = "#disabled#"

func (pw *Passwd) parse(content string) {
	var pending []string
	inUsers := false
	for _, raw := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		// 兼容原版禁用写法：#disabled#user = pass
		// 该行同时具备"被注释"与"真实账号"两种含义，需要识别而不是当作注释丢弃
		if strings.HasPrefix(line, disabledPrefix) {
			rest := strings.TrimSpace(line[len(disabledPrefix):])
			if idx := strings.Index(rest, "="); idx > 0 {
				name := strings.TrimSpace(rest[:idx])
				pass := strings.TrimSpace(rest[idx+1:])
				if name != "" {
					if _, ok := pw.Pass[name]; !ok {
						pw.Users = append(pw.Users, name)
					}
					pw.Pass[name] = pass
					pw.Disabled[name] = true
					pending = nil
					continue
				}
			}
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			pending = append(pending, raw)
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inUsers = strings.EqualFold(strings.TrimSpace(line[1:len(line)-1]), "users")
			if !inUsers {
				pw.Head = append(pw.Head, pending...)
			}
			pending = nil
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 || !inUsers {
			pending = nil
			continue
		}
		name := strings.TrimSpace(line[:idx])
		pass := strings.TrimSpace(line[idx+1:])
		if name == "" {
			continue
		}
		if _, ok := pw.Pass[name]; !ok {
			pw.Users = append(pw.Users, name)
		}
		pw.Comment[name] = pending
		pw.Pass[name] = pass
		pending = nil
	}
}

// Get 获取用户密码
func (pw *Passwd) Get(name string) (string, bool) {
	v, ok := pw.Pass[name]
	return v, ok
}

// Set 设置用户密码（不存在则追加）
func (pw *Passwd) Set(name, pass string) {
	if _, ok := pw.Pass[name]; !ok {
		pw.Users = append(pw.Users, name)
	}
	pw.Pass[name] = pass
}

// Del 删除用户
func (pw *Passwd) Del(name string) {
	if _, ok := pw.Pass[name]; !ok {
		return
	}
	delete(pw.Pass, name)
	delete(pw.Comment, name)
	delete(pw.Disabled, name)
	for i, u := range pw.Users {
		if u == name {
			pw.Users = append(pw.Users[:i], pw.Users[i+1:]...)
			break
		}
	}
}

// Has 判断用户是否存在
func (pw *Passwd) Has(name string) bool {
	_, ok := pw.Pass[name]
	return ok
}

// Names 返回全部用户名（有序）
func (pw *Passwd) Names() []string { return pw.Users }

// Save 写回 passwd 文件
func (pw *Passwd) Save() error {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	var sb strings.Builder
	for _, l := range pw.Head {
		sb.WriteString(l + "\n")
	}
	sb.WriteString("[users]\n")
	for _, name := range pw.Users {
		for _, c := range pw.Comment[name] {
			sb.WriteString(c + "\n")
		}
		// 保持原版禁用标记不丢失
		if pw.Disabled[name] {
			sb.WriteString(disabledPrefix + name + " = " + pw.Pass[name] + "\n")
			continue
		}
		sb.WriteString(name + " = " + pw.Pass[name] + "\n")
	}
	if err := os.MkdirAll(dirOf(pw.Path), 0755); err != nil {
		return err
	}
	return os.WriteFile(pw.Path, []byte(sb.String()), 0600)
}

// ---------------------------------------------------------------------------
// httpPasswd（Apache htpasswd，apr1 格式）
// ---------------------------------------------------------------------------

// HTTPPasswd httpPasswd 文件
type HTTPPasswd struct {
	Path  string
	Users []string
	Hash  map[string]string
	mu    sync.Mutex
}

// LoadHTTPPasswd 读取 httpPasswd 文件
func LoadHTTPPasswd() (*HTTPPasswd, error) {
	p := config.C().SVN.HTTPPasswdFile()
	hp := &HTTPPasswd{Path: p, Hash: map[string]string{}, Users: []string{}}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return hp, nil
		}
		return nil, err
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.Index(line, ":")
		if i <= 0 {
			continue
		}
		name := line[:i]
		if _, ok := hp.Hash[name]; !ok {
			hp.Users = append(hp.Users, name)
		}
		hp.Hash[name] = line[i+1:]
	}
	return hp, nil
}

// Set 设置用户（明文密码自动做 apr1 散列）
func (hp *HTTPPasswd) Set(name, plain string) error {
	h, err := Apr1Hash(plain)
	if err != nil {
		return err
	}
	if _, ok := hp.Hash[name]; !ok {
		hp.Users = append(hp.Users, name)
	}
	hp.Hash[name] = h
	return nil
}

// Del 删除用户
func (hp *HTTPPasswd) Del(name string) {
	if _, ok := hp.Hash[name]; !ok {
		return
	}
	delete(hp.Hash, name)
	for i, u := range hp.Users {
		if u == name {
			hp.Users = append(hp.Users[:i], hp.Users[i+1:]...)
			break
		}
	}
}

// Save 写回 httpPasswd 文件
func (hp *HTTPPasswd) Save() error {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	var sb strings.Builder
	for _, name := range hp.Users {
		sb.WriteString(name + ":" + hp.Hash[name] + "\n")
	}
	if err := os.MkdirAll(dirOf(hp.Path), 0755); err != nil {
		return err
	}
	return os.WriteFile(hp.Path, []byte(sb.String()), 0600)
}
