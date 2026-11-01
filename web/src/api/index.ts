/*!
 * Copyright (C) 2026 wolcen@msn.com
 * SPDX-License-Identifier: GPL-3.0-or-later
 *
 * SVNAdmin4 is free software: you can redistribute it and/or modify it under
 * the terms of the GNU General Public License as published by the Free Software
 * Foundation, either version 3 of the License, or (at your option) any later
 * version. See the LICENSE file in the project root for the full license text.
 */

import { http, type PageData } from './request'

/* ------------------------------------------------------------------ */
/* 登录 / 公共                                                          */
/* ------------------------------------------------------------------ */

export interface LoginPayload {
  userName: string
  password: string
  uuid: string
  code: string
  role?: string
}

export interface LoginResult {
  token: string
  role: string
  userName: string
  userId: number
  expire: number
  functions?: string[]
}

export const apiLogin = (data: LoginPayload) =>
  http<LoginResult>({ url: '/common/login', method: 'post', data })

export const apiLogout = () => http({ url: '/common/logout', method: 'post' })

export const apiVerifyCode = () =>
  http<{ uuid: string; svg: string }>({ url: '/common/verifyCode' })

export const apiVerifyOption = () =>
  http<{
    enableVerifyCode: boolean
    enableHTTP: boolean
    svnVersion: string
    svnserveStatus: boolean
    version: string
  }>({ url: '/common/verifyOption' })

export const apiUserInfo = () =>
  http<{ userId: number; userName: string; role: string; functions?: string[] }>({ url: '/common/userInfo' })

export const apiStatistics = () => http<any>({ url: '/common/statistics' })

export const apiUpdatePass = (data: { oldPass: string; newPass: string }) =>
  http({ url: '/common/updatePass', method: 'post', data })

/* ------------------------------------------------------------------ */
/* 仓库                                                                */
/* ------------------------------------------------------------------ */

export interface SvnRep {
  repId: number
  repName: string
  repSize: number
  repRev: number
  repUuid: string
  repNote: string
}

export const apiRepList = (params: any) =>
  http<PageData<SvnRep>>({ url: '/rep/list', params })

export const apiRepAll = () => http<SvnRep[]>({ url: '/rep/all' })

export const apiRepCreate = (data: { repName: string; repNote?: string; initStruct?: string }) =>
  http({ url: '/rep/create', method: 'post', data })

export const apiRepDel = (data: { repName: string }) =>
  http({ url: '/rep/del', method: 'post', data })

export const apiRepUpdName = (data: { oldName: string; newName: string }) =>
  http({ url: '/rep/updName', method: 'post', data })

export const apiRepUpdNote = (data: { repName: string; repNote: string }) =>
  http({ url: '/rep/updNote', method: 'post', data })

export const apiRepDetail = (repName: string) =>
  http<any>({ url: '/rep/detail', params: { repName } })

export const apiRepSetUUID = (data: { repName: string }) =>
  http<{ uuid: string }>({ url: '/rep/setUUID', method: 'post', data })

export const apiRepSyncSize = (repName: string) =>
  http<{ size: number }>({ url: '/rep/syncSize', method: 'post', data: { repName } })

export const apiRepSyncRev = (repName: string) =>
  http<{ rev: number }>({ url: '/rep/syncRev', method: 'post', data: { repName } })

export interface TreeNode {
  name: string
  path: string
  type: string
  size: number
  rev: number
  author: string
  date: string
  children?: TreeNode[]
}

export const apiRepTree = (repName: string, path: string) =>
  http<TreeNode[]>({ url: '/rep/tree', params: { repName, path } })

export const apiRepCon = (repName: string, path: string) =>
  http<TreeNode[]>({ url: '/rep/con', params: { repName, path } })

export const apiRepCreateFolder = (data: { repName: string; path: string; message?: string }) =>
  http({ url: '/rep/createFolder', method: 'post', data })

export const apiRepCheckout = (repName: string) =>
  http<{ svn: string; http: string }>({ url: '/rep/checkout', params: { repName } })

export const apiCheckAuthz = () => http({ url: '/rep/checkAuthz' })

