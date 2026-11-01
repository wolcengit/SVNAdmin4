// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package store

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"svnadmin/internal/config"
	"svnadmin/internal/model"
	"svnadmin/internal/util"

	sqlite "github.com/glebarez/sqlite"
)

var (
	db   *gorm.DB
	once sync.Once
)

// DB 返回全局数据库句柄
func DB() *gorm.DB { return db }

// Init 初始化数据库（幂等）
func Init() error {
	var err error
	once.Do(func() { err = initDB() })
	return err
}

func initDB() error {
	cfg := config.C()
	if err := cfg.SVN.EnsureDirs(); err != nil {
		return fmt.Errorf("初始化数据目录失败: %w", err)
	}

	var dialector gorm.Dialector
	switch strings.ToLower(cfg.Database.Driver) {
	case "mysql":
		m := cfg.Database.MySQL
		if m.Port == 0 {
			m.Port = 3306
		}
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			m.User, m.Password, m.Host, m.Port, m.Name)
		dialector = mysql.Open(dsn)
	default:
		p := cfg.Database.SQLitePath
		if dir := filepath.Dir(p); dir != "" {
			_ = os.MkdirAll(dir, 0755)
		}
		dialector = sqlite.Open(p)
	}

	gormLog := logger.Default.LogMode(logger.Silent)
	if strings.EqualFold(cfg.Server.Mode, "debug") {
		gormLog = logger.Default.LogMode(logger.Info)
	}

	conn, err := gorm.Open(dialector, &gorm.Config{Logger: gormLog})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	if err := conn.AutoMigrate(
		&model.AdminUser{},
		&model.Subadmin{},
		&model.SvnUser{},
		&model.SvnGroup{},
		&model.SvnRep{},
		&model.SvnSecondPri{},
		&model.Log{},
		&model.Option{},
		&model.Task{},
		&model.Crond{},
		&model.BlackToken{},
	); err != nil {
		return fmt.Errorf("自动迁移表结构失败: %w", err)
	}
	// 清理已过期的黑名单 token
	conn.Where("black_token_expire < ?", time.Now().Format("2006-01-02 15:04:05")).Delete(&model.BlackToken{})

	db = conn
	seed()
	log.Println("[store] 数据库初始化完成")
	return nil
}

// seed 写入默认数据
func seed() {
	// 默认管理员 admin / admin
	var count int64
	db.Model(&model.AdminUser{}).Count(&count)
	if count == 0 {
		db.Create(&model.AdminUser{
			AdminUserName:     "admin",
			AdminUserPassword: util.Hash("admin"),
		})
	}

	defaultOptions := map[string]string{
		"svnserve_status":    "0",
		"http_status":        "0",
		"ldap_status":        "0",
		"enable_verify_code": boolStr(config.C().Security.EnableVerifyCode),
		"rep_init_default":   "01",
		"system_version":     "4.0.0",
	}
	for k, v := range defaultOptions {
		var o model.Option
		if err := db.Where("option_name = ?", k).First(&o).Error; err != nil {
			db.Create(&model.Option{OptionName: k, OptionValue: v})
		}
	}
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
