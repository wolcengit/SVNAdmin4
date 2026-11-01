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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"svnadmin/internal/config"
	"svnadmin/internal/model"
)

// ---------------------------------------------------------------------------
// 消息推送触发点
// ---------------------------------------------------------------------------

const (
	MailTriggerLogin      = "Common/Login"
	MailTriggerAdminName  = "Personal/EditAdminUserName"
	MailTriggerAdminPass  = "Personal/EditAdminUserPass"
	MailTriggerSvnUserPas = "Personal/EditSvnUserPass"
	// 以下两项为 SVNAdmin4 扩展（原版仅支持前 4 项）
	MailTriggerTaskSuccess = "Task/Success"
	MailTriggerTaskFail    = "Task/Fail"
)

// PushTrigger 推送触发点定义
type PushTrigger struct {
	Trigger string `json:"trigger"`
	Type    string `json:"type"`
	Note    string `json:"note"`
	Enable  bool   `json:"enable"`
}

// DefaultPushTriggers 默认推送触发点
func DefaultPushTriggers() []PushTrigger {
	return []PushTrigger{
		{MailTriggerLogin, "mail", "用户登录时推送", false},
		{MailTriggerAdminName, "mail", "管理人员修改账户名时推送", false},
		{MailTriggerAdminPass, "mail", "管理人员修改密码时推送", false},
		{MailTriggerSvnUserPas, "mail", "SVN 用户修改密码时推送", false},
		{MailTriggerTaskSuccess, "mail", "后台任务执行成功时推送", false},
		{MailTriggerTaskFail, "mail", "后台任务执行失败时推送", false},
	}
}

// ---------------------------------------------------------------------------
// SMTP 配置
// ---------------------------------------------------------------------------

// SMTPConfig 邮件服务配置
type SMTPConfig struct {
	Host       string   `json:"host"`
	Port       int      `json:"port"`
	Encryption string   `json:"encryption"` // none | SSL | TLS
	AutoTLS    bool     `json:"autoTls"`
	Auth       bool     `json:"auth"`
	User       string   `json:"user"`
	Pass       string   `json:"pass"`
	From       string   `json:"from"`
	To         []string `json:"to"`
	Timeout    int      `json:"timeout"`
	Status     bool     `json:"status"`
}

// GetSMTP 读取邮件配置（缺失时用 yaml 配置初始化）
func GetSMTP() SMTPConfig {
	var c SMTPConfig
	raw := GetOption(OptionMailSMTP, "")
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &c); err == nil {
			return c
		}
	}
	m := config.C().Mail
	c = SMTPConfig{
		Host:       m.Host,
		Port:       m.Port,
		Encryption: m.Encryption,
		AutoTLS:    m.AutoTLS,
		Auth:       m.Auth,
		User:       m.User,
		Pass:       m.Pass,
		From:       m.From,
		Timeout:    m.Timeout,
		Status:     m.Status,
	}
	for _, t := range strings.Split(m.To, ",") {
		if t = strings.TrimSpace(t); t != "" {
			c.To = append(c.To, t)
		}
	}
	return c
}

// SaveSMTP 保存邮件配置
func SaveSMTP(c SMTPConfig) error {
	if c.Port == 0 {
		c.Port = 25
	}
	if c.Encryption == "" {
		c.Encryption = "none"
	}
	if c.Timeout <= 0 {
		c.Timeout = 5
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return SetOption(OptionMailSMTP, string(data), "邮件服务 SMTP 配置")
}

// GetPushTriggers 读取推送开关
func GetPushTriggers() []PushTrigger {
	def := DefaultPushTriggers()
	raw := GetOption(OptionMessagePush, "")
	if raw == "" {
		return def
	}
	var saved []PushTrigger
	if err := json.Unmarshal([]byte(raw), &saved); err != nil || len(saved) == 0 {
		return def
	}
	// 与默认清单合并，保证新增触发点能自动出现
	idx := map[string]PushTrigger{}
	for _, t := range saved {
		idx[t.Trigger] = t
	}
	out := make([]PushTrigger, 0, len(def))
	for _, d := range def {
		if s, ok := idx[d.Trigger]; ok {
			d.Enable = s.Enable
			if s.Note != "" {
				d.Note = s.Note
			}
		}
		out = append(out, d)
	}
	return out
}

// SavePushTriggers 保存推送开关
func SavePushTriggers(list []PushTrigger) error {
	data, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return SetOption(OptionMessagePush, string(data), "消息推送开关")
}

// ---------------------------------------------------------------------------
// 发送
// ---------------------------------------------------------------------------

func smtpAddr(c SMTPConfig) string {
	port := c.Port
	if port == 0 {
		if c.Encryption == "SSL" {
			port = 465
		} else {
			port = 25
		}
	}
	return fmt.Sprintf("%s:%d", c.Host, port)
}

func dialSMTP(c SMTPConfig) (*smtp.Client, error) {
	timeout := time.Duration(c.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if c.Encryption == "SSL" {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", smtpAddr(c),
			&tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return nil, fmt.Errorf("SSL 连接失败：%v", err)
		}
		return smtp.NewClient(conn, c.Host)
	}
	conn, err := net.DialTimeout("tcp", smtpAddr(c), timeout)
	if err != nil {
		return nil, fmt.Errorf("连接邮件服务器失败：%v", err)
	}
	client, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		return nil, err
	}
	if c.Encryption == "TLS" || c.AutoTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return nil, fmt.Errorf("STARTTLS 协商失败：%v", err)
			}
		} else if c.Encryption == "TLS" {
			return nil, fmt.Errorf("邮件服务器不支持 STARTTLS")
		}
	}
	return client, nil
}

