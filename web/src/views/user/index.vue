<!--
  Copyright (C) 2026 wolcen@msn.com
  SPDX-License-Identifier: GPL-3.0-or-later

  SVNAdmin4 is free software: you can redistribute it and/or modify it under
  the terms of the GNU General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option) any later
  version. See the LICENSE file in the project root for the full license text.
-->

<template>
  <div class="page-container">
    <div class="page-header">
      <h2>SVN 用户</h2>
      <div class="toolbar" style="margin: 0">
        <el-button type="primary" :icon="Plus" @click="openCreate">新建用户</el-button>
        <el-button :icon="Refresh" @click="load">刷新</el-button>
        <el-button :icon="Search" @click="openScan">扫描导入</el-button>
        <el-button :icon="Upload" @click="openMigrate">用户迁入</el-button>
        <el-button :icon="RefreshRight" @click="syncUsers">同步</el-button>
      </div>
    </div>

    <div class="toolbar">
      <el-input
        v-model="query.keyword"
        placeholder="用户名 / 备注"
        clearable
        style="width: 220px"
        @keyup.enter="load"
        @clear="load"
      />
      <el-select v-model="query.status" placeholder="状态" clearable style="width: 130px" @change="load">
        <el-option label="启用" :value="1" />
        <el-option label="禁用" :value="0" />
      </el-select>
      <el-button type="primary" :icon="Search" @click="load">查询</el-button>
    </div>

    <el-table v-loading="loading" :data="list" border size="small">
      <el-table-column prop="svnUserId" label="ID" width="70" />
      <el-table-column prop="svnUserName" label="用户名" min-width="140" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-switch :model-value="row.svnUserStatus === 1" @change="(v: any) => toggleStatus(row, v)" />
        </template>
      </el-table-column>
      <el-table-column prop="svnUserNote" label="备注" min-width="160" show-overflow-tooltip />
      <el-table-column label="邮箱" min-width="180" show-overflow-tooltip>
        <template #default="{ row }">
          <span>{{ row.svnUserMail || '-' }}</span>
          <el-button link type="primary" size="small" style="margin-left: 6px" @click="openEditMail(row)">
            编辑
          </el-button>
        </template>
      </el-table-column>
      <el-table-column label="二次授权" width="100">
        <template #default="{ row }">
          <el-button link type="warning" size="small" @click="openSecondPri(row)">配置</el-button>
        </template>
      </el-table-column>
      <el-table-column prop="svnUserLastLogin" label="最后登录" width="160" />
      <el-table-column label="操作" width="220" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openEditNote(row)">备注</el-button>
          <el-button link type="primary" @click="openEditPass(row)">密码</el-button>
          <el-button link type="primary" @click="openPerm(row)">权限</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-pagination
      style="margin-top: 12px; justify-content: flex-end"
      layout="total, sizes, prev, pager, next"
      :total="total"
      v-model:current-page="query.page"
      v-model:page-size="query.pageSize"
      :page-sizes="[10, 20, 50, 100]"
      @change="load"
    />

    <!-- 新建 -->
    <el-dialog v-model="createVisible" title="新建 SVN 用户" width="440px">
      <el-form :model="createForm" label-width="80px">
        <el-form-item label="用户名" required>
          <el-input v-model="createForm.svnUserName" placeholder="不能包含 / \ : * @ & = 等字符" />
        </el-form-item>
        <el-form-item label="密码" required>
          <el-input v-model="createForm.svnUserPass" show-password />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="createForm.svnUserMail" placeholder="用于消息推送" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="createForm.svnUserNote" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" @click="submitCreate">确定</el-button>
      </template>
    </el-dialog>

    <!-- 改密 -->
    <el-dialog v-model="passVisible" title="修改密码" width="400px">
      <el-form label-width="80px">
        <el-form-item label="用户名">
          <el-input :model-value="current.svnUserName" disabled />
        </el-form-item>
        <el-form-item label="新密码" required>
          <el-input v-model="newPass" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passVisible = false">取消</el-button>
        <el-button type="primary" @click="submitPass">确定</el-button>
      </template>
    </el-dialog>

    <!-- 备注 -->
    <el-dialog v-model="noteVisible" title="修改备注" width="400px">
      <el-form label-width="80px">
        <el-form-item label="用户名">
          <el-input :model-value="current.svnUserName" disabled />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="newNote" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="noteVisible = false">取消</el-button>
        <el-button type="primary" @click="submitNote">确定</el-button>
      </template>
    </el-dialog>

    <!-- 邮箱 -->
    <el-dialog v-model="mailVisible" title="修改邮箱" width="420px">
      <el-form label-width="80px">
        <el-form-item label="用户名">
          <el-input :model-value="current.svnUserName" disabled />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="newMail" placeholder="用于消息推送与密码找回" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="mailVisible = false">取消</el-button>
        <el-button type="primary" @click="submitMail">确定</el-button>
      </template>
    </el-dialog>

    <!-- 扫描导入 -->
    <el-dialog v-model="scanVisible" title="扫描 passwd 文件" width="560px">
      <el-alert
        type="info"
        :closable="false"
        title="下列用户存在于 passwd 文件中，但尚未纳入系统管理，可勾选后导入。"
        style="margin-bottom: 12px"
      />
      <el-checkbox-group v-model="importNames">
        <div v-for="n in scanList" :key="n" style="padding: 4px 0">
          <el-checkbox :value="n" :label="n">{{ n }}</el-checkbox>
        </div>
      </el-checkbox-group>
      <el-empty v-if="!scanList.length" description="没有发现新用户" :image-size="70" />
      <template #footer>
        <el-button @click="scanVisible = false">取消</el-button>
        <el-button type="primary" :disabled="!importNames.length" @click="submitImport">
          导入 {{ importNames.length }} 个
        </el-button>
      </template>
    </el-dialog>

    <!-- 用户迁入三步向导 -->
    <el-dialog v-model="migrateVisible" title="用户迁入" width="680px">
      <el-steps :active="migrateStep" finish-status="success" style="margin-bottom: 18px">
        <el-step title="粘贴 passwd 文本" />
        <el-step title="确认待迁入用户" />
        <el-step title="迁入结果" />
      </el-steps>

      <div v-show="migrateStep === 0">
        <el-alert
          type="info"
          :closable="false"
          style="margin-bottom: 10px"
          title="将已有 passwd 文件内容粘贴到下方，系统会自动解析出用户名（支持 [users] 段落与注释行）。"
        />
        <el-input
          v-model="migrateText"
          type="textarea"
          :rows="12"
          class="text-mono"
          placeholder="[users]&#10;zhangsan = 123456&#10;lisi = abcdef"
        />
      </div>

      <div v-show="migrateStep === 1">
        <el-alert
          type="warning"
          :closable="false"
          style="margin-bottom: 10px"
          title="请确认待迁入的用户，可移除不需要迁入的条目。已存在的用户会被跳过。"
        />
        <el-table :data="migrateUsers" size="small" border max-height="320">
          <el-table-column prop="name" label="用户名" min-width="200" />
          <el-table-column prop="pass" label="密码" min-width="160">
            <template #default="{ row }">
              <span>{{ row.pass ? '••••••' : '(无)' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="90">
            <template #default="{ $index }">
              <el-button link type="danger" size="small" @click="migrateUsers.splice($index, 1)">移除</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!migrateUsers.length" description="未解析到用户" :image-size="70" />
      </div>

      <div v-show="migrateStep === 2">
        <el-result
          :icon="migrateResult.failed ? 'warning' : 'success'"
          :title="migrateResult.failed ? '部分用户迁入失败' : '迁入完成'"
        >
          <template #sub-title>
            成功 {{ migrateResult.success }} 个，跳过 {{ migrateResult.skipped }} 个，失败
            {{ migrateResult.failed }} 个
          </template>
        </el-result>
        <el-table v-if="migrateResult.details?.length" :data="migrateResult.details" size="small" border max-height="240">
          <el-table-column prop="name" label="用户名" min-width="160" />
          <el-table-column prop="result" label="结果" min-width="120" />
          <el-table-column prop="message" label="说明" min-width="200" show-overflow-tooltip />
        </el-table>
      </div>

      <template #footer>
        <el-button v-if="migrateStep > 0 && migrateStep < 2" @click="migrateStep--">上一步</el-button>
        <el-button v-if="migrateStep === 0" type="primary" :disabled="!migrateText.trim()" @click="parseMigrate">
          解析
        </el-button>
        <el-button
          v-else-if="migrateStep === 1"
          type="primary"
          :disabled="!migrateUsers.length"
          :loading="migrateRunning"
          @click="runMigrate"
        >
          开始迁入
        </el-button>
        <el-button v-else type="primary" @click="migrateVisible = false">完成</el-button>
        <el-button @click="migrateVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 权限 -->
    <el-drawer v-model="permVisible" :title="`${current.svnUserName} 的仓库权限`" size="50%">
      <el-alert type="info" :closable="false" title="权限由 authz 文件推导，包含所属分组继承的权限" />
      <el-table :data="permList" size="small" style="margin-top: 12px">
        <el-table-column prop="repName" label="仓库" min-width="140" />
        <el-table-column prop="perm" label="根路径权限" width="110">
          <template #default="{ row }">
            <el-tag :type="row.perm === 'rw' ? 'success' : 'primary'" size="small">
              {{ row.perm || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="可访问路径" min-width="200">
          <template #default="{ row }">
            <div v-for="p in row.paths" :key="p" class="text-mono">{{ p }}</div>
          </template>
        </el-table-column>
      </el-table>
    </el-drawer>

    <!-- 二次授权 -->
    <el-drawer v-model="secondVisible" :title="`二次授权 - ${current.svnUserName}`" size="60%">
      <el-alert
        type="info"
        :closable="false"
        title="开启后该 SVN 用户（以自身身份登录时）只能管理白名单内的用户 / 分组 / 别名；未开启则可管理全部对象。"
        style="margin-bottom: 12px"
      />
      <el-table v-loading="secondLoading" :data="secondRows" size="small" border>
        <el-table-column prop="repName" label="仓库" min-width="140" />
        <el-table-column prop="path" label="路径" min-width="180">
          <template #default="{ row }"><span class="text-mono">{{ row.path || '/' }}</span></template>
        </el-table-column>
        <el-table-column label="二次授权" width="110">
          <template #default="{ row }">
            <el-switch :model-value="row.enable" @change="(v: any) => toggleSecond(row, v)" />
          </template>
        </el-table-column>
        <el-table-column label="可管理对象" width="160">
          <template #default="{ row }">
            <el-button link type="primary" size="small" :disabled="!row.enable" @click="openObjects(row)">
              白名单 ({{ row.objectCount ?? 0 }})
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!secondRows.length" description="该用户暂无任何仓库路径权限" :image-size="80" />
    </el-drawer>

    <!-- 二次授权白名单 -->
    <el-dialog v-model="objVisible" :title="`可管理对象 - ${objRow.repName}${objRow.path}`" width="720px">
      <el-space style="margin-bottom: 10px">
        <el-select v-model="objForm.objectType" style="width: 120px">
          <el-option label="用户" value="user" />
          <el-option label="分组" value="group" />
          <el-option label="别名" value="aliase" />
        </el-select>
        <el-select
          v-model="objForm.objectName"
          filterable
          clearable
          :placeholder="objPlaceholder"
          style="width: 260px"
          :loading="objCandidatesLoading"
        >
          <el-option v-for="n in objCandidates" :key="n" :label="n" :value="n" />
        </el-select>
        <el-button type="primary" :disabled="!objForm.objectName" @click="addObject">添加</el-button>
      </el-space>

      <el-table :data="objList" size="small" border max-height="360">
        <el-table-column prop="svnObjectType" label="类型" width="100">
          <template #default="{ row }">{{ objectTypeText(row.svnObjectType) }}</template>
        </el-table-column>
        <el-table-column prop="svnObjectName" label="名称" min-width="220" />
        <el-table-column prop="secondPriAddTime" label="添加时间" width="170" />
        <el-table-column label="操作" width="90">
          <template #default="{ row }">
            <el-button link type="danger" size="small" @click="delObject(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!objList.length" description="白名单为空" :image-size="70" />
      <template #footer>
        <el-button type="primary" @click="objVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, RefreshRight, Search, Upload } from '@element-plus/icons-vue'
import {
  apiAliaseList,
  apiGroupList,
  apiSecondPriObjectCreate,
  apiSecondPriObjectDel,
  apiSecondPriObjectList,
  apiSecondPriUpdate,
  apiSecondPriUserPaths,
  apiSyncUsers,
  apiUserCreate,
  apiUserDel,
  apiUserImport,
  apiUserList,
  apiUserScan,
  apiUserUpdMail,
  apiUserUpdNote,
  apiUserUpdPass,
  apiUserUpdStatus,
  apiUserRepList2,
  type SvnUser
} from '@/api'

const list = ref<SvnUser[]>([])
const total = ref(0)
const loading = ref(false)
const query = reactive({ page: 1, pageSize: 20, keyword: '', status: undefined as any })

const createVisible = ref(false)
const createForm = reactive({ svnUserName: '', svnUserPass: '', svnUserMail: '', svnUserNote: '' })

const current = ref<any>({})
const passVisible = ref(false)
const noteVisible = ref(false)
const mailVisible = ref(false)
const newPass = ref('')
const newNote = ref('')
const newMail = ref('')

const scanVisible = ref(false)
const scanList = ref<string[]>([])
const importNames = ref<string[]>([])

const permVisible = ref(false)
const permList = ref<any[]>([])

/* 用户迁入向导 */
const migrateVisible = ref(false)
const migrateStep = ref(0)
const migrateText = ref('')
const migrateUsers = ref<any[]>([])
const migrateRunning = ref(false)
const migrateResult = reactive<any>({ success: 0, skipped: 0, failed: 0, details: [] })

/* 二次授权 */
const secondVisible = ref(false)
const secondLoading = ref(false)
const secondRows = ref<any[]>([])
const objVisible = ref(false)
const objRow = reactive<any>({ repName: '', path: '' })
const objList = ref<any[]>([])
const objForm = reactive<any>({ objectType: 'user', objectName: '' })
const objCandidates = ref<string[]>([])
const objCandidatesLoading = ref(false)

const objPlaceholder = computed(() => {
  const map: any = { user: '选择 SVN 用户', group: '选择分组', aliase: '选择别名' }
  return map[objForm.objectType] || '选择对象'
})

const objectTypeText = (t: string) => ({ user: '用户', group: '分组', aliase: '别名' }[t] || t || '-')

/* -------------------- 列表 -------------------- */

const load = async () => {
  loading.value = true
  try {
    const res = await apiUserList(query)
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

const syncUsers = async () => {
  await apiSyncUsers()
  ElMessage.success('用户数据已同步')
  load()
}

/* -------------------- 增删改 -------------------- */

const openCreate = () => {
  Object.assign(createForm, { svnUserName: '', svnUserPass: '', svnUserMail: '', svnUserNote: '' })
  createVisible.value = true
}

const submitCreate = async () => {
  if (!createForm.svnUserName || !createForm.svnUserPass) {
    ElMessage.warning('用户名和密码不能为空')
    return
  }
  await apiUserCreate({ ...createForm })
  ElMessage.success('创建成功')
  createVisible.value = false
  load()
}

const toggleStatus = async (row: any, v: any) => {
  await apiUserUpdStatus({ svnUserName: row.svnUserName, svnUserStatus: v ? 1 : 0 })
  ElMessage.success(v ? '已启用' : '已禁用')
  load()
}

const openEditPass = (row: any) => {
  current.value = row
  newPass.value = ''
  passVisible.value = true
}

const submitPass = async () => {
  if (!newPass.value) {
    ElMessage.warning('请输入新密码')
    return
  }
  await apiUserUpdPass({ svnUserName: current.value.svnUserName, svnUserPass: newPass.value })
  ElMessage.success('密码已更新')
  passVisible.value = false
}

const openEditNote = (row: any) => {
  current.value = row
  newNote.value = row.svnUserNote || ''
  noteVisible.value = true
}

const submitNote = async () => {
  await apiUserUpdNote({ svnUserName: current.value.svnUserName, svnUserNote: newNote.value })
  ElMessage.success('备注已更新')
  noteVisible.value = false
  load()
}

const openEditMail = (row: any) => {
  current.value = row
  newMail.value = row.svnUserMail || ''
  mailVisible.value = true
}

const submitMail = async () => {
  await apiUserUpdMail({ svnUserName: current.value.svnUserName, svnUserMail: newMail.value })
  ElMessage.success('邮箱已更新')
  mailVisible.value = false
  load()
}

const remove = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确认删除用户 [${row.svnUserName}]？其分组与授权会一并移除。`, '警告', {
      type: 'warning'
    })
  } catch {
    return
  }
  await apiUserDel({ svnUserName: row.svnUserName })
  ElMessage.success('删除成功')
  load()
}

const openScan = async () => {
  scanList.value = await apiUserScan()
  importNames.value = [...scanList.value]
  scanVisible.value = true
}

const submitImport = async () => {
  const res = await apiUserImport({ names: importNames.value })
  ElMessage.success(`成功导入 ${res.count} 个用户`)
  scanVisible.value = false
  load()
}

const openPerm = async (row: any) => {
  current.value = row
  permList.value = await apiUserRepList2(row.svnUserName)
  permVisible.value = true
}

/* -------------------- 用户迁入 -------------------- */

const openMigrate = () => {
  migrateStep.value = 0
  migrateText.value = ''
  migrateUsers.value = []
  Object.assign(migrateResult, { success: 0, skipped: 0, failed: 0, details: [] })
  migrateVisible.value = true
}

const parseMigrate = () => {
  const seen = new Set<string>()
  const users: any[] = []
  migrateText.value.split(/\r?\n/).forEach((raw) => {
    let line = raw.trim()
    if (!line || line.startsWith('#') || line.startsWith(';')) return
    if (line.startsWith('[')) return // [users] / [groups] 段标记
    const eq = line.indexOf('=')
    if (eq < 0) return
    const name = line.slice(0, eq).trim()
    const pass = line.slice(eq + 1).trim()
    if (!name || seen.has(name)) return
    seen.add(name)
    users.push({ name, pass })
  })
  migrateUsers.value = users
  migrateStep.value = 1
}

const runMigrate = async () => {
  migrateRunning.value = true
  Object.assign(migrateResult, { success: 0, skipped: 0, failed: 0, details: [] })
  try {
    for (const u of migrateUsers.value) {
      if (list.value.some((x: any) => x.svnUserName === u.name)) {
        migrateResult.skipped++
        migrateResult.details.push({ name: u.name, result: '跳过', message: '用户已存在' })
        continue
      }
      try {
        await apiUserCreate({ svnUserName: u.name, svnUserPass: u.pass || u.name, svnUserNote: '用户迁入' })
        migrateResult.success++
        migrateResult.details.push({ name: u.name, result: '成功', message: '' })
      } catch (e: any) {
        migrateResult.failed++
        migrateResult.details.push({
          name: u.name,
          result: '失败',
          message: e?.response?.data?.message || e?.message || '未知错误'
        })
      }
    }
    migrateStep.value = 2
    load()
  } finally {
    migrateRunning.value = false
  }
}

/* -------------------- 二次授权 -------------------- */

const openSecondPri = async (row: any) => {
  current.value = row
  secondVisible.value = true
  await loadSecond()
}

const loadSecond = async () => {
  secondLoading.value = true
  try {
    const perms = await apiUserRepList2(current.value.svnUserName)
    const enabled = await apiSecondPriUserPaths(current.value.svnUserName)
    const keys = new Set(
      (enabled || []).map((x: any) => `${x.secondPriRepName || x.repName}||${x.secondPriPath || x.path || ''}`)
    )
    const rows: any[] = []
    ;(perms || []).forEach((p: any) => {
      const paths: string[] = Array.isArray(p.paths) && p.paths.length ? p.paths : ['']
      paths.forEach((path) => {
        const key = `${p.repName}||${path}`
        rows.push({ repName: p.repName, path, enable: keys.has(key), objectCount: 0 })
      })
    })
    secondRows.value = rows
    for (const r of rows) {
      if (!r.enable) continue
      const objs = await apiSecondPriObjectList({
        svnUserName: current.value.svnUserName,
        repName: r.repName,
        priPath: r.path
      })
      r.objectCount = (objs || []).filter((o: any) => o.svnObjectName).length
    }
  } finally {
    secondLoading.value = false
  }
}

const toggleSecond = async (row: any, v: any) => {
  await apiSecondPriUpdate({
    svnUserName: current.value.svnUserName,
    repName: row.repName,
    priPath: row.path,
    enable: !!v
  })
  row.enable = !!v
  if (!v) row.objectCount = 0
  ElMessage.success(v ? '已开启二次授权' : '已关闭二次授权并清空白名单')
}

const openObjects = async (row: any) => {
  Object.assign(objRow, { repName: row.repName, path: row.path })
  objForm.objectName = ''
  objVisible.value = true
  await Promise.all([loadObjects(), loadCandidates()])
}

const loadObjects = async () => {
  objList.value = await apiSecondPriObjectList({
    svnUserName: current.value.svnUserName,
    repName: objRow.repName,
    priPath: objRow.path
  })
  objList.value = (objList.value || []).filter((o: any) => o.svnObjectName)
}

const loadCandidates = async () => {
  objCandidatesLoading.value = true
  try {
    if (objForm.objectType === 'user') {
      const res = await apiUserList({ page: 1, pageSize: 1000 })
      objCandidates.value = (res.list || []).map((u: any) => u.svnUserName).filter((n: string) => n)
    } else if (objForm.objectType === 'group') {
      const res = await apiGroupList({ page: 1, pageSize: 1000 })
      objCandidates.value = (res.list || []).map((g: any) => g.svnGroupName).filter((n: string) => n)
    } else {
      const m = await apiAliaseList()
      objCandidates.value = Object.keys(m || {})
    }
  } finally {
    objCandidatesLoading.value = false
  }
}

watch(() => objForm.objectType, () => {
  objForm.objectName = ''
  loadCandidates()
})

const addObject = async () => {
  await apiSecondPriObjectCreate({
    svnUserName: current.value.svnUserName,
    repName: objRow.repName,
    priPath: objRow.path,
    objectType: objForm.objectType,
    objectName: objForm.objectName
  })
  ElMessage.success('已添加')
  objForm.objectName = ''
  await loadObjects()
  const r = secondRows.value.find(
    (x) => x.repName === objRow.repName && x.path === objRow.path
  )
  if (r) r.objectCount = objList.value.length
}

const delObject = async (row: any) => {
  await apiSecondPriObjectDel(row.svnSecondPriId)
  ElMessage.success('已删除')
  await loadObjects()
  const r = secondRows.value.find((x) => x.repName === objRow.repName && x.path === objRow.path)
  if (r) r.objectCount = objList.value.length
}

onMounted(load)
</script>
