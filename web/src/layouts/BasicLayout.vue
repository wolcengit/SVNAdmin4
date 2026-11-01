<!--
  Copyright (C) 2026 wolcen@msn.com
  SPDX-License-Identifier: GPL-3.0-or-later

  SVNAdmin4 is free software: you can redistribute it and/or modify it under
  the terms of the GNU General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option) any later
  version. See the LICENSE file in the project root for the full license text.
-->

<template>
  <el-container class="layout">
    <el-aside :width="collapsed ? '64px' : '210px'" class="layout-aside">
      <div class="logo">
        <el-icon :size="22" color="#409eff"><Files /></el-icon>
        <span v-show="!collapsed">SVNAdmin4</span>
      </div>
      <el-menu
        :default-active="activePath"
        :collapse="collapsed"
        :collapse-transition="false"
        background-color="#1f2937"
        text-color="#c0c4cc"
        active-text-color="#ffffff"
        router
      >
        <template v-for="item in menus" :key="item.path">
          <el-menu-item :index="item.path">
            <el-icon><component :is="item.icon" /></el-icon>
            <template #title>{{ item.title }}</template>
          </el-menu-item>
        </template>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="layout-header">
        <div class="left">
          <el-button text :icon="collapsed ? Expand : Fold" @click="collapsed = !collapsed" />
          <el-tag v-if="svnserve" type="success" size="small" effect="plain">svnserve 运行中</el-tag>
          <el-tag v-else type="info" size="small" effect="plain">svnserve 未运行</el-tag>
        </div>
        <div class="right">
          <el-dropdown @command="onCommand">
            <span class="user-info">
              <el-icon><User /></el-icon>
              <span>{{ store.userName || '未登录' }}</span>
              <el-tag size="small" style="margin-left: 6px">{{ roleText }}</el-tag>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="personal">个人信息</el-dropdown-item>
                <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="layout-main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { ArrowDown, Expand, Fold } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { apiVerifyOption } from '@/api'

const store = useUserStore()
const route = useRoute()
const router = useRouter()

const collapsed = ref(false)
const svnserve = ref(false)

// funcKey：子管理员进入该页面所需的后端接口权限（对应权限树 necessaryFunctions 的代表键）
const allMenus: Array<{ path: string; title: string; icon: string; manage?: boolean; admin?: boolean; svnuser?: boolean; funcKey?: string }> = [
  { path: '/dashboard', title: '概览', icon: 'DataAnalysis', funcKey: 'common/statistics' },
  { path: '/myrepo', title: '我的仓库', icon: 'Collection', svnuser: true },
  { path: '/repository', title: 'SVN 仓库', icon: 'Files', manage: true, funcKey: 'rep/list' },
  { path: '/user', title: 'SVN 用户', icon: 'User', manage: true, funcKey: 'user/list' },
  { path: '/group', title: 'SVN 分组', icon: 'UserFilled', manage: true, funcKey: 'group/list' },
  { path: '/crond', title: '任务计划', icon: 'Timer', manage: true, funcKey: 'crond/list' },
  { path: '/task', title: '运行任务', icon: 'Loading', manage: true, funcKey: 'task/list' },
  { path: '/logs', title: '操作日志', icon: 'Document', manage: true, funcKey: 'log/list' },
  { path: '/subadmin', title: '子管理员', icon: 'Avatar', admin: true },
  { path: '/setting', title: '系统设置', icon: 'Setting', manage: true, funcKey: 'setting/list' },
  { path: '/personal', title: '个人信息', icon: 'User' }
]

const menus = computed(() =>
  allMenus.filter((m) => {
    if (m.admin) return store.isAdmin
    if (m.svnuser) return store.role === 'svnuser'
    if (m.manage && !store.canManage) return false
    // 子管理员按功能白名单过滤菜单（对齐原版动态路由）
    if (store.role === 'subadmin' && m.funcKey && !store.hasFunc(m.funcKey)) {
      return false
    }
    return true
  })
)

const activePath = computed(() => '/' + (route.path.split('/')[1] || 'dashboard'))

const roleText = computed(() => {
  if (store.role === 'admin') return '管理员'
  if (store.role === 'subadmin') return '子管理员'
  if (store.role === 'svnuser') return 'SVN 用户'
  return '未登录'
})

const onCommand = async (cmd: string) => {
  if (cmd === 'personal') {
    router.push('/personal')
  } else if (cmd === 'logout') {
    await ElMessageBox.confirm('确认退出登录？', '提示', { type: 'warning' })
    await store.logout()
    router.push('/login')
  }
}

onMounted(async () => {
  if (!store.userName) {
    await store.fetchUserInfo()
  }
  try {
    const opt = await apiVerifyOption()
    svnserve.value = opt.svnserveStatus
  } catch {
    /* 忽略 */
  }
})
</script>

<style scoped>
.layout {
  height: 100%;
}

.layout-aside {
  background: #1f2937;
  transition: width 0.2s;
  overflow-x: hidden;
}

.logo {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #fff;
  font-size: 16px;
  font-weight: 600;
  border-bottom: 1px solid #2c3949;
}

.layout-aside :deep(.el-menu) {
  border-right: none;
}

.layout-header {
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  border-bottom: 1px solid #ebeef5;
  padding: 0 16px;
}

.layout-header .left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  outline: none;
  font-size: 14px;
}

.layout-main {
  padding: 0;
  overflow: auto;
}
</style>
