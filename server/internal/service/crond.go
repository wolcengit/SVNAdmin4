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
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"svnadmin/internal/config"
	"svnadmin/internal/model"
	"svnadmin/internal/store"
)

// 定时任务类型（与原版编号保持一致，3/4 在原版为空实现，此处为完整实现）
const (
	CrondTypeDumpFull   = 1 // 仓库备份 dump 全量
	CrondTypeDumpDelta  = 2 // 仓库备份 dump 增量 deltas
	CrondTypeHotcopyAll = 3 // 仓库备份 hotcopy 全量
	CrondTypeHotcopyInc = 4 // 仓库备份 hotcopy 增量
	CrondTypeVerify     = 5 // 仓库检查 svnadmin verify
	CrondTypeShell      = 6 // shell 脚本
	CrondTypeSyncUser   = 7 // 同步 SVN 用户
	CrondTypeSyncGroup  = 8 // 同步 SVN 分组
	CrondTypeSyncRep    = 9 // 同步 SVN 仓库
)

// 消息通知取值
const (
	NoticeNone = 0 // 不通知
	NoticeSucc = 1 // 仅成功
	NoticeFail = 2 // 仅失败
	NoticeAll  = 3 // 成功 + 失败
)

// CrondTypes 定时任务类型清单
func CrondTypes() []map[string]interface{} {
	return []map[string]interface{}{
		{"value": CrondTypeDumpFull, "label": "仓库备份[dump-全量]", "needRep": true},
		{"value": CrondTypeDumpDelta, "label": "仓库备份[dump-增量-deltas]", "needRep": true},
		{"value": CrondTypeHotcopyAll, "label": "仓库备份[hotcopy-全量]", "needRep": true},
		{"value": CrondTypeHotcopyInc, "label": "仓库备份[hotcopy-增量]", "needRep": true},
		{"value": CrondTypeVerify, "label": "仓库检查", "needRep": true},
		{"value": CrondTypeShell, "label": "shell 脚本", "needRep": false},
		{"value": CrondTypeSyncUser, "label": "同步 SVN 用户", "needRep": false},
		{"value": CrondTypeSyncGroup, "label": "同步 SVN 分组", "needRep": false},
		{"value": CrondTypeSyncRep, "label": "同步 SVN 仓库", "needRep": false},
	}
}

// CycleTypes 支持的周期类型
func CycleTypes() []map[string]interface{} {
	return []map[string]interface{}{
		{"value": "minute", "label": "每分钟"},
		{"value": "minute_n", "label": "每隔 N 分钟"},
		{"value": "hour", "label": "每小时"},
		{"value": "hour_n", "label": "每隔 N 小时"},
		{"value": "day", "label": "每天"},
		{"value": "day_n", "label": "每隔 N 天"},
		{"value": "week", "label": "每周"},
		{"value": "month", "label": "每月"},
	}
}

var (
	cronMu    sync.Mutex
	cronSched = cron.New(cron.WithSeconds())
	cronJobs  = map[int64]cron.EntryID{}
)

// cronSpec 依据周期类型生成 cron 表达式（含秒），与原版 8 种周期一致
func cronSpec(c *model.Crond) (string, error) {
	m := c.Minute
	if m < 0 {
		m = 0
	}
	if m > 59 {
		m = 59
	}
	h := c.Hour
	if h < 0 {
		h = 0
	}
	if h > 23 {
		h = 23
	}
	switch c.CycleType {
	case "minute":
		return "0 * * * * *", nil
	case "minute_n":
		n := c.Minute
		if n <= 1 {
			n = 2
		}
		if n > 59 {
			n = 59
		}
		return fmt.Sprintf("0 */%d * * * *", n), nil
	case "hour":
		return fmt.Sprintf("0 %d * * * *", m), nil
	case "hour_n":
		n := c.Hour
		if n <= 1 {
			n = 2
		}
		if n > 23 {
			n = 23
		}
		return fmt.Sprintf("0 %d */%d * * *", m, n), nil
	case "day":
		return fmt.Sprintf("0 %d %d * * *", m, h), nil
	case "day_n":
		n := c.Day
		if n <= 1 {
			n = 2
		}
		if n > 31 {
			n = 31
		}
		return fmt.Sprintf("0 %d %d */%d * *", m, h, n), nil
	case "week":
		w := c.Week
		if w < 0 || w > 6 {
			w = 0
		}
		return fmt.Sprintf("0 %d %d * * %d", m, h, w), nil
	case "month":
		d := c.Day
		if d <= 0 {
			d = 1
		}
		if d > 31 {
			d = 31
		}
		return fmt.Sprintf("0 %d %d %d * *", m, h, d), nil
	}
	// 兼容旧数据：直接使用自定义 cron 表达式
	if c.Code != "" && strings.Count(c.Code, " ") >= 4 {
		return strings.TrimSpace(c.Code), nil
	}
	return "", fmt.Errorf("不支持的周期类型 [%s]", c.CycleType)
}