export const apiUserRepList = () => http<any[]>({ url: '/rep/userList' })

export const apiUserRepList2 = (svnUserName: string) =>
  http<any[]>({ url: '/rep/userList2', params: { svnUserName } })

/* ------------------------------------------------------------------ */
/* 用户                                                                */
/* ------------------------------------------------------------------ */

export interface SvnUser {
  svnUserId: number
  svnUserName: string
  svnUserStatus: number
  svnUserNote: string
  svnUserLastLogin: string
  svnUserMail: string
}

export const apiUserList = (params: any) =>
  http<PageData<SvnUser>>({ url: '/user/list', params })

export const apiUserAll = () => http<SvnUser[]>({ url: '/user/all' })

export const apiUserCreate = (data: any) => http({ url: '/user/create', method: 'post', data })
export const apiUserDel = (data: { svnUserName: string }) =>
  http({ url: '/user/del', method: 'post', data })
export const apiUserUpdPass = (data: { svnUserName: string; svnUserPass: string }) =>
  http({ url: '/user/updPass', method: 'post', data })
export const apiUserUpdStatus = (data: { svnUserName: string; svnUserStatus: number }) =>
  http({ url: '/user/updStatus', method: 'post', data })
export const apiUserUpdNote = (data: { svnUserName: string; svnUserNote: string }) =>
  http({ url: '/user/updNote', method: 'post', data })
export const apiUserScan = () => http<string[]>({ url: '/user/scan' })
export const apiUserImport = (data: { names: string[] }) =>
  http<{ count: number }>({ url: '/user/import', method: 'post', data })

/* ------------------------------------------------------------------ */
/* 分组                                                                */
/* ------------------------------------------------------------------ */

export interface SvnGroup {
  svnGroupId: number
  svnGroupName: string
  includeUserCount: number
  includeGroupCount: number
  includeAliaseCount: number
  svnGroupNote: string
}

export const apiGroupList = (params: any) =>
  http<PageData<SvnGroup>>({ url: '/group/list', params })
export const apiGroupAll = () => http<string[]>({ url: '/group/all' })
export const apiGroupCreate = (data: any) => http({ url: '/group/create', method: 'post', data })
export const apiGroupDel = (data: { svnGroupName: string }) =>
  http({ url: '/group/del', method: 'post', data })
export const apiGroupUpdName = (data: { oldName: string; newName: string }) =>
  http({ url: '/group/updName', method: 'post', data })
export const apiGroupUpdNote = (data: { svnGroupName: string; svnGroupNote: string }) =>
  http({ url: '/group/updNote', method: 'post', data })
export const apiGroupMember = (svnGroupName: string) =>
  http<{ users: string[]; groups: string[]; aliases: string[] }>({
    url: '/group/member',
    params: { svnGroupName }
  })
export const apiGroupUpdMember = (data: any) =>
  http({ url: '/group/updMember', method: 'post', data })

/* ------------------------------------------------------------------ */
/* 权限                                                                */
/* ------------------------------------------------------------------ */

export interface PriItem {
  repName: string
  priPath: string
  objectType: string
  objectName: string
  perm: string
  unique: string
  inheritPath: string
}

export const apiPriList = (repName: string, priPath: string) =>
  http<{ current: PriItem[]; inherit: PriItem[] }>({
    url: '/pri/list',
    params: { repName, priPath }
  })

export const apiPriCreate = (data: any) => http({ url: '/pri/create', method: 'post', data })
export const apiPriUpdate = (data: any) => http({ url: '/pri/update', method: 'post', data })
export const apiPriDelete = (data: any) => http({ url: '/pri/delete', method: 'post', data })

/* ------------------------------------------------------------------ */
/* 备份 / 钩子                                                          */
/* ------------------------------------------------------------------ */

export interface BackupFile {
  name: string
  size: number
  sizeHr: string
  modTime: string
  repName: string
}

export const apiBackupList = (repName?: string) =>
  http<BackupFile[]>({ url: '/backup/list', params: { repName } })
export const apiBackupDump = (data: { repName: string; revRange?: string; incremental?: boolean }) =>
  http<any>({ url: '/backup/dump', method: 'post', data })
