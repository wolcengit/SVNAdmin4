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
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"svnadmin/internal/auth"
	"svnadmin/internal/config"
	"svnadmin/internal/model"
	"svnadmin/internal/store"
	"svnadmin/internal/util"
)

// LoginResult 登录结果
type LoginResult struct {
	Token     string   `json:"token"`
	Role      string   `json:"role"`
	UserName  string   `json:"userName"`
	UserID    int64    `json:"userId"`
	Expire    int      `json:"expire"`
	Functions []string `json:"functions,omitempty"` // 子管理员的接口白名单（前端按此渲染菜单）
}

// Login 登录。role 为可选项（admin / subadmin / svnuser），
// 传入时只在对应身份表中查找，避免同名账号被优先命中；为空则按顺序自动识别。
func Login(userName, password, code, uuid, role string) (*LoginResult, error) {
	if VerifyEnabled() {
		if !CheckVerifyCode(uuid, code) {
			noteLoginFail(userName)
			return nil, errors.New("验证码错误或已失效")
		}
	}
	if userName == "" || password == "" {
		return nil, errors.New("用户名或密码不能为空")
	}
	// 登录失败锁定：连续失败达阈值后临时锁定账号
	if left := loginLockLeft(userName); left > 0 {
		return nil, fmt.Errorf("登录失败次数过多，账号已锁定，请 %d 秒后重试", left)
	}

	now := model.Now()

	// 指定角色时只走对应分支
	var (
		res *LoginResult
		err error
	)
	switch strings.ToLower(strings.TrimSpace(role)) {
	case auth.RoleAdmin, "管理人员", "超级管理员":
		res, err = loginAdmin(userName, password, now)
	case auth.RoleSubadmin, "子管理员":
		res, err = loginSubadmin(userName, password, now)
	case auth.RoleSvnUser, "svn", "SVN用户":
		res, err = loginSvnUser(userName, password, now)
	case "":
		// 自动识别，按 管理员 → 子管理员 → SVN 用户 顺序
	default:
		return nil, errors.New("登录角色不合法")
	}
	if res != nil || err != nil {
		if res != nil {
			clearLoginFail(userName)
			// 登录成功推送（触发点可在消息推送中开关）
			go notifyLogin(userName)
			return res, nil
		}
		noteLoginFail(userName)
		return nil, err
	}

	// 1. 管理员
	if res, err := loginAdmin(userName, password, now); err == nil {
		clearLoginFail(userName)
		go notifyLogin(userName)
		return res, nil
	} else if !isBadCredential(err) {
		noteLoginFail(userName)
		return nil, err
	}
	// 2. 子管理员
	if res, err := loginSubadmin(userName, password, now); err == nil {
		clearLoginFail(userName)
		go notifyLogin(userName)
		return res, nil
	} else if !isBadCredential(err) {
		noteLoginFail(userName)
		return nil, err
	}
	// 3. SVN 用户
	if res, err := loginSvnUser(userName, password, now); err == nil {
		clearLoginFail(userName)
		go notifyLogin(userName)
		return res, nil
	} else if !isBadCredential(err) {
		noteLoginFail(userName)
		return nil, err
	}
	noteLoginFail(userName)
	return nil, errors.New("用户名或密码错误")
}

// notifyLogin 登录成功邮件推送（受消息推送开关控制）
func notifyLogin(userName string) {
	SendMailByTrigger(MailTriggerLogin, "用户登录成功通知",
		fmt.Sprintf("账号：%s\n时间：%s", userName, model.Now()))
}

// ---------------------------------------------------------------------------
// 登录失败锁定（受安全配置 login_fail_lock 与 login_fail_limit / login_lock_seconds 控制）
// ---------------------------------------------------------------------------

type loginFailItem struct {
	count int
	until time.Time
}

var (
	loginFailMu sync.Mutex
	loginFail   = map[string]loginFailItem{}
)

// loginLockEnabled 是否启用登录失败锁定
func loginLockEnabled() bool {
	for _, s := range GetSafeConfig() {
		if s.Name == "login_fail_lock" {
			return s.Enable
		}
	}
	return config.C().Security.LoginFailLimit > 0
}

