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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"svnadmin/internal/config"
	"svnadmin/internal/model"
	"svnadmin/internal/store"
	"svnadmin/internal/svn"
	"svnadmin/internal/util"
)

// 任务状态
const (
	TaskStatusWait   = 0 // 排队中
	TaskStatusRun    = 1 // 执行中
	TaskStatusDone   = 2 // 已完成
	TaskStatusFail   = 3 // 失败
	TaskStatusCancel = 4 // 已取消/已中断
)

// 任务类型
const (
	TaskTypeDump    = "dump"
	TaskTypeLoad    = "load"
	TaskTypeVerify  = "verify"
	TaskTypeSync    = "sync"
	TaskTypeShell   = "shell"
	TaskTypeCrond   = "crond"
	TaskTypeHotcopy = "hotcopy"
)

// 日志文件超过该大小时只返回路径提示，避免一次性拉取过大内容
const taskLogMaxRead = 2 << 20 // 2MB

// CreateTask 创建任务记录
func CreateTask(name, typ, cmd string) *model.Task {
	t := &model.Task{
		TaskName:       name,
		TaskType:       typ,
		TaskCmd:        cmd,
		TaskUnique:     util.RandomHex(16),
		TaskStatus:     TaskStatusWait,
		TaskCreateTime: model.Now(),
	}
	store.DB().Create(t)
	return t
}

// TaskLogFile 返回任务日志文件路径
func TaskLogFile(t *model.Task) string {
	return filepath.Join(config.C().SVN.LogBasePath(), t.TaskUnique+".log")
}

// ---------------------------------------------------------------------------
// 任务队列
// ---------------------------------------------------------------------------

type queueItem struct {
	task   *model.Task
	build  func() (*exec.Cmd, error)
	after  func() error
	cancel chan struct{}
}

var (
	taskQueue   chan *queueItem
	taskQueueMu sync.Mutex
	// running 保存正在执行任务的进程 ID，key 为 task_id
	running   = map[int64]int{}
	runningMu sync.Mutex
	// cancelSignals 保存运行中任务的取消信号
	cancelSignals = map[int64]chan struct{}{}
)

// StartTaskWorker 启动任务队列工作协程
// concurrency 为并发度，同一时刻最多执行多少个任务；<=0 时取 1（串行，避免 svnadmin 并发冲突）
func StartTaskWorker(concurrency int) {
	if concurrency <= 0 {
		concurrency = 1
	}
	taskQueueMu.Lock()
	defer taskQueueMu.Unlock()
	if taskQueue != nil {
		return
	}
	taskQueue = make(chan *queueItem, 1024)
	// 清理上次进程遗留的任务：运行中的进程已随主进程退出，
	// 若不清理会永久停留在"执行中"（对齐原版守护进程 ClearTask 的语义）
	ClearStaleTasks()
	for i := 0; i < concurrency; i++ {
		go func() {
			for item := range taskQueue {
				runQueueItem(item)
			}
		}()
	}
}

func pushTask(item *queueItem) {
	StartTaskWorker(1)
	select {
	case taskQueue <- item:
	default:
		// 队列满时直接标记失败，避免阻塞调用方
		finishTask(item.task, errors.New("任务队列已满，请稍后重试"), nil)
	}
}