export const apiBackupDel = (data: { name: string }) =>
  http({ url: '/backup/del', method: 'post', data })
export const apiBackupLoad = (data: { repName: string; fileName: string }) =>
  http<any>({ url: '/backup/load', method: 'post', data })

export const apiHookList = (repName: string) => http<any[]>({ url: '/hook/list', params: { repName } })
export const apiHookUpdate = (data: any) => http({ url: '/hook/update', method: 'post', data })
export const apiHookDel = (data: any) => http({ url: '/hook/del', method: 'post', data })
export const apiHookRecommend = () => http<any[]>({ url: '/hook/recommend' })

/* ------------------------------------------------------------------ */
/* 系统                                                                */
/* ------------------------------------------------------------------ */

export const apiLogList = (params: any) => http<PageData<any>>({ url: '/log/list', params })
export const apiLogTypes = () => http<string[]>({ url: '/log/types' })
export const apiLogClear = () => http({ url: '/log/clear', method: 'post' })

export const apiOptionList = () => http<any[]>({ url: '/setting/list' })
export const apiOptionUpdate = (data: { optionName: string; optionValue: string }) =>
  http({ url: '/setting/update', method: 'post', data })
export const apiSvnserveStart = () => http({ url: '/setting/svnserveStart', method: 'post' })
export const apiSvnserveStop = () => http({ url: '/setting/svnserveStop', method: 'post' })

export const apiTaskList = (params: any) => http<PageData<any>>({ url: '/task/list', params })
export const apiTaskDetail = (taskUnique: string) =>
  http<any>({ url: '/task/detail', params: { taskUnique } })

export const apiCrondList = () => http<any[]>({ url: '/crond/list' })
export const apiCrondCreate = (data: any) => http({ url: '/crond/create', method: 'post', data })
export const apiCrondUpdate = (data: any) => http({ url: '/crond/update', method: 'post', data })
export const apiCrondDel = (crondId: number) =>
  http({ url: '/crond/del', method: 'post', params: { crondId } })

export const apiSubadminList = (params?: {
  page?: number
  pageSize?: number
  keyword?: string
  sortName?: string
  sortType?: string
}) => http<any[]>({ url: '/subadmin/list', params })
export const apiSubadminCreate = (data: any) =>
  http({ url: '/subadmin/create', method: 'post', data })
export const apiSubadminDel = (subadminId: number) =>
  http({ url: '/subadmin/del', method: 'post', params: { subadminId } })
export const apiSubadminUpdStatus = (subadminId: number, status: number) =>
  http({ url: '/subadmin/updStatus', method: 'post', params: { subadminId, status } })
export const apiSubadminUpdPass = (data: { subadminId: number; subadminPass: string }) =>
  http({ url: '/subadmin/updPass', method: 'post', data })

export const apiAliaseList = () => http<Record<string, string>>({ url: '/aliase/list' })

/* ------------------------------------------------------------------ */
/* 系统监控                                                            */
/* ------------------------------------------------------------------ */

export interface MonitorInfo {
  load: { load1Min: number; load5Min: number; load15Min: number; percent: number; color: string; title: string }
  cpu: { percent: number; color: string; cpu: string[]; cpuPhysical: number; cpuCore: number; cpuProcessor: number }
  mem: { memTotal: number; memUsed: number; memFree: number; percent: number; color: string }
  disk: { fileSystem: string; mountedOn: string; size: number; used: number; avail: number; percent: number; color: string }[]
  host: { os: string; platform: string; arch: string; hostname: string; goVersion: string; uptime: number }
}

export const apiMonitor = () => http<MonitorInfo>({ url: '/common/monitor' })
export const apiLoadInfo = () => http<MonitorInfo['load']>({ url: '/common/loadInfo' })
export const apiDiskInfo = () => http<MonitorInfo['disk']>({ url: '/common/diskInfo' })
export const apiVersionInfo = () => http<any>({ url: '/common/version' })

/* ------------------------------------------------------------------ */
/* 任务控制                                                            */
/* ------------------------------------------------------------------ */

