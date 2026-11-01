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
    <div class="page-header"><h2>个人信息</h2></div>

    <el-row :gutter="16">
      <el-col :xs="24" :md="10">
        <el-card shadow="never">
          <template #header>账号信息</template>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="用户名">{{ store.userName }}</el-descriptions-item>
            <el-descriptions-item label="角色">{{ roleText }}</el-descriptions-item>
            <el-descriptions-item label="用户 ID">{{ store.userId }}</el-descriptions-item>
          </el-descriptions>
          <div v-if="store.role === 'admin'" style="margin-top: 12px">
            <el-button size="small" type="warning" @click="nameVisible = true">修改管理员账号名</el-button>
          </div>
        </el-card>
      </el-col>
      <el-col :xs="24" :md="14">
        <el-card shadow="never">
          <template #header>修改密码</template>
          <el-form label-width="100px" style="max-width: 420px">
            <el-form-item label="原密码">
              <el-input v-model="oldPass" type="password" show-password />
            </el-form-item>
            <el-form-item label="新密码">
              <el-input v-model="newPass" type="password" show-password />
            </el-form-item>
            <el-form-item label="确认密码">
              <el-input v-model="confirmPass" type="password" show-password />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="loading" @click="submit">提交</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>
    </el-row>

    <!-- 修改管理员账号名 -->
    <el-dialog v-model="nameVisible" title="修改管理员账号名" width="440px">
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 12px"
        title="修改成功后需要使用新账号名重新登录。"
      />
      <el-form label-width="90px">
        <el-form-item label="原账号名">
          <el-input :model-value="store.userName" disabled />
        </el-form-item>
        <el-form-item label="新账号名" required>
          <el-input v-model="nameForm.newName" placeholder="字母、数字、点、下划线、短横线，1-32 位" />
        </el-form-item>
        <el-form-item label="登录密码" required>
          <el-input v-model="nameForm.pass" type="password" show-password placeholder="输入当前登录密码以确认身份" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="nameVisible = false">取消</el-button>
        <el-button type="primary" :loading="nameLoading" @click="submitName">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useUserStore } from '@/stores/user'
import { apiEditAdminUserName, apiUpdatePass } from '@/api'

const store = useUserStore()
const oldPass = ref('')
const newPass = ref('')
const confirmPass = ref('')
const loading = ref(false)

const nameVisible = ref(false)
const nameLoading = ref(false)
const nameForm = reactive({ newName: '', pass: '' })

const roleText = computed(() => {
  if (store.role === 'admin') return '超级管理员'
  if (store.role === 'subadmin') return '子管理员'
  if (store.role === 'svnuser') return 'SVN 用户'
  return '未登录'
})

const submit = async () => {
  if (!oldPass.value || !newPass.value) {
    ElMessage.warning('请填写原密码与新密码')
    return
  }
  if (newPass.value !== confirmPass.value) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  loading.value = true
  try {
    await apiUpdatePass({ oldPass: oldPass.value, newPass: newPass.value })
    ElMessage.success('密码修改成功，请使用新密码重新登录')
    oldPass.value = ''
    newPass.value = ''
    confirmPass.value = ''
    setTimeout(() => store.logout(), 800)
  } finally {
    loading.value = false
  }
}

const submitName = async () => {
  if (!nameForm.newName) {
    ElMessage.warning('请输入新账号名')
    return
  }
  if (!/^[A-Za-z0-9._-]{1,32}$/.test(nameForm.newName)) {
    ElMessage.warning('账号名只能包含字母、数字、点、下划线、短横线，长度 1-32 位')
    return
  }
  if (!nameForm.pass) {
    ElMessage.warning('请输入当前登录密码')
    return
  }
  nameLoading.value = true
  try {
    await apiEditAdminUserName({
      oldName: store.userName,
      newName: nameForm.newName,
      pass: nameForm.pass
    })
    ElMessage.success('账号名已修改，请使用新账号名重新登录')
    nameVisible.value = false
    nameForm.newName = ''
    nameForm.pass = ''
    setTimeout(() => store.logout(), 800)
  } finally {
    nameLoading.value = false
  }
}
</script>
