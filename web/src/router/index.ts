/*!
 * Copyright (C) 2026 wolcen@msn.com
 * SPDX-License-Identifier: GPL-3.0-or-later
 *
 * SVNAdmin4 is free software: you can redistribute it and/or modify it under
 * the terms of the GNU General Public License as published by the Free Software
 * Foundation, either version 3 of the License, or (at your option) any later
 * version. See the LICENSE file in the project root for the full license text.
 */

import { createRouter, createWebHashHistory, type RouteRecordRaw } from 'vue-router'
import { useUserStore } from '@/stores/user'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/login/index.vue'),
    meta: { title: '登录', public: true }
  },
  {
    path: '/',
    component: () => import('@/layouts/BasicLayout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'dashboard',
        component: () => import('@/views/dashboard/index.vue'),
        meta: { title: '概览', icon: 'DataAnalysis' }
      },
      {
        path: 'repository',
        name: 'repository',
        component: () => import('@/views/repository/index.vue'),
        meta: { title: 'SVN 仓库', icon: 'Files', manage: true }
      },
      {
        path: 'user',
        name: 'user',
        component: () => import('@/views/user/index.vue'),
        meta: { title: 'SVN 用户', icon: 'User', manage: true }
      },
      {
        path: 'group',
        name: 'group',
        component: () => import('@/views/group/index.vue'),
        meta: { title: 'SVN 分组', icon: 'UserFilled', manage: true }
      },
      {
        path: 'crond',
        name: 'crond',
        component: () => import('@/views/crond/index.vue'),
        meta: { title: '任务计划', icon: 'Timer', manage: true }
      },
      {
        path: 'task',
        name: 'task',
        component: () => import('@/views/task/index.vue'),
        meta: { title: '运行任务', icon: 'Loading', manage: true }
      },
      {
        path: 'logs',
        name: 'logs',
        component: () => import('@/views/logs/index.vue'),
        meta: { title: '操作日志', icon: 'Document', manage: true }
      },
      {
        path: 'subadmin',
        name: 'subadmin',
        component: () => import('@/views/subadmin/index.vue'),
        meta: { title: '子管理员', icon: 'Avatar', admin: true }
      },
      {
        path: 'setting',
        name: 'setting',
        component: () => import('@/views/setting/index.vue'),
        meta: { title: '系统设置', icon: 'Setting', manage: true }
      },
      {
        path: 'myrepo',
        name: 'myrepo',
        component: () => import('@/views/myrepo/index.vue'),
        meta: { title: '我的仓库', icon: 'Collection' }
      },
      {
        path: 'personal',
        name: 'personal',
        component: () => import('@/views/personal/index.vue'),
        meta: { title: '个人信息', icon: 'User' }
      }
    ]
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

router.beforeEach(async (to) => {
  document.title = to.meta.title ? `SVNAdmin4 - ${to.meta.title}` : 'SVNAdmin4'
  const store = useUserStore()
  if (to.meta.public) {
    return true
  }
  if (!store.token) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  if (!store.userName) {
    await store.fetchUserInfo()
  }
  if (to.meta.admin && !store.isAdmin) {
    return { path: '/dashboard' }
  }
  if (to.meta.manage && !store.canManage) {
    return { path: '/personal' }
  }
  return true
})

export default router