export const apiTaskRun = () => http<any>({ url: '/task/run' })
export const apiTaskLog = (taskId: number) => http<{ log: string; trimed: boolean }>({ url: '/task/log', params: { taskId } })
export const apiTaskStop = (taskId: number) => http({ url: '/task/stop', method: 'post', data: { taskId } })
export const apiTaskDel = (taskId: number) => http({ url: '/task/del', method: 'post', data: { taskId } })
export const apiTaskClear = () => http({ url: '/task/clear', method: 'post' })
export const apiTaskTypes = () => http<{ types: any[]; status: any[] }>({ url: '/task/types' })

/* ------------------------------------------------------------------ */
/* 定时任务                                                            */
/* ------------------------------------------------------------------ */

export const apiCrondDict = () => http<any>({ url: '/crond/dict' })
export const apiCrondTrigger = (crondId: number) => http<any>({ url: '/crond/trigger', method: 'post', data: { crondId } })
export const apiCrondLog = (crondId: number) => http<{ log: string }>({ url: '/crond/log', params: { crondId } })

/* ------------------------------------------------------------------ */
/* 二次授权                                                            */
/* ------------------------------------------------------------------ */

export const apiSecondPriUpdate = (data: any) => http({ url: '/secondpri/update', method: 'post', data })
export const apiSecondPriObjectList = (params: any) => http<any[]>({ url: '/secondpri/objectList', params })
export const apiSecondPriObjectCreate = (data: any) => http({ url: '/secondpri/objectCreate', method: 'post', data })
export const apiSecondPriObjectDel = (secondPriId: number) => http({ url: '/secondpri/objectDel', method: 'post', data: { secondPriId } })
export const apiSecondPriUserPaths = (svnUserName: string) => http<any[]>({ url: '/secondpri/userPaths', params: { svnUserName } })

/* ------------------------------------------------------------------ */
/* 邮件 / 推送 / 安全                                                   */
/* ------------------------------------------------------------------ */

export const apiMailInfo = () => http<any>({ url: '/mail/info' })
export const apiMailUpdate = (data: any) => http({ url: '/mail/update', method: 'post', data })
export const apiMailTest = (to?: string) => http({ url: '/mail/test', method: 'post', data: { to } })
export const apiPushInfo = () => http<any[]>({ url: '/push/info' })
export const apiPushUpdate = (data: any[]) => http({ url: '/push/update', method: 'post', data })
export const apiSafeInfo = () => http<any[]>({ url: '/safe/info' })
export const apiSafeUpdate = (data: any[]) => http({ url: '/safe/update', method: 'post', data })

/* ------------------------------------------------------------------ */
/* 设置扩展                                                            */
/* ------------------------------------------------------------------ */

export const apiDirInfo = () => http<any>({ url: '/setting/dirInfo' })
export const apiSvnInfo = () => http<any>({ url: '/setting/svnInfo' })
export const apiApacheInfo = () => http<any>({ url: '/setting/apacheInfo' })
export const apiSubversionEnable = (prefix: string) => http({ url: '/setting/subversionEnable', method: 'post', data: { prefix } })
export const apiSvnEnable = () => http({ url: '/setting/svnEnable', method: 'post' })
export const apiHttpPrefixUpdate = (prefix: string) => http({ url: '/setting/httpPrefix', method: 'post', data: { prefix } })
export const apiHttpPortUpdate = (port: number) => http({ url: '/setting/httpPort', method: 'post', data: { port } })
export const apiSvnservePortUpdate = (port: number) => http({ url: '/setting/svnservePort', method: 'post', data: { port } })
export const apiSvnserveHostUpdate = (host: string) => http({ url: '/setting/svnserveHost', method: 'post', data: { host } })
export const apiUsersourceUpdate = (protocol: string, source: any) => http({ url: '/setting/usersource', method: 'post', data: { protocol, source } })
export const apiDockerHostInfo = () => http<any>({ url: '/setting/dockerHost' })
export const apiDockerHostUpdate = (data: any) => http({ url: '/setting/dockerHost', method: 'post', data })
export const apiSaslInfo = () => http<any>({ url: '/setting/saslInfo' })
export const apiSaslStart = () => http({ url: '/setting/saslStart', method: 'post' })
export const apiSaslStop = () => http({ url: '/setting/saslStop', method: 'post' })
export const apiCheckUpdate = () => http<any>({ url: '/setting/checkUpdate' })

