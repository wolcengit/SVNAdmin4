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
      <h2>运行任务</h2>
      <el-space>
        <el-checkbox v-model="autoRefresh" style="margin-right: 8px">自动刷新(2s)</el-checkbox>
        <el-button :icon="Refresh" @click="loadAll">刷新</el-button>
      </el-space>
    </div>

    <el-tabs v-model="tab">
      <!-- ---------------- 当前任务 ---------------- -->
      <el-tab-pane name="run">
        <template #label>
          <span>当前任务<el-badge v-if="runInfo.queueCount" :value="runInfo.queueCount" class="tab-badge" /></span>
        </template>

        <el-empty v-if="!runInfo.running" description="当前没有正在执行的任务" />

        <template v-else>
          <el-descriptions :column="3" border size="small" style="margin-bottom: 12px">
            <el-descriptions-item label="任务名称">{{ runInfo.task?.taskName }}</el-descriptions-item>
            <el-descriptions-item label="任务类型">
              {{ typeText(runInfo.task?.taskType) }}
            </el-descriptions-item>
            <el-descriptions-item label="状态">
              <el-tag :type="statusType(runInfo.task?.taskStatus)" size="small">
                {{ statusText(runInfo.task?.taskStatus) }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="任务 ID">{{ runInfo.task?.taskId }}</el-descriptions-item>
            <el-descriptions-item label="创建时间">{{ runInfo.task?.taskCreateTime }}</el-descriptions-item>
            <el-descriptions-item label="更新时间">{{ runInfo.task?.taskUpdateTime }}</el-descriptions-item>
            <el-descriptions-item label="执行内容" :span="3">
              <span class="mono">{{ runInfo.task?.taskCmd || runInfo.task?.taskMessage || '-' }}</span>
            </el-descriptions-item>
          </el-descriptions>

          <div class="log-head">
            <span>实时日志</span>
            <el-space>
              <el-tag v-if="runInfo.logTrimed" type="warning" size="small">日志过大已截断</el-tag>
              <el-button size="small" :icon="DocumentCopy" @click="copyLog(runInfo.log)">复制</el-button>
              <el-button size="small" type="danger" :icon="VideoPause" @click="stopTask(runInfo.task?.taskId)">
                中断任务
              </el-button>
            </el-space>
          </div>
          <pre ref="logBox" class="log-box">{{ runInfo.log || '（暂无日志）' }}</pre>
        </template>
      </el-tab-pane>

      <!-- ---------------- 排队任务 ---------------- -->
      <el-tab-pane name="queue">
        <template #label>
          <span>
            排队任务
            <el-badge v-if="runInfo.queue?.length" :value="runInfo.queue.length" class="tab-badge" />
          </span>
        </template>

        <el-table v-loading="loading" :data="runInfo.queue" border size="small">
          <el-table-column type="index" label="#" width="55" />
          <el-table-column prop="taskId" label="ID" width="70" />
          <el-table-column prop="taskName" label="任务" min-width="200" />
          <el-table-column label="类型" width="110">
            <template #default="{ row }">{{ typeText(row.taskType) }}</template>
          </el-table-column>
          <el-table-column prop="taskCreateTime" label="入队时间" width="170" />
          <el-table-column label="操作" width="110" fixed="right">
            <template #default="{ row }">
              <el-button link type="danger" size="small" @click="stopTask(row.taskId)">取消</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-if="!runInfo.queue?.length" description="队列为空" />
      </el-tab-pane>

      <!-- ---------------- 历史任务 ---------------- -->
      <el-tab-pane name="history" label="历史任务">
        <div class="filter-bar">
          <el-select
            v-model="query.status"
            multiple
            collapse-tags
            collapse-tags-tooltip
            clearable
            placeholder="状态筛选"
            style="width: 240px"
          >
            <el-option v-for="(v, k) in statusMap" :key="k" :label="v" :value="Number(k)" />
          </el-select>
          <el-input v-model="query.keyword" clearable placeholder="关键字" style="width: 200px" @keyup.enter="loadHistory" />
          <el-button :icon="Search" @click="loadHistory">查询</el-button>
          <el-button type="danger" plain :icon="Delete" @click="clearHistory">清空历史</el-button>
        </div>

        <el-table v-loading="loading" :data="list" border size="small">
          <el-table-column prop="taskId" label="ID" width="70" />
          <el-table-column prop="taskName" label="任务" min-width="180" show-overflow-tooltip />
          <el-table-column label="类型" width="100">
            <template #default="{ row }">{{ typeText(row.taskType) }}</template>
          </el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="statusType(row.taskStatus)" size="small">{{ statusText(row.taskStatus) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="taskCmd" label="命令" min-width="200" show-overflow-tooltip />
          <el-table-column prop="taskMessage" label="结果" min-width="160" show-overflow-tooltip />
          <el-table-column prop="taskCreateTime" label="创建时间" width="165" />
          <el-table-column prop="taskUpdateTime" label="更新时间" width="165" />
          <el-table-column label="操作" width="150" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" size="small" @click="viewLog(row)">日志</el-button>
              <el-button link type="danger" size="small" @click="delTask(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-pagination
          style="margin-top: 12px; justify-content: flex-end"
          layout="total, sizes, prev, pager, next"
          :total="total"
          v-model:current-page="query.page"
          v-model:page-size="query.pageSize"
          :page-sizes="[20, 50, 100]"
          @change="loadHistory"
        />
      </el-tab-pane>
    </el-tabs>

    <!-- 日志弹窗 -->
    <el-dialog v-model="logVisible" :title="logTitle" width="70%" top="6vh">
      <pre class="log-box dialog-log">{{ logContent || '（暂无日志）' }}</pre>
      <template #footer>
        <el-button size="small" @click="copyLog(logContent)">复制</el-button>
        <el-button size="small" type="primary" @click="logVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { Delete, DocumentCopy, Refresh, Search, VideoPause } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { apiTaskClear, apiTaskDel, apiTaskList, apiTaskLog, apiTaskRun, apiTaskStop, apiTaskTypes } from '@/api'

const tab = ref('run')
const loading = ref(false)
const autoRefresh = ref(true)

const runInfo = reactive<any>({ running: false, task: null, log: '', logTrimed: false, queue: [], queueCount: 0 })
const list = ref<any[]>([])
const total = ref(0)
const query = reactive<any>({ page: 1, pageSize: 20, status: [] as number[], keyword: '' })

const typeMap = ref<Record<string, string>>({})
const statusMap = ref<any>({ 0: '等待中', 1: '运行中', 2: '已完成', 3: '失败', 4: '已取消' })

const logVisible = ref(false)
const logTitle = ref('任务日志')
const logContent = ref('')
const logBox = ref<HTMLElement>()

const typeText = (t: any) => typeMap.value[String(t)] || t || '-'
const statusText = (s: number) => statusMap.value[String(s)] || `状态 ${s}`
const statusType = (s: number): any =>
  ({ 0: 'info', 1: 'primary', 2: 'success', 3: 'danger', 4: 'warning' }[s] || 'info')

/* -------------------- 数据加载 -------------------- */

const loadRun = async () => {
  const res = await apiTaskRun()
  runInfo.running = !!res.running
  runInfo.task = res.task || null
  runInfo.log = res.log || ''
  runInfo.logTrimed = !!res.logTrimed
  runInfo.queue = res.queue || []
  runInfo.queueCount = res.queueCount || res.queue?.length || 0
  if (tab.value === 'run') scrollLog()
}

const loadHistory = async () => {
  loading.value = true
  try {
    const params: any = { page: query.page, pageSize: query.pageSize, keyword: query.keyword }
    if (query.status?.length) params.status = query.status.join(',')
    const res = await apiTaskList(params)
    list.value = res.list || []
    total.value = res.total || 0
  } finally {
    loading.value = false
  }
}

const loadDict = async () => {
  try {
    const res = await apiTaskTypes()
    const m: Record<string, string> = {}
    ;(res.types || []).forEach((t: any) => {
      if (typeof t === 'object' && t !== null) {
        const k = String(t.value ?? t.id ?? t.type ?? '')
        m[k] = String(t.label ?? t.name ?? t.title ?? k)
      } else {
        m[String(t)] = String(t)
      }
    })
    typeMap.value = m
    if (res.status && Object.keys(res.status).length) statusMap.value = res.status
  } catch {
    /* 字典接口失败时沿用内置映射 */
  }
}

const loadAll = async () => {
  await loadRun()
  if (tab.value === 'history') await loadHistory()
}

/* -------------------- 操作 -------------------- */

const stopTask = async (id?: number) => {
  if (!id) return
  try {
    await ElMessageBox.confirm('确定要中断/取消该任务吗？', '提示', { type: 'warning' })
  } catch {
    return
  }
  await apiTaskStop(id)
  ElMessage.success('已发送中断信号')
  loadAll()
}

const delTask = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确定删除任务「${row.taskName}」的记录吗？`, '提示', { type: 'warning' })
  } catch {
    return
  }
  await apiTaskDel(row.taskId)
  ElMessage.success('已删除')
  loadHistory()
}

const clearHistory = async () => {
  try {
    await ElMessageBox.confirm('确定清空全部历史任务记录吗？该操作不可恢复。', '提示', { type: 'warning' })
  } catch {
    return
  }
  await apiTaskClear()
  ElMessage.success('已清空')
  loadHistory()
}

const viewLog = async (row: any) => {
  logTitle.value = `任务日志 - ${row.taskName}`
  logContent.value = ''
  logVisible.value = true
  const res = await apiTaskLog(row.taskId)
  logContent.value = res.log || '（暂无日志）'
}

const copyLog = (text: string) => {
  navigator.clipboard?.writeText(text || '').then(
    () => ElMessage.success('已复制到剪贴板'),
    () => ElMessage.warning('复制失败')
  )
}

const scrollLog = () => {
  requestAnimationFrame(() => {
    const el = logBox.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

/* -------------------- 生命周期 -------------------- */

let timer: number | undefined
const startTimer = () => {
  stopTimer()
  timer = window.setInterval(() => {
    if (!autoRefresh.value) return
    loadRun().catch(() => {})
  }, 2000)
}
const stopTimer = () => {
  if (timer) window.clearInterval(timer)
  timer = undefined
}

watch(autoRefresh, (v) => (v ? startTimer() : stopTimer()))
watch(tab, (v) => {
  if (v === 'history' && !list.value.length) loadHistory()
  if (v === 'run') scrollLog()
})

onMounted(async () => {
  await loadDict()
  await loadAll()
  startTimer()
})
onBeforeUnmount(stopTimer)
</script>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.page-header h2 {
  margin: 0;
  font-size: 18px;
}
.filter-bar {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.log-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
  font-weight: 600;
}
.log-box {
  margin: 0;
  padding: 10px 12px;
  height: 46vh;
  overflow: auto;
  background: var(--el-fill-color-light);
  color: var(--el-text-color-primary);
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
  font-family: Consolas, Menlo, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}
.dialog-log {
  height: 56vh;
}
.mono {
  font-family: Consolas, Menlo, monospace;
  word-break: break-all;
}
.tab-badge {
  margin-left: 6px;
}
:deep(.el-badge__content) {
  transform: translateY(-8px) scale(0.85);
}
</style>
