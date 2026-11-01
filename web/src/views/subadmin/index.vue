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
      <h2>子管理员</h2>
      <div class="toolbar" style="margin: 0">
        <el-button type="primary" :icon="Plus" @click="openCreate">新建子管理员</el-button>
        <el-button :icon="Refresh" @click="load">刷新</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="list" border size="small">
      <el-table-column prop="subadminId" label="ID" width="70" />
      <el-table-column prop="subadminName" label="账号" min-width="130" />
      <el-table-column label="备注" min-width="180" show-overflow-tooltip>
        <template #default="{ row }">
          <span>{{ row.subadminNote || '-' }}</span>
          <el-button link type="primary" size="small" style="margin-left: 6px" @click="openNote(row)">编辑</el-button>
        </template>
      </el-table-column>
      <el-table-column prop="subadminEmail" label="邮箱" min-width="170" show-overflow-tooltip />
      <el-table-column prop="subadminPhone" label="手机号" min-width="130" />
      <el-table-column label="在线" width="80">
        <template #default="{ row }">
          <el-tag :type="row.online ? 'success' : 'info'" size="small" effect="plain">
            {{ row.online ? '在线' : '离线' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-switch :model-value="row.subadminStatus === 1" @change="(v: any) => toggle(row, v)" />
        </template>
      </el-table-column>
      <el-table-column prop="subadminLastLogin" label="最后登录" width="170" />
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button link type="warning" @click="openTree(row)">功能权限</el-button>
          <el-button link type="primary" @click="openPass(row)">改密</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="createVisible" title="新建子管理员" width="460px">
      <el-form label-width="90px">
        <el-form-item label="账号" required><el-input v-model="f.subadminName" /></el-form-item>
        <el-form-item label="密码" required><el-input v-model="f.subadminPass" show-password /></el-form-item>
        <el-form-item label="备注"><el-input v-model="f.subadminNote" /></el-form-item>
        <el-form-item label="邮箱"><el-input v-model="f.subadminEmail" /></el-form-item>
        <el-form-item label="手机号"><el-input v-model="f.subadminPhone" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" @click="submitCreate">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="passVisible" title="修改密码" width="400px">
      <el-form label-width="90px">
        <el-form-item label="账号">
          <el-input :model-value="current.subadminName" disabled />
        </el-form-item>
        <el-form-item label="新密码" required><el-input v-model="newPass" show-password /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passVisible = false">取消</el-button>
        <el-button type="primary" @click="submitPass">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="noteVisible" title="修改备注" width="400px">
      <el-form label-width="90px">
        <el-form-item label="账号">
          <el-input :model-value="current.subadminName" disabled />
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

    <!-- 功能权限树 -->
    <el-dialog v-model="treeVisible" :title="`功能权限 - ${current.subadminName}`" width="720px" top="6vh">
      <el-alert
        v-if="needUpdateTree"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 12px"
        title="系统功能权限项已发生变化（或该账号尚未授权），请重新勾选后保存。"
      />
      <el-space style="margin-bottom: 10px">
        <el-button size="small" @click="checkAll(true)">全选</el-button>
        <el-button size="small" @click="checkAll(false)">全不选</el-button>
        <el-checkbox v-model="showFunctions" size="small">显示接口明细</el-checkbox>
      </el-space>

      <el-tree
        ref="treeRef"
        :data="treeData"
        show-checkbox
        node-key="key"
        default-expand-all
        :default-checked-keys="checkedKeys"
        :props="{ label: 'title', children: 'children', disabled: 'disabled' }"
        style="max-height: 52vh; overflow: auto"
      >
        <template #default="{ data }">
          <span class="tree-node">
            <span>{{ data.title }}</span>
            <el-tag v-if="data.disabled" size="small" type="info" effect="plain">必选</el-tag>
            <span v-if="showFunctions && data.necessaryFunctions?.length" class="func-text">
              {{ data.necessaryFunctions.join(' / ') }}
            </span>
          </span>
        </template>
      </el-tree>

      <template #footer>
        <el-button @click="treeVisible = false">取消</el-button>
        <el-button type="primary" @click="submitTree">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh } from '@element-plus/icons-vue'
import {
  apiSubadminCreate,
  apiSubadminDel,
  apiSubadminList,
  apiSubadminTree,
  apiSubadminUpdNote,
  apiSubadminUpdPass,
  apiSubadminUpdStatus,
  apiSubadminUpdTree
} from '@/api'

const list = ref<any[]>([])
const loading = ref(false)
const createVisible = ref(false)
const passVisible = ref(false)
const noteVisible = ref(false)
const newPass = ref('')
const newNote = ref('')
const current = ref<any>({})

const f = reactive({
  subadminName: '',
  subadminPass: '',
  subadminNote: '',
  subadminEmail: '',
  subadminPhone: ''
})

/* 权限树 */
const treeVisible = ref(false)
const treeRef = ref<any>()
const treeData = ref<any[]>([])
const rawTree = ref<any[]>([])
const checkedKeys = ref<string[]>([])
const needUpdateTree = ref(false)
const showFunctions = ref(false)

const load = async () => {
  loading.value = true
  try {
    const res: any = await apiSubadminList()
    list.value = res?.list || []
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  Object.assign(f, { subadminName: '', subadminPass: '', subadminNote: '', subadminEmail: '', subadminPhone: '' })
  createVisible.value = true
}

const submitCreate = async () => {
  if (!f.subadminName || !f.subadminPass) {
    ElMessage.warning('账号和密码不能为空')
    return
  }
  await apiSubadminCreate({ ...f })
  ElMessage.success('创建成功')
  createVisible.value = false
  load()
}

const toggle = async (row: any, v: any) => {
  await apiSubadminUpdStatus(row.subadminId, v ? 1 : 0)
  ElMessage.success(v ? '已启用' : '已禁用')
  load()
}

const openPass = (row: any) => {
  current.value = row
  newPass.value = ''
  passVisible.value = true
}

const submitPass = async () => {
  if (!newPass.value) {
    ElMessage.warning('请输入新密码')
    return
  }
  await apiSubadminUpdPass({ subadminId: current.value.subadminId, subadminPass: newPass.value })
  ElMessage.success('密码已更新')
  passVisible.value = false
}

const openNote = (row: any) => {
  current.value = row
  newNote.value = row.subadminNote || ''
  noteVisible.value = true
}

const submitNote = async () => {
  await apiSubadminUpdNote(current.value.subadminId, newNote.value)
  ElMessage.success('备注已更新')
  noteVisible.value = false
  load()
}

const remove = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确认删除子管理员 [${row.subadminName}]？`, '警告', { type: 'warning' })
  } catch {
    return
  }
  await apiSubadminDel(row.subadminId)
  ElMessage.success('删除成功')
  load()
}

/* -------------------- 权限树 -------------------- */

// 为每个节点生成稳定的 key（按标题路径），并记录 checked 状态
const decorate = (nodes: any[], prefix = ''): any[] =>
  (nodes || []).map((n, i) => {
    const key = prefix ? `${prefix}/${n.title}` : `${i}:${n.title}`
    const item = {
      ...n,
      key,
      children: n.children ? decorate(n.children, key) : undefined
    }
    if (!item.children) delete item.children
    return item
  })

const collectChecked = (nodes: any[], out: string[]) => {
  ;(nodes || []).forEach((n) => {
    if (n.checked || n.disabled) out.push(n.key)
    if (n.children) collectChecked(n.children, out)
  })
  return out
}

const collectAllKeys = (nodes: any[], out: string[]) => {
  ;(nodes || []).forEach((n) => {
    out.push(n.key)
    if (n.children) collectAllKeys(n.children, out)
  })
  return out
}

const openTree = async (row: any) => {
  current.value = row
  const res = await apiSubadminTree(row.subadminId)
  rawTree.value = res.tree || []
  treeData.value = decorate(res.tree || [])
  needUpdateTree.value = !!res.needUpdateTree
  checkedKeys.value = collectChecked(treeData.value, [])
  treeVisible.value = true
  await nextTick()
  treeRef.value?.setCheckedKeys(checkedKeys.value, false)
}

const checkAll = (v: boolean) => {
  if (!v) treeRef.value?.setCheckedKeys([], false)
  else treeRef.value?.setCheckedKeys(collectAllKeys(treeData.value, []), false)
}

// 依据勾选状态回写原始树的 checked 字段后提交
const applyCheckedRaw = (nodes: any[], keys: Set<string>): any[] =>
  (nodes || []).map((n) => {
    const key = n.key
    const checked = n.disabled || keys.has(key)
    return {
      ...n,
      key: undefined,
      checked,
      children: n.children ? applyCheckedRaw(n.children, keys) : undefined
    }
  })

const submitTree = async () => {
  const keys = new Set<string>([
    ...(treeRef.value?.getCheckedKeys() || []),
    ...(treeRef.value?.getHalfCheckedKeys() || [])
  ])
  const payload = applyCheckedRaw(treeData.value, keys)
  await apiSubadminUpdTree(current.value.subadminId, payload)
  ElMessage.success('功能权限已保存')
  treeVisible.value = false
  load()
}

onMounted(load)
</script>

<style scoped>
.tree-node {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.func-text {
  font-family: Consolas, Menlo, monospace;
  font-size: 11px;
  color: var(--el-text-color-secondary);
}
</style>