/* ------------------------------------------------------------------ */
/* LDAP                                                               */
/* ------------------------------------------------------------------ */

export const apiLdapTest = (data: any) => http<any>({ url: '/ldap/test', method: 'post', data })
export const apiLdapUsers = (config: any) => http<string[]>({ url: '/ldap/users', method: 'post', data: config })
export const apiLdapGroups = (config: any) => http<any[]>({ url: '/ldap/groups', method: 'post', data: config })
export const apiLdapSync = (kind: string) => http<any>({ url: '/ldap/sync', method: 'post', data: { kind } })

/* ------------------------------------------------------------------ */
/* 数据同步                                                            */
/* ------------------------------------------------------------------ */

export const apiSyncAll = () => http({ url: '/sync/all', method: 'post' })
export const apiSyncUsers = () => http({ url: '/user/sync', method: 'post' })
export const apiSyncGroups = () => http({ url: '/group/sync', method: 'post' })
export const apiSyncReps = () => http({ url: '/rep/syncAll', method: 'post' })

/* ------------------------------------------------------------------ */
/* 仓库扩展                                                            */
/* ------------------------------------------------------------------ */

export const apiRepVerify = (repName: string) => http<any>({ url: '/rep/verify', method: 'post', data: { repName } })
export const apiRepHotcopy = (repName: string, incremental: boolean) =>
  http<any>({ url: '/rep/hotcopy', method: 'post', data: { repName, incremental } })
export const apiRepTree2 = (repName: string, path: string, svnUserName: string, first?: boolean) =>
  http<TreeNode[]>({ url: '/rep/tree2', params: { repName, path, svnUserName, first: first ? 1 : 0 } })
export const apiRepUserCon = (repName: string, path: string, svnUserName: string) =>
  http<any>({ url: '/rep/userCon', params: { repName, path, svnUserName } })

/* ------------------------------------------------------------------ */
/* 分片上传                                                            */
/* ------------------------------------------------------------------ */

export const apiUploadInfo = () => http<any>({ url: '/backup/uploadInfo' })
export const apiUploadInit = (data: { fileName: string; md5: string; numBlobTotal: number }) =>
  http<any>({ url: '/backup/uploadInit', method: 'post', data })
export const apiUploadMerge = (uploadId: string, deleteOnMerge = true) =>
  http<{ name: string }>({ url: '/backup/uploadMerge', method: 'post', data: { uploadId, deleteOnMerge } })
export const apiUploadAbort = (uploadId: string) =>
  http({ url: '/backup/uploadAbort', method: 'post', params: { uploadId } })
export const apiUploadStatus = (uploadId: string) => http<any>({ url: '/backup/uploadStatus', params: { uploadId } })
export const UPLOAD_PART_URL = '/api/backup/uploadPart'

/* ------------------------------------------------------------------ */
/* 子管理员权限树                                                       */
/* ------------------------------------------------------------------ */

export const apiSubadminTree = (subadminId: number) => http<any>({ url: '/subadmin/tree', params: { subadminId } })
export const apiSubadminUpdTree = (subadminId: number, tree: any[]) =>
  http({ url: '/subadmin/updTree', method: 'post', data: { subadminId, tree } })
export const apiSubadminUpdNote = (subadminId: number, subadminNote: string) =>
  http({ url: '/subadmin/updNote', method: 'post', data: { subadminId, subadminNote } })

/* ------------------------------------------------------------------ */
/* 个人信息                                                            */
/* ------------------------------------------------------------------ */

export const apiEditAdminUserName = (data: { oldName: string; newName: string; pass: string }) =>
  http({ url: '/common/editAdminUserName', method: 'post', data })

export const apiUserUpdMail = (data: { svnUserName: string; svnUserMail: string }) =>
  http({ url: '/user/updMail', method: 'post', data })
