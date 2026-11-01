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
      <h2>信息统计</h2>
      <el-space>
        <el-checkbox v-model="autoRefresh" size="small">3 秒自动刷新监控</el-checkbox>
        <el-button :icon="Refresh" @click="load">刷新</el-button>
      </el-space>
    </div>

    <!-- 数字卡片 -->
    <el-row :gutter="12">
      <el-col :xs="12" :sm="8" :md="6" :lg="4" v-for="card in cards" :key="card.label">
        <el-card shadow="hover" class="stat-card">
          <div class="stat-value" :style="{ color: card.color }">{{ card.value }}</div>
          <div class="stat-label">{{ card.label }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 系统状态仪表盘 -->
    <el-card shadow="never" style="margin-top: 8px">
      <template #header>
        <span>系统状态</span>
        <el-text type="info" size="small" style="margin-left: 8px">
          {{ monitor.host?.hostname ? monitor.host.hostname + ' · ' : '' }}{{ monitor.host?.os || '' }}
        </el-text>
      </template>
      <el-row :gutter="16" v-loading="monitorLoading">
        <el-col :xs="12" :sm="6">
          <div class="gauge-box">
            <el-progress type="dashboard" :percentage="pct(monitor.load?.percent)" :color="gaugeColor(monitor.load?.color)" :width="130">
              <template #default>
                <div class="gauge-text">
                  <div class="gauge-value">{{ monitor.load?.load1Min?.toFixed(2) ?? '-' }}</div>
                  <div class="gauge-label">系统负载</div>
                </div>
              </template>
            </el-progress>
            <el-tooltip placement="top">
              <template #content>
                <div>1 分钟：{{ monitor.load?.load1Min?.toFixed(2) ?? '-' }}</div>
                <div>5 分钟：{{ monitor.load?.load5Min?.toFixed(2) ?? '-' }}</div>
                <div>15 分钟：{{ monitor.load?.load15Min?.toFixed(2) ?? '-' }}</div>
              </template>
              <div class="gauge-foot">1 / 5 / 15 分钟负载</div>
            </el-tooltip>
          </div>
        </el-col>

        <el-col :xs="12" :sm="6">
          <div class="gauge-box">
            <el-progress type="dashboard" :percentage="pct(monitor.cpu?.percent)" :color="gaugeColor(monitor.cpu?.color)" :width="130">
              <template #default>
                <div class="gauge-text">
                  <div class="gauge-value">{{ monitor.cpu?.percent?.toFixed(1) ?? '-' }}%</div>
                  <div class="gauge-label">CPU</div>
                </div>
              </template>
            </el-progress>
            <el-tooltip placement="top">
              <template #content>
                <div>{{ monitor.cpu?.cpu?.[0] || '-' }}</div>
                <div>物理 CPU：{{ monitor.cpu?.cpuPhysical ?? '-' }}</div>
                <div>核心数：{{ monitor.cpu?.cpuCore ?? '-' }}</div>
                <div>线程数：{{ monitor.cpu?.cpuProcessor ?? '-' }}</div>
              </template>
              <div class="gauge-foot">{{ monitor.cpu?.cpuCore ?? '-' }} 核</div>
            </el-tooltip>
          </div>
        </el-col>

        <el-col :xs="12" :sm="6">
          <div class="gauge-box">
            <el-progress type="dashboard" :percentage="pct(monitor.mem?.percent)" :color="gaugeColor(monitor.mem?.color)" :width="130">
              <template #default>
                <div class="gauge-text">
                  <div class="gauge-value">{{ monitor.mem?.percent?.toFixed(1) ?? '-' }}%</div>
                  <div class="gauge-label">内存</div>
                </div>
              </template>
            </el-progress>
            <el-tooltip placement="top">
              <template #content>
                <div>总量：{{ monitor.mem?.memTotal ?? '-' }} MB</div>
                <div>已用：{{ monitor.mem?.memUsed ?? '-' }} MB</div>
                <div>空闲：{{ monitor.mem?.memFree ?? '-' }} MB</div>
              </template>
              <div class="gauge-foot">{{ monitor.mem?.memUsed ?? '-' }} / {{ monitor.mem?.memTotal ?? '-' }} MB</div>
            </el-tooltip>
          </div>
        </el-col>

        <el-col :xs="12" :sm="6">
          <div class="gauge-box">
            <div class="disk-list">
              <div v-for="d in (monitor.disk || []).slice(0, 4)" :key="d.mountedOn" class="disk-item">
                <el-tooltip placement="left">
                  <template #content>
                    <div>文件系统：{{ d.fileSystem }}</div>
                    <div>容量：{{ d.size }} GB</div>
                    <div>已用：{{ d.used }} GB</div>
                    <div>可用：{{ d.avail }} GB</div>
                  </template>
                  <div class="disk-head">
                    <span class="disk-path">{{ d.mountedOn }}</span>
                    <span class="disk-pct">{{ d.percent }}%</span>
                  </div>
                </el-tooltip>
                <el-progress :percentage="pct(d.percent)" :color="gaugeColor(d.color)" :stroke-width="8" />
              </div>
              <el-empty v-if="!monitor.disk?.length" description="无磁盘数据" :image-size="50" />
            </div>
            <div class="gauge-foot">磁盘使用</div>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :xs="24" :md="14">
        <el-card shadow="never">
          <template #header>仓库体积排行</template>
          <el-table :data="ranks" size="small" max-height="320">
            <el-table-column prop="repName" label="仓库" min-width="160" />
            <el-table-column label="版本数" width="90" align="right">
              <template #default="{ row }">{{ row.repRev }}</template>
            </el-table-column>
            <el-table-column label="体积" width="110" align="right">
              <template #default="{ row }">{{ fmtSize(row.repSize) }}</template>
            </el-table-column>
          </el-table>
          <el-empty v-if="!ranks.length" description="暂无仓库" :image-size="70" />
        </el-card>
      </el-col>
      <el-col :xs="24" :md="10">
        <el-card shadow="never">
          <template #header>运行状态</template>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="Subversion">{{ stat.svnVersion || '-' }}</el-descriptions-item>
            <el-descriptions-item label="svnserve">
              <el-tag :type="stat.svnserve ? 'success' : 'info'" size="small">
                {{ stat.svnserve ? '运行中' : '未运行' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="后台任务">
              <el-tag :type="runInfo.running ? 'warning' : 'info'" size="small">
                {{ runInfo.running ? '执行中' : '空闲' }}
              </el-tag>
              <span v-if="runInfo.queueCount" style="margin-left: 6px; color: #909399">
                排队 {{ runInfo.queueCount }}
              </span>
            </el-descriptions-item>
            <el-descriptions-item label="定时任务">
              {{ crondStatus.jobCount ?? 0 }} 个已启用
            </el-descriptions-item>
            <el-descriptions-item label="仓库总数">{{ stat.repCount ?? 0 }}</el-descriptions-item>
            <el-descriptions-item label="用户总数">{{ stat.userCount ?? 0 }}</el-descriptions-item>
            <el-descriptions-item label="分组总数">{{ stat.groupCount ?? 0 }}</el-descriptions-item>
            <el-descriptions-item label="别名总数">{{ stat.aliaseCount ?? 0 }}</el-descriptions-item>
            <el-descriptions-item label="备份数量">{{ stat.backupCount ?? 0 }}</el-descriptions-item>
            <el-descriptions-item label="仓库占用">{{ stat.totalSizeHr || '0 B' }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { apiCrondDict, apiMonitor, apiRepAll, apiStatistics, apiTaskRun } from '@/api'

const stat = ref<any>({})
const ranks = ref<any[]>([])
const monitor = ref<any>({})
const runInfo = ref<any>({})
const crondStatus = ref<any>({})
const monitorLoading = ref(false)
const autoRefresh = ref(false)
let timer: any = null

const cards = computed(() => [
  { label: 'SVN 仓库', value: stat.value.repCount ?? 0, color: '#409eff' },
  { label: '仓库占用', value: stat.value.totalSizeHr || '0 B', color: '#409eff' },
  { label: '仓库备份', value: stat.value.backupCount ?? 0, color: '#67c23a' },
  { label: '备份占用', value: stat.value.backupSizeHr || '0 B', color: '#67c23a' },
  { label: 'SVN 用户', value: stat.value.userCount ?? 0, color: '#e6a23c' },
  { label: 'SVN 分组', value: stat.value.groupCount ?? 0, color: '#e6a23c' },
  { label: 'SVN 别名', value: stat.value.aliaseCount ?? 0, color: '#e6a23c' },
  { label: '子管理员', value: stat.value.subadminCount ?? 0, color: '#f56c6c' },
  { label: '运行日志', value: stat.value.logCount ?? 0, color: '#909399' }
])

const pct = (v: number | undefined) => Math.min(100, Math.max(0, Math.round(v ?? 0)))

const gaugeColor = (c: string) => {
  switch (c) {
    case 'red': return '#f56c6c'
    case 'orange': return '#e6a23c'
    case 'blue': return '#409eff'
    default: return '#67c23a'
  }
}

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

const loadMonitor = async () => {
  monitorLoading.value = true
  try {
    monitor.value = await apiMonitor()
    runInfo.value = await apiTaskRun()
    const dict = await apiCrondDict()
    crondStatus.value = dict.status || {}
  } catch {
    /* 忽略 */
  } finally {
    monitorLoading.value = false
  }
}

const load = async () => {
  try {
    stat.value = await apiStatistics()
    const all = await apiRepAll()
    ranks.value = [...all].sort((a, b) => (b.repSize || 0) - (a.repSize || 0)).slice(0, 10)
  } catch {
    /* 忽略 */
  }
  loadMonitor()
}

watch(autoRefresh, (v) => {
  if (timer) clearInterval(timer)
  if (v) {
    timer = setInterval(loadMonitor, 3000)
  }
})

onMounted(load)
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
.stat-card {
  text-align: center;
  margin-bottom: 12px;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  line-height: 1.2;
}

.stat-label {
  margin-top: 6px;
  color: #909399;
  font-size: 13px;
}

.gauge-box {
  text-align: center;
  padding: 8px 0;
}

.gauge-text {
  text-align: center;
}

.gauge-value {
  font-size: 20px;
  font-weight: 600;
}

.gauge-label {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}

.gauge-foot {
  margin-top: 6px;
  font-size: 12px;
  color: #909399;
  cursor: default;
}

.disk-list {
  min-height: 130px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8px;
  padding: 4px 6px;
}

.disk-head {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #606266;
  margin-bottom: 2px;
}

.disk-path {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
