<!--
  Copyright (C) 2026 wolcen@msn.com
  SPDX-License-Identifier: GPL-3.0-or-later

  SVNAdmin4 is free software: you can redistribute it and/or modify it under
  the terms of the GNU General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option) any later
  version. See the LICENSE file in the project root for the full license text.
-->

<template>
  <div class="login-page">
    <div class="login-box">
      <div class="brand">
        <el-icon :size="28" color="#409eff"><Files /></el-icon>
        <h1>SVNAdmin4</h1>
      </div>
      <p class="sub">基于 Web 的 Subversion 服务端管理系统</p>

      <el-form ref="formRef" :model="form" :rules="rules" size="large" @keyup.enter="onSubmit">
        <el-form-item prop="userName">
          <el-input v-model="form.userName" placeholder="用户名" :prefix-icon="User" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            placeholder="密码"
            :prefix-icon="Lock"
          />
        </el-form-item>
        <el-form-item prop="role">
          <el-select v-model="form.role" placeholder="登录身份" style="width: 100%">
            <el-option label="自动识别" value="" />
            <el-option label="管理人员" value="admin" />
            <el-option label="子管理员" value="subadmin" />
            <el-option label="SVN 用户" value="svnuser" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="enableCode" prop="code">
          <div class="code-row">
            <el-input v-model="form.code" placeholder="验证码" :prefix-icon="Key" />
            <div class="code-img" title="点击刷新" @click="refreshCode" v-html="svg"></div>
          </div>
        </el-form-item>
        <el-button type="primary" class="submit" :loading="loading" @click="onSubmit">
          登 录
        </el-button>
      </el-form>

      <div class="footer">
        <span class="muted">默认账号 admin / admin</span>
        <span class="muted">Subversion {{ svnVersion }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Key, Lock, User } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { apiVerifyCode, apiVerifyOption } from '@/api'

const store = useUserStore()
const router = useRouter()
const route = useRoute()

const formRef = ref<FormInstance>()
const loading = ref(false)
const enableCode = ref(true)
const svnVersion = ref('')
const svg = ref('')
const uuid = ref('')

const form = reactive({ userName: 'admin', password: 'admin', code: '', role: '' })

const rules: FormRules = {
  userName: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
  code: [{ required: true, message: '请输入验证码', trigger: 'blur' }]
}

const refreshCode = async () => {
  const res = await apiVerifyCode()
  svg.value = res.svg
  uuid.value = res.uuid
}

const onSubmit = async () => {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    await store.login({
      userName: form.userName,
      password: form.password,
      uuid: uuid.value,
      code: form.code,
      role: form.role
    })
    ElMessage.success('登录成功')
    const redirect = (route.query.redirect as string) || '/dashboard'
    router.push(redirect)
  } catch (e: any) {
    ElMessage.error(e?.message || '登录失败')
    if (enableCode.value) {
      form.code = ''
      await refreshCode()
    }
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    const opt = await apiVerifyOption()
    enableCode.value = opt.enableVerifyCode
    svnVersion.value = opt.svnVersion
    if (opt.enableVerifyCode) {
      await refreshCode()
    }
  } catch {
    /* 忽略 */
  }
})
</script>

<style scoped>
.login-page {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #e8f2ff 0%, #f5f7fa 50%, #eef3ff 100%);
}

.login-box {
  width: 380px;
  padding: 36px 32px 24px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 12px 40px rgba(31, 45, 61, 0.12);
}

.brand {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.brand h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
}

.sub {
  text-align: center;
  color: #909399;
  font-size: 13px;
  margin: 8px 0 24px;
}

.code-row {
  display: flex;
  gap: 10px;
  width: 100%;
}

.code-img {
  width: 120px;
  height: 40px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  overflow: hidden;
  cursor: pointer;
  flex-shrink: 0;
}

.code-img :deep(svg) {
  width: 120px;
  height: 40px;
  display: block;
}

.submit {
  width: 100%;
}

.footer {
  margin-top: 18px;
  display: flex;
  justify-content: space-between;
  font-size: 12px;
}
</style>
