#!/usr/bin/env bash
# SVNAdmin4 后端接口冒烟测试
# 用法: bash scripts/smoke.sh [端口]
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${1:-8080}"
# 绕过可能的 HTTP 代理，确保直连本机服务
export no_proxy='*' NO_PROXY='*'
BASE="http://127.0.0.1:${PORT}"
BIN="${ROOT}/build/svnadmin4"
LOG="${ROOT}/.tmp/server-smoke.log"

fail_count=0
pass_count=0

check() { # check <名称> <期望包含的字符串> <实际输出>
  local name="$1" expect="$2" actual="$3"
  if [[ "$actual" == *"$expect"* ]]; then
    pass_count=$((pass_count + 1))
    echo "  [PASS] ${name}"
  else
    fail_count=$((fail_count + 1))
    echo "  [FAIL] ${name}"
    echo "         期望包含: ${expect}"
    echo "         实际输出: ${actual}"
  fi
}

req() { # req <方法> <路径> [json]
  local m="$1" p="$2" d="${3:-}"
  if [[ -n "$d" ]]; then
    curl -s -X "$m" "${BASE}${p}" -H 'Content-Type: application/json' -H "X-Token: ${TOKEN}" -d "$d"
  else
    curl -s -X "$m" "${BASE}${p}" -H "X-Token: ${TOKEN}"
  fi
}

echo "== 启动服务 =="
mkdir -p "$(dirname "${LOG}")"
"${BIN}" -c "${ROOT}/server/configs/config.yaml" -web "" >"${LOG}" 2>&1 &
SRV=$!
trap 'kill ${SRV} 2>/dev/null' EXIT
sleep 2

echo "== 登录 =="
CODE_JSON=$(curl -s "${BASE}/api/common/verifyCode")
UUID=$(echo "$CODE_JSON" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["uuid"])')
CODE=$(echo "$CODE_JSON" | python3 -c 'import sys,json,re;s=json.load(sys.stdin)["data"]["svg"];print("".join(re.findall(r"<text[^>]*>(.)</text>",s)))')
LOGIN=$(curl -s -X POST "${BASE}/api/common/login" -H 'Content-Type: application/json' \
  -d "{\"userName\":\"admin\",\"password\":\"admin\",\"uuid\":\"${UUID}\",\"code\":\"${CODE}\"}")
check "管理员登录" '"role":"admin"' "$LOGIN"
TOKEN=$(echo "$LOGIN" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["token"])' 2>/dev/null || echo "")

echo "== 用户管理 =="
check "新建用户 u1" '"status":true' "$(req POST /api/user/create '{"svnUserName":"u1","svnUserPass":"pass1","svnUserNote":"测试用户"}')"
check "新建用户 u2" '"status":true' "$(req POST /api/user/create '{"svnUserName":"u2","svnUserPass":"pass2"}')"
check "用户列表含 u1" 'u1' "$(req GET '/api/user/list?page=1&pageSize=10')"
check "修改用户备注" '"status":true' "$(req POST /api/user/updNote '{"svnUserName":"u1","svnUserNote":"备注已改"}')"
check "修改用户密码" '"status":true' "$(req POST /api/user/updPass '{"svnUserName":"u1","svnUserPass":"newpass"}')"
check "禁用用户 u2" '"status":true' "$(req POST /api/user/updStatus '{"svnUserName":"u2","svnUserStatus":0}')"
check "启用用户 u2" '"status":true' "$(req POST /api/user/updStatus '{"svnUserName":"u2","svnUserStatus":1}')"

echo "== 分组管理 =="
check "新建分组 dev" '"status":true' "$(req POST /api/group/create '{"svnGroupName":"dev","svnGroupNote":"研发组"}')"
check "分组加入成员" '"status":true' "$(req POST /api/group/updMember '{"svnGroupName":"dev","users":["u1","u2"]}')"
check "分组成员含 u1" 'u1' "$(req GET '/api/group/member?svnGroupName=dev')"
check "分组列表含 dev" 'dev' "$(req GET '/api/group/list?page=1&pageSize=10')"

