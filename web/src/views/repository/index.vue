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
      <h2>SVN 仓库</h2>
      <div class="toolbar" style="margin: 0">
        <el-button type="primary" :icon="Plus" @click="openCreate">新建仓库</el-button>
        <el-button :icon="Refresh" @click="load">刷新</el-button>
        <el-button :icon="CircleCheck" @click="checkAuthz">校验 authz</el-button>
      </div>
    </div>

    <div class="toolbar">
      <el-input
        v-model="query.keyword"
        placeholder="仓库名 / 备注"
        clearable
        style="width: 220px"
        @keyup.enter="load"
        @clear="load"
      />
      <el-button type="primary" :icon="Search" @click="load">查询</el-button>
    </div>

    <el-table v-loading="loading" :data="list" border size="small">
      <el-table-column prop="repId" label="ID" width="70" />
      <el-table-column prop="repName" label="仓库名" min-width="150" />
      <el-table-column label="版本数" width="90" align="right">
        <template #default="{ row }">{{ row.repRev }}</template>
      </el-table-column>
      <el-table-column label="体积" width="100" align="right">
        <template #default="{ row }">{{ fmtSize(row.repSize) }}</template>
      </el-table-column>
      <el-table-column prop="repUuid" label="UUID" min-width="220" show-overflow-tooltip />
      <el-table-column prop="repNote" label="备注" min-width="160" show-overflow-tooltip />
      <el-table-column label="操作" width="260" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDetail(row)">管理</el-button>
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

    <!-- 新建 -->
    <el-dialog v-model="createVisible" title="新建仓库" width="460px">
      <el-form label-width="100px">
        <el-form-item label="仓库名" required>
          <el-input v-model="createForm.repName" placeholder="不能包含空格与 / \ : * 等字符" />
        </el-form-item>
        <el-form-item label="初始化结构">
          <el-select v-model="createForm.initStruct" style="width: 100%">
            <el-option label="不套用模板" value="" />
            <el-option label="默认模板 01" value="01" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="createForm.repNote" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createVisible = false">取消</el-button>
        <el-button type="primary" @click="submitCreate">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="renameVisible" title="修改仓库名" width="420px">
      <el-form label-width="80px">
        <el-form-item label="原名">
          <el-input :model-value="current.repName" disabled />
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
        <el-form-item label="仓库名">
          <el-input :model-value="current.repName" disabled />
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

    <!-- 仓库详情抽屉 -->
    <el-drawer v-model="detailVisible" :title="`仓库管理 - ${current.repName}`" size="72%">
      <el-tabs v-model="activeTab">
        <!-- 属性 -->
        <el-tab-pane label="仓库属性" name="info">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="仓库名">{{ detail.repName }}</el-descriptions-item>
            <el-descriptions-item label="最新版本">{{ detail.repRev }}</el-descriptions-item>
            <el-descriptions-item label="占用体积">{{ detail.repSizeHr }}</el-descriptions-item>
            <el-descriptions-item label="UUID">{{ detail.repUuid }}</el-descriptions-item>
            <el-descriptions-item label="库格式">{{ detail.format }}</el-descriptions-item>
            <el-descriptions-item label="存储类型">{{ detail.fsType }}</el-descriptions-item>
            <el-descriptions-item label="物理路径" :span="2">{{ detail.repPath }}</el-descriptions-item>
            <el-descriptions-item label="备注" :span="2">{{ detail.repNote || '-' }}</el-descriptions-item>
          </el-descriptions>
          <div style="margin-top: 12px; display: flex; gap: 8px; flex-wrap: wrap">
            <el-button size="small" @click="syncSize">同步体积</el-button>
            <el-button size="small" @click="syncRev">同步版本数</el-button>
            <el-button size="small" @click="resetUUID">重设 UUID</el-button>
            <el-button size="small" @click="loadCheckout">获取检出地址</el-button>
          </div>
          <div v-if="checkout.svn" style="margin-top: 12px">
            <el-input :model-value="checkout.svn" readonly size="small">
              <template #prepend>svn</template>
              <template #append>
                <el-button :icon="DocumentCopy" @click="copyText(checkout.svn)" />
              </template>
            </el-input>
            <el-input
              v-if="checkout.http"
              :model-value="checkout.http"
              readonly
              size="small"
              style="margin-top: 6px"
            >
              <template #prepend>http</template>
              <template #append>
                <el-button :icon="DocumentCopy" @click="copyText(checkout.http)" />
              </template>
            </el-input>
          </div>
          <div style="margin-top: 12px; display: flex; gap: 8px; flex-wrap: wrap">
            <el-button size="small" type="warning" @click="doVerify">仓库检查(verify)</el-button>
            <el-button size="small" type="warning" @click="doHotcopy">热拷贝备份(hotcopy)</el-button>
          </div>
        </el-tab-pane>

        <!-- 目录树 -->
        <el-tab-pane label="目录结构" name="tree">
          <div class="toolbar">
            <el-input v-model="newFolder" placeholder="新目录路径，如 /trunk/src" style="width: 240px" />
            <el-button type="primary" size="small" @click="createFolder">创建目录</el-button>
            <el-button size="small" @click="reloadTree">刷新</el-button>
            <el-select
              v-model="treeUser"
              clearable
              filterable
              placeholder="以 SVN 用户视角浏览"
              style="width: 200px"
              @change="reloadTree"
            >
              <el-option v-for="u in allUsers" :key="u" :label="u" :value="u" />
            </el-select>
          </div>
          <el-tree
            ref="treeRef"
            v-loading="treeLoading"
            :data="tree"
            :props="{ label: 'name', children: 'children', isLeaf: 'leaf' }"
            node-key="path"
            lazy
            :load="loadNode"
            @node-click="onNodeClick"
            @node-context-menu="onContextMenu"
          >
            <template #default="{ data }">
              <span class="tree-node">
                <el-icon v-if="data.type === 'dir'" color="#e6a23c"><Folder /></el-icon>
                <el-icon v-else color="#909399"><Document /></el-icon>
                <span>{{ data.name }}</span>
                <span class="muted" style="margin-left: 8px">r{{ data.rev }} · {{ data.author }}</span>
              </span>
            </template>
          </el-tree>
          <el-alert
            style="margin-top: 10px"
            :closable="false"
            type="info"
            title="目录按需懒加载。右键节点可新建子目录 / 复制路径 / 将该路径设为权限编辑目标。"
          />
        </el-tab-pane>

        <!-- 权限 -->
        <el-tab-pane label="路径权限" name="pri">
          <div class="toolbar">
            <el-input v-model="priPath" placeholder="路径，如 /trunk" style="width: 220px" @keyup.enter="loadPri" />
            <el-button type="primary" size="small" @click="loadPri">加载</el-button>
            <el-button type="success" size="small" @click="openPriCreate">新增授权</el-button>
          </div>
          <el-alert
            v-if="pri.inherit?.length"
            type="info"
            :closable="false"
            :title="`该路径继承了上级路径的 ${pri.inherit.length} 条权限`"
            style="margin-bottom: 10px"
          />
          <el-table :data="pri.current || []" size="small" border>
            <el-table-column prop="priPath" label="路径" min-width="120" />
            <el-table-column label="对象类型" width="90">
              <template #default="{ row }">{{ typeText(row.objectType) }}</template>
            </el-table-column>
            <el-table-column label="对象" min-width="130">
              <template #default="{ row }">
                <span class="text-mono">{{ row.invert ? '~' : '' }}{{ row.objectName }}</span>
                <el-tag v-if="row.invert" size="small" type="warning" effect="plain" style="margin-left: 6px">
                  取反
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="权限" width="90">
              <template #default="{ row }">
                <el-tag :type="row.perm === 'rw' ? 'success' : row.perm ? 'primary' : 'info'" size="small">
                  {{ row.perm || '无' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="150">
              <template #default="{ row }">
                <el-button link type="primary" @click="openPriEdit(row)">修改</el-button>
                <el-button link type="danger" @click="delPri(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <!-- 备份 -->
        <el-tab-pane label="备份与恢复" name="backup">
          <div class="toolbar">
            <el-button type="primary" size="small" @click="doDump">立即备份</el-button>
            <el-button size="small" @click="loadBackup">刷新</el-button>
            <el-checkbox v-model="incremental" label="增量备份" />
            <el-input v-model="revRange" placeholder="版本范围 如 1:10" style="width: 150px" />
            <el-button size="small" :icon="Upload" @click="pickFile">上传备份文件</el-button>
            <input
              ref="fileInput"
              type="file"
              accept=".dump"
              style="display: none"
              @change="onFilePicked"
            />
          </div>

          <div v-if="uploading" class="upload-bar">
            <el-progress :percentage="uploadPercent" :status="uploadPercent >= 100 ? 'success' : undefined" />
            <span class="upload-text">
              {{ uploadStage }}　{{ fmtSize(uploadDone) }} / {{ fmtSize(uploadTotal) }}
              （{{ uploadDoneParts }}/{{ uploadTotalParts }} 分片）
            </span>
            <el-button size="small" type="danger" link @click="abortUpload">取消上传</el-button>
          </div>

          <el-table :data="backups" size="small" border>
            <el-table-column prop="name" label="文件名" min-width="220" show-overflow-tooltip />
            <el-table-column prop="sizeHr" label="大小" width="100" align="right" />
            <el-table-column prop="modTime" label="生成时间" width="170" />
            <el-table-column label="操作" width="180">
              <template #default="{ row }">
                <el-button link type="primary" @click="doLoad(row)">导入</el-button>
                <el-button link type="primary" @click="doDownload(row)">下载</el-button>
                <el-button link type="danger" @click="delBackup(row)">删除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <!-- 钩子 -->
        <el-tab-pane label="仓库钩子" name="hook">
          <el-table :data="hooks" size="small" border max-height="380">
            <el-table-column prop="name" label="钩子" width="180" />
            <el-table-column prop="description" label="说明" min-width="200" />
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <el-tag :type="row.exists ? 'success' : 'info'" size="small">
                  {{ row.exists ? '已配置' : '未配置' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="150">
              <template #default="{ row }">
                <el-button link type="primary" @click="openHook(row)">编辑</el-button>
                <el-button link type="danger" @click="delHook(row)">清空</el-button>
              </template>
            </el-table-column>
          </el-table>

          <el-divider content-position="left">常用（推荐）钩子</el-divider>
          <el-empty v-if="!recommends.length" description="未配置推荐钩子目录" :image-size="70" />
          <el-table v-else :data="recommends" size="small" border max-height="280">
            <el-table-column prop="name" label="名称" min-width="160" />
            <el-table-column prop="type" label="对应钩子" width="150" />
            <el-table-column prop="description" label="说明" min-width="220" show-overflow-tooltip />
            <el-table-column label="操作" width="160">
              <template #default="{ row }">
                <el-button link type="primary" @click="viewRecommend(row)">查看</el-button>
                <el-button link type="success" @click="applyRecommend(row)">套用到仓库</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-drawer>

    <!-- 授权 -->
    <el-dialog v-model="priVisible" :title="priEdit ? '修改授权' : '新增授权'" width="460px">
      <el-form label-width="90px">
        <el-form-item label="路径">
          <el-input v-model="priForm.priPath" placeholder="/trunk" :disabled="priEdit" />
        </el-form-item>
        <el-form-item label="对象类型">
          <el-select v-model="priForm.objectType" :disabled="priEdit" style="width: 100%">
            <el-option label="用户" value="user" />
            <el-option label="分组" value="group" />
            <el-option label="别名" value="aliase" />
            <el-option label="所有用户" value="all" />
            <el-option label="已认证用户" value="authenticated" />
            <el-option label="匿名用户" value="anonymous" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="['user', 'group', 'aliase'].includes(priForm.objectType)" label="对象名">
          <el-select
            v-if="priForm.objectType === 'user'"
            v-model="priForm.objectName"
            filterable
            style="width: 100%"
          >
            <el-option v-for="u in allUsers" :key="u" :label="u" :value="u" />
          </el-select>
          <el-select
            v-else-if="priForm.objectType === 'group'"
            v-model="priForm.objectName"
            filterable
            style="width: 100%"
          >
            <el-option v-for="g in allGroups" :key="g" :label="g" :value="g" />
          </el-select>
          <el-input v-else v-model="priForm.objectName" />
        </el-form-item>
        <el-form-item label="权限">
          <el-radio-group v-model="priForm.perm">
            <el-radio value="rw">读写 (rw)</el-radio>
            <el-radio value="r">只读 (r)</el-radio>
            <el-radio value="w">只写 (w)</el-radio>
            <el-radio value="">禁止 (空)</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="!['all', 'authenticated', 'anonymous'].includes(priForm.objectType)" label="取反">
          <el-switch v-model="priForm.invert" />
          <span class="hint">
            开启后写为 <span class="text-mono">~{{ '{' }}{{ priForm.objectName || '对象' }}{{ '}' }}</span>，表示"除该对象以外的所有人"
          </span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="priVisible = false">取消</el-button>
        <el-button type="primary" @click="submitPri">确定</el-button>
      </template>
    </el-dialog>

    <!-- 钩子编辑 -->
    <el-dialog v-model="hookVisible" :title="`编辑钩子 - ${hookForm.hookName}`" width="720px">
      <el-input v-model="hookForm.content" type="textarea" :rows="14" class="text-mono" />
      <template #footer>
        <el-button @click="hookVisible = false">取消</el-button>
        <el-button type="primary" @click="submitHook">保存</el-button>
      </template>
    </el-dialog>

    <!-- 目录树右键菜单 -->
    <div
      v-if="ctxMenu.visible"
      class="ctx-menu"
      :style="{ left: ctxMenu.x + 'px', top: ctxMenu.y + 'px' }"
      @mouseleave="ctxMenu.visible = false"
    >
      <div class="ctx-item" @click="ctxNewFolder">在此新建子目录</div>
      <div class="ctx-item" @click="ctxCopyPath">复制路径</div>
      <div class="ctx-item" @click="ctxSetPriPath">设为权限编辑路径</div>
      <div class="ctx-item" @click="ctxViewCon">查看目录条目</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CircleCheck, Document, DocumentCopy, Folder, Plus, Refresh, Search, Upload } from '@element-plus/icons-vue'
import {
  apiBackupDel,
  apiBackupDump,
  apiBackupList,
  apiBackupLoad,
  apiCheckAuthz,
  apiHookDel,
  apiHookList,
  apiHookRecommend,
  apiHookUpdate,
  apiPriCreate,
  apiPriDelete,
  apiPriList,
  apiPriUpdate,
  apiRepCheckout,
  apiRepCon,
  apiRepCreate,
  apiRepCreateFolder,
  apiRepDel,
  apiRepDetail,
  apiRepHotcopy,
  apiRepList,
  apiRepSetUUID,
  apiRepSyncRev,
  apiRepSyncSize,
  apiRepTree,
  apiRepTree2,
  apiRepUpdName,
  apiRepUpdNote,
  apiRepVerify,
  apiUploadAbort,
  apiUploadInfo,
  apiUploadInit,
  apiUploadMerge,
  apiUserAll,
  apiGroupAll,
  type BackupFile,
  type SvnRep,
  type TreeNode
} from '@/api'
import request, { getToken } from '@/api/request'

const list = ref<SvnRep[]>([])
const total = ref(0)
const loading = ref(false)
const query = reactive({ page: 1, pageSize: 20, keyword: '' })

const current = ref<any>({})
const createVisible = ref(false)
const createForm = reactive({ repName: '', repNote: '', initStruct: '' })

const renameVisible = ref(false)
const newName = ref('')
const noteVisible = ref(false)
const newNote = ref('')

const detailVisible = ref(false)
const activeTab = ref('info')
const detail = ref<any>({})
const checkout = reactive({ svn: '', http: '' })

const treeRef = ref<any>()
const tree = ref<TreeNode[]>([])
const treeLoading = ref(false)
const newFolder = ref('')
const treeUser = ref('')

const ctxMenu = reactive({ visible: false, x: 0, y: 0, node: null as any })

const pri = ref<{ current: any[]; inherit: any[] }>({ current: [], inherit: [] })
const priPath = ref('/')
const priVisible = ref(false)
const priEdit = ref(false)
const priForm = reactive({ priPath: '/', objectType: 'user', objectName: '', perm: 'rw', invert: false })
const allUsers = ref<string[]>([])
const allGroups = ref<string[]>([])

const backups = ref<BackupFile[]>([])
const incremental = ref(false)
const revRange = ref('')

/* 分片上传 */
const fileInput = ref<HTMLInputElement>()
const uploading = ref(false)
const uploadStage = ref('')
const uploadPercent = ref(0)
const uploadDone = ref(0)
const uploadTotal = ref(0)
const uploadDoneParts = ref(0)
const uploadTotalParts = ref(0)
const uploadAborted = ref(false)
let uploadId = ''

const hooks = ref<any[]>([])
const recommends = ref<any[]>([])
const hookVisible = ref(false)
const hookForm = reactive({ hookName: '', content: '' })

const fmtSize = (n: number) => {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(1)} ${units[i]}`
}

const typeText = (t: string) =>
  ({ user: '用户', group: '分组', aliase: '别名', all: '所有用户', authenticated: '已认证', anonymous: '匿名' }[
    t
  ] || t)

const load = async () => {
  loading.value = true
  try {
    const res = await apiRepList(query)
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  createForm.repName = ''
  createForm.repNote = ''
  createForm.initStruct = ''
  createVisible.value = true
}

const submitCreate = async () => {
  if (!createForm.repName) {
    ElMessage.warning('请输入仓库名')
    return
  }
  await apiRepCreate({ ...createForm })
  ElMessage.success('创建成功')
  createVisible.value = false
  load()
}

const openRename = (row: any) => {
  current.value = row
  newName.value = row.repName
  renameVisible.value = true
}

const submitRename = async () => {
  await apiRepUpdName({ oldName: current.value.repName, newName: newName.value })
  ElMessage.success('修改成功')
  renameVisible.value = false
  load()
}

const openNote = (row: any) => {
  current.value = row
  newNote.value = row.repNote || ''
  noteVisible.value = true
}

const submitNote = async () => {
  await apiRepUpdNote({ repName: current.value.repName, repNote: newNote.value })
  ElMessage.success('修改成功')
  noteVisible.value = false
  load()
}

const remove = async (row: any) => {
  await ElMessageBox.confirm(
    `确认删除仓库 [${row.repName}]？仓库数据与其全部授权将被移除，且不可恢复。`,
    '危险操作',
    { type: 'warning' }
  )
  await apiRepDel({ repName: row.repName })
  ElMessage.success('删除成功')
  load()
}

const checkAuthz = async () => {
  await apiCheckAuthz()
  ElMessage.success('authz 文件校验通过')
}

/* ---------- 详情 ---------- */

const openDetail = async (row: any) => {
  current.value = row
  activeTab.value = 'info'
  treeUser.value = ''
  detailVisible.value = true
  await Promise.all([loadDetail(), reloadTree(), loadPri(), loadBackup(), loadHooks(), loadDicts(), loadCheckout()])
}

const loadDetail = async () => {
  detail.value = await apiRepDetail(current.value.repName)
}

const loadCheckout = async () => {
  const res = await apiRepCheckout(current.value.repName)
  checkout.svn = res.svn
  checkout.http = res.http
}

const syncSize = async () => {
  const res = await apiRepSyncSize(current.value.repName)
  ElMessage.success(`体积：${fmtSize(res.size)}`)
  await loadDetail()
  load()
}

const syncRev = async () => {
  const res = await apiRepSyncRev(current.value.repName)
  ElMessage.success(`最新版本：${res.rev}`)
  await loadDetail()
  load()
}

const resetUUID = async () => {
  await ElMessageBox.confirm('重设 UUID 会导致已检出的工作副本失效，确认继续？', '提示', {
    type: 'warning'
  })
  const res = await apiRepSetUUID({ repName: current.value.repName })
  ElMessage.success(`新 UUID：${res.uuid}`)
  await loadDetail()
}

/* ---------- 目录树（懒加载） ---------- */

const fetchNodes = async (path: string): Promise<TreeNode[]> => {
  const repName = current.value.repName
  if (!repName) return []
  if (treeUser.value) {
    // SVN 用户视角：走真实鉴权，只返回该用户可见内容
    return await apiRepTree2(repName, path, treeUser.value, path === '/')
  }
  return await apiRepTree(repName, path)
}

const loadNode = async (node: any, resolve: any) => {
  if (node.level === 0) {
    const root = await fetchNodes('/')
    resolve(markLeaf(root))
    return
  }
  const children = await fetchNodes(node.data.path)
  resolve(markLeaf(children))
}

const markLeaf = (nodes: TreeNode[]): any[] =>
  (nodes || []).map((n) => ({ ...n, leaf: n.type !== 'dir' }))

const reloadTree = async () => {
  treeLoading.value = true
  try {
    tree.value = markLeaf(await fetchNodes('/'))
  } catch {
    tree.value = []
  } finally {
    treeLoading.value = false
  }
}

const createFolder = async () => {
  if (!newFolder.value) {
    ElMessage.warning('请输入目录路径')
    return
  }
  await apiRepCreateFolder({ repName: current.value.repName, path: newFolder.value })
  ElMessage.success('目录已创建')
  newFolder.value = ''
  await reloadTree()
}

const onNodeClick = async (node: TreeNode) => {
  if (node.type !== 'dir') return
  priPath.value = node.path
  await loadPri()
}

/* 右键菜单 */

const onContextMenu = (e: MouseEvent, data: any) => {
  ctxMenu.visible = true
  ctxMenu.x = e.clientX
  ctxMenu.y = e.clientY
  ctxMenu.node = data
}

const closeCtx = () => {
  ctxMenu.visible = false
}

const ctxNewFolder = async () => {
  const parent = ctxMenu.node?.path || '/'
  closeCtx()
  try {
    const { value } = await ElMessageBox.prompt(
      `将在 [${parent}] 下创建子目录，请输入目录名`,
      '新建子目录',
      { inputPlaceholder: '例如 src' }
    )
    if (!value) return
    await apiRepCreateFolder({
      repName: current.value.repName,
      path: `${parent.replace(/\/$/, '')}/${value.replace(/^\/+/, '')}`
    })
    ElMessage.success('目录已创建')
    await reloadTree()
  } catch {
    /* 取消 */
  }
}

const ctxCopyPath = () => {
  closeCtx()
  copyText(ctxMenu.node?.path || '/')
}

const ctxSetPriPath = async () => {
  const p = ctxMenu.node?.path || '/'
  closeCtx()
  priPath.value = p
  activeTab.value = 'pri'
  await loadPri()
  ElMessage.success(`权限编辑路径已切换为 ${p}`)
}

const ctxViewCon = async () => {
  const node = ctxMenu.node
  closeCtx()
  if (!node || node.type !== 'dir') {
    ElMessage.warning('请选择目录节点')
    return
  }
  const rows = await apiRepCon(current.value.repName, node.path)
  const html = (rows || [])
    .map(
      (r: any) =>
        `<div style="font-family:Consolas,Menlo,monospace;font-size:12px;line-height:1.8">
           ${r.type === 'dir' ? '[DIR]' : '[FILE]'} ${escapeHtml(r.name)} · r${r.rev} · ${escapeHtml(r.author || '')}
         </div>`
    )
    .join('') || '<div>（空目录）</div>'
  ElMessageBox.alert(
    `<div style="max-height:50vh;overflow:auto;text-align:left">${html}</div>`,
    `目录条目 - ${node.path}`,
    { dangerouslyUseHTMLString: true, customStyle: { width: '60vw' } }
  )
}

const escapeHtml = (s: string) =>
  String(s || '').replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c] as string))

const copyText = (text: string) => {
  navigator.clipboard?.writeText(text || '').then(
    () => ElMessage.success('已复制到剪贴板'),
    () => ElMessage.warning('复制失败')
  )
}

/* ---------- 仓库检查 / 热拷贝 ---------- */

const doVerify = async () => {
  try {
    await ElMessageBox.confirm('将对仓库执行 svnadmin verify，大仓库耗时较长，确认继续？', '提示', {
      type: 'warning'
    })
  } catch {
    return
  }
  const task = await apiRepVerify(current.value.repName)
  ElMessage.success(`检查任务已提交：${task.taskName}，可在【运行任务】查看日志`)
}

const doHotcopy = async () => {
  try {
    await ElMessageBox.confirm(
      '将执行 svnadmin hotcopy 生成仓库物理副本（原版为未实现的空壳功能），确认继续？',
      '提示',
      { type: 'warning' }
    )
  } catch {
    return
  }
  const task = await apiRepHotcopy(current.value.repName, incremental.value)
  ElMessage.success(`热拷贝任务已提交：${task.taskName}`)
  setTimeout(loadBackup, 2500)
}

/* ---------- 权限 ---------- */

const loadDicts = async () => {
  const [users, groups] = await Promise.all([apiUserAll(), apiGroupAll()])
  allUsers.value = users.map((u: any) => u.svnUserName)
  allGroups.value = groups
}

const loadPri = async () => {
  pri.value = await apiPriList(current.value.repName, priPath.value || '/')
}

const openPriCreate = () => {
  priEdit.value = false
  Object.assign(priForm, {
    priPath: priPath.value || '/',
    objectType: 'user',
    objectName: '',
    perm: 'rw',
    invert: false
  })
  priVisible.value = true
}

const openPriEdit = (row: any) => {
  priEdit.value = true
  Object.assign(priForm, {
    priPath: row.priPath,
    objectType: row.objectType,
    objectName: row.objectName,
    perm: row.perm,
    invert: !!row.invert
  })
  priVisible.value = true
}

const submitPri = async () => {
  if (priEdit.value) {
    await apiPriUpdate({
      repName: current.value.repName,
      priPath: priForm.priPath,
      objectName: priForm.objectName,
      perm: priForm.perm,
      invert: priForm.invert
    })
  } else {
    await apiPriCreate({ repName: current.value.repName, ...priForm })
  }
  ElMessage.success('保存成功')
  priVisible.value = false
  await loadPri()
}

const delPri = async (row: any) => {
  await ElMessageBox.confirm(`确认删除 [${row.objectName}] 在 [${row.priPath}] 的权限？`, '提示', {
    type: 'warning'
  })
  await apiPriDelete({
    repName: current.value.repName,
    priPath: row.priPath,
    objectName: row.objectName
  })
  ElMessage.success('删除成功')
  await loadPri()
}

/* ---------- 备份 ---------- */

const loadBackup = async () => {
  backups.value = await apiBackupList(current.value.repName)
}

const doDump = async () => {
  const task = await apiBackupDump({
    repName: current.value.repName,
    revRange: revRange.value,
    incremental: incremental.value
  })
  ElMessage.success(`备份任务已提交：${task.taskName}`)
  setTimeout(loadBackup, 2500)
}

const doLoad = async (row: any) => {
  await ElMessageBox.confirm(
    `确认将 [${row.name}] 导入到仓库 [${current.value.repName}]？`,
    '提示',
    { type: 'warning' }
  )
  const task = await apiBackupLoad({ repName: current.value.repName, fileName: row.name })
  ElMessage.success(`导入任务已提交：${task.taskName}`)
}

const doDownload = (row: any) => {
  window.open(
    `/api/backup/download?name=${encodeURIComponent(row.name)}&token=${encodeURIComponent(getToken())}`
  )
}

const delBackup = async (row: any) => {
  await ElMessageBox.confirm(`确认删除备份 [${row.name}]？`, '提示', { type: 'warning' })
  await apiBackupDel({ name: row.name })
  ElMessage.success('删除成功')
  await loadBackup()
}

/* ---------- 分片上传 ---------- */

const pickFile = () => fileInput.value?.click()

const onFilePicked = async (e: Event) => {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  await startUpload(file)
}

const startUpload = async (file: File) => {
  if (!/\.dump$/i.test(file.name)) {
    ElMessage.warning('仅支持上传 .dump 备份文件')
    return
  }
  const info = await apiUploadInfo()
  const chunkSize = info.chunkSize || 8 << 20
  const total = Math.max(1, Math.ceil(file.size / chunkSize))
  uploadAborted.value = false
  uploading.value = true
  uploadStage.value = '初始化…'
  uploadPercent.value = 0
  uploadDone.value = 0
  uploadTotal.value = file.size
  uploadDoneParts.value = 0
  uploadTotalParts.value = total
  try {
    const s = await apiUploadInit({ fileName: file.name, md5: '', numBlobTotal: total })
    uploadId = s.uploadId
    for (let i = 0; i < total; i++) {
      if (uploadAborted.value) return
      const blob = file.slice(i * chunkSize, Math.min((i + 1) * chunkSize, file.size))
      const fd = new FormData()
      fd.append('uploadId', uploadId)
      fd.append('index', String(i))
      fd.append('file', blob)
      await request.post('/backup/uploadPart', fd, {
        headers: { 'Content-Type': 'multipart/form-data' },
        timeout: 0
      })
      uploadDone.value += blob.size
      uploadDoneParts.value = i + 1
      uploadStage.value = '上传中…'
      uploadPercent.value = Math.min(99, Math.floor((uploadDone.value / file.size) * 100))
    }
    if (uploadAborted.value) return
    uploadStage.value = '合并中…'
    const res = await apiUploadMerge(uploadId)
    uploadPercent.value = 100
    uploadStage.value = '完成'
    ElMessage.success(`备份文件已上传：${res.name}`)
    await loadBackup()
  } catch (err: any) {
    if (!uploadAborted.value) ElMessage.error(err?.message || '上传失败')
  } finally {
    if (uploadPercent.value >= 100 || uploadAborted.value) uploading.value = false
  }
}

const abortUpload = async () => {
  uploadAborted.value = true
  if (uploadId) {
    try {
      await apiUploadAbort(uploadId)
    } catch {
      /* 忽略 */
    }
  }
  uploading.value = false
  ElMessage.info('已取消上传')
}

/* ---------- 钩子 ---------- */

const loadHooks = async () => {
  hooks.value = await apiHookList(current.value.repName)
  recommends.value = await apiHookRecommend().catch(() => [])
}

const viewRecommend = (row: any) => {
  ElMessageBox.alert(
    `<pre style="max-height:50vh;overflow:auto;text-align:left;font-size:12px">${escapeHtml(row.content || '')}</pre>`,
    `推荐钩子 - ${row.name}`,
    { dangerouslyUseHTMLString: true, customStyle: { width: '70vw' } }
  )
}

const applyRecommend = async (row: any) => {
  if (!row.type) {
    ElMessage.warning('该推荐钩子未声明对应钩子类型')
    return
  }
  try {
    await ElMessageBox.confirm(
      `将 [${row.name}] 的内容写入 [${row.type}] 钩子，覆盖原有内容，确认继续？`,
      '提示',
      { type: 'warning' }
    )
  } catch {
    return
  }
  await apiHookUpdate({
    repName: current.value.repName,
    hookName: row.type,
    content: row.content || ''
  })
  ElMessage.success(`已套用到 ${row.type}`)
  await loadHooks()
}

const openHook = (row: any) => {
  hookForm.hookName = row.name
  hookForm.content = row.content || ''
  hookVisible.value = true
}

const submitHook = async () => {
  await apiHookUpdate({
    repName: current.value.repName,
    hookName: hookForm.hookName,
    content: hookForm.content
  })
  ElMessage.success('钩子已保存')
  hookVisible.value = false
  await loadHooks()
}

const delHook = async (row: any) => {
  await apiHookDel({ repName: current.value.repName, hookName: row.name })
  ElMessage.success('钩子已清空')
  await loadHooks()
}

onMounted(load)
</script>

<style scoped>
.tree-node {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
}
.hint {
  margin-left: 8px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.text-mono {
  font-family: Consolas, Menlo, monospace;
}
.upload-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.upload-bar :deep(.el-progress) {
  flex: 1;
}
.upload-text {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}
.ctx-menu {
  position: fixed;
  z-index: 3000;
  min-width: 170px;
  padding: 4px 0;
  background: var(--el-bg-color-overlay);
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
  box-shadow: var(--el-box-shadow-light);
}
.ctx-item {
  padding: 7px 14px;
  font-size: 13px;
  cursor: pointer;
  color: var(--el-text-color-regular);
}
.ctx-item:hover {
  background: var(--el-fill-color-light);
  color: var(--el-color-primary);
}
</style>