// SendMail 发送邮件；to 为空时使用配置中的收件人
func SendMail(to []string, subject, body string) error {
	return sendMailWith(GetSMTP(), to, subject, body)
}

// sendMailWith 使用指定 SMTP 配置发送邮件
func sendMailWith(c SMTPConfig, to []string, subject, body string) error {
	if !c.Status {
		return fmt.Errorf("邮件服务未启用")
	}
	if c.Host == "" {
		return fmt.Errorf("未配置 SMTP 服务器")
	}
	if len(to) == 0 {
		to = c.To
	}
	if len(to) == 0 {
		return fmt.Errorf("未配置收件人")
	}
	from := c.From
	if from == "" {
		from = c.User
	}
	if from == "" {
		return fmt.Errorf("未配置发件人")
	}

	client, err := dialSMTP(c)
	if err != nil {
		return err
	}
	defer client.Close()

	if c.Auth && c.User != "" {
		host := c.Host
		if i := strings.LastIndex(host, ":"); i > 0 {
			host = host[:i]
		}
		if err := client.Auth(smtp.PlainAuth("", c.User, c.Pass, host)); err != nil {
			// PLAIN 失败时回退 LOGIN，兼容 Exchange 等服务器
			if err2 := loginAuth(client, c.User, c.Pass); err2 != nil {
				return fmt.Errorf("SMTP 认证失败：%v", err)
			}
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, r := range to {
		if err := client.Rcpt(r); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(buildMailBody(from, to, subject, body))
	if err != nil {
		return err
	}
	return w.Close()
}

// loginAuth LOGIN 认证（部分服务器不接受 PLAIN）
func loginAuth(c *smtp.Client, user, pass string) error {
	enc := base64.StdEncoding
	if _, err := c.Text.Cmd("AUTH LOGIN"); err != nil {
		return err
	}
	if _, err := c.Text.Cmd(enc.EncodeToString([]byte(user))); err != nil {
		return err
	}
	_, err := c.Text.Cmd(enc.EncodeToString([]byte(pass)))
	return err
}

func buildMailBody(from string, to []string, subject, body string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(to, ",") + "\r\n")
	b.WriteString("Subject: =?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(subject)) + "?=\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(base64.StdEncoding.EncodeToString([]byte(body)) + "\r\n")
	return []byte(b.String())
}

// SendMailTest 发送测试邮件。
// override 为页面上可能尚未保存的参数（非零字段覆盖已保存配置，对齐原版"先测后存"体验）；
// 全部为空时退化为使用已保存配置。
func SendMailTest(override SMTPConfig, to string) error {
	c := GetSMTP()
	// 判断是否携带了实质参数：全部为零值时不覆盖任何字段（避免零值 bool 误覆盖已保存开关）
	hasOverride := override.Host != "" || override.Port > 0 || override.Encryption != "" ||
		override.User != "" || override.Pass != "" || override.From != "" ||
		override.Timeout > 0 || len(override.To) > 0 || override.AutoTLS || override.Auth
	if hasOverride {
		if override.Host != "" {
			c.Host = override.Host
		}
		if override.Port > 0 {
			c.Port = override.Port
		}
		if override.Encryption != "" {
			c.Encryption = override.Encryption
		}
		if override.User != "" {
			c.User = override.User
		}
		if override.Pass != "" {
			c.Pass = override.Pass
		}
		if override.From != "" {
			c.From = override.From
		}
		if override.Timeout > 0 {
			c.Timeout = override.Timeout
		}
		if len(override.To) > 0 {
			c.To = override.To
		}
		if override.AutoTLS {
			c.AutoTLS = true
		}
		if override.Auth {
			c.Auth = true
		}
		// 显式发起测试视为开启邮件功能
		c.Status = true
	}
	var list []string
	if to != "" {
		list = []string{to}
	}
	return sendMailWith(c, list, "SVNAdmin4 的测试邮件",
		"这是一封来自 SVNAdmin4 的测试邮件。\r\n时间："+model.Now()+"\r\n如果您收到了这封邮件，说明邮件服务配置正确。")
}

// SendMailByTrigger 按触发点发送邮件（自动校验开关）
func SendMailByTrigger(trigger, subject, body string) {
	triggers := GetPushTriggers()
	for _, t := range triggers {
		if t.Trigger == trigger {
			if !t.Enable {
				return
			}
			if err := SendMail(nil, subject, body); err != nil {
				AddLog("邮件推送", fmt.Sprintf("推送 [%s] 失败：%v", trigger, err), "system")
			}
			return
		}
	}
}

// SendMailDirect 跳过触发点校验直接发送（定时任务通知等场景）
func SendMailDirect(subject, body string) error {
	return SendMail(nil, subject, body)
}