func runQueueItem(item *queueItem) {
	t := item.task
	select {
	case <-item.cancel:
		markTaskCanceled(t, "已取消排队")
		return
	default:
	}

	cmd, err := item.build()
	if err != nil {
		finishTask(t, err, nil)
		return
	}
	if cmd.Path == "" && len(cmd.Args) == 0 {
		// 纯函数型任务由调用方自行处理
		return
	}
	setProcGroup(cmd)

	logPath := TaskLogFile(t)
	lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		finishTask(t, err, nil)
		return
	}
	defer lf.Close()

	_, _ = fmt.Fprintf(lf, "==> 任务开始：%s\n==> 命令：%s\n==> 时间：%s\n", t.TaskName, t.TaskCmd, model.Now())
	if cmd.Stderr == nil {
		cmd.Stderr = lf
	} else {
		cmd.Stderr = io.MultiWriter(cmd.Stderr, lf)
	}
	if cmd.Stdout == nil {
		cmd.Stdout = lf
	} else {
		cmd.Stdout = io.MultiWriter(cmd.Stdout, lf)
	}

	store.DB().Model(t).Updates(map[string]interface{}{
		"task_status":      TaskStatusRun,
		"task_update_time": model.Now(),
		"task_log_file":    logPath,
	})

	if err := cmd.Start(); err != nil {
		finishTask(t, err, lf)
		return
	}
	pid := cmd.Process.Pid
	runningMu.Lock()
	running[t.TaskID] = pid
	cancelSignals[t.TaskID] = item.cancel
	runningMu.Unlock()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var runErr error
	select {
	case runErr = <-done:
	case <-item.cancel:
		killProcGroup(pid)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
		runErr = errors.New("任务已被中断")
	}

	runningMu.Lock()
	delete(running, t.TaskID)
	delete(cancelSignals, t.TaskID)
	runningMu.Unlock()

	if runErr == nil && item.after != nil {
		runErr = item.after()
	}
	finishTask(t, runErr, lf)
}

func markTaskCanceled(t *model.Task, msg string) {
	store.DB().Model(&model.Task{}).Where("task_id = ?", t.TaskID).Updates(map[string]interface{}{
		"task_status":      TaskStatusCancel,
		"task_message":     msg,
		"task_update_time": model.Now(),
	})
}

func finishTask(t *model.Task, err error, lf io.Writer) {
	upd := map[string]interface{}{
		"task_update_time": model.Now(),
	}
	if lf != nil {
		if err != nil {
			_, _ = fmt.Fprintf(lf, "\n==> 执行失败：%v\n", err)
		} else {
			_, _ = fmt.Fprintf(lf, "\n==> 执行成功，结束时间：%s\n", model.Now())
		}
	}
	if err != nil {
		if err.Error() == "任务已被中断" {
			upd["task_status"] = TaskStatusCancel
		} else {
			upd["task_status"] = TaskStatusFail
		}
		upd["task_message"] = err.Error()
	} else {
		upd["task_status"] = TaskStatusDone
		upd["task_message"] = "执行成功"
	}
	store.DB().Model(&model.Task{}).Where("task_id = ?", t.TaskID).Updates(upd)

	// 任务结束后按推送开关发送邮件通知
	if err == nil {
		go SendMailByTrigger(MailTriggerTaskSuccess, "SVNAdmin4 任务执行成功", taskMailBody(t, nil))
	} else if err.Error() != "任务已被中断" {
		go SendMailByTrigger(MailTriggerTaskFail, "SVNAdmin4 任务执行失败", taskMailBody(t, err))
	}
}

func taskMailBody(t *model.Task, err error) string {
	var b strings.Builder
	b.WriteString("任务名称：" + t.TaskName + "\n")
	b.WriteString("任务类型：" + t.TaskType + "\n")
	b.WriteString("执行命令：" + t.TaskCmd + "\n")
	b.WriteString("创建时间：" + t.TaskCreateTime + "\n")
	if err != nil {
		b.WriteString("失败原因：" + err.Error() + "\n")
	}
	return b.String()
}

// RunTaskAsync 提交任务到队列异步执行
// build 构造实际执行的命令；after 在命令成功结束后回调（失败时不执行）
func RunTaskAsync(t *model.Task, build func() (*exec.Cmd, error), after func() error) {
	pushTask(&queueItem{task: t, build: build, after: after, cancel: make(chan struct{})})
}

// RunTaskFunc 提交一个纯 Go 函数任务（无外部进程，可取消排队，运行中不可中断）
func RunTaskFunc(t *model.Task, fn func() error) {
	go func() {
		store.DB().Model(t).Updates(map[string]interface{}{
			"task_status":      TaskStatusRun,
			"task_update_time": model.Now(),
		})
		err := fn()
		finishTask(t, err, nil)
	}()
}

// ---------------------------------------------------------------------------
// 查询
// ---------------------------------------------------------------------------