echo "== 仓库管理 =="
check "新建仓库 demo(套用模板01)" '"status":true' "$(req POST /api/rep/create '{"repName":"demo","repNote":"演示仓库","initStruct":"01"}')"
check "仓库列表含 demo" 'demo' "$(req GET '/api/rep/list?page=1&pageSize=10')"
check "模板生成 trunk" 'trunk' "$(req GET '/api/rep/tree?repName=demo&path=/')"
check "模板生成 branches" 'branches' "$(req GET '/api/rep/tree?repName=demo&path=/')"
check "模板生成 tags" 'tags' "$(req GET '/api/rep/tree?repName=demo&path=/')"
check "在线创建目录 /docs" '"status":true' "$(req POST /api/rep/createFolder '{"repName":"demo","path":"/docs","message":"init"}')"
check "目录树含 trunk" 'trunk' "$(req GET '/api/rep/tree?repName=demo&path=/')"
check "仓库属性含 UUID" 'repUuid' "$(req GET '/api/rep/detail?repName=demo')"
check "检出地址" 'svn://' "$(req GET '/api/rep/checkout?repName=demo')"

echo "== 权限管理 =="
check "授权 @dev 读写 /trunk" '"status":true' "$(req POST /api/pri/create '{"repName":"demo","priPath":"/trunk","objectType":"group","objectName":"dev","perm":"rw"}')"
check "授权 u1 只读 /" '"status":true' "$(req POST /api/pri/create '{"repName":"demo","priPath":"/","objectType":"user","objectName":"u1","perm":"r"}')"
check "权限列表含 @dev" '@dev' "$(req GET '/api/pri/list?repName=demo&priPath=/trunk')"
check "修改权限为 r" '"status":true' "$(req POST /api/pri/update '{"repName":"demo","priPath":"/trunk","objectName":"@dev","perm":"r"}')"
check "删除权限" '"status":true' "$(req POST /api/pri/delete '{"repName":"demo","priPath":"/trunk","objectName":"@dev"}')"
check "用户可访问仓库" 'demo' "$(req GET '/api/rep/userList2?svnUserName=u1')"

echo "== 备份 =="
DUMP=$(req POST /api/backup/dump '{"repName":"demo"}')
check "发起备份任务" 'taskUnique' "$DUMP"
sleep 3
check "备份文件列表" '.dump' "$(req GET '/api/backup/list')"
check "任务列表" 'taskUnique' "$(req GET '/api/task/list')"

echo "== 钩子 =="
check "钩子列表含 post-commit" 'post-commit' "$(req GET '/api/hook/list?repName=demo')"
check "写入 post-commit" '"status":true' "$(req POST /api/hook/update '{"repName":"demo","hookName":"post-commit","content":"#!/bin/bash\nexit 0"}')"

echo "== 系统 =="
check "统计概览" 'repCount' "$(req GET '/api/common/statistics')"
check "日志列表" 'logId' "$(req GET '/api/log/list?page=1&pageSize=10')"
check "配置项列表" 'optionName' "$(req GET '/api/setting/list')"
check "authz 校验" '"status":true' "$(req GET '/api/rep/checkAuthz')"
check "定时任务列表" '[' "$(req GET '/api/crond/list')"

echo "== 清理 =="
check "删除仓库 demo" '"status":true' "$(req POST /api/rep/del '{"repName":"demo"}')"
check "删除分组 dev" '"status":true' "$(req POST /api/group/del '{"svnGroupName":"dev"}')"
check "删除用户 u1" '"status":true' "$(req POST /api/user/del '{"svnUserName":"u1"}')"
check "删除用户 u2" '"status":true' "$(req POST /api/user/del '{"svnUserName":"u2"}')"
check "注销" '"status":true' "$(req POST /api/common/logout)"

echo
echo "================ 结果 ================"
echo "通过: ${pass_count}   失败: ${fail_count}"
exit $((fail_count > 0 ? 1 : 0))