// loginLockLeft 返回账号剩余锁定秒数，0 表示未锁定
func loginLockLeft(userName string) int {
	if !loginLockEnabled() || userName == "" {
		return 0
	}
	loginFailMu.Lock()
	defer loginFailMu.Unlock()
	it, ok := loginFail[userName]
	if !ok || it.until.IsZero() {
		return 0
	}
	left := int(time.Until(it.until).Seconds())
	if left <= 0 {
		delete(loginFail, userName)
		return 0
	}
	return left
}

// noteLoginFail 记录一次登录失败
func noteLoginFail(userName string) {
	if !loginLockEnabled() || userName == "" {
		return
	}
	limit := config.C().Security.LoginFailLimit
	if limit <= 0 {
		limit = 5
	}
	secs := config.C().Security.LoginLockSeconds
	if secs <= 0 {
		secs = 300
	}
	loginFailMu.Lock()
	defer loginFailMu.Unlock()
	it := loginFail[userName]
	it.count++
	if it.count >= limit {
		it.until = time.Now().Add(time.Duration(secs) * time.Second)
		it.count = 0
		AddLog("安全", fmt.Sprintf("账号 [%s] 连续登录失败达 %d 次，已锁定 %d 秒", userName, limit, secs), userName)
	}
	loginFail[userName] = it
}

// clearLoginFail 登录成功后清除失败计数
func clearLoginFail(userName string) {
	if userName == "" {
		return
	}
	loginFailMu.Lock()
	delete(loginFail, userName)
	loginFailMu.Unlock()
}

// isBadCredential 判断是否为"凭据不匹配"类错误（用于自动识别时继续尝试下一种身份）
func isBadCredential(err error) bool {
	return err != nil && strings.Contains(err.Error(), "用户名或密码")
}

// loginAdmin 以管理员身份登录
func loginAdmin(userName, password, now string) (*LoginResult, error) {
	cfg := config.C()
	var admin model.AdminUser
	if err := store.DB().Where("admin_user_name = ?", userName).First(&admin).Error; err != nil {
		return nil, errors.New("用户名或密码错误")
	}
	if !util.SecureEqual(admin.AdminUserPassword, util.Hash(password)) {
		return nil, errors.New("用户名或密码错误")
	}
	jti := util.RandomHex(8)
	token, err := auth.Generate(&auth.User{ID: admin.AdminUserID, Name: admin.AdminUserName, Role: auth.RoleAdmin, Token: jti})
	if err != nil {
		return nil, err
	}
	AddLog("登录", fmt.Sprintf("管理员 [%s] 登录系统", userName), userName)
	return &LoginResult{Token: token, Role: auth.RoleAdmin, UserName: admin.AdminUserName, UserID: admin.AdminUserID, Expire: cfg.Server.TokenExpire}, nil
}

// loginSubadmin 以子管理员身份登录
func loginSubadmin(userName, password, now string) (*LoginResult, error) {
	cfg := config.C()
	var sub model.Subadmin
	if err := store.DB().Where("subadmin_name = ?", userName).First(&sub).Error; err != nil {
		return nil, errors.New("用户名或密码错误")
	}
	if sub.SubadminStatus != 1 {
		return nil, errors.New("账号已被禁用")
	}
	if !util.SecureEqual(sub.SubadminPassword, util.Hash(password)) {
		return nil, errors.New("用户名或密码错误")
	}
	jti := util.RandomHex(8)
	token, err := auth.Generate(&auth.User{ID: sub.SubadminID, Name: sub.SubadminName, Role: auth.RoleSubadmin, Token: jti})
	if err != nil {
		return nil, err
	}
	store.DB().Model(&model.Subadmin{}).Where("subadmin_id = ?", sub.SubadminID).Update("subadmin_last_login", now)
	AddLog("登录", fmt.Sprintf("子管理员 [%s] 登录系统", userName), userName)
	return &LoginResult{Token: token, Role: auth.RoleSubadmin, UserName: sub.SubadminName, UserID: sub.SubadminID, Expire: cfg.Server.TokenExpire, Functions: keysOf(SubadminFunctions(sub.SubadminID))}, nil
}