// TaskList 分页查询任务；status 为空表示全部
func TaskList(page, pageSize int, status []int, keyword string) (int64, []model.Task, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	q := store.DB().Model(&model.Task{})
	if len(status) > 0 {
		q = q.Where("task_status IN ?", status)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("task_name LIKE ? OR task_type LIKE ?", kw, kw)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	var list []model.Task
	if err := q.Order("task_id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return 0, nil, err
	}
	return total, list, nil
}

// TaskByUnique 按唯一标识查询任务
func TaskByUnique(unique string) (*model.Task, error) {
	var t model.Task
	if err := store.DB().Where("task_unique = ?", unique).First(&t).Error; err != nil {
		return nil, errors.New("任务不存在")
	}
	return &t, nil
}

// TaskByID 按 ID 查询任务
func TaskByID(id int64) (*model.Task, error) {
	var t model.Task
	if err := store.DB().Where("task_id = ?", id).First(&t).Error; err != nil {
		return nil, errors.New("任务不存在")
	}
	return &t, nil
}

// TaskRunInfo 当前运行任务概览
type TaskRunInfo struct {
	Running   bool         `json:"running"`
	Task      *model.Task  `json:"task"`
	Log       string       `json:"log"`
	LogTrimed bool         `json:"logTrimed"`
	Queue     []model.Task `json:"queue"`
	QueueCnt  int64        `json:"queueCount"`
}

// TaskRunning 返回当前运行中的任务及其日志与排队情况
func TaskRunning() (*TaskRunInfo, error) {
	var runTask model.Task
	err := store.DB().Where("task_status = ?", TaskStatusRun).Order("task_id ASC").First(&runTask).Error
	info := &TaskRunInfo{}
	if err == nil {
		info.Running = true
		info.Task = &runTask
		info.Log, info.LogTrimed = readTaskLog(&runTask)
	}
	var queue []model.Task
	store.DB().Where("task_status = ?", TaskStatusWait).Order("task_id ASC").Find(&queue)
	info.Queue = queue
	var cnt int64
	store.DB().Model(&model.Task{}).Where("task_status IN ?", []int{TaskStatusWait, TaskStatusRun}).Count(&cnt)
	info.QueueCnt = cnt
	return info, nil
}

// TaskLog 读取任务日志
func TaskLog(id int64) (string, bool, error) {
	t, err := TaskByID(id)
	if err != nil {
		return "", false, err
	}
	s, trimmed := readTaskLog(t)
	return s, trimmed, nil
}

func readTaskLog(t *model.Task) (string, bool) {
	path := t.TaskLogFile
	if path == "" {
		path = TaskLogFile(t)
	}
	st, err := os.Stat(path)
	if err != nil {
		if t.TaskStatus == TaskStatusWait {
			return "任务排队中，尚未产生日志", false
		}
		return "（暂无日志）", false
	}
	if st.Size() > taskLogMaxRead {
		return fmt.Sprintf("日志文件过大（%.1f MB），请直接查看：%s", float64(st.Size())/(1<<20), path), true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "（日志读取失败）", false
	}
	s := string(data)
	if s == "" {
		return "（暂无日志）", false
	}
	return s, false
}

// ---------------------------------------------------------------------------
// 控制
// ---------------------------------------------------------------------------

// TaskStop 中断运行中的任务或取消排队中的任务
func TaskStop(id int64) error {
	t, err := TaskByID(id)
	if err != nil {
		return err
	}
	switch t.TaskStatus {
	case TaskStatusRun:
		runningMu.Lock()
		pid, ok := running[t.TaskID]
		sig, hasSig := cancelSignals[t.TaskID]
		runningMu.Unlock()
		if hasSig {
			select {
			case <-sig:
			default:
				close(sig)
			}
		}
		if ok {
			killProcGroup(pid)
		}
		markTaskCanceled(t, "已被手动中断")
		return nil
	case TaskStatusWait:
		markTaskCanceled(t, "已取消排队")
		return nil
	default:
		return fmt.Errorf("任务已结束，无需中断")
	}
}

// TaskDel 删除任务记录及其日志文件
func TaskDel(id int64) error {
	t, err := TaskByID(id)
	if err != nil {
		return err
	}
	if t.TaskStatus == TaskStatusRun {
		return errors.New("任务正在执行，请先中断")
	}
	path := t.TaskLogFile
	if path == "" {
		path = TaskLogFile(t)
	}
	_ = os.Remove(path)
	return store.DB().Where("task_id = ?", id).Delete(&model.Task{}).Error
}

// TaskClear 清空历史任务（已完成/失败/取消）
func TaskClear() error {
	status := []int{TaskStatusDone, TaskStatusFail, TaskStatusCancel}
	var list []model.Task
	store.DB().Where("task_status IN ?", status).Find(&list)
	for i := range list {
		p := list[i].TaskLogFile
		if p == "" {
			p = TaskLogFile(&list[i])
		}
		_ = os.Remove(p)
	}
	return store.DB().Where("task_status IN ?", status).Delete(&model.Task{}).Error
}

// ---------------------------------------------------------------------------
// 备份 / 恢复 / 检查 / 热拷贝任务
// ---------------------------------------------------------------------------

// StartDump 启动仓库备份任务，支持版本范围与增量
func StartDump(repName, revRange string, incremental bool) (*model.Task, error) {
	if !svn.RepExists(repName) {
		return nil, fmt.Errorf("仓库 [%s] 不存在", repName)
	}
	name := fmt.Sprintf("%s_%s.dump", repName, time.Now().Format("20060102150405"))
	bin, err := svn.Resolve(svn.BinSVNAdmin)
	if err != nil {
		return nil, err
	}

	args := []string{"dump", svn.RepPath(repName), "--quiet"}
	if incremental {
		args = append(args, "--deltas")
	}
	if revRange != "" {
		args = append(args, "-r", revRange)
	}

	task := CreateTask(fmt.Sprintf("备份仓库 [%s]", repName), TaskTypeDump, svn.BinSVNAdmin+" "+strings.Join(args, " "))
	task.TaskOptional = name
	store.DB().Model(task).Update("task_optional", name)

	RunTaskAsync(task, func() (*exec.Cmd, error) {
		outFile, err := BackupPath(name)
		if err != nil {
			return nil, err
		}
		f, err := os.Create(outFile)
		if err != nil {
			return nil, err
		}
		cmd := exec.Command(bin, args...)
		cmd.Stdout = f
		return cmd, nil
	}, func() error {
		outFile, _ := BackupPath(name)
		st, err := os.Stat(outFile)
		if err != nil {
			return errors.New("备份文件未生成")
		}
		if st.Size() == 0 {
			_ = os.Remove(outFile)
			return errors.New("备份文件为空，备份失败")
		}
		return nil
	})
	return task, nil
}

// StartLoad 启动仓库恢复任务
func StartLoad(repName, fileName string) (*model.Task, error) {
	inFile, err := BackupPath(fileName)
	if err != nil {
		return nil, err
	}
	if _, err := statFile(inFile); err != nil {
		return nil, errors.New("备份文件不存在")
	}
	bin, err := svn.Resolve(svn.BinSVNAdmin)
	if err != nil {
		return nil, err
	}
	task := CreateTask(fmt.Sprintf("导入仓库 [%s]", repName), TaskTypeLoad,
		fmt.Sprintf("%s load %s < %s", svn.BinSVNAdmin, svn.RepPath(repName), fileName))
	task.TaskOptional = fileName
	store.DB().Model(task).Update("task_optional", fileName)

	RunTaskAsync(task, func() (*exec.Cmd, error) {
		if !svn.RepExists(repName) {
			if err := svn.CreateRep(repName, ""); err != nil {
				return nil, err
			}
			store.DB().Create(&model.SvnRep{RepName: repName, RepNote: "由备份文件导入"})
		}
		f, err := os.Open(inFile)
		if err != nil {
			return nil, err
		}
		cmd := exec.Command(bin, "load", svn.RepPath(repName), "--quiet")
		cmd.Stdin = f
		return cmd, nil
	}, func() error {
		refreshRep(repName)
		return nil
	})
	return task, nil
}

// StartVerify 启动仓库检查任务（svnadmin verify）
func StartVerify(repName string) (*model.Task, error) {
	if !svn.RepExists(repName) {
		return nil, fmt.Errorf("仓库 [%s] 不存在", repName)
	}
	bin, err := svn.Resolve(svn.BinSVNAdmin)
	if err != nil {
		return nil, err
	}
	task := CreateTask(fmt.Sprintf("检查仓库 [%s]", repName), TaskTypeVerify,
		fmt.Sprintf("%s verify %s", svn.BinSVNAdmin, svn.RepPath(repName)))
	RunTaskAsync(task, func() (*exec.Cmd, error) {
		return exec.Command(bin, "verify", svn.RepPath(repName)), nil
	}, nil)
	return task, nil
}

// StartHotcopy 启动仓库热拷贝任务（原版为空实现，此处为完整实现）
func StartHotcopy(repName string, incremental bool) (*model.Task, error) {
	if !svn.RepExists(repName) {
		return nil, fmt.Errorf("仓库 [%s] 不存在", repName)
	}
	bin, err := svn.Resolve(svn.BinSVNAdmin)
	if err != nil {
		return nil, err
	}
	suffix := "full"
	args := []string{"hotcopy", svn.RepPath(repName)}
	if incremental {
		suffix = "incr"
		args = append(args, "--incremental")
	}
	dst := filepath.Join(config.C().SVN.BackupBasePath(),
		fmt.Sprintf("%s_hotcopy_%s_%s", repName, suffix, time.Now().Format("20060102150405")))
	args = append(args, dst)

	task := CreateTask(fmt.Sprintf("热拷贝仓库 [%s]", repName), TaskTypeHotcopy, svn.BinSVNAdmin+" "+strings.Join(args, " "))
	RunTaskAsync(task, func() (*exec.Cmd, error) {
		if incremental {
			if _, err := os.Stat(dst); err != nil {
				return nil, errors.New("增量热拷贝需要已存在的目标目录，请先执行一次全量热拷贝")
			}
		}
		return exec.Command(bin, args...), nil
	}, nil)
	return task, nil
}

// StartShell 启动自定义脚本任务
func StartShell(name, script string) (*model.Task, error) {
	if strings.TrimSpace(script) == "" {
		return nil, errors.New("脚本内容不能为空")
	}
	task := CreateTask(name, TaskTypeShell, script)
	RunTaskAsync(task, func() (*exec.Cmd, error) {
		return buildShellCmd(script)
	}, nil)
	return task, nil
}

// TaskTypes 返回任务类型清单
func TaskTypes() []map[string]interface{} {
	return []map[string]interface{}{
		{"value": TaskTypeDump, "label": "仓库备份"},
		{"value": TaskTypeLoad, "label": "仓库恢复"},
		{"value": TaskTypeVerify, "label": "仓库检查"},
		{"value": TaskTypeHotcopy, "label": "仓库热拷贝"},
		{"value": TaskTypeSync, "label": "数据同步"},
		{"value": TaskTypeShell, "label": "脚本任务"},
		{"value": TaskTypeCrond, "label": "定时任务"},
	}
}

// TaskStatusMap 状态编码说明
func TaskStatusMap() []map[string]interface{} {
	return []map[string]interface{}{
		{"value": TaskStatusWait, "label": "排队中"},
		{"value": TaskStatusRun, "label": "执行中"},
		{"value": TaskStatusDone, "label": "已完成"},
		{"value": TaskStatusFail, "label": "失败"},
		{"value": TaskStatusCancel, "label": "已取消"},
	}
}

// ClearStaleTasks 清理上次进程遗留的未完成任务。
// 原版由守护进程 ClearTask 完成：运行中的任务其进程已消失则标记为异常终止。
// 这里统一标记为"已取消（进程重启中断）"，避免任务永久停留在执行中无法操作。
func ClearStaleTasks() int {
	var list []model.Task
	if err := store.DB().Where("task_status IN ?", []int{TaskStatusWait, TaskStatusRun}).Find(&list).Error; err != nil {
		return 0
	}
	now := model.Now()
	for _, t := range list {
		msg := "服务重启，任务已中断"
		if t.TaskStatus == TaskStatusWait {
			msg = "服务重启，排队任务已取消"
		}
		store.DB().Model(&model.Task{}).Where("task_id = ?", t.TaskID).Updates(map[string]interface{}{
			"task_status":      TaskStatusCancel,
			"task_message":     msg,
			"task_update_time": now,
		})
	}
	return len(list)
}
