// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package service

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// LDAP 相关错误
var (
	errInvalidHost     = errors.New("主机地址不能包含 http:// 或 https:// 前缀")
	errLDAPNotConfig   = errors.New("未配置 LDAP 服务器地址")
	errLDAPPortInvalid = errors.New("LDAP 端口不合法")
)

// ldapAddr 拼接 LDAP 服务地址
func ldapAddr(s LDAPSettings) (string, bool, error) {
	host := strings.TrimSpace(s.Host)
	if host == "" {
		return "", false, errLDAPNotConfig
	}
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return "", false, errInvalidHost
	}
	host = strings.TrimPrefix(host, "ldap://")
	tlsMode := strings.HasPrefix(host, "ldaps://")
	host = strings.TrimPrefix(host, "ldaps://")
	host = strings.TrimSuffix(host, "/")
	if host == "" {
		return "", false, errLDAPNotConfig
	}
	port := s.Port
	if port == 0 {
		if tlsMode {
			port = 636
		} else {
			port = 389
		}
	}
	if port < 1 || port > 65535 {
		return "", false, errLDAPPortInvalid
	}
	return fmt.Sprintf("%s:%d", host, port), tlsMode, nil
}

func dialLDAP(s LDAPSettings) (*ldap.Conn, error) {
	addr, tlsMode, err := ldapAddr(s)
	if err != nil {
		return nil, err
	}
	timeout := 10 * time.Second
	if tlsMode {
		return ldap.DialTLS("tcp", addr, &tls.Config{
			ServerName:         strings.Split(addr, ":")[0],
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: false,
		})
	}
	c, err := ldap.DialURL("ldap://"+addr, ldap.DialWithDialer(&net.Dialer{Timeout: timeout}))
	if err != nil {
		return nil, err
	}
	c.SetTimeout(timeout)
	return c, nil
}

// LDAPEntry 查询结果条目
type LDAPEntry struct {
	DN         string            `json:"dn"`
	Attributes map[string]string `json:"attributes"`
}

func (s LDAPSettings) search(filter string, attrs []string, baseDN string) ([]LDAPEntry, error) {
	c, err := dialLDAP(s)
	if err != nil {
		return nil, fmt.Errorf("连接 LDAP 失败：%v", err)
	}
	defer c.Close()
	if s.BindDN != "" {
		if err := c.Bind(s.BindDN, s.BindPassword); err != nil {
			return nil, fmt.Errorf("LDAP 绑定失败：%v", err)
		}
	} else {
		_ = c.UnauthenticatedBind("")
	}
	req := ldap.NewSearchRequest(baseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false, filter, attrs, nil)
	res, err := c.Search(req)
	if err != nil {
		return nil, fmt.Errorf("LDAP 查询失败：%v", err)
	}
	out := make([]LDAPEntry, 0, len(res.Entries))
	for _, e := range res.Entries {
		m := map[string]string{}
		for _, a := range e.Attributes {
			if len(a.Values) > 0 {
				m[a.Name] = a.Values[0]
			}
		}
		out = append(out, LDAPEntry{DN: e.DN, Attributes: m})
	}
	return out, nil
}

func splitAttrs(s string, def string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{def}
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{def}
	}
	return out
}

// LDAPTestResult 测试结果
type LDAPTestResult struct {
	OK      bool     `json:"ok"`
	Message string   `json:"message"`
	Users   []string `json:"users"`
	Groups  []string `json:"groups"`
}

// LDAPTest 测试 LDAP 配置：connection / user / group
func LDAPTest(kind string, s LDAPSettings) (*LDAPTestResult, error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = "connection"
	}
	switch kind {
	case "connection":
		c, err := dialLDAP(s)
		if err != nil {
			return &LDAPTestResult{OK: false, Message: err.Error()}, nil
		}
		defer c.Close()
		if s.BindDN != "" {
			if err := c.Bind(s.BindDN, s.BindPassword); err != nil {
				return &LDAPTestResult{OK: false, Message: "连接成功但绑定失败：" + err.Error()}, nil
			}
		}
		return &LDAPTestResult{OK: true, Message: "LDAP 连接成功"}, nil

	case "user":
		if s.UserBaseDN == "" {
			return nil, errors.New("请先填写用户 Base DN")
		}
		filter := s.UserSearchFilter
		if filter == "" {
			filter = "(objectClass=inetOrgPerson)"
		}
		attrs := splitAttrs(s.UserAttributes, "uid")
		entries, err := s.search(filter, attrs, s.UserBaseDN)
		if err != nil {
			return &LDAPTestResult{OK: false, Message: err.Error()}, nil
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if v, ok := e.Attributes[attrs[0]]; ok && v != "" {
				names = append(names, v)
			}
		}
		if len(names) > 50 {
			names = names[:50]
		}
		return &LDAPTestResult{OK: true, Message: fmt.Sprintf("查询到 %d 个用户", len(entries)), Users: names}, nil

	case "group":
		if s.GroupBaseDN == "" {
			return nil, errors.New("请先填写分组 Base DN")
		}
		filter := s.GroupSearchFilter
		if filter == "" {
			filter = "(objectClass=groupOfNames)"
		}
		attrs := splitAttrs(s.GroupAttributes, "cn")
		entries, err := s.search(filter, attrs, s.GroupBaseDN)
		if err != nil {
			return &LDAPTestResult{OK: false, Message: err.Error()}, nil
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if v, ok := e.Attributes[attrs[0]]; ok && v != "" {
				names = append(names, v)
			}
		}
		if len(names) > 50 {
			names = names[:50]
		}
		return &LDAPTestResult{OK: true, Message: fmt.Sprintf("查询到 %d 个分组", len(entries)), Groups: names}, nil
	}
	return nil, fmt.Errorf("不支持的测试类型 [%s]", kind)
}