// loginSvnUser 以 SVN 用户身份登录
func loginSvnUser(userName, password, now string) (*LoginResult, error) {
	cfg := config.C()
	var su model.SvnUser
	if err := store.DB().Where("svn_user_name = ?", userName).First(&su).Error; err != nil {
		return nil, errors.New("用户名或密码错误")
	}
	if su.SvnUserStatus != 1 {
		return nil, errors.New("账号已被禁用")
	}
	// 数据源为 LDAP 时改为远程 bind 认证（与原版 LdapUserLogin 语义一致），
	// 认证成功后把密码回写本地库，便于会话期间复用；失败则返回明确文案。
	if !localAuthOK(su, password) {
		if !ldapAuthOK(su, userName, password) {
			return nil, errors.New("用户名或密码错误")
		}
	}
	jti := util.RandomHex(8)
	token, err := auth.Generate(&auth.User{ID: su.SvnUserID, Name: su.SvnUserName, Role: auth.RoleSvnUser, Token: jti})
	if err != nil {
		return nil, err
	}
	// 记录 token 到期时间（Unix 秒）用于列表在线态判定，对齐原版 token 落库语义
	expireAt := time.Now().Add(time.Duration(cfg.Server.TokenExpire) * time.Second).Unix()
	store.DB().Model(&model.SvnUser{}).Where("svn_user_id = ?", su.SvnUserID).
		Updates(map[string]interface{}{"svn_user_last_login": now, "svn_user_token": strconv.FormatInt(expireAt, 10)})
	AddLog("登录", fmt.Sprintf("SVN 用户 [%s] 登录系统", userName), userName)
	return &LoginResult{Token: token, Role: auth.RoleSvnUser, UserName: su.SvnUserName, UserID: su.SvnUserID, Expire: cfg.Server.TokenExpire}, nil
}

// localAuthOK 本地口令比对
func localAuthOK(su model.SvnUser, password string) bool {
	return util.SecureEqual(su.SvnUserPass, password)
}

// ldapAuthOK 数据源为 LDAP 时通过远程 bind 校验账号
// 返回 false 表示未通过（含未启用 LDAP、连接失败、凭据错误三种情况）
func ldapAuthOK(su model.SvnUser, userName, password string) bool {
	ds := GetDataSource(OptionSvnDatasource)
	if ds.UserSource != SourceLDAP {
		// HTTP 协议下同样支持 ldap 用户来源
		dsHTTP := GetDataSource(OptionHttpDatasource)
		if dsHTTP.UserSource != SourceLDAP {
			return false
		}
		ds = dsHTTP
	}
	if strings.Contains(userName, "|") {
		return false
	}
	if err := LDAPBindAuth(ds.LDAP, userName, password); err != nil {
		AddLog("登录", fmt.Sprintf("LDAP 用户 [%s] 认证失败：%v", userName, err), userName)
		return false
	}
	// 认证成功后回写本地口令，保证离线/切换数据源后仍可登录
	store.DB().Model(&model.SvnUser{}).Where("svn_user_id = ?", su.SvnUserID).Update("svn_user_pass", password)
	return true
}

// Logout 注销：将尚未过期的 token 加入黑名单
func Logout(token string) error {
	if token == "" {
		return errors.New("token 为空")
	}
	u, err := auth.Parse(token)
	if err != nil {
		return nil
	}
	exp := time.Now().Add(time.Duration(config.C().Server.TokenExpire) * time.Second)
	auth.Blacklist(token, exp)
	// 落库持久化：服务重启后黑名单仍生效（对齐原版 black_token 表语义）
	_ = store.DB().Where(&model.BlackToken{BlackTokenToken: token}).
		Assign(&model.BlackToken{BlackTokenExpire: exp.Format("2006-01-02 15:04:05")}).
		FirstOrCreate(&model.BlackToken{}).Error
	// SVN 用户注销后清除在线态（token 到期标记）
	if u.Role == auth.RoleSvnUser {
		store.DB().Model(&model.SvnUser{}).Where("svn_user_id = ?", u.ID).Update("svn_user_token", "-")
	}
	AddLog("注销", fmt.Sprintf("用户 [%s] 退出登录", u.Name), u.Name)
	return nil
}

