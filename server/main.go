// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"svnadmin/internal/config"
	"svnadmin/internal/router"
	"svnadmin/internal/service"
	"svnadmin/internal/store"
	"svnadmin/internal/version"
)

func main() {
	cfgPath := flag.String("c", "./configs/config.yaml", "配置文件路径")
	webDir := flag.String("web", "./web/dist", "前端静态资源目录")
	showVer := flag.Bool("v", false, "显示版本号")
	flag.Parse()

	if *showVer {
		fmt.Println(version.Short())
		return
	}

	if _, err := os.Stat(*cfgPath); err != nil {
		log.Printf("[warn] 未找到配置文件 %s，使用内置默认配置", *cfgPath)
		config.Default()
	} else if _, err := config.Load(*cfgPath); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	cfg := config.C()

	if err := store.Init(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// 保证 authz / passwd / svnserve.conf 存在
	if err := service.EnsureAuthzFile(); err != nil {
		log.Printf("[warn] 初始化 SVN 配置文件失败: %v", err)
	}
	if err := service.EnsureSvnserveConfDefault(); err != nil {
		log.Printf("[warn] 写入 svnserve.conf 失败: %v", err)
	}
	if err := service.EnsureTemplates(); err != nil {
		log.Printf("[warn] 生成仓库结构模板失败: %v", err)
	}

	// 初始化系统配置项
	service.InitOptions()

	// 首次启动时同步一次仓库列表
	if err := service.SyncReps(); err != nil {
		log.Printf("[warn] 同步仓库列表失败: %v", err)
	}
	// 启动后台任务队列（串行执行，避免 svnadmin 并发冲突）
	service.StartTaskWorker(1)
	// 启动定时任务调度器
	if err := service.CrondStart(); err != nil {
		log.Printf("[warn] 启动定时任务失败: %v", err)
	} else {
		defer service.CrondStop()
	}
	// 清理过期的上传会话
	go func() {
		t := time.NewTicker(1 * time.Hour)
		defer t.Stop()
		for range t.C {
			service.CleanUploadSessions()
		}
	}()

	engine := router.Engine(*webDir)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // 备份下载可能耗时较长，不限制写超时
	}

	go func() {
		log.Printf("%s 启动成功，监听 %s", version.Short(), addr)
		log.Printf("数据目录: %s", cfg.SVN.HomePath)
		log.Printf("Subversion: %s", service.VersionText())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在停止服务 ...")
	service.CrondStop()
	_ = srv.Close()
	log.Println("服务已停止")
}
