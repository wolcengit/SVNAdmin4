// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package model

import (
	"time"
)

// ---------------------------------------------------------------------------
// 管理员
// ---------------------------------------------------------------------------

// AdminUser 系统管理员
type AdminUser struct {
	AdminUserID       int64  `gorm:"primaryKey;autoIncrement;column:admin_user_id" json:"adminUserId"`
	AdminUserName     string `gorm:"size:45;not null;column:admin_user_name" json:"adminUserName"`
	AdminUserPassword string `gorm:"size:128;not null;column:admin_user_password" json:"-"`
	AdminUserPhone    string `gorm:"size:11;column:admin_user_phone" json:"adminUserPhone"`
	AdminUserEmail    string `gorm:"size:128;column:admin_user_email" json:"adminUserEmail"`
	AdminUserToken    string `gorm:"size:255;column:admin_user_token" json:"-"`
}

func (AdminUser) TableName() string { return "admin_users" }

// Subadmin 子管理员
type Subadmin struct {
	SubadminID         int64  `gorm:"primaryKey;column:subadmin_id" json:"subadminId"`
	SubadminName       string `gorm:"size:45;not null;column:subadmin_name" json:"subadminName"`
	SubadminPassword   string `gorm:"size:128;not null;column:subadmin_password" json:"-"`
	SubadminPhone      string `gorm:"size:11;column:subadmin_phone" json:"subadminPhone"`
	SubadminEmail      string `gorm:"size:128;column:subadmin_email" json:"subadminEmail"`
	SubadminStatus     int    `gorm:"not null;column:subadmin_status" json:"subadminStatus"`
	SubadminNote       string `gorm:"size:1000;column:subadmin_note" json:"subadminNote"`
	SubadminLastLogin  string `gorm:"size:45;column:subadmin_last_login" json:"subadminLastLogin"`
	SubadminCreateTime string `gorm:"size:45;not null;column:subadmin_create_time" json:"subadminCreateTime"`
	SubadminTree       string `gorm:"type:text;column:subadmin_tree" json:"subadminTree"`
	SubadminFunctions  string `gorm:"type:text;column:subadmin_functions" json:"subadminFunctions"`
	SubadminToken      string `gorm:"size:255;column:subadmin_token" json:"-"`
	// Online 是否在线（内存态最近活跃判定，不落库；对齐原版 online 语义）
	Online bool `gorm:"-" json:"online"`
}

func (Subadmin) TableName() string { return "subadmin" }

// ---------------------------------------------------------------------------
// SVN 对象
// ---------------------------------------------------------------------------

// SvnUser SVN 用户
type SvnUser struct {
	SvnUserID        int64  `gorm:"primaryKey;autoIncrement;column:svn_user_id" json:"svnUserId"`
	SvnUserName      string `gorm:"size:200;not null;uniqueIndex;column:svn_user_name" json:"svnUserName"`
	SvnUserPass      string `gorm:"size:200;not null;column:svn_user_pass" json:"-"`
	SvnUserStatus    int    `gorm:"not null;default:1;column:svn_user_status" json:"svnUserStatus"`
	SvnUserNote      string `gorm:"size:1000;column:svn_user_note" json:"svnUserNote"`
	SvnUserLastLogin string `gorm:"size:45;column:svn_user_last_login" json:"svnUserLastLogin"`
	SvnUserToken     string `gorm:"size:255;column:svn_user_token" json:"-"`
	SvnUserMail      string `gorm:"size:128;column:svn_user_mail" json:"svnUserMail"`
	// Online 在线态（登录 token 未过期即在线；svn_user_token 存到期 Unix 秒）
	Online bool `gorm:"-" json:"online"`
}

func (SvnUser) TableName() string { return "svn_users" }

// SvnGroup SVN 分组
type SvnGroup struct {
	SvnGroupID         int64  `gorm:"primaryKey;autoIncrement;column:svn_group_id" json:"svnGroupId"`
	SvnGroupName       string `gorm:"size:200;not null;uniqueIndex;column:svn_group_name" json:"svnGroupName"`
	IncludeUserCount   int    `gorm:"not null;default:0;column:include_user_count" json:"includeUserCount"`
	IncludeGroupCount  int    `gorm:"not null;default:0;column:include_group_count" json:"includeGroupCount"`
	IncludeAliaseCount int    `gorm:"not null;default:0;column:include_aliase_count" json:"includeAliaseCount"`
	SvnGroupNote       string `gorm:"size:1000;column:svn_group_note" json:"svnGroupNote"`
}

func (SvnGroup) TableName() string { return "svn_groups" }

// SvnRep SVN 仓库
type SvnRep struct {
	RepID   int64  `gorm:"primaryKey;autoIncrement;column:rep_id" json:"repId"`
	RepName string `gorm:"size:1000;not null;uniqueIndex;column:rep_name" json:"repName"`
	RepSize int64  `gorm:"column:rep_size" json:"repSize"`
	RepRev  int64  `gorm:"column:rep_rev" json:"repRev"`
	RepUUID string `gorm:"size:100;column:rep_uuid" json:"repUuid"`
	RepNote string `gorm:"size:1000;column:rep_note" json:"repNote"`
}

func (SvnRep) TableName() string { return "svn_reps" }

