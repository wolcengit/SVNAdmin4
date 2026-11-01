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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"svnadmin/internal/config"
)

// 常用可执行文件
const (
	BinSVN      = "svn"
	BinSVNAdmin = "svnadmin"
	BinSVNLook  = "svnlook"
	BinSVNServe = "svnserve"
	BinSvnDump  = "svnrdump"
)

// BinDir 返回 svn 可执行文件所在目录
func BinDir() string { return config.C().SVN.BinPath }

// Resolve 解析可执行文件绝对路径
func Resolve(name string) (string, error) {
	if dir := BinDir(); dir != "" {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("未找到可执行文件 %s，请确认已安装 subversion 并配置 svn.bin_path", name)
	}
	return p, nil
}

// Result 命令执行结果
type Result struct {
	Stdout string
	Stderr string
	Code   int
}

// Success 是否执行成功
func (r Result) Success() bool { return r.Code == 0 }

// Error 将失败结果转换为 error
func (r Result) Error() error {
	if r.Success() {
		return nil
	}
	msg := strings.TrimSpace(r.Stderr)
	if msg == "" {
		msg = strings.TrimSpace(r.Stdout)
	}
	return errors.New(msg)
}

// Command 构造一个 svn 命令
type Command struct {
	name string
	args []string
	env  []string
	dir  string
	out  io.Writer
}

// New 创建命令
func New(name string, args ...string) *Command {
	return &Command{name: name, args: args}
}

// WithEnv 追加环境变量
func (c *Command) WithEnv(kv ...string) *Command {
	c.env = append(c.env, kv...)
	return c
}

// WithDir 设置工作目录
func (c *Command) WithDir(dir string) *Command {
	c.dir = dir
	return c
}

// WithStdout 将标准输出直接写入 w（用于 dump 等大输出场景）
func (c *Command) WithStdout(w io.Writer) *Command {
	c.out = w
	return c
}

// Run 执行命令并返回结果
func (c *Command) Run() (Result, error) {
	bin, err := Resolve(c.name)
	if err != nil {
		return Result{Code: -1}, err
	}
	cmd := exec.Command(bin, c.args...)
	if c.dir != "" {
		cmd.Dir = c.dir
	}
	if len(c.env) > 0 {
		cmd.Env = append(os.Environ(), c.env...)
	}
	// 关闭 svn 的交互提示与本地配置污染
	if c.name == BinSVN {
		cmd.Args = append(cmd.Args, "--non-interactive", "--no-auth-cache")
	}

	var stdout, stderr bytes.Buffer
	if c.out != nil {
		cmd.Stdout = io.MultiWriter(c.out, &stdout)
	} else {
		cmd.Stdout = &stdout
	}
	cmd.Stderr = &stderr

	err = cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.Code = ee.ExitCode()
			return res, res.Error()
		}
		res.Code = -1
		return res, err
	}
	res.Code = 0
	return res, nil
}

// Run 快捷执行并返回标准输出
func Run(name string, args ...string) (string, error) {
	res, err := New(name, args...).Run()
	if err != nil {
		return res.Stdout, err
	}
	return res.Stdout, nil
}

// CommonArgs 返回 svn 命令的通用鉴权参数
func CommonArgs(user, pass string) []string {
	if user == "" {
		return nil
	}
	args := []string{"--username", user}
	if pass != "" {
		args = append(args, "--password", pass)
	}
	return args
}

// RepPath 返回仓库在磁盘上的绝对路径
func RepPath(repName string) string {
	return filepath.Join(config.C().SVN.RepBasePath(), repName)
}

// RepURL 返回仓库的 file:// 访问地址
func RepURL(repName string) string {
	abs, err := filepath.Abs(RepPath(repName))
	if err != nil {
		abs = RepPath(repName)
	}
	abs = filepath.ToSlash(abs)
	if !strings.HasPrefix(abs, "/") {
		abs = "/" + abs
	}
	return "file://" + abs
}

// Version 返回 subversion 版本号
func Version() string {
	out, err := Run(BinSVN, "--version", "--quiet")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(out)
}

