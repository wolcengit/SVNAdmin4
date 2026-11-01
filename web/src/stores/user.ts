/*!
 * Copyright (C) 2026 wolcen@msn.com
 * SPDX-License-Identifier: GPL-3.0-or-later
 *
 * SVNAdmin4 is free software: you can redistribute it and/or modify it under
 * the terms of the GNU General Public License as published by the Free Software
 * Foundation, either version 3 of the License, or (at your option) any later
 * version. See the LICENSE file in the project root for the full license text.
 */

import { defineStore } from 'pinia'
import { getToken, getUser, setToken, setUser, clearToken } from '@/api/request'
import { apiLogin, apiLogout, apiUserInfo } from '@/api'

interface UserState {
  token: string
  userId: number
  userName: string
  role: string
  functions: string[]
}

export const useUserStore = defineStore('user', {
  state: (): UserState => {
    const cached = getUser()
    return {
      token: getToken(),
      userId: cached?.userId || 0,
      userName: cached?.userName || '',
      role: cached?.role || '',
      functions: cached?.functions || []
    }
  },
  getters: {
    isLogin: (s) => !!s.token,
    isAdmin: (s) => s.role === 'admin',
    canManage: (s) => s.role === 'admin' || s.role === 'subadmin',
    hasFunc: (s) => (key: string) =>
      s.role === 'admin' ? true : s.role === 'subadmin' ? s.functions.includes(key) : false
  },
  actions: {
    async login(payload: {
      userName: string
      password: string
      uuid: string
      code: string
      role?: string
    }) {
      const res = await apiLogin(payload)
      this.token = res.token
      this.userId = res.userId
      this.userName = res.userName
      this.role = res.role
      this.functions = res.functions || []
      setToken(res.token)
      setUser({ userId: res.userId, userName: res.userName, role: res.role, functions: res.functions || [] })
      return res
    },
    async fetchUserInfo() {
      if (!this.token) return
      try {
        const res = await apiUserInfo()
        this.userId = res.userId
        this.userName = res.userName
        this.role = res.role
        this.functions = res.functions || []
        setUser({ userId: res.userId, userName: res.userName, role: res.role, functions: res.functions || [] })
      } catch {
        this.logout()
      }
    },
    async logout() {
      try {
        await apiLogout()
      } catch {
        /* 忽略服务端异常，本地仍然清理 */
      }
      this.token = ''
      this.userId = 0
      this.userName = ''
      this.role = ''
      this.functions = []
      clearToken()
    }
  }
})