// TokenBlacklisted 判断 token 是否已注销（内存黑名单 + 落库黑名单双重判定）
func TokenBlacklisted(token string) bool {
	if token == "" {
		return false
	}
	if auth.IsBlacklisted(token) {
		return true
	}
	var bt model.BlackToken
	if err := store.DB().Where("black_token_token = ?", token).First(&bt).Error; err != nil {
		return false
	}
	if bt.BlackTokenExpire < time.Now().Format("2006-01-02 15:04:05") {
		// 已过期，顺手清理
		store.DB().Delete(&bt)
		return false
	}
	// 回填内存缓存，后续请求不再查库
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", bt.BlackTokenExpire, time.Local); err == nil {
		auth.Blacklist(token, t)
	}
	return true
}

// ---------------------------------------------------------------------------
// 验证码（SVG 形式，无需字体依赖）
// ---------------------------------------------------------------------------

type verifyItem struct {
	code string
	exp  time.Time
}

var (
	verifyMu sync.Mutex
	verify   = map[string]verifyItem{}
)

// NewVerifyCode 生成验证码，返回 uuid 与 svg 内容
func NewVerifyCode() (string, string, error) {
	code := randomCode(4)
	uuid := util.RandomHex(16)
	verifyMu.Lock()
	verify[uuid] = verifyItem{code: code, exp: time.Now().Add(3 * time.Minute)}
	cleanVerifyLocked()
	verifyMu.Unlock()
	return uuid, captchaSVG(code), nil
}

// CheckVerifyCode 校验验证码
func CheckVerifyCode(uuid, code string) bool {
	if uuid == "" || code == "" {
		return false
	}
	verifyMu.Lock()
	defer verifyMu.Unlock()
	item, ok := verify[uuid]
	if !ok {
		return false
	}
	delete(verify, uuid)
	if item.exp.Before(time.Now()) {
		return false
	}
	return strings.EqualFold(item.code, code)
}

// VerifyEnabled 返回系统是否开启验证码
// VerifyEnabled 是否启用登录验证码。
// 优先取安全配置中的 login_verify_code（与原版一致，可在设置页实时开关），
// 未配置时回落 yaml。
func VerifyEnabled() bool {
	for _, s := range GetSafeConfig() {
		if s.Name == "login_verify_code" {
			return s.Enable
		}
	}
	return config.C().Security.EnableVerifyCode
}

// PassCheckEnabled 是否启用 SVN 用户密码强度校验（至少 4 位且非纯空格）
func PassCheckEnabled() bool {
	for _, s := range GetSafeConfig() {
		if s.Name == "svn_user_pass_check" {
			return s.Enable
		}
	}
	return false
}

// CheckUserPass 校验 SVN 用户密码强度（开关关闭时只做非空校验）
func CheckUserPass(pass string) error {
	if strings.TrimSpace(pass) == "" {
		return errors.New("密码不能为空")
	}
	if !PassCheckEnabled() {
		return nil
	}
	if len(pass) < 4 {
		return errors.New("密码至少 4 位")
	}
	if strings.TrimSpace(pass) == "" {
		return errors.New("密码不能为纯空格")
	}
	return nil
}

func cleanVerifyLocked() {
	now := time.Now()
	for k, v := range verify {
		if v.exp.Before(now) {
			delete(verify, k)
		}
	}
}

func randomCode(n int) string {
	const chars = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	b := make([]byte, n)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range b {
		b[i] = chars[r.Intn(len(chars))]
	}
	return string(b)
}

func captchaSVG(code string) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var sb strings.Builder
	sb.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="120" height="44" viewBox="0 0 120 44">`)
	sb.WriteString(`<rect width="120" height="44" fill="#f2f6fc"/>`)
	// 干扰线
	for i := 0; i < 4; i++ {
		sb.WriteString(fmt.Sprintf(
			`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`,
			r.Intn(120), r.Intn(44), r.Intn(120), r.Intn(44), randColor(r)))
	}
	// 字符
	for i, ch := range code {
		sb.WriteString(fmt.Sprintf(
			`<text x="%d" y="%d" font-family="monospace" font-size="26" font-weight="bold" fill="%s" transform="rotate(%d %d %d)">%c</text>`,
			12+i*26, 32+r.Intn(6)-3, randColor(r), r.Intn(30)-15, 12+i*26, 28, ch))
	}
	sb.WriteString(`</svg>`)
	return sb.String()
}

func randColor(r *rand.Rand) string {
	return fmt.Sprintf("#%02x%02x%02x", 30+r.Intn(180), 30+r.Intn(180), 30+r.Intn(180))
}
