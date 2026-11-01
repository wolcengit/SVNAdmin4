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
      <h2>系统设置</h2>
      <el-button :icon="Refresh" @click="loadAll">刷新</el-button>
    </div>

    <el-tabs v-model="tab">
      <!-- ============ 主机配置 ============ -->
      <el-tab-pane label="主机配置" name="host">
        <el-card shadow="never">
          <template #header>宿主机配置（用于生成检出地址，容器部署场景必填）</template>
          <el-form label-width="140px" style="max-width: 620px">
            <el-form-item label="宿主机地址">
              <el-input v-model="dockerHost.host" placeholder="127.0.0.1 或对外域名" />
            </el-form-item>
            <el-form-item label="svn 协议端口">
              <el-input-number v-model="dockerHost.svnPort" :min="1" :max="65535" />
            </el-form-item>
            <el-form-item label="http 协议端口">
              <el-input-number v-model="dockerHost.httpPort" :min="1" :max="65535" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveDockerHost">保存</el-button>
            </el-form-item>
          </el-form>
          <el-alert
            :closable="false"
            type="info"
            show-icon
            title="若 SVNAdmin 运行在容器内，仓库检出地址需要指向宿主机而非容器内部地址，请在此填写宿主机的 IP/域名与实际映射端口。"
          />
        </el-card>

        <el-card shadow="never" style="margin-top: 12px">
          <template #header>服务状态</template>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="svnserve">
              <el-tag :type="svnInfo.svnserveStatus ? 'success' : 'info'" size="small">
                {{ svnInfo.svnserveStatus ? '运行中' : '未运行' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="Subversion">
              {{ svnInfo.svnVersion || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="检出方式">
              {{ svnInfo.httpPrefix ? 'HTTP 协议' : 'SVN 协议' }}
            </el-descriptions-item>
            <el-descriptions-item label="svnserve 监听">
              {{ svnInfo.svnserveHost }}:{{ svnInfo.svnservePort }}
            </el-descriptions-item>
          </el-descriptions>
          <div style="margin-top: 12px; display: flex; gap: 8px">
            <el-button type="success" size="small" @click="svnserveStart">启动 svnserve</el-button>
            <el-button type="danger" size="small" @click="svnserveStop">停止 svnserve</el-button>
            <el-button size="small" :icon="RefreshRight" @click="syncAll">同步全部数据</el-button>
          </div>
          <el-alert
            style="margin-top: 12px"
            :closable="false"
            type="info"
            title="svnserve 由本进程托管启动，进程重启后需再次点击启动；生产环境建议使用 systemd 托管。"
          />
        </el-card>
      </el-tab-pane>

      <!-- ============ 路径信息 ============ -->
      <el-tab-pane label="路径信息" name="dir">
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item v-for="d in dirFields" :key="d.key" :label="d.label">
            <span class="path-text">{{ dirInfo[d.key] || '-' }}</span>
          </el-descriptions-item>
        </el-descriptions>
      </el-tab-pane>

      <!-- ============ SVN 协议检出 ============ -->
      <el-tab-pane label="SVN 协议检出" name="svn">
        <el-card shadow="never">
          <template #header>svnserve 配置</template>
          <el-form label-width="150px" style="max-width: 620px">
            <el-form-item label="监听地址">
              <el-input v-model="svnForm.host" placeholder="0.0.0.0" style="width: 240px" />
              <el-button style="margin-left: 8px" @click="saveSvnHost">保存</el-button>
            </el-form-item>
            <el-form-item label="监听端口">
              <el-input-number v-model="svnForm.port" :min="1" :max="65535" />
              <el-button style="margin-left: 8px" @click="saveSvnPort">保存</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card shadow="never" style="margin-top: 12px">
          <template #header>用户与分组来源（SVN 协议）</template>
          <el-form label-width="150px" style="max-width: 620px">
            <el-form-item label="用户来源">
              <el-select v-model="svnSource.userSource" style="width: 260px">
                <el-option label="passwd 文件" value="passwd" />
                <el-option label="LDAP" value="ldap" />
              </el-select>
            </el-form-item>
            <el-form-item label="分组来源">
              <el-select v-model="svnSource.groupSource" style="width: 260px">
                <el-option label="authz 文件" value="authz" />
                <el-option label="LDAP" value="ldap" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveSvnSource">保存并应用</el-button>
            </el-form-item>
          </el-form>
          <el-alert
            :closable="false"
            type="warning"
            show-icon
            title="切换来源会按新来源重建用户/分组数据，请确认已备份 authz 与 passwd 文件。"
          />
        </el-card>

        <el-card shadow="never" style="margin-top: 12px">
          <template #header>SASL（saslauthd，LDAP 认证依赖）</template>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="安装状态">
              <el-tag :type="sasl.installed ? 'success' : 'info'" size="small">
                {{ sasl.installed ? '已安装' : '未安装' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="运行状态">
              <el-tag :type="sasl.running ? 'success' : 'info'" size="small">
                {{ sasl.running ? `运行中 (pid ${sasl.pid})` : '未运行' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="版本">{{ sasl.version || '-' }}</el-descriptions-item>
            <el-descriptions-item label="认证机制">
              {{ sasl.mechanisms?.join(', ') || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="配置文件" :span="2">
              <span class="path-text">{{ sasl.confFile || '-' }}</span>
            </el-descriptions-item>
          </el-descriptions>
          <div style="margin-top: 12px; display: flex; gap: 8px">
            <el-button type="success" size="small" @click="saslStart">启动 saslauthd</el-button>
            <el-button type="danger" size="small" @click="saslStop">停止 saslauthd</el-button>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ============ HTTP 协议检出 ============ -->
      <el-tab-pane label="HTTP 协议检出" name="http">
        <el-card shadow="never">
          <template #header>Apache + mod_dav_svn</template>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="Apache">
              <el-tag :type="apache.installed ? 'success' : 'danger'" size="small">
                {{ apache.installed ? '已安装' : '未检测到' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="版本">{{ apache.version || '-' }}</el-descriptions-item>
            <el-descriptions-item label="可执行路径" :span="2">
              <span class="path-text">{{ apache.binPath || '-' }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="mod_dav_svn">
              <el-tag :type="apache.davSvnModule ? 'success' : 'danger'" size="small">
                {{ apache.davSvnModule ? '已加载' : '未加载' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="mod_authz_svn">
              <el-tag :type="apache.authzSvnModule ? 'success' : 'danger'" size="small">
                {{ apache.authzSvnModule ? '已加载' : '未加载' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="配置文件" :span="2">
              <span class="path-text">{{ apache.confPath || '-' }}</span>
            </el-descriptions-item>
          </el-descriptions>
          <el-alert
            v-for="(w, i) in apache.warnings || []"
            :key="i"
            style="margin-top: 10px"
            :closable="false"
            type="warning"
            show-icon
            :title="w"
          />
        </el-card>

        <el-card shadow="never" style="margin-top: 12px">
          <template #header>协议切换与访问前缀</template>
          <el-form label-width="150px" style="max-width: 620px">
            <el-form-item label="访问前缀">
              <el-input v-model="httpForm.prefix" placeholder="/svn" style="width: 240px" />
              <el-button style="margin-left: 8px" @click="saveHttpPrefix">保存</el-button>
            </el-form-item>
            <el-form-item label="HTTP 端口">
              <el-input-number v-model="httpForm.port" :min="1" :max="65535" />
              <el-button style="margin-left: 8px" @click="saveHttpPort">保存</el-button>
            </el-form-item>
            <el-form-item>
              <el-button type="warning" :disabled="!apache.installed" @click="enableHttp">
                启用 HTTP 协议检出
              </el-button>
              <el-button type="info" @click="enableSvn">切换回 SVN 协议检出</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card shadow="never" style="margin-top: 12px">
          <template #header>用户与分组来源（HTTP 协议）</template>
          <el-form label-width="150px" style="max-width: 620px">
            <el-form-item label="用户来源">
              <el-select v-model="httpSource.userSource" style="width: 260px">
                <el-option label="httpPasswd 文件" value="passwd" />
                <el-option label="LDAP" value="ldap" />
              </el-select>
            </el-form-item>
            <el-form-item label="分组来源">
              <el-select v-model="httpSource.groupSource" style="width: 260px">
                <el-option label="authz 文件" value="authz" />
                <el-option label="LDAP" value="ldap" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveHttpSource">保存并应用</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <!-- ============ LDAP ============ -->
      <el-tab-pane label="LDAP" name="ldap">
        <el-card shadow="never">
          <template #header>LDAP 服务配置</template>
          <el-form label-width="160px" style="max-width: 760px">
            <el-row :gutter="12">
              <el-col :span="16">
                <el-form-item label="服务地址">
                  <el-input v-model="ldap.host" placeholder="ldap.example.com" />
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="端口" label-width="60px">
                  <el-input-number v-model="ldap.port" :min="1" :max="65535" :controls="false" />
                </el-form-item>
              </el-col>
            </el-row>
            <el-row :gutter="12">
              <el-col :span="12">
                <el-form-item label="协议版本">
                  <el-select v-model="ldap.version" style="width: 100%">
                    <el-option label="LDAP v2" :value="2" />
                    <el-option label="LDAP v3" :value="3" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="启用">
                  <el-switch v-model="ldap.enabled" />
                </el-form-item>
              </el-col>
            </el-row>
            <el-form-item label="管理员 DN">
              <el-input v-model="ldap.bindDn" placeholder="cn=admin,dc=example,dc=com" />
            </el-form-item>
            <el-form-item label="管理员密码">
              <el-input v-model="ldap.bindPassword" type="password" show-password />
            </el-form-item>
            <el-divider content-position="left">用户</el-divider>
            <el-form-item label="用户 Base DN">
              <el-input v-model="ldap.userBaseDn" placeholder="ou=users,dc=example,dc=com" />
            </el-form-item>
            <el-form-item label="用户过滤">
              <el-input v-model="ldap.userSearchFilter" placeholder="(objectClass=inetOrgPerson)" />
            </el-form-item>
            <el-form-item label="用户属性">
              <el-input v-model="ldap.userAttributes" placeholder="uid" />
            </el-form-item>
            <el-divider content-position="left">分组</el-divider>
            <el-form-item label="分组 Base DN">
              <el-input v-model="ldap.groupBaseDn" placeholder="ou=groups,dc=example,dc=com" />
            </el-form-item>
            <el-form-item label="分组过滤">
              <el-input v-model="ldap.groupSearchFilter" placeholder="(objectClass=posixGroup)" />
            </el-form-item>
            <el-form-item label="分组属性">
              <el-input v-model="ldap.groupAttributes" placeholder="cn" />
            </el-form-item>
            <el-form-item label="成员属性">
              <el-input v-model="ldap.groupsToUserAttribute" placeholder="memberUid" />
            </el-form-item>
            <el-form-item label="成员值属性">
              <el-input v-model="ldap.groupsToUserAttributeValue" placeholder="uid" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveLdap">保存配置</el-button>
              <el-button @click="ldapTest('connection')">测试连接</el-button>
              <el-button @click="ldapTest('user')">测试用户</el-button>
              <el-button @click="ldapTest('group')">测试分组</el-button>
              <el-button @click="ldapList('user')">用户列表</el-button>
              <el-button @click="ldapList('group')">分组列表</el-button>
              <el-button type="warning" @click="ldapSync('all')">同步用户+分组</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-dialog v-model="ldapVisible" :title="ldapTitle" width="620px" top="8vh">
          <pre v-if="ldapPre" class="log-box">{{ ldapResult }}</pre>
          <el-table v-else :data="ldapRows" size="small" max-height="420" border>
            <el-table-column prop="value" :label="ldapTitle" show-overflow-tooltip />
          </el-table>
          <template #footer>
            <el-button size="small" type="primary" @click="ldapVisible = false">关闭</el-button>
          </template>
        </el-dialog>
      </el-tab-pane>

      <!-- ============ 邮件服务 ============ -->
      <el-tab-pane label="邮件服务" name="mail">
        <el-card shadow="never">
          <template #header>SMTP 配置</template>
          <el-form label-width="140px" style="max-width: 700px">
            <el-row :gutter="12">
              <el-col :span="16">
                <el-form-item label="SMTP 服务器">
                  <el-input v-model="mail.host" placeholder="smtp.example.com" />
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="端口" label-width="60px">
                  <el-input-number v-model="mail.port" :min="1" :max="65535" :controls="false" />
                </el-form-item>
              </el-col>
            </el-row>
            <el-row :gutter="12">
              <el-col :span="8">
                <el-form-item label="加密方式">
                  <el-select v-model="mail.encryption" style="width: 100%">
                    <el-option label="无" value="none" />
                    <el-option label="SSL" value="SSL" />
                    <el-option label="TLS" value="TLS" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="自动 TLS">
                  <el-switch v-model="mail.autoTls" />
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="启用服务">
                  <el-switch v-model="mail.status" />
                </el-form-item>
              </el-col>
            </el-row>
            <el-row :gutter="12">
              <el-col :span="12">
                <el-form-item label="发信账号">
                  <el-input v-model="mail.user" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="发信密码" label-width="90px">
                  <el-input v-model="mail.pass" type="password" show-password />
                </el-form-item>
              </el-col>
            </el-row>
            <el-form-item label="发件人">
              <el-input v-model="mail.from" placeholder="SVNAdmin <noreply@example.com>" />
            </el-form-item>
            <el-form-item label="收件人">
              <el-select
                v-model="mail.to"
                multiple
                filterable
                allow-create
                default-first-option
                style="width: 100%"
                placeholder="输入邮箱后回车添加"
              />
            </el-form-item>
            <el-form-item label="超时(秒)">
              <el-input-number v-model="mail.timeout" :min="1" :max="120" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveMail">保存</el-button>
              <el-button @click="sendTest">发送测试邮件</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-tab-pane>

      <!-- ============ 消息推送 ============ -->
      <el-tab-pane label="消息推送" name="push">
        <el-card shadow="never">
          <template #header>推送触发点（按邮件方式送达，需先在【邮件服务】配置 SMTP）</template>
          <el-table :data="pushList" border size="small">
            <el-table-column prop="note" label="触发场景" min-width="300" />
            <el-table-column prop="trigger" label="标识" min-width="220" />
            <el-table-column label="启用" width="100">
              <template #default="{ row }">
                <el-switch v-model="row.enable" />
              </template>
            </el-table-column>
          </el-table>
          <div style="margin-top: 12px">
            <el-button type="primary" @click="savePush">保存</el-button>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ============ 安全配置 ============ -->
      <el-tab-pane label="安全配置" name="safe">
        <el-card shadow="never">
          <template #header>安全开关</template>
          <el-table :data="safeList" border size="small">
            <el-table-column prop="note" label="说明" min-width="340" />
            <el-table-column prop="name" label="标识" min-width="200" />
            <el-table-column label="启用" width="100">
              <template #default="{ row }">
                <el-switch v-model="row.enable" />
              </template>
            </el-table-column>
          </el-table>
          <div style="margin-top: 12px">
            <el-button type="primary" @click="saveSafe">保存</el-button>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ============ 关于与更新 ============ -->
      <el-tab-pane label="关于与更新" name="about">
        <el-card shadow="never">
          <template #header>版本信息</template>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="当前版本">
              <el-tag type="success" size="small">v{{ version.version || '-' }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="Subversion">{{ version.svnVersion || '-' }}</el-descriptions-item>
            <el-descriptions-item label="Go 运行时">{{ version.goVersion || '-' }}</el-descriptions-item>
            <el-descriptions-item label="运行平台">{{ version.platform || '-' }}</el-descriptions-item>
            <el-descriptions-item label="参考项目" :span="2">
              <el-link :href="version.repository" target="_blank" type="primary">
                {{ version.repository }}
              </el-link>
            </el-descriptions-item>
          </el-descriptions>
          <div style="margin-top: 12px">
            <el-button type="primary" :loading="checking" @click="checkUpdate">检测更新</el-button>
          </div>
        </el-card>

        <el-card v-if="update.checked" shadow="never" style="margin-top: 12px">
          <template #header>更新检测结果</template>
          <el-alert
            v-if="update.message"
            :closable="false"
            type="info"
            show-icon
            :title="update.message"
            style="margin-bottom: 12px"
          />
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="最新版本">
              <el-tag :type="update.hasUpdate ? 'warning' : 'success'" size="small">
                {{ update.latest || update.current }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="发布时间">{{ update.publishedAt || '-' }}</el-descriptions-item>
            <el-descriptions-item label="标题" :span="2">{{ update.title || '-' }}</el-descriptions-item>
          </el-descriptions>

          <template v-if="update.fixed?.length">
            <el-divider content-position="left">修复</el-divider>
            <ul class="chg">
              <li v-for="(x, i) in update.fixed" :key="i">{{ x }}</li>
            </ul>
          </template>
          <template v-if="update.added?.length">
            <el-divider content-position="left">新增</el-divider>
            <ul class="chg">
              <li v-for="(x, i) in update.added" :key="i">{{ x }}</li>
            </ul>
          </template>
          <template v-if="update.removed?.length">
            <el-divider content-position="left">移除</el-divider>
            <ul class="chg">
              <li v-for="(x, i) in update.removed" :key="i">{{ x }}</li>
            </ul>
          </template>

          <div v-if="update.download || update.url" style="margin-top: 12px">
            <el-button v-if="update.download" type="primary" @click="openUrl(update.download)">
              下载最新版
            </el-button>
            <el-button v-if="update.url" @click="openUrl(update.url)">发布说明</el-button>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ============ 配置项 ============ -->
      <el-tab-pane label="配置项" name="option">
        <el-table :data="options" size="small" border max-height="560">
          <el-table-column prop="optionName" label="名称" min-width="180" />
          <el-table-column prop="optionValue" label="值" min-width="200">
            <template #default="{ row }">
              <el-input
                v-if="editing === row.optionId"
                v-model="row.optionValue"
                size="small"
                @keyup.enter="saveOption(row)"
              />
              <span v-else class="path-text">{{ row.optionValue }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="130">
            <template #default="{ row }">
              <el-button v-if="editing !== row.optionId" link type="primary" @click="editing = row.optionId">
                编辑
              </el-button>
              <template v-else>
                <el-button link type="primary" @click="saveOption(row)">保存</el-button>
                <el-button link @click="loadOptions">取消</el-button>
              </template>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, RefreshRight } from '@element-plus/icons-vue'
import {
  apiApacheInfo,
  apiCheckUpdate,
  apiDirInfo,
  apiDockerHostInfo,
  apiDockerHostUpdate,
  apiHttpPortUpdate,
  apiHttpPrefixUpdate,
  apiLdapGroups,
  apiLdapSync,
  apiLdapTest,
  apiLdapUsers,
  apiMailInfo,
  apiMailTest,
  apiMailUpdate,
  apiOptionList,
  apiOptionUpdate,
  apiPushInfo,
  apiPushUpdate,
  apiSafeInfo,
  apiSafeUpdate,
  apiSaslInfo,
  apiSaslStart,
  apiSaslStop,
  apiSubversionEnable,
  apiSvnEnable,
  apiSvnInfo,
  apiSvnserveHostUpdate,
  apiSvnservePortUpdate,
  apiSvnserveStart,
  apiSvnserveStop,
  apiSyncAll,
  apiUsersourceUpdate,
  apiVersionInfo
} from '@/api'

const tab = ref('host')

const dirInfo = ref<any>({})
const dirFields = [
  { key: 'homePath', label: 'SVN 主目录' },
  { key: 'repBasePath', label: '仓库父目录' },
  { key: 'authzFile', label: '授权文件 authz' },
  { key: 'passwdFile', label: 'SVN 用户文件 passwd' },
  { key: 'httpPasswd', label: 'HTTP 用户文件 httpPasswd' },
  { key: 'svnConfFile', label: 'svnserve.conf' },
  { key: 'backupPath', label: '备份目录' },
  { key: 'crondPath', label: '定时任务目录' },
  { key: 'logPath', label: '日志目录' },
  { key: 'templetePath', label: '仓库模板目录' },
  { key: 'hookPath', label: '推荐钩子目录' },
  { key: 'saslPath', label: 'SASL 目录' },
  { key: 'binPath', label: 'SVN 命令目录' },
  { key: 'configFile', label: '配置文件' },
  { key: 'database', label: '数据库' }
]

const svnInfo = ref<any>({})
const apache = ref<any>({})
const sasl = ref<any>({})
const version = ref<any>({})
const update = ref<any>({ checked: false })
const checking = ref(false)

const dockerHost = reactive<any>({ host: '127.0.0.1', svnPort: 3690, httpPort: 80 })
const svnForm = reactive<any>({ host: '0.0.0.0', port: 3690 })
const httpForm = reactive<any>({ prefix: '/svn', port: 80 })
const svnSource = reactive<any>({ userSource: 'passwd', groupSource: 'authz', ldap: null })
const httpSource = reactive<any>({ userSource: 'passwd', groupSource: 'authz', ldap: null })

const mail = reactive<any>({
  host: '',
  port: 465,
  encryption: 'SSL',
  autoTls: true,
  auth: true,
  user: '',
  pass: '',
  from: '',
  to: [] as string[],
  timeout: 10,
  status: false
})
const pushList = ref<any[]>([])
const safeList = ref<any[]>([])

const ldap = reactive<any>({
  host: '',
  port: 389,
  version: 3,
  bindDn: '',
  bindPassword: '',
  enabled: false,
  userBaseDn: '',
  userSearchFilter: '',
  userAttributes: '',
  groupBaseDn: '',
  groupSearchFilter: '',
  groupAttributes: '',
  groupsToUserAttribute: '',
  groupsToUserAttributeValue: ''
})
const ldapVisible = ref(false)
const ldapTitle = ref('')
const ldapResult = ref('')
const ldapPre = ref(false)
const ldapRows = ref<any[]>([])

const options = ref<any[]>([])
const editing = ref(0)

/* -------------------- 加载 -------------------- */

const loadAll = async () => {
  await Promise.all([loadDir(), loadSvn(), loadApache(), loadMail(), loadPush(), loadSafe(), loadOptions(), loadVersion()])
}

const loadDir = async () => {
  dirInfo.value = await apiDirInfo()
}

const loadSvn = async () => {
  const res = await apiSvnInfo()
  svnInfo.value = res
  sasl.value = res.sasl || {}
  svnForm.host = res.svnserveHost || '0.0.0.0'
  svnForm.port = res.svnservePort || 3690
  httpForm.prefix = res.httpPrefix || '/svn'
  httpForm.port = res.httpPort?.httpPort || res.httpPort?.port || (res.dockerHost?.httpPort ?? 80)
  const dh = res.dockerHost || {}
  dockerHost.host = dh.host || '127.0.0.1'
  dockerHost.svnPort = dh.svnPort || res.svnservePort || 3690
  dockerHost.httpPort = dh.httpPort || 80
  const ds = res.dataSource || {}
  svnSource.userSource = ds.userSource || 'passwd'
  svnSource.groupSource = ds.groupSource || 'authz'
  svnSource.ldap = ds.ldap || null
  const hds = res.httpDataSource || {}
  httpSource.userSource = hds.userSource || 'passwd'
  httpSource.groupSource = hds.groupSource || 'authz'
  httpSource.ldap = hds.ldap || null
  Object.assign(ldap, defaultLdap(), pickLdap(svnSource.ldap || httpSource.ldap))
}

const loadApache = async () => {
  apache.value = await apiApacheInfo()
}

const loadMail = async () => {
  const res = await apiMailInfo()
  Object.assign(mail, { ...defaultMail(), ...(res || {}) })
  if (!Array.isArray(mail.to)) mail.to = []
}

const loadPush = async () => {
  pushList.value = await apiPushInfo()
}

const loadSafe = async () => {
  safeList.value = await apiSafeInfo()
}

const loadOptions = async () => {
  options.value = await apiOptionList()
  editing.value = 0
}

const loadVersion = async () => {
  version.value = await apiVersionInfo()
}

const defaultMail = () => ({
  host: '',
  port: 465,
  encryption: 'SSL',
  autoTls: true,
  auth: true,
  user: '',
  pass: '',
  from: '',
  to: [] as string[],
  timeout: 10,
  status: false
})

const defaultLdap = () => ({
  host: '',
  port: 389,
  version: 3,
  bindDn: '',
  bindPassword: '',
  enabled: false,
  userBaseDn: '',
  userSearchFilter: '',
  userAttributes: '',
  groupBaseDn: '',
  groupSearchFilter: '',
  groupAttributes: '',
  groupsToUserAttribute: '',
  groupsToUserAttributeValue: ''
})

const pickLdap = (src: any) => {
  if (!src) return {}
  const out: any = {}
  Object.keys(defaultLdap()).forEach((k) => {
    if (src[k] !== undefined && src[k] !== null) out[k] = src[k]
  })
  return out
}

/* -------------------- 主机 / svnserve -------------------- */

const saveDockerHost = async () => {
  await apiDockerHostUpdate(dockerHost)
  ElMessage.success('已保存')
  loadSvn()
}

const svnserveStart = async () => {
  await apiSvnserveStart()
  ElMessage.success('svnserve 已启动')
  setTimeout(loadSvn, 800)
}

const svnserveStop = async () => {
  await apiSvnserveStop()
  ElMessage.success('svnserve 已停止')
  setTimeout(loadSvn, 500)
}

const saveSvnHost = async () => {
  await apiSvnserveHostUpdate(svnForm.host)
  ElMessage.success('已保存，需重启 svnserve 生效')
  loadSvn()
}

const saveSvnPort = async () => {
  await apiSvnservePortUpdate(svnForm.port)
  ElMessage.success('已保存，需重启 svnserve 生效')
  loadSvn()
}

const syncAll = async () => {
  await apiSyncAll()
  ElMessage.success('用户 / 分组 / 仓库已同步')
}

/* -------------------- 数据源 -------------------- */

const buildSource = (s: any) => ({ userSource: s.userSource, groupSource: s.groupSource, ldap: { ...ldap, enabled: true } })

const saveSvnSource = async () => {
  try {
    await ElMessageBox.confirm('切换来源会重建 SVN 协议下的用户/分组数据，确认继续？', '提示', { type: 'warning' })
  } catch {
    return
  }
  await apiUsersourceUpdate('svn', buildSource(svnSource))
  ElMessage.success('已保存')
  loadSvn()
}

const saveHttpSource = async () => {
  try {
    await ElMessageBox.confirm('切换来源会重建 HTTP 协议下的用户/分组数据，确认继续？', '提示', { type: 'warning' })
  } catch {
    return
  }
  await apiUsersourceUpdate('http', buildSource(httpSource))
  ElMessage.success('已保存')
  loadSvn()
}

/* -------------------- HTTP -------------------- */

const saveHttpPrefix = async () => {
  await apiHttpPrefixUpdate(httpForm.prefix)
  ElMessage.success('已保存')
  loadSvn()
}

const saveHttpPort = async () => {
  await apiHttpPortUpdate(httpForm.port)
  ElMessage.success('已保存')
  loadSvn()
}

const enableHttp = async () => {
  try {
    await ElMessageBox.confirm('将写入 Apache 配置并重载服务，确认启用 HTTP 协议检出？', '提示', { type: 'warning' })
  } catch {
    return
  }
  await apiSubversionEnable(httpForm.prefix)
  ElMessage.success('已启用 HTTP 协议检出')
  loadSvn()
  loadApache()
}

const enableSvn = async () => {
  try {
    await ElMessageBox.confirm('将移除 subversion 配置并重载 Apache，确认切换回 SVN 协议？', '提示', { type: 'warning' })
  } catch {
    return
  }
  await apiSvnEnable()
  ElMessage.success('已切换回 SVN 协议检出')
  loadSvn()
  loadApache()
}

/* -------------------- SASL -------------------- */

const saslStart = async () => {
  await apiSaslStart()
  ElMessage.success('saslauthd 已启动')
  setTimeout(loadSvn, 800)
}

const saslStop = async () => {
  await apiSaslStop()
  ElMessage.success('saslauthd 已停止')
  setTimeout(loadSvn, 500)
}

/* -------------------- LDAP -------------------- */

const saveLdap = async () => {
  await apiUsersourceUpdate('svn', { ...buildSource(svnSource), ldap: { ...ldap, enabled: ldap.enabled } })
  ElMessage.success('LDAP 配置已保存')
  loadSvn()
}

const ldapTest = async (type: string) => {
  const res = await apiLdapTest({ type, config: ldap })
  ldapTitle.value = { connection: '连接测试结果', user: '用户测试结果', group: '分组测试结果' }[type] || '测试结果'
  if (typeof res === 'string') {
    ldapPre.value = true
    ldapResult.value = res
  } else {
    ldapPre.value = true
    ldapResult.value = JSON.stringify(res, null, 2)
  }
  ldapVisible.value = true
}

const ldapList = async (kind: string) => {
  if (kind === 'user') {
    const users = await apiLdapUsers(ldap)
    ldapTitle.value = `LDAP 用户（${users.length}）`
    ldapRows.value = users.map((u: any) => ({ value: String(u) }))
  } else {
    const groups = await apiLdapGroups(ldap)
    ldapTitle.value = `LDAP 分组（${groups.length}）`
    ldapRows.value = groups.map((g: any) => ({ value: typeof g === 'string' ? g : JSON.stringify(g) }))
  }
  ldapPre.value = false
  ldapVisible.value = true
}

const ldapSync = async (kind: string) => {
  try {
    await ElMessageBox.confirm('将按 LDAP 数据重建本地用户/分组，确认同步？', '提示', { type: 'warning' })
  } catch {
    return
  }
  const res = await apiLdapSync(kind)
  ElMessage.success(`同步完成：用户 ${res.userCount ?? 0} 个，分组 ${res.groupCount ?? 0} 个`)
}

/* -------------------- 邮件 / 推送 / 安全 -------------------- */

const saveMail = async () => {
  await apiMailUpdate({ ...mail })
  ElMessage.success('已保存')
  loadMail()
}

const sendTest = async () => {
  await apiMailTest(mail.to?.[0])
  ElMessage.success('测试邮件已发送，请查收')
}

const savePush = async () => {
  await apiPushUpdate(pushList.value)
  ElMessage.success('已保存')
}

const saveSafe = async () => {
  await apiSafeUpdate(safeList.value)
  ElMessage.success('已保存')
}

/* -------------------- 配置项 / 更新 -------------------- */

const saveOption = async (row: any) => {
  await apiOptionUpdate({ optionName: row.optionName, optionValue: row.optionValue })
  ElMessage.success('已保存')
  editing.value = 0
  loadOptions()
}

const checkUpdate = async () => {
  checking.value = true
  try {
    update.value = await apiCheckUpdate()
  } finally {
    checking.value = false
  }
}

const openUrl = (u: string) => window.open(u, '_blank')

onMounted(loadAll)
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
.path-text {
  font-family: Consolas, Menlo, monospace;
  font-size: 12px;
  word-break: break-all;
}
.log-box {
  margin: 0;
  padding: 10px 12px;
  max-height: 50vh;
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
.chg {
  margin: 0 0 8px 18px;
  padding: 0;
  font-size: 13px;
  line-height: 1.8;
  color: var(--el-text-color-regular);
}
</style>
