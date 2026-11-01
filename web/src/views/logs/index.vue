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
      <h2>操作日志</h2>
      <div class="toolbar" style="margin: 0">
        <el-button :icon="Refresh" @click="load">刷新</el-button>
        <el-button :icon="Download" :loading="exporting" @click="exportExcel">导出 Excel</el-button>
        <el-button type="danger" :icon="Delete" @click="clear">清空</el-button>
      </div>
    </div>

    <div class="toolbar">
      <el-select v-model="query.typeName" placeholder="日志类型" clearable style="width: 180px" @change="load">
        <el-option v-for="t in types" :key="t" :label="t" :value="t" />
      </el-select>
      <el-input
        v-model="query.keyword"
        placeholder="操作人 / 内容关键字"
        clearable
        style="width: 240px"
        @keyup.enter="load"
        @clear="load"
      />
      <el-button type="primary" :icon="Search" @click="load">查询</el-button>
    </div>

    <el-table v-loading="loading" :data="list" border size="small">
      <el-table-column prop="logId" label="ID" width="70" />
      <el-table-column prop="logTypeName" label="类型" width="130" />
      <el-table-column prop="logAddUserName" label="操作人" width="130" />
      <el-table-column prop="logContent" label="内容" min-width="360" show-overflow-tooltip />
      <el-table-column prop="logAddTime" label="时间" width="170" />
    </el-table>

    <el-pagination
      style="margin-top: 12px; justify-content: flex-end"
      layout="total, sizes, prev, pager, next"
      :total="total"
      v-model:current-page="query.page"
      v-model:page-size="query.pageSize"
      :page-sizes="[20, 50, 100, 200]"
      @change="load"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Download, Refresh, Search } from '@element-plus/icons-vue'
import { apiLogClear, apiLogList, apiLogTypes } from '@/api'

const list = ref<any[]>([])
const types = ref<string[]>([])
const total = ref(0)
const loading = ref(false)
const exporting = ref(false)
const query = reactive({ page: 1, pageSize: 20, keyword: '', typeName: '' })

const load = async () => {
  loading.value = true
  try {
    const res = await apiLogList(query)
    list.value = res.list
    total.value = res.total
  } finally {
    loading.value = false
  }
}

const clear = async () => {
  await ElMessageBox.confirm('确认清空全部操作日志？', '警告', { type: 'warning' })
  await apiLogClear()
  ElMessage.success('已清空')
  load()
}

/* ---------- 导出 Excel（SpreadsheetML 2003，Excel/WPS 可直接打开） ---------- */

const xmlEscape = (s: any) =>
  String(s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')

const exportExcel = async () => {
  exporting.value = true
  try {
    // 一次性拉取全部符合条件的日志（上限 50000 条，防止浏览器卡死）
    const params: any = { page: 1, pageSize: 50000, keyword: query.keyword, typeName: query.typeName }
    const res = await apiLogList(params)
    const rows = res.list || []
    if (!rows.length) {
      ElMessage.warning('当前条件下没有可导出的日志')
      return
    }
    const head = ['ID', '类型', '操作人', '内容', '时间']
    const body = rows
      .map(
        (r: any) =>
          `<Row>` +
          head
            .map((_, i) => {
              const v = [r.logId, r.logTypeName, r.logAddUserName, r.logContent, r.logAddTime][i]
              return `<Cell><Data ss:Type="String">${xmlEscape(v)}</Data></Cell>`
            })
            .join('') +
          `</Row>`
      )
      .join('')
    const xml =
      `<?xml version="1.0" encoding="UTF-8"?><?mso-application progid="Excel.Sheet"?>` +
      `<Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet" ` +
      `xmlns:ss="urn:schemas-microsoft-com:office:spreadsheet">` +
      `<Worksheet ss:Name="操作日志"><Table>` +
      `<Row>${head.map((h) => `<Cell><Data ss:Type="String">${xmlEscape(h)}</Data></Cell>`).join('')}</Row>` +
      body +
      `</Table></Worksheet></Workbook>`
    const blob = new Blob([xml], { type: 'application/vnd.ms-excel;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `操作日志_${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.xls`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
    ElMessage.success(`已导出 ${rows.length} 条日志`)
  } finally {
    exporting.value = false
  }
}

onMounted(async () => {
  types.value = await apiLogTypes().catch(() => [])
  load()
})
</script>