// SvnSecondPri 二次授权记录
// 业务含义：SVN 用户可把自己某条路径权限再下放给白名单内的 user/group/aliase
// 记录分两类：
//   - 开关记录：objectType 与 objectName 均为空，SecondPri=1 表示开启
//   - 白名单记录：objectType/objectName 指定可被下放的对象
type SvnSecondPri struct {
	SvnSecondPriID   int64  `gorm:"primaryKey;autoIncrement;column:svn_second_pri_id" json:"svnSecondPriId"`
	SvnUserPriPathID int64  `gorm:"not null;default:0;column:svnn_user_pri_path_id" json:"svnUserPriPathId"`
	SvnObjectType    string `gorm:"size:20;column:svn_object_type" json:"svnObjectType"`
	SvnObjectName    string `gorm:"size:200;column:svn_object_name" json:"svnObjectName"`
	SecondPriOwner   string `gorm:"size:200;not null;column:second_pri_owner" json:"secondPriOwner"`
	SecondPriPath    string `gorm:"size:1000;not null;column:second_pri_path" json:"secondPriPath"`
	SecondPriRepName string `gorm:"size:1000;not null;column:second_pri_rep_name" json:"secondPriRepName"`
	SecondPri        string `gorm:"size:10;not null;default:0;column:second_pri" json:"secondPri"`
	SecondPriUnique  string `gorm:"size:64;not null;index;column:second_pri_unique" json:"secondPriUnique"`
	SecondPriAddTime string `gorm:"size:45;column:second_pri_add_time" json:"secondPriAddTime"`
}

func (SvnSecondPri) TableName() string { return "svn_second_pri" }

// ---------------------------------------------------------------------------
// 系统
// ---------------------------------------------------------------------------

// Log 操作日志
type Log struct {
	LogID          int64  `gorm:"primaryKey;autoIncrement;column:log_id" json:"logId"`
	LogTypeName    string `gorm:"size:200;not null;column:log_type_name" json:"logTypeName"`
	LogContent     string `gorm:"size:5000;column:log_content" json:"logContent"`
	LogAddUserName string `gorm:"size:200;not null;column:log_add_user_name" json:"logAddUserName"`
	LogAddTime     string `gorm:"size:45;not null;column:log_add_time" json:"logAddTime"`
}

func (Log) TableName() string { return "logs" }

// BlackToken 已注销的 token 黑名单（落库持久化，服务重启后仍生效，对齐原版 black_token 表）
type BlackToken struct {
	BlackTokenID     int64  `gorm:"primaryKey;autoIncrement;column:black_token_id" json:"blackTokenId"`
	BlackTokenToken  string `gorm:"size:1000;not null;uniqueIndex;column:black_token_token" json:"-"`
	BlackTokenExpire string `gorm:"size:45;not null;column:black_token_expire" json:"blackTokenExpire"`
}

func (BlackToken) TableName() string { return "black_token" }

// Option 系统配置项
type Option struct {
	OptionID          int64  `gorm:"primaryKey;autoIncrement;column:option_id" json:"optionId"`
	OptionName        string `gorm:"size:200;not null;uniqueIndex;column:option_name" json:"optionName"`
	OptionValue       string `gorm:"type:text;column:option_value" json:"optionValue"`
	OptionDescription string `gorm:"size:500;column:option_description" json:"optionDescription"`
}

func (Option) TableName() string { return "options" }

// Task 后台任务
type Task struct {
	TaskID         int64  `gorm:"primaryKey;autoIncrement;column:task_id" json:"taskId"`
	TaskName       string `gorm:"size:200;not null;column:task_name" json:"taskName"`
	TaskStatus     int    `gorm:"not null;column:task_status" json:"taskStatus"` // 0 等待 1 运行中 2 完成 3 失败 4 取消
	TaskCmd        string `gorm:"type:text;column:task_cmd" json:"taskCmd"`
	TaskType       string `gorm:"size:45;not null;column:task_type" json:"taskType"`
	TaskUnique     string `gorm:"size:64;not null;column:task_unique" json:"taskUnique"`
	TaskLogFile    string `gorm:"size:500;column:task_log_file" json:"taskLogFile"`
	TaskOptional   string `gorm:"type:text;column:task_optional" json:"taskOptional"`
	TaskCreateTime string `gorm:"size:45;not null;column:task_create_time" json:"taskCreateTime"`
	TaskUpdateTime string `gorm:"size:45;column:task_update_time" json:"taskUpdateTime"`
	TaskMessage    string `gorm:"type:text;column:task_message" json:"taskMessage"`
}

func (Task) TableName() string { return "tasks" }

// Crond 定时任务
type Crond struct {
	CrondID      int64  `gorm:"primaryKey;autoIncrement;column:crond_id" json:"crondId"`
	Sign         string `gorm:"size:64;not null;column:sign" json:"sign"`
	TaskType     int    `gorm:"not null;column:task_type" json:"taskType"`
	TaskName     string `gorm:"size:200;not null;column:task_name" json:"taskName"`
	CycleType    string `gorm:"size:20;not null;column:cycle_type" json:"cycleType"`
	CycleDesc    string `gorm:"size:200;not null;column:cycle_desc" json:"cycleDesc"`
	Status       int    `gorm:"not null;column:status" json:"status"`
	SaveCount    int    `gorm:"not null;column:save_count" json:"saveCount"`
	RepName      string `gorm:"size:200;column:rep_name" json:"repName"`
	Week         int    `gorm:"column:week" json:"week"`
	Day          int    `gorm:"column:day" json:"day"`
	Hour         int    `gorm:"column:hour" json:"hour"`
	Minute       int    `gorm:"column:minute" json:"minute"`
	Notice       int    `gorm:"not null;column:notice" json:"notice"`
	Code         string `gorm:"type:text;not null;column:code" json:"code"`
	Shell        string `gorm:"type:text;column:shell" json:"shell"`
	LastExecTime string `gorm:"size:45;not null;column:last_exec_time" json:"lastExecTime"`
	CreateTime   string `gorm:"size:45;not null;column:create_time" json:"createTime"`
}

func (Crond) TableName() string { return "crond" }

// Now 统一时间格式
func Now() string { return time.Now().Format("2006-01-02 15:04:05") }
