/*!
 * Copyright (C) 2026 wolcen@msn.com
 * SPDX-License-Identifier: GPL-3.0-or-later
 *
 * SVNAdmin4 is free software: you can redistribute it and/or modify it under
 * the terms of the GNU General Public License as published by the Free Software
 * Foundation, either version 3 of the License, or (at your option) any later
 * version. See the LICENSE file in the project root for the full license text.
 */

import axios, { type AxiosInstance } from 'axios'
import { ElMessage } from 'element-plus'
import type { Result } from '@/env'

const TOKEN_KEY = 'svnadmin4-token'
const USER_KEY = 'svnadmin4-user'

export const getToken = (): string => localStorage.getItem(TOKEN_KEY) || ''
export const setToken = (t: string) => localStorage.setItem(TOKEN_KEY, t)
export const clearToken = () => {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

export const getUser = (): any => {
  const raw = localStorage.getItem(USER_KEY)
  return raw ? JSON.parse(raw) : null
}
export const setUser = (u: any) => localStorage.setItem(USER_KEY, JSON.stringify(u))

const request: AxiosInstance = axios.create({
  baseURL: '/api',
  timeout: 30000
})

request.interceptors.request.use((config) => {
  const token = getToken()
  if (token) {
    config.headers['X-Token'] = token
  }
  return config
})

request.interceptors.response.use(
  (resp) => {
    const body = resp.data as Result
    // 登录失效
    if (body && body.code === 402) {
      clearToken()
      ElMessage.error(body.message || '登录状态已失效')
      setTimeout(() => {
        if (!location.hash.startsWith('#/login')) {
          location.href = '#/login'
          location.reload()
        }
      }, 600)
      return Promise.reject(new Error(body.message))
    }
    if (body && body.status === false) {
      ElMessage.error(body.message || '请求失败')
      return Promise.reject(new Error(body.message))
    }
    return body as any
  },
  (error) => {
    const msg = error?.response?.data?.message || error.message || '网络异常'
    ElMessage.error(msg)
    return Promise.reject(error)
  }
)

/** 分页数据 */
export interface PageData<T = any> {
  total: number
  list: T[]
}

/** 业务请求：直接返回 data 部分 */
export async function http<T = any>(config: {
  url: string
  method?: 'get' | 'post' | 'put' | 'delete'
  params?: any
  data?: any
}): Promise<T> {
  const res = (await request.request({
    url: config.url,
    method: config.method || 'get',
    params: config.params,
    data: config.data
  })) as unknown as Result<T>
  return res.data
}

export default request
