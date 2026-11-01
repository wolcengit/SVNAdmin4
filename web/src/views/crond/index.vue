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
      <h2>任务计划</h2>
      <div class="toolbar" style="margin: 0">
        <el-tag type="success" size="small" effect="plain">
          调度器：内置 cron（已加载 {{ schedStatus.jobCount || 0 }} 个任务）
        </el-tag>
        <el-button type="primary" :icon="Plus" @click="openCreate">新建任务</el-button>
        <el-button :icon="Refresh" @click="load">刷新</el-button>
      </div>
    </div>

    <el-alert
      v-if="schedStatus.nextRun"
      :closable="false"
      type="info"
      show-icon
      style="margin-bottom: 12px"
      :title="`下一次触发时间：${schedStatus.nextRun}　（${schedStatus.scheduler || ''}）`"
    />

    <el-table v-loading="loading" :data="list" border size="small">
      <el-table-column prop="crondId" label="ID" width="60" />
      <el-table-column prop="taskName" label="任务名" min-width="150" show-overflow-tooltip />
      <el-table-column label="类型" width="160">
        <template #default="{ row }">
          <el-tag size="small" effect="plain">{{ typeText(row.taskType) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="cycleDesc" label="周期" min-width="160" show-overflow-tooltip />
      <el-table-column label="目标仓库" min-width="140">
        <template #default="{ row }">
          <span v-if="!typeNeedRep(row.taskType)">-</span>
          <span v-else-if="isAllRep(row)">全部仓库</span>
          <span v-else>{{ repNames(row).join('、') }}</span>
        </template>
      </el-table-column>
      <el-table-column label="保留" width="70" align="center">
        <template #default="{ row }">{{ typeNeedRep(row.taskType) ? row.saveCount : '-' }}</template>
      </el-table-column>
      <el-table-column label="通知" width="90">
        <template #default="{ row }">{{ noticeText(row.notice) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-switch :model-value="row.status === 1" @change="(v: any) => toggle(row, v)" />
        </template>
      </el-table-column>
      <el-table-column prop="lastExecTime" label="上次执行" width="165" />
      <el-table-column label="操作" width="230" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="trigger(row)">执行</el-button>
          <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button link type="info" @click="viewLog(row)">日志</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 新建 / 编辑 -->
    <el-dialog v-model="visible" :title="editing ? '编辑任务' : '新建任务'" width="640px">
      <el-form label-width="110px">
        <el-form-item label="任务名" required>
          <el-input v-model="form.taskName" placeholder="例如：每晚全量备份" />
        </el-form-item>

        <el-form-item label="任务类型">
          <el-select v-model="form.taskType" style="width: 100%">
            <el-option v-for="t in dict.types" :key="t.value" :label="t.label" :value="t.value" />
          </el-select>
        </el-form-item>

        <el-form-item v-if="typeNeedRep(form.taskType)" label="目标仓库">
          <el-select
            v-model="form.repNames"
            multiple
            filterable
            collapse-tags
            collapse-tags-tooltip
            style="width: 100%"
            placeholder="不选择或选择【全部仓库】则作用于所有仓库"
          >
            <el-option label="全部仓库" value="-1" />
            <el-option v-for="r in reps" :key="r.repName" :label="r.repName" :value="r.repName" />
          </el-select>
        </el-form-item>

        <el-form-item label="执行周期">
          <el-select v-model="form.cycleType" style="width: 100%">
            <el-option v-for="c in dict.cycleTypes" :key="c.value" :label="c.label" :value="c.value" />
          </el-select>
        </el-form-item>

        <el-form-item v-if="form.cycleType === 'week'" label="星期">
          <el-select v-model="form.week" style="width: 100%">
            <el-option v-for="(d, i) in weekNames" :key="i" :label="d" :value="i" />
          </el-select>
        </el-form-item>

        <el-form-item v-if="['month', 'day_n'].includes(form.cycleType)" label="日期/间隔">
          <el-input-number
            v-model="form.day"
            :min="1"
            :max="31"
            :controls="false"
          />
          <span class="hint">{{ form.cycleType === 'day_n' ? '每隔 N 天' : '每月第 N 日' }}</span>
        </el-form-item>

        <el-form-item
          v-if="['day', 'week', 'month', 'day_n'].includes(form.cycleType)"
          label="小时"
        >
          <el-input-number v-model="form.hour" :min="0" :max="23" :controls="false" />
          <span class="hint">0-23</span>
        </el-form-item>

        <el-form-item v-if="form.cycleType === 'hour_n'" label="间隔小时">
          <el-input-number v-model="form.hour" :min="1" :max="23" :controls="false" />
        </el-form-item>

        <el-form-item v-if="form.cycleType === 'minute_n'" label="间隔分钟">
          <el-input-number v-model="form.minute" :min="1" :max="59" :controls="false" />
        </el-form-item>

        <el-form-item
          v-if="['hour', 'hour_n', 'day', 'week', 'month', 'day_n'].includes(form.cycleType)"
          label="分钟"
        >
          <el-input-number v-model="form.minute" :min="0" :max="59" :controls="false" />
          <span class="hint">0-59</span>
        </el-form-item>

        <el-form-item v-if="form.taskType === 6" label="脚本内容">
          <el-input
            v-model="form.shell"
            type="textarea"
            :rows="6"
            class="text-mono"
            placeholder="#!/bin/bash&#10;echo hello"
          />
        </el-form-item>

        <el-form-item v-if="typeNeedRep(form.taskType)" label="保留份数">
          <el-input-number v-model="form.saveCount" :min="1" :max="999" />
          <span class="hint">超出份数自动清理最旧的备份文件</span>
        </el-form-item>

        <el-form-item label="消息通知">
          <el-select v-model="form.notice" style="width: 100%">
            <el-option v-for="n in dict.notices" :key="n.value" :label="n.label" :value="n.value" />
          </el-select>
        </el-form-item>

        <el-form-item label="启用">
          <el-switch v-model="form.statusOn" />
        </el-form-item>

        <el-form-item label="周期说明">
          <el-input v-model="form.cycleDesc" placeholder="留空则自动生成" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" @click="submit">确定</el-button>
      </template>
    </el-dialog>

    <!-- 执行日志 -->
    <el-dialog v-model="logVisible" :title="`执行日志 - ${logTitle}`" width="70%" top="6vh">
      <pre class="log-box">{{ logContent || '（暂无日志）' }}</pre>
      <template #footer>
        <el-button size="small" @click="copyLog">复制</el-button>
        <el-button size="small" type="primary" @click="logVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh } from '@element-plus/icons-vue'
import {
  apiCrondCreate,
  apiCrondDel,
  apiCrondDict,
  apiCrondList,
  apiCrondLog,
  apiCrondTrigger,
  apiCrondUpdate,
  apiRepAll
} from '@/api'

const list = ref<any[]>([])
const reps = ref<any[]>([])
const loading = ref(false)
const visible = ref(false)
const editing = ref(false)
const weekNames = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

const dict = reactive<any>({ types: [], cycleTypes: [], notices: [] })
const schedStatus = ref<any>({})

const logVisible = ref(false)
const logTitle = ref('')
const logContent = ref('')

const defaultForm = () => ({
  crondId: 0,
  taskName: '',
  taskType: 1,
  cycleType: 'day',
  cycleDesc: '',
  repNames: [] as string[],
  week: 1,
  day: 1,
  hour: 2,
  minute: 0,
  saveCount: 10,
  notice: 0,
  shell: '',
  statusOn: true
})

const form = reactive(defaultForm())

const typeNeedRep = (t: number) => !!dict.types.find((x: any) => x.value === t)?.needRep
const typeText = (t: number) => dict.types.find((x: any) => x.value === t)?.label || `类型 ${t}`
const noticeText = (n: number) => dict.notices.find((x: any) => x.value === n)?.label || '-'

const isAllRep = (row: any) => repNames(row).length === 1 && repNames(row)[0] === '-1'
const repNames = (row: any) => {
  const raw = String(row.repName || '')
  if (!raw) return []
  if (raw.startsWith('[')) {
    try {
      const arr = JSON.parse(raw)
      if (Array.isArray(arr)) return arr.map((x) => String(x))
    } catch {
      return raw
        .replace(/^\[|\]$/g, '')
        .split(',')
        .map((s) => s.trim().replace(/"/g, ''))
        .filter(Boolean)
    }
  }
  return [raw]
}

const buildCycleDesc = () => {
  const hh = String(form.hour)
  const mm = String(form.minute).padStart(2, '0')
  switch (form.cycleType) {
    case 'minute':
      return '每分钟执行一次'
    case 'minute_n':
      return `每隔 ${form.minute} 分钟执行一次`
    case 'hour':
      return `每小时第 ${form.minute} 分钟执行一次`
    case 'hour_n':
      return `每隔 ${form.hour} 小时的第 ${form.minute} 分钟执行一次`
    case 'day':
      return `每天 ${hh}:${mm} 执行一次`
    case 'day_n':
      return `每隔 ${form.day} 天的 ${hh}:${mm} 执行一次`
    case 'week':
      return `每周${weekNames[form.week]} ${hh}:${mm} 执行一次`
    case 'month':
      return `每月 ${form.day} 日 ${hh}:${mm} 执行一次`
  }
  return ''
}

const repNamePayload = computed(() => {
  const names = form.repNames || []
  if (names.includes('-1')) return '-1'
  return JSON.stringify(names)
})

const load = async () => {
  loading.value = true
  try {
    const [l, r] = await Promise.all([apiCrondList(), apiRepAll()])
    list.value = l || []
    reps.value = r || []
  } finally {
    loading.value = false
  }
}

const loadDict = async () => {
  const res = await apiCrondDict()
  dict.types = res.types || []
  dict.cycleTypes = res.cycleTypes || []
  dict.notices = res.notices || []
  schedStatus.value = res.status || {}
}

const openCreate = () => {
  editing.value = false
  Object.assign(form, defaultForm())
  visible.value = true
}

const openEdit = (row: any) => {
  editing.value = true
  Object.assign(form, defaultForm(), {
    crondId: row.crondId,
    taskName: row.taskName,
    taskType: row.taskType,
    cycleType: row.cycleType,
    cycleDesc: row.cycleDesc,
    repNames: repNames(row),
    week: row.week ?? 1,
    day: row.day ?? 1,
    hour: row.hour ?? 0,
    minute: row.minute ?? 0,
    saveCount: row.saveCount || 10,
    notice: row.notice ?? 0,
    shell: row.shell || '',
    statusOn: row.status === 1
  })
  visible.value = true
}

const submit = async () => {
  if (!form.taskName) {
    ElMessage.warning('请输入任务名')
    return
  }
  if (typeNeedRep(form.taskType) && !form.repNames.length) {
    ElMessage.warning('请选择目标仓库（可勾选【全部仓库】）')
    return
  }
  if (form.taskType === 6 && !form.shell) {
    ElMessage.warning('请填写脚本内容')
    return
  }
  const payload: any = {
    crondId: form.crondId,
    taskName: form.taskName,
    taskType: form.taskType,
    cycleType: form.cycleType,
    cycleDesc: form.cycleDesc || buildCycleDesc(),
    repName: repNamePayload.value,
    week: form.week,
    day: form.day,
    hour: form.hour,
    minute: form.minute,
    saveCount: form.saveCount,
    notice: form.notice,
    code: '',
    shell: form.shell,
    status: form.statusOn ? 1 : 0
  }
  if (editing.value) await apiCrondUpdate(payload)
  else await apiCrondCreate(payload)
  ElMessage.success('保存成功')
  visible.value = false
  load()
  loadDict()
}

const toggle = async (row: any, v: any) => {
  await apiCrondUpdate({ ...row, status: v ? 1 : 0 })
  ElMessage.success(v ? '已启用' : '已停用')
  load()
  loadDict()
}

const trigger = async (row: any) => {
  try {
    await ElMessageBox.confirm(`立即执行一次任务 [${row.taskName}]？`, '提示', { type: 'warning' })
  } catch {
    return
  }
  const task = await apiCrondTrigger(row.crondId)
  ElMessage.success(`已提交，任务 ID ${task?.taskId ?? '-'}，可在【运行任务】查看日志`)
}

const viewLog = async (row: any) => {
  logTitle.value = row.taskName
  logContent.value = ''
  logVisible.value = true
  const res = await apiCrondLog(row.crondId)
  logContent.value = res.log || '（暂无日志）'
}

const copyLog = () => {
  navigator.clipboard?.writeText(logContent.value || '').then(
    () => ElMessage.success('已复制到剪贴板'),
    () => ElMessage.warning('复制失败')
  )
}

const remove = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确认删除任务 [${row.taskName}]？`, '提示', { type: 'warning' })
  } catch {
    return
  }
  await apiCrondDel(row.crondId)
  ElMessage.success('删除成功')
  load()
}

onMounted(async () => {
  await loadDict()
  await load()
})
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
.toolbar {
  display: flex;
  gap: 8px;
  align-items: center;
}
.hint {
  margin-left: 8px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.text-mono :deep(textarea) {
  font-family: Consolas, Menlo, monospace;
  font-size: 12px;
}
.log-box {
  margin: 0;
  padding: 10px 12px;
  height: 56vh;
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
</style>
