<!--
  Copyright (C) 2026 wolcen@msn.com
  SPDX-License-Identifier: GPL-3.0-or-later

  SVNAdmin4 is free software: you can redistribute it and/or modify it under
  the terms of the GNU General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option) any later
  version. See the LICENSE file in the project root for the full license text.
-->

<template>
  <div class="page">
    <el-card shadow="never">
      <template #header>
        <div class="card-head">
          <span>我的仓库</span>
          <span class="muted">以当前 SVN 用户身份查看被授权的仓库与可访问路径</span>
        </div>
      </template>

      <!-- 仓库列表 -->
      <el-table v-loading="loading" :data="list" size="default" border>
        <el-table-column prop="repName" label="仓库" min-width="160">
          <template #default="{ row }">
            <el-link type="primary" @click="openRep(row)">{{ row.repName }}</el-link>
          </template>
        </el-table-column>
        <el-table-column prop="repNote" label="备注" min-width="140" show-overflow-tooltip />
        <el-table-column prop="repRev" label="版本数" width="90" align="right" />
        <el-table-column label="体积" width="100" align="right">
          <template #default="{ row }">{{ fmtSize(row.repSize) }}</template>
        </el-table-column>
        <el-table-column label="我的权限" width="110">
          <template #default="{ row }">
            <el-tag :type="row.perm === 'rw' ? 'success' : 'primary'" size="small">{{ row.perm || '无' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="可访问路径" min-width="220">
          <template #default="{ row }">
            <el-tag
              v-for="p in (row.paths || []).slice(0, 8)"
              :key="p"
              size="small"
              effect="plain"
              style="margin-right: 4px"
            >
              {{ p }}
            </el-tag>
            <span v-if="!row.paths?.length" class="muted">/</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="280">
          <template #default="{ row }">
            <el-button link type="primary" @click="openRep(row)">浏览内容</el-button>
            <el-button link type="primary" @click="copyCheckout(row)">复制检出地址</el-button>
            <el-button link type="warning" @click="openSecondPri(row)">二次授权</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 二次授权自助管理 -->
    <el-dialog v-model="spVisible" :title="`二次授权 - ${spRep.repName}`" width="720px" top="6vh">
      <div class="sp-toolbar">
        <el-select v-model="spPath" placeholder="选择路径" size="small" style="width: 240px">
          <el-option v-for="p in spPathOptions" :key="p" :label="p" :value="p" />
        </el-select>
        <el-switch
          v-model="spEnabled"
          active-text="已开启"
          inactive-text="已关闭"
          :loading="spSwitching"
          @change="toggleSecondPri"
        />
      </div>
      <el-table v-loading="spLoading" :data="spObjects" size="small" border max-height="360">
        <el-table-column prop="svnObjectType" label="类型" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.svnObjectType === 'user' ? 'primary' : row.svnObjectType === 'group' ? 'success' : 'warning'">
              {{ row.svnObjectType }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="svnObjectName" label="可下放对象" min-width="180" />
        <el-table-column prop="secondPriAddTime" label="添加时间" width="170" />
        <el-table-column label="操作" width="80">
          <template #default="{ row }">
            <el-button link type="danger" size="small" @click="removeObject(row)">移除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="sp-add">
        <el-select v-model="newObjectType" size="small" style="width: 110px">
          <el-option label="用户" value="user" />
          <el-option label="分组" value="group" />
          <el-option label="别名" value="aliase" />
        </el-select>
        <el-input v-model="newObjectName" placeholder="对象名称" size="small" style="width: 220px" />
        <el-button size="small" type="primary" :disabled="!newObjectName || !spPath" @click="addObject">加入白名单</el-button>
      </div>
      <div class="muted" style="margin-top: 8px">
        开启后，您可以把该路径的权限下放给白名单内的对象（在仓库权限页对该对象授权即生效）。
      </div>
    </el-dialog>

    <!-- 目录浏览 -->
    <el-dialog v-model="browseVisible" :title="`仓库浏览 - ${current.repName}`" width="860px" top="6vh">
      <div class="toolbar">
        <el-breadcrumb separator="/">
          <el-breadcrumb-item>
            <el-link :underline="false" @click="goPath('/')">{{ current.repName }}</el-link>
          </el-breadcrumb-item>
          <el-breadcrumb-item v-for="(seg, i) in pathSegs" :key="i">
            <el-link :underline="false" @click="goPath(pathSegs.slice(0, i + 1).join('/'))">{{ seg }}</el-link>
          </el-breadcrumb-item>
        </el-breadcrumb>
        <el-input
          v-model="svnUser"
          placeholder="以该 SVN 用户身份浏览（默认当前登录名）"
          size="small"
          style="width: 260px"
        />
      </div>
      <el-table v-loading="conLoading" :data="entries" size="small" border max-height="480">
        <el-table-column label="名称" min-width="220">
          <template #default="{ row }">
            <el-icon v-if="row.type === 'dir'" color="#e6a23c"><Folder /></el-icon>
            <el-icon v-else color="#909399"><Document /></el-icon>
            <el-link
              v-if="row.type === 'dir'"
              type="primary"
              style="margin-left: 6px"
              @click="goPath(row.path)"
            >
              {{ row.name }}
            </el-link>
            <span v-else style="margin-left: 6px">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="rev" label="版本" width="80" align="center" />
        <el-table-column prop="author" label="作者" width="120" />
        <el-table-column prop="date" label="时间" width="170" />
        <el-table-column label="大小" width="100" align="right">
          <template #default="{ row }">{{ row.type === 'dir' ? '-' : fmtSize(row.size) }}</template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Document, Folder } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import {
  apiRepCheckout,
  apiRepUserCon,
  apiSecondPriObjectCreate,
  apiSecondPriObjectDel,
  apiSecondPriObjectList,
  apiSecondPriUpdate,
  apiSecondPriUserPaths,
  apiUserRepList,
  type SvnRep
} from '@/api'

const store = useUserStore()

const loading = ref(false)
const list = ref<any[]>([])

const browseVisible = ref(false)
const current = ref<Partial<SvnRep>>({})
const entries = ref<any[]>([])
const conLoading = ref(false)
const curPath = ref('/')
const svnUser = ref('')

const pathSegs = computed(() =>
  curPath.value
    .split('/')
    .filter(Boolean)
)

const fmtSize = (n?: number) => {
  if (!n || n <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`
}

const load = async () => {
  loading.value = true
  try {
    list.value = (await apiUserRepList()) || []
  } catch {
    list.value = []
  } finally {
    loading.value = false
  }
}

const openRep = async (row: any) => {
  current.value = row
  curPath.value = '/'
  svnUser.value = store.userName
  browseVisible.value = true
  await loadCon()
}

const loadCon = async () => {
  if (!current.value.repName) return
  conLoading.value = true
  try {
    entries.value =
      (await apiRepUserCon(current.value.repName, curPath.value, svnUser.value || store.userName)) || []
  } catch (e: any) {
    entries.value = []
    ElMessage.error(e?.message || '读取目录失败（可能无权限）')
  } finally {
    conLoading.value = false
  }
}

const goPath = (p: string) => {
  curPath.value = p === '/' ? '/' : p
  loadCon()
}

const copyCheckout = async (row: any) => {
  try {
    const res = await apiRepCheckout(row.repName)
    const url = res?.svn || res?.http || ''
    if (!url) {
      ElMessage.warning('未获取到检出地址')
      return
    }
    await navigator.clipboard?.writeText(url)
    ElMessage.success('检出地址已复制')
  } catch (e: any) {
    ElMessage.error(e?.message || '获取检出地址失败')
  }
}

// ---------------------------------------------------------------------------
// 二次授权自助管理（SVN 用户把自身权限路径下放给白名单对象）
// ---------------------------------------------------------------------------

const spVisible = ref(false)
const spRep = ref<any>({})
const spPath = ref('/')
const spPathOptions = ref<string[]>([])
const spEnabled = ref(false)
const spSwitching = ref(false)
const spLoading = ref(false)
const spObjects = ref<any[]>([])
const newObjectType = ref('user')
const newObjectName = ref('')

const openSecondPri = async (row: any) => {
  spRep.value = row
  spPathOptions.value = ['/'].concat((row.paths || []).filter((p: string) => p !== '/'))
  spPath.value = '/'
  spVisible.value = true
  try {
    const paths = (await apiSecondPriUserPaths(store.userName)) || []
    const hit = paths.find((p: any) => p.repName === row.repName && p.priPath === '/')
    spEnabled.value = !!hit
  } catch {
    spEnabled.value = false
  }
  await loadObjects()
}

const loadObjects = async () => {
  spLoading.value = true
  try {
    spObjects.value = (await apiSecondPriObjectList({ repName: spRep.value.repName, priPath: spPath.value })) || []
  } catch {
    spObjects.value = []
  } finally {
    spLoading.value = false
  }
}

const toggleSecondPri = async (val: boolean | string | number) => {
  spSwitching.value = true
  try {
    await apiSecondPriUpdate({ repName: spRep.value.repName, priPath: spPath.value, enable: !!val })
    ElMessage.success(val ? '已开启二次授权' : '已关闭二次授权（白名单已清空）')
    if (!val) spObjects.value = []
  } catch (e: any) {
    spEnabled.value = !val
    ElMessage.error(e?.message || '操作失败')
  } finally {
    spSwitching.value = false
  }
}

const addObject = async () => {
  try {
    await apiSecondPriObjectCreate({
      repName: spRep.value.repName,
      priPath: spPath.value,
      objectType: newObjectType.value,
      objectName: newObjectName.value
    })
    ElMessage.success('已加入白名单')
    newObjectName.value = ''
    await loadObjects()
  } catch (e: any) {
    ElMessage.error(e?.message || '添加失败')
  }
}

const removeObject = async (row: any) => {
  try {
    await apiSecondPriObjectDel(row.svnSecondPriId)
    ElMessage.success('已移除')
    await loadObjects()
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

onMounted(load)
</script>

<style scoped>
.card-head {
  display: flex;
  align-items: baseline;
  gap: 12px;
}
.muted {
  color: #909399;
  font-size: 12px;
}
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}
.sp-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}
.sp-add {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-top: 12px;
}
</style>