var (
	svnserveMu   sync.Mutex
	svnserveCmd  *exec.Cmd
	svnserveStop chan struct{}
)

// StartSvnserve 启动 svnserve（-d 守护模式），端口/地址来自配置
func StartSvnserve() error {
	return StartSvnserveWith(config.C().SVN.SvnservePort, config.C().SVN.SvnserveHost)
}

// StartSvnserveWith 按指定的监听端口与地址启动 svnserve。
// 说明：svnserve -d 会自行 fork 成守护进程并让父进程立即退出，
// 因此不能依赖 exec.Cmd 的 Process.Kill 停止服务，必须读 pid 文件。
func StartSvnserveWith(port int, host string) error {
	svnserveMu.Lock()
	defer svnserveMu.Unlock()
	if SvnserveStatus() {
		return errors.New("svnserve 已在运行")
	}
	cfg := config.C().SVN
	bin, err := Resolve(BinSVNServe)
	if err != nil {
		return err
	}
	// 清掉上一次的 pid 文件，避免把旧 pid 误判为当前实例
	_ = os.Remove(cfg.SvnservePidFile())
	args := []string{
		"-d",
		"-r", cfg.RepBasePath(),
		"--listen-port", fmt.Sprintf("%d", port),
		"--config-file", cfg.SvnConfFile(),
		"--pid-file", cfg.SvnservePidFile(),
		"--log-file", cfg.SvnserveLogFile(),
	}
	if host != "" {
		args = append(args, "--listen-host", host)
	}
	cmd := exec.Command(bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("启动 svnserve 失败：%v %s", err, strings.TrimSpace(string(out)))
	}
	// 等待 pid 文件写入（最多 3 秒）
	for i := 0; i < 30; i++ {
		if SvnserveStatus() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = cmd.Process
	return nil
}

// StopSvnserve 停止 svnserve：读 pid 文件发送 SIGTERM，必要时 SIGKILL
func StopSvnserve() error {
	svnserveMu.Lock()
	defer svnserveMu.Unlock()
	cfg := config.C().SVN
	data, err := os.ReadFile(cfg.SvnservePidFile())
	if err != nil || strings.TrimSpace(string(data)) == "" {
		// pid 文件缺失时退回进程表扫描，尽力而为
		if !killSvnserveByScan() {
			svnserveCmd = nil
			return nil
		}
		_ = os.Remove(cfg.SvnservePidFile())
		svnserveCmd = nil
		return nil
	}
	pid := strings.TrimSpace(string(data))
	procAlive := func() bool {
		_, e := os.Stat("/proc/" + pid)
		return e == nil
	}
	if procAlive() {
		_ = exec.Command("kill", "-15", pid).Run()
		for i := 0; i < 20 && procAlive(); i++ {
			time.Sleep(100 * time.Millisecond)
		}
		if procAlive() {
			_ = exec.Command("kill", "-9", pid).Run()
			time.Sleep(200 * time.Millisecond)
		}
		if procAlive() {
			return errors.New("svnserve 服务停止失败")
		}
	}
	_ = os.Remove(cfg.SvnservePidFile())
	svnserveCmd = nil
	return nil
}

// killSvnserveByScan 在 pid 文件缺失时按进程名扫描并终止 svnserve
func killSvnserveByScan() bool {
	out, err := exec.Command("sh", "-c", "ps -eo pid,args | grep -v grep | grep '[s]vnserve' | awk '{print $1}'").Output()
	if err != nil {
		return false
	}
	killed := false
	for _, f := range strings.Fields(string(out)) {
		if f == "" {
			continue
		}
		_ = exec.Command("kill", "-15", f).Run()
		killed = true
	}
	return killed
}

// SvnserveStatus 检测 svnserve 是否在运行
func SvnserveStatus() bool {
	cfg := config.C().SVN
	data, err := os.ReadFile(cfg.SvnservePidFile())
	if err != nil {
		return false
	}
	pid := strings.TrimSpace(string(data))
	if pid == "" {
		return false
	}
	_, err = os.Stat("/proc/" + pid)
	return err == nil
}