// LDAPUsers 查询 LDAP 用户
func LDAPUsers(s LDAPSettings) ([]string, error) {
	if s.UserBaseDN == "" {
		return nil, errors.New("未配置用户 Base DN")
	}
	filter := s.UserSearchFilter
	if filter == "" {
		filter = "(objectClass=inetOrgPerson)"
	}
	attrs := splitAttrs(s.UserAttributes, "uid")
	entries, err := s.search(filter, attrs, s.UserBaseDN)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, e := range entries {
		v := e.Attributes[attrs[0]]
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out, nil
}

// LDAPGroupMember 分组及其成员
type LDAPGroupMember struct {
	Group   string   `json:"group"`
	Members []string `json:"members"`
}

// LDAPGroups 查询 LDAP 分组及其成员
func LDAPGroups(s LDAPSettings) ([]LDAPGroupMember, error) {
	if s.GroupBaseDN == "" {
		return nil, errors.New("未配置分组 Base DN")
	}
	filter := s.GroupSearchFilter
	if filter == "" {
		filter = "(objectClass=groupOfNames)"
	}
	attrs := splitAttrs(s.GroupAttributes, "cn")
	memberAttr := s.GroupsToUserAttr
	if memberAttr == "" {
		memberAttr = "member"
	}
	attrs = append(attrs, memberAttr)
	entries, err := s.search(filter, attrs, s.GroupBaseDN)
	if err != nil {
		return nil, err
	}
	out := make([]LDAPGroupMember, 0, len(entries))
	for _, e := range entries {
		name := e.Attributes[attrs[0]]
		if name == "" {
			continue
		}
		g := LDAPGroupMember{Group: name, Members: []string{}}
		c, err := dialLDAP(s)
		if err != nil {
			out = append(out, g)
			continue
		}
		if s.BindDN != "" {
			_ = c.Bind(s.BindDN, s.BindPassword)
		}
		req := ldap.NewSearchRequest(s.GroupBaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			fmt.Sprintf("(&(%s=%s)(%s=%s))", attrs[0], ldap.EscapeFilter(name),
				func() string { return "objectClass" }(), "*"), []string{memberAttr}, nil)
		if res, err := c.Search(req); err == nil && len(res.Entries) > 0 {
			for _, v := range res.Entries[0].GetAttributeValues(memberAttr) {
				g.Members = append(g.Members, extractCN(v))
			}
		}
		c.Close()
		out = append(out, g)
	}
	return out, nil
}

// extractCN 从 member DN 中提取用户名
func extractCN(dn string) string {
	for _, part := range strings.Split(dn, ",") {
		p := strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(p), "uid=") {
			return strings.TrimPrefix(p, "uid=")
		}
		if strings.HasPrefix(strings.ToLower(p), "cn=") {
			return strings.TrimPrefix(p, "cn=")
		}
	}
	return dn
}

// LDAPBindAuth 使用 LDAP 校验账号密码
func LDAPBindAuth(s LDAPSettings, user, pass string) error {
	if pass == "" {
		return errors.New("密码不能为空")
	}
	// 先用 BindDN 找到用户 DN，再用该 DN 校验密码
	filter := s.UserSearchFilter
	if filter == "" {
		filter = "(objectClass=inetOrgPerson)"
	}
	attrs := splitAttrs(s.UserAttributes, "uid")
	entries, err := s.search(fmt.Sprintf("(&%s(%s=%s))", filter, attrs[0], ldap.EscapeFilter(user)), []string{"dn"}, s.UserBaseDN)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return errors.New("LDAP 中不存在该用户")
	}
	addr, tlsMode, err := ldapAddr(s)
	if err != nil {
		return err
	}
	var c *ldap.Conn
	if tlsMode {
		c, err = ldap.DialTLS("tcp", addr, &tls.Config{ServerName: strings.Split(addr, ":")[0], MinVersion: tls.VersionTLS12})
	} else {
		c, err = ldap.Dial("tcp", addr)
	}
	if err != nil {
		return err
	}
	defer c.Close()
	return c.Bind(entries[0].DN, pass)
}

// SyncLDAPToDB 将 LDAP 用户同步到 svn_users 表
func SyncLDAPToDB(operator string) (int, error) {
	ds := GetDataSource(OptionSvnDatasource)
	if ds.UserSource != SourceLDAP {
		return 0, errors.New("当前 SVN 用户来源不是 LDAP")
	}
	users, err := LDAPUsers(ds.LDAP)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, u := range users {
		if err := ensureSvnUser(u, "LDAP 同步"); err == nil {
			n++
		}
	}
	AddLog("数据同步", fmt.Sprintf("从 LDAP 同步用户 %d 个", n), operator)
	return n, nil
}

// SyncLDAPGroupsToAuthz 将 LDAP 分组同步到 authz 的 [groups] 段
func SyncLDAPGroupsToAuthz(operator string) (int, error) {
	ds := GetDataSource(OptionSvnDatasource)
	if ds.GroupSource != GroupSourceLDAP {
		return 0, errors.New("当前 SVN 分组来源不是 LDAP")
	}
	groups, err := LDAPGroups(ds.LDAP)
	if err != nil {
		return 0, err
	}
	if err := SyncGroups(); err != nil {
		return 0, err
	}
	n := 0
	for _, g := range groups {
		if err := CreateGroup(g.Group, "LDAP 同步", operator); err == nil {
			n++
		}
		if len(g.Members) > 0 {
			_ = UpdGroupMember(g.Group, g.Members, nil, nil, operator)
		}
	}
	AddLog("数据同步", fmt.Sprintf("从 LDAP 同步分组 %d 个", n), operator)
	return n, nil
}