// crondDesc 生成执行周期的中文描述
func crondDesc(c *model.Crond) string {
	switch c.CycleType {
	case "minute":
		return "每分钟执行一次"
	case "minute_n":
		return fmt.Sprintf("每隔 %d 分钟执行一次", c.Minute)
	case "hour":
		return fmt.Sprintf("每小时第 %d 分钟执行一次", c.Minute)
	case "hour_n":
		return fmt.Sprintf("每隔 %d 小时的第 %d 分钟执行一次", c.Hour, c.Minute)
	case "day":
		return fmt.Sprintf("每天 %d:%02d 执行一次", c.Hour, c.Minute)
	case "day_n":
		return fmt.Sprintf("每隔 %d 天的 %d:%02d 执行一次", c.Day, c.Hour, c.Minute)
	case "week":
		names := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
		w := c.Week
		if w < 0 || w > 6 {
			w = 0
		}
		return fmt.Sprintf("每周%s %d:%02d 执行一次", names[w], c.Hour, c.Minute)
	case "month":
		return fmt.Sprintf("每月 %d 日 %d:%02d 执行一次", c.Day, c.Hour, c.Minute)
	}
	return c.CycleDesc
}

// CrondList 查询定时任务
// CrondList 定时任务列表。
// page/pageSize/keyword 均为空时返回全量（兼容前端全量拉取）；
// 携带分页参数时返回 SQL 分页结果（对齐原版 GetCrontabList 的服务端分页/搜索能力）。
func CrondList(page, pageSize int, keyword string) (interface{}, error) {
	q := store.DB().Model(&model.Crond{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("task_name LIKE ? OR rep_name LIKE ?", like, like)
	}
	if page <= 0 || pageSize <= 0 {
		var all []model.Crond
		err := q.Order("crond_id DESC").Find(&all).Error
		return all, err
	}
	if pageSize > 500 {
		pageSize = 500
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var list []model.Crond
	if err := q.Order("crond_id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"total": total, "list": list}, nil
}

// CrondCreate 新建定时任务
func CrondCreate(c *model.Crond) error {
	if c.CycleDesc == "" {
		c.CycleDesc = crondDesc(c)
	}
	spec, err := cronSpec(c)
	if err != nil {
		return err
	}
	if c.SaveCount <= 0 {
		c.SaveCount = 1
	}
	if c.SaveCount > 100 {
		c.SaveCount = 100
	}
	c.Code = spec
	c.Sign = fmt.Sprintf("%s-%d", c.CycleType, time.Now().UnixNano())
	c.LastExecTime = "-"
	c.CreateTime = model.Now()
	if err := store.DB().Create(c).Error; err != nil {
		return err
	}
	c.Sign = fmt.Sprintf("%s-%d", c.CycleType, c.CrondID)
	_ = store.DB().Model(&model.Crond{}).Where("crond_id = ?", c.CrondID).Update("sign", c.Sign)
	if c.Status == 1 {
		registerCrond(c, spec)
	}
	AddLog("任务计划", fmt.Sprintf("新建定时任务 [%s]", c.TaskName), "system")
	return nil
}

// CrondUpdate 修改定时任务
func CrondUpdate(c *model.Crond) error {
	c.CycleDesc = crondDesc(c)
	spec, err := cronSpec(c)
	if err != nil {
		return err
	}
	var old model.Crond
	if err := store.DB().Where("crond_id = ?", c.CrondID).First(&old).Error; err != nil {
		return errors.New("定时任务不存在")
	}
	unregisterCrond(c.CrondID)
	if c.SaveCount <= 0 {
		c.SaveCount = 1
	}
	if err := store.DB().Model(&model.Crond{}).Where("crond_id = ?", c.CrondID).
		Updates(map[string]interface{}{
			"task_type":  c.TaskType,
			"task_name":  c.TaskName,
			"cycle_type": c.CycleType,
			"cycle_desc": c.CycleDesc,
			"status":     c.Status,
			"save_count": c.SaveCount,
			"rep_name":   c.RepName,
			"week":       c.Week,
			"day":        c.Day,
			"hour":       c.Hour,
			"minute":     c.Minute,
			"notice":     c.Notice,
			"code":       spec,
			"shell":      c.Shell,
		}).Error; err != nil {
		return err
	}
	if c.Status == 1 {
		registerCrond(c, spec)
	}
	AddLog("任务计划", fmt.Sprintf("修改定时任务 [%s]", c.TaskName), "system")
	return nil
}

// CrondDel 删除定时任务
func CrondDel(id int64) error {
	unregisterCrond(id)
	_ = os.Remove(crondLogPathByID(id))
	if err := store.DB().Where("crond_id = ?", id).Delete(&model.Crond{}).Error; err != nil {
		return err
	}
	AddLog("任务计划", fmt.Sprintf("删除定时任务 ID=%d", id), "system")
	return nil
}

// CrondStart 加载并启动全部启用的定时任务
func CrondStart() error {
	var list []model.Crond
	if err := store.DB().Order("crond_id DESC").Find(&list).Error; err != nil {
		return err
	}
	for i := range list {
		c := list[i]
		if c.Status != 1 {
			continue
		}
		spec, err := cronSpec(&c)
		if err != nil {
			continue
		}
		registerCrond(&c, spec)
	}
	cronSched.Start()
	return nil
}

// CrondStop 停止调度器
func CrondStop() { cronSched.Stop() }

// CrondStatus 调度器运行状态
func CrondStatus() map[string]interface{} {
	cronMu.Lock()
	cnt := len(cronJobs)
	cronMu.Unlock()
	entries := cronSched.Entries()
	next := ""
	if len(entries) > 0 {
		next = entries[0].Next.Format("2006-01-02 15:04:05")
	}
	return map[string]interface{}{
		"running":   true,
		"jobCount":  cnt,
		"nextRun":   next,
		"scheduler": "内置调度器（不依赖系统 crond/atd）",
	}
}

func registerCrond(c *model.Crond, spec string) {
	cronMu.Lock()
	defer cronMu.Unlock()
	if id, ok := cronJobs[c.CrondID]; ok {
		cronSched.Remove(id)
		delete(cronJobs, c.CrondID)
	}
	id, err := cronSched.AddFunc(spec, func() { execCrond(c) })
	if err != nil {
		return
	}
	cronJobs[c.CrondID] = id
}

func unregisterCrond(id int64) {
	cronMu.Lock()
	defer cronMu.Unlock()
	if eid, ok := cronJobs[id]; ok {
		cronSched.Remove(eid)
		delete(cronJobs, id)
	}
}

// CrondTrigger 手动立即执行一次定时任务
func CrondTrigger(id int64) (*model.Task, error) {
	var c model.Crond
	if err := store.DB().Where("crond_id = ?", id).First(&c).Error; err != nil {
		return nil, errors.New("定时任务不存在")
	}
	reps := crondRepNames(&c)
	task, err := startCrondTask(&c, reps)
	if err != nil {
		return nil, err
	}
	AddLog("任务计划", fmt.Sprintf("手动触发定时任务 [%s]", c.TaskName), "system")
	return task, nil
}

// crondLogPathByID 返回定时任务日志文件路径
func crondLogPathByID(id int64) string {
	var c model.Crond
	if err := store.DB().Where("crond_id = ?", id).First(&c).Error; err != nil {
		return ""
	}
	return config.C().SVN.CrondLogFile(c.Sign)
}

// CrondLog 读取定时任务执行日志
func CrondLog(id int64) (string, error) {
	path := crondLogPathByID(id)
	if path == "" {
		return "", errors.New("定时任务不存在")
	}
	st, err := os.Stat(path)
	if err != nil {
		return "（暂无日志，任务尚未执行或日志未生成）", nil
	}
	if st.Size() > taskLogMaxRead {
		return fmt.Sprintf("日志文件过大（%.1f MB），请直接查看：%s", float64(st.Size())/(1<<20), path), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return "（暂无日志）", nil
	}
	return string(data), nil
}

// crondRepNames 解析定时任务作用的仓库列表；含 "-1" 表示全部仓库
func crondRepNames(c *model.Crond) []string {
	raw := strings.TrimSpace(c.RepName)
	if raw == "" {
		return nil
	}
	var names []string
	if strings.HasPrefix(raw, "[") {
		for _, p := range strings.Split(strings.Trim(raw, "[]"), ",") {
			p = strings.Trim(strings.TrimSpace(p), `"`)
			if p != "" {
				names = append(names, p)
			}
		}
	} else {
		names = []string{raw}
	}
	if len(names) == 1 && names[0] == "-1" {
		var all []model.SvnRep
		store.DB().Order("rep_name").Find(&all)
		out := make([]string, 0, len(all))
		for _, r := range all {
			out = append(out, r.RepName)
		}
		return out
	}
	return names
}

// startCrondTask 按任务类型启动执行
func startCrondTask(c *model.Crond, reps []string) (*model.Task, error) {
	switch c.TaskType {
	case CrondTypeDumpFull:
		if len(reps) == 0 {
			return nil, errors.New("请先选择仓库")
		}
		var last *model.Task
		for _, r := range reps {
			t, err := StartDump(r, "", false)
			if err != nil {
				return nil, err
			}
			last = t
			crondWatch(c, t)
		}
		if c.SaveCount > 0 {
			go cleanBackupsByCount(reps, "", c.SaveCount)
		}
		return last, nil

	case CrondTypeDumpDelta:
		if len(reps) == 0 {
			return nil, errors.New("请先选择仓库")
		}
		var last *model.Task
		for _, r := range reps {
			t, err := StartDump(r, "", true)
			if err != nil {
				return nil, err
			}
			last = t
			crondWatch(c, t)
		}
		if c.SaveCount > 0 {
			go cleanBackupsByCount(reps, "_deltas_", c.SaveCount)
		}
		return last, nil

	case CrondTypeHotcopyAll:
		if len(reps) == 0 {
			return nil, errors.New("请先选择仓库")
		}
		t, err := StartHotcopy(reps[0], false)
		if err != nil {
			return nil, err
		}
		crondWatch(c, t)
		return t, nil

	case CrondTypeHotcopyInc:
		if len(reps) == 0 {
			return nil, errors.New("请先选择仓库")
		}
		t, err := StartHotcopy(reps[0], true)
		if err != nil {
			return nil, err
		}
		crondWatch(c, t)
		return t, nil

	case CrondTypeVerify:
		if len(reps) == 0 {
			return nil, errors.New("请先选择仓库")
		}
		var last *model.Task
		for _, r := range reps {
			t, err := StartVerify(r)
			if err != nil {
				return nil, err
			}
			last = t
			crondWatch(c, t)
		}
		return last, nil

	case CrondTypeShell:
		if strings.TrimSpace(c.Shell) == "" {
			return nil, errors.New("脚本内容不能为空")
		}
		t, err := StartShell(fmt.Sprintf("定时任务 [%s]", c.TaskName), c.Shell)
		if err != nil {
			return nil, err
		}
		crondWatch(c, t)
		return t, nil

	case CrondTypeSyncUser, CrondTypeSyncGroup, CrondTypeSyncRep:
		ctype := c.TaskType
		label := map[int]string{
			CrondTypeSyncUser:  "同步 SVN 用户",
			CrondTypeSyncGroup: "同步 SVN 分组",
			CrondTypeSyncRep:   "同步 SVN 仓库",
		}[ctype]
		t := CreateTask(fmt.Sprintf("定时任务 [%s]", c.TaskName), TaskTypeSync, label)
		RunTaskFunc(t, func() error {
			switch ctype {
			case CrondTypeSyncUser:
				return SyncUsers()
			case CrondTypeSyncGroup:
				return SyncGroups()
			default:
				return SyncReps()
			}
		})
		return t, nil
	}
	return nil, fmt.Errorf("不支持的任务类型 [%d]", c.TaskType)
}

func execCrond(c *model.Crond) {
	store.DB().Model(&model.Crond{}).Where("crond_id = ?", c.CrondID).Update("last_exec_time", model.Now())
	reps := crondRepNames(c)
	task, err := startCrondTask(c, reps)
	if err != nil {
		appendCrondLog(c, fmt.Sprintf("[%s] 执行失败：%v", model.Now(), err))
		AddLog("任务计划", fmt.Sprintf("定时任务 [%s] 执行失败：%v", c.TaskName, err), "system")
		return
	}
	if task != nil {
		appendCrondLog(c, fmt.Sprintf("[%s] 已启动任务：%s（任务ID=%d）", model.Now(), task.TaskName, task.TaskID))
	}
	AddLog("任务计划", fmt.Sprintf("定时任务 [%s] 执行完成", c.TaskName), "system")
}

// crondWatch 监听任务结束并写日志与通知
func crondWatch(c *model.Crond, t *model.Task) {
	go func() {
		cnt := 0
		for cnt < 3600 {
			var cur model.Task
			if err := store.DB().Where("task_id = ?", t.TaskID).First(&cur).Error; err != nil {
				return
			}
			if cur.TaskStatus != TaskStatusWait && cur.TaskStatus != TaskStatusRun {
				status := "成功"
				if cur.TaskStatus != TaskStatusDone {
					status = "失败：" + cur.TaskMessage
				}
				appendCrondLog(c, fmt.Sprintf("[%s] 任务结束，状态：%s", model.Now(), status))
				crondNotify(c, status == "成功", cur.TaskName+" "+status)
				return
			}
			time.Sleep(2 * time.Second)
			cnt++
		}
	}()
}

// crondNotify 按通知开关发送邮件
func crondNotify(c *model.Crond, success bool, body string) {
	if c.Notice == NoticeNone {
		return
	}
	if success && c.Notice != NoticeSucc && c.Notice != NoticeAll {
		return
	}
	if !success && c.Notice != NoticeFail && c.Notice != NoticeAll {
		return
	}
	subject := "SVNAdmin4 定时任务通知"
	if !success {
		subject += "（失败）"
	}
	_ = SendMailDirect(subject, "定时任务："+c.TaskName+"\n执行周期："+c.CycleDesc+"\n结果："+body+"\n时间："+model.Now())
}

func appendCrondLog(c *model.Crond, line string) {
	path := config.C().SVN.CrondLogFile(c.Sign)
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line + "\n")
}

// cleanBackupByCount 按保存份数清理旧备份（含关键字用于区分增量备份）
func cleanBackupByCount(repName, keyword string, saveCount int) {
	if saveCount <= 0 {
		return
	}
	dir := config.C().SVN.BackupBasePath()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := repName + "_"
	if keyword != "" {
		prefix = repName + keyword
	}
	var files []os.FileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasPrefix(e.Name(), prefix) || !strings.HasSuffix(e.Name(), ".dump") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, info)
	}
	if len(files) <= saveCount {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().After(files[j].ModTime())
	})
	for _, f := range files[saveCount:] {
		_ = os.Remove(filepath.Join(dir, f.Name()))
	}
}

// cleanBackupsByCount 按保留份数清理过期备份。
// 原版仅支持单仓库，这里扩展为对每个目标仓库分别保留最新的 saveCount 份。
func cleanBackupsByCount(repNames []string, keyword string, saveCount int) {
	for _, rep := range repNames {
		if rep == "" {
			continue
		}
		cleanBackupByCount(rep, keyword, saveCount)
	}
}
