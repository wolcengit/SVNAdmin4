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
      <h2>SVN 分组</h2>
      <div class="toolbar" style="margin: 0">
        <el-button type="primary" :icon="Plus" @click="openCreate">新建分组</el-button>
        <el-button :icon="Refresh" @click="load">刷新</el-button>
      </div>
    </div>

    <div class="toolbar">
      <el-input
        v-model="query.keyword"
        placeholder="分组名 / 备注"
        clearable
        style="width: 220px"
        @keyup.enter="load"
        @clear="load"
      />
      <el-button type="primary" :icon="Search" @click="load">查询</el-button>
    </div>

    <el-table v-loading="loading" :data="list" border size="small">
      <el-table-column prop="svnGroupId" label="ID" width="70" />
      <el-table-column prop="svnGroupName" label="分组名" min-width="150" />
      <el-table-column label="用户" width="90" align="right">
        <template #default="{ row }">{{ row.includeUserCount }}</template>
      </el-table-column>
      <el-table-column label="分组" width="90" align="right">
        <template #default="{ row }">{{ row.includeGroupCount }}</template>
      </el-table-column>
      <el-table-column label="别名" width="90" align="right">
        <template #default="{ row }">{{ row.includeAliaseCount }}</template>
      </el-table-column>
      <el-table-column prop="svnGroupNote" label="备注" min-width="180" show-overflow-tooltip />
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openMember(row)">成员</el-button>
          <el-button link type="primary" @click="openRename(row)">改名</el-button>
          <el-button link type="primary" @click="openNote(row)">备注</el-button>
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

    <el-dialog v-model="createVisible" title="新建分组" width="420px">
      <el-form label-width="80px">
        <el-form-item label="分组名" required>
          <el-input v-model="createName" placeholder="不能包含 / \ : * @ & = 等字符" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="createNote" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" @click="submitCreate">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="renameVisible" title="修改分组名" width="420px">
      <el-form label-width="80px">
        <el-form-item label="原名">
          <el-input :model-value="current.svnGroupName" disabled />
        </el-form-item>
        <el-form-item label="新名" required>
          <el-input v-model="newName" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="renameVisible = false">取消</el-button>
        <el-button type="primary" @click="submitRename">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="noteVisible" title="修改备注" width="420px">
      <el-form label-width="80px">
        <el-form-item label="分组名">
          <el-input :model-value="current.svnGroupName" disabled />
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

    <el-drawer v-model="memberVisible" :title="`分组成员 - ${current.svnGroupName}`" size="45%">
      <el-form label-position="top">
        <el-form-item label="包含用户">
          <el-select v-model="members.users" multiple filterable clearable style="width: 100%">
            <el-option v-for="u in allUsers" :key="u" :label="u" :value="u" />
          </el-select>
        </el-form-item>
        <el-form-item label="包含分组（嵌套）">
          <el-select v-model="members.groups" multiple filterable clearable style="width: 100%">
            <el-option v-for="g in allGroups" :key="g" :label="g" :value="g" />
          </el-select>
        </el-form-item>
        <el-form-item label="包含别名">
          <el-select v-model="members.aliases" multiple filterable allow-create clearable style="width: 100%">
            <el-option v-for="a in allAliases" :key="a" :label="a" :value="a" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="memberVisible = false">取消</el-button>
        <el-button type="primary" @click="submitMember">保存</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, Search } from '@element-plus/icons-vue'
import {
  apiGroupCreate,
  apiGroupDel,
  apiGroupList,
  apiGroupMember,
  apiGroupUpdMember,
  apiGroupUpdName,
  apiGroupUpdNote,
  apiUserAll,
  apiGroupAll,
  apiAliaseList,
  type SvnGroup
} from '@/api'

const list = ref<SvnGroup[]>([])
const total = ref(0)
const loading = ref(false)
const query = reactive({ page: 1, pageSize: 20, keyword: '' })

const current = ref<any>({})
const createVisible = ref(false)
const createName = ref('')
const createNote = ref('')

const renameVisible = ref(false)
const newName = ref('')

const noteVisible = ref(false)
const newNote = ref('')

const memberVisible = ref(false)
const members = reactive<{ users: string[]; groups: string[]; aliases: string[] }>({
  users: [],
  groups: [],
  aliases: []
})
const allUsers = ref<string[]>([])
const allGroups = ref<string[]>([])
const allAliases = ref<string[]>([])

const load = async () => {
  loading.value = true
  try {
    const res = await apiGroupList(query)
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  createName.value = ''
  createNote.value = ''
  createVisible.value = true
}

const submitCreate = async () => {
  if (!createName.value) {
    ElMessage.warning('请输入分组名')
    return
  }
  await apiGroupCreate({ svnGroupName: createName.value, svnGroupNote: createNote.value })
  ElMessage.success('创建成功')
  createVisible.value = false
  load()
}

const openRename = (row: any) => {
  current.value = row
  newName.value = row.svnGroupName
  renameVisible.value = true
}

const submitRename = async () => {
  await apiGroupUpdName({ oldName: current.value.svnGroupName, newName: newName.value })
  ElMessage.success('修改成功')
  renameVisible.value = false
  load()
}

const openNote = (row: any) => {
  current.value = row
  newNote.value = row.svnGroupNote || ''
  noteVisible.value = true
}

const submitNote = async () => {
  await apiGroupUpdNote({ svnGroupName: current.value.svnGroupName, svnGroupNote: newNote.value })
  ElMessage.success('修改成功')
  noteVisible.value = false
  load()
}

const remove = async (row: any) => {
  await ElMessageBox.confirm(`确认删除分组 [${row.svnGroupName}]？`, '警告', { type: 'warning' })
  await apiGroupDel({ svnGroupName: row.svnGroupName })
  ElMessage.success('删除成功')
  load()
}

const openMember = async (row: any) => {
  current.value = row
  const [users, groups, aliases, member] = await Promise.all([
    apiUserAll(),
    apiGroupAll(),
    apiAliaseList(),
    apiGroupMember(row.svnGroupName)
  ])
  allUsers.value = users.map((u) => u.svnUserName)
  allGroups.value = groups.filter((g) => g !== row.svnGroupName)
  allAliases.value = Object.keys(aliases || {})
  members.users = member.users || []
  members.groups = member.groups || []
  members.aliases = member.aliases || []
  memberVisible.value = true
}

const submitMember = async () => {
  await apiGroupUpdMember({
    svnGroupName: current.value.svnGroupName,
    users: members.users,
    groups: members.groups,
    aliases: members.aliases
  })
  ElMessage.success('成员已保存')
  memberVisible.value = false
  load()
}

onMounted(load)
</script>
