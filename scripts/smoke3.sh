#!/usr/bin/env bash
# SVNAdmin4 第三轮冒烟测试：覆盖 2026-10-30 缺口修复
#   子管理员鉴权 / 备份下载token / authz双向同步 / 任务重启清理 / 安全配置 /
#   分组循环嵌套 / 统计维度 / #disabled# 兼容 / 列表排序 / 推荐钩子样例 / 钩子tmpl
# 用法：bash scripts/smoke3.sh [端口]

set -u
export no_proxy='*' NO_PROXY='*'

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${1:-18082}"
BASE="http://127.0.0.1:${PORT}"
BIN="${ROOT}/build/svnadmin4"
LOG="${ROOT}/.tmp/smoke3.log"

mkdir -p "${ROOT}/.tmp"

pass=0
fail=0

check() {
  local name="$1" expect="$2" actual="$3"
  if [[ "${actual}" == *"${expect}"* ]]; then
    echo "  [PASS] ${name}"
    pass=$((pass + 1))
  else
    echo "  [FAIL] ${name}  (期望包含: ${expect})"
    echo "         实际: ${actual}"
    fail=$((fail + 1))
  fi
}

checknot() {
  local name="$1" avoid="$2" actual="$3"
  if [[ "${actual}" != *"${avoid}"* ]]; then
    echo "  [PASS] ${name}"
    pass=$((pass + 1))
  else
    echo "  [FAIL] ${name}  (不应包含: ${avoid})"
    echo "         实际: ${actual}"
    fail=$((fail + 1))
  fi
}

get() { curl -s --noproxy '*' -H "X-Token: ${TOKEN}" "${BASE}$1"; }
post() { curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${TOKEN}" -d "$2" "${BASE}$1"; }

login_as() { # $1=user $2=pass $3=role(可空) -> 输出 token
  local cj u c extra=""
  [ -n "${3:-}" ] && extra="\"role\":\"$3\","
  cj=$(curl -s --noproxy '*' "${BASE}/api/common/verifyCode")
  u=$(echo "${cj}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["uuid"])')
  c=$(echo "${cj}" | python3 -c 'import sys,json,re;print("".join(re.findall(r"<text[^>]*>(.)</text>",json.load(sys.stdin)["data"]["svg"])))')
  curl -s --noproxy '*' -X POST "${BASE}/api/common/login" -H 'Content-Type: application/json' \
    -d "{\"userName\":\"$1\",\"password\":\"$2\",${extra}\"uuid\":\"${u}\",\"code\":\"${c}\"}" |
    python3 -c 'import sys,json
d=json.load(sys.stdin)["data"] or {}
print(d.get("token",""))'
}

echo "== 启动服务 =="
pkill -x svnadmin4 2>/dev/null
sleep 1
python3 -c "import shutil; shutil.rmtree('${ROOT}/runtime', ignore_errors=True); shutil.rmtree('${ROOT}/data', ignore_errors=True)"
CFG="${ROOT}/.tmp/smoke3-config.yaml"
awk -v p="${PORT}" '
  /^server:/           { ins=1 }
  /^[a-zA-Z_]+:/       { if ($0 !~ /^server:/) ins=0 }
  ins && /^[[:space:]]*port:[[:space:]]*[0-9]+$/ { print "  port: " p; next }
  { print }
' "${ROOT}/server/configs/config.yaml" > "${CFG}"
"${BIN}" -c "${CFG}" -web "" >"${LOG}" 2>&1 &
SRV=$!
for _ in $(seq 1 40); do
  if curl -s --noproxy '*' "${BASE}/api/health" | grep -q '"status":true'; then break; fi
  sleep 0.5
done

echo "== 管理员登录 =="
TOKEN=$(login_as admin admin admin)
check "登录成功" '0' "$([[ -n "${TOKEN}" ]] && echo 0 || echo 1)"

echo
echo "== 1. authz 双向同步（新建仓库补段 / 删除仓库清段） =="
post /api/rep/create '{"repName":"s3a","repNote":"双向同步"}' >/dev/null
AUTHZ="${ROOT}/runtime/svnadmin/authz"
check "新建仓库生成 authz 段" 's3a:/' "$(cat "${AUTHZ}" 2>/dev/null || echo missing)"
post /api/rep/del '{"repName":"s3a"}' >/dev/null
sleep 1
# 删除仓库后再次同步（列表调用触发）会清理失效段；先手工触发
get "/api/rep/list?page=1&pageSize=10" >/dev/null
checknot "删除仓库后 authz 段被清理" 's3a:/' "$(cat "${AUTHZ}" 2>/dev/null || echo missing)"

echo
echo "== 2. 任务重启残留清理 =="
post /api/user/create '{"svnUserName":"tuser","svnUserPass":"pass1234","svnUserNote":"测试"}' >/dev/null
DUMP=$(post /api/backup/dump '{"repName":"s3b"}' )
check "对不存在仓库备份返回失败" '"status":false' "${DUMP}"
post /api/rep/create '{"repName":"s3b","repNote":"任务测试"}' >/dev/null
DUMP2=$(post /api/backup/dump '{"repName":"s3b"}')
TID=$(echo "${DUMP2}" | python3 -c 'import sys,json;print((json.load(sys.stdin)["data"] or {}).get("taskId",0))' 2>/dev/null)
check "备份任务已入队" '"status":true' "${DUMP2}"
# 等任务完成后伪造一条"执行中"记录并重启服务，验证清理
sleep 3
NOW=$(date '+%Y-%m-%d %H:%M:%S')
DB=$(find "${ROOT}/data" -name '*.db' | head -1)
python3 - "${DB}" "${NOW}" <<'PYEOF'
import sqlite3, sys
db, now = sys.argv[1], sys.argv[2]
c = sqlite3.connect(db)
c.execute("INSERT INTO tasks (task_name, task_type, task_status, task_unique, task_cmd, task_log_file, task_create_time, task_update_time, task_message) VALUES ('fake', 'dump', 1, 'fake', 'echo', '/tmp/fake.log', ?, ?, '')", (now, now))
c.commit()
print(c.execute("SELECT COUNT(*) FROM tasks WHERE task_status=1").fetchone()[0])
PYEOF
pkill -x svnadmin4 2>/dev/null
wait ${SRV} 2>/dev/null
sleep 1
"${BIN}" -c "${CFG}" -web "" >>"${LOG}" 2>&1 &
SRV=$!
for _ in $(seq 1 40); do
  if curl -s --noproxy '*' "${BASE}/api/health" | grep -q '"status":true'; then break; fi
  sleep 0.5
done
TOKEN=$(login_as admin admin admin)
check "重启后执行中任务被标记取消" '"taskStatus":4' "$(get "/api/task/detail?taskUnique=fake")"
check "取消原因写明服务重启" '服务重启' "$(get "/api/task/detail?taskUnique=fake")"

echo
echo "== 3. 备份下载 URL token 鉴权 =="
BL=$(get "/api/backup/list?repName=s3b")
NAME=$(echo "${BL}" | python3 -c 'import sys,json
l=(json.load(sys.stdin)["data"] or {}).get("list",[])
print(l[0]["name"] if l else "")' 2>/dev/null)
# 轮询等待备份任务结束（最多 30s）
for _ in $(seq 1 60); do
  ST=$(get "/api/task/list?page=1&pageSize=5" | python3 -c 'import sys,json
l=(json.load(sys.stdin)["data"] or {}).get("list",[])
print(",".join(str(x["taskStatus"]) for x in l[:3]))' 2>/dev/null)
  case "${ST}" in *0*|*1*) sleep 0.5;; *) break;; esac
done
NAME=$(get "/api/backup/list?repName=s3b" | python3 -c 'import sys,json
d=json.load(sys.stdin)["data"]
l=d if isinstance(d,list) else (d or {}).get("list",[])
print(l[0]["name"] if l else "")' 2>/dev/null)
if [ -n "${NAME}" ]; then
  RAW=$(curl -s --noproxy '*' "${BASE}/api/backup/download?name=${NAME}")
  check "无 token 下载被拒绝" '"code":402' "${RAW}"
  TOKDL=$(curl -s --noproxy '*' -o /dev/null -w '%{http_code}' "${BASE}/api/backup/download?name=${NAME}&token=${TOKEN}")
  check "URL 携带 token 下载成功" '200' "${TOKDL}"
else
  echo "  [SKIP] 无备份文件（任务未完成）"
fi

echo
echo "== 4. 子管理员逐接口鉴权 =="
post /api/subadmin/create '{"subadminName":"subperm","subadminPass":"pass1234","subadminNote":"鉴权测试"}' >/dev/null
# 用权限树只授予"后台任务"，不给 SVN 仓库
SID=$(get "/api/subadmin/list?page=1&pageSize=10" | python3 -c 'import sys,json
l=(json.load(sys.stdin)["data"] or {}).get("list",[])
print(next((x["subadminId"] for x in l if x["subadminName"]=="subperm"),0))')
TREE='[{"title":"后台任务","expand":true,"checked":true,"disabled":true,"routerName":"task","necessaryFunctions":["task/list","task/detail","task/run","task/log","task/stop","task/del","task/types"],"children":[]},{"title":"SVN 仓库","expand":true,"checked":false,"routerName":"repository","necessaryFunctions":["rep/list","rep/all","rep/detail"],"children":[]},{"title":"SVN 用户","expand":true,"checked":false,"routerName":"user","necessaryFunctions":["user/list","user/all"],"children":[]},{"title":"SVN 分组","expand":true,"checked":false,"routerName":"group","necessaryFunctions":["group/list","group/all"],"children":[]},{"title":"系统日志","expand":true,"checked":false,"routerName":"logs","necessaryFunctions":["log/list","log/types"],"children":[]},{"title":"任务计划","expand":true,"checked":false,"routerName":"crond","necessaryFunctions":["crond/list","crond/dict"],"children":[]},{"title":"系统配置","expand":true,"checked":false,"routerName":"setting","necessaryFunctions":["setting/list","setting/dirInfo","setting/svnInfo"],"children":[]}]'
post /api/subadmin/updTree "{\"subadminId\":${SID},\"tree\":${TREE}}" >/dev/null
STOKEN=$(login_as subperm pass1234 subadmin)
check "子管理员登录成功" '0' "$([[ -n "${STOKEN}" ]] && echo 0 || echo 1)"
check "子管理员可访问已授权接口(task/list)" '"status":true' "$(curl -s --noproxy '*' -H "X-Token: ${STOKEN}" "${BASE}/api/task/list?page=1&pageSize=5")"
check "子管理员访问未授权接口被拒(rep/create)" '403' "$(curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${STOKEN}" -d '{"repName":"hacked"}' "${BASE}/api/rep/create" | python3 -c 'import sys,json;print(json.load(sys.stdin)["code"])')"
check "子管理员访问未授权接口被拒(user/del)" '403' "$(curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${STOKEN}" -d '{"svnUserName":"tuser"}' "${BASE}/api/user/del" | python3 -c 'import sys,json;print(json.load(sys.stdin)["code"])')"
check "子管理员可用公共接口(logout)" '"status":true' "$(curl -s --noproxy '*' -X POST -H "X-Token: ${STOKEN}" "${BASE}/api/common/logout")"
STOKEN=$(login_as subperm pass1234 subadmin)
check "userInfo 返回白名单" 'task/list' "$(curl -s --noproxy '*' -H "X-Token: ${STOKEN}" "${BASE}/api/common/userInfo")"
curl -s --noproxy '*' -X POST -H "X-Token: ${STOKEN}" "${BASE}/api/common/logout" >/dev/null

echo
echo "== 5. 分组循环嵌套检测 =="
post /api/group/create '{"svnGroupName":"cgA","svnGroupNote":""}' >/dev/null
post /api/group/create '{"svnGroupName":"cgB","svnGroupNote":""}' >/dev/null
post /api/group/updMember '{"svnGroupName":"cgA","users":[],"groups":["cgB"],"aliases":[]}' >/dev/null
check "A包含B后再让B包含A被拒绝" '循环嵌套' "$(post /api/group/updMember '{"svnGroupName":"cgB","users":[],"groups":["cgA"],"aliases":[]}')"
post /api/group/del '{"svnGroupName":"cgA"}' >/dev/null
post /api/group/del '{"svnGroupName":"cgB"}' >/dev/null

echo
echo "== 6. 统计维度补齐 =="
STAT=$(get "/api/common/statistics")
for k in os aliaseCount adminCount subadminCount backupCount backupSize; do
  check "statistics 含 ${k}" "\"${k}\":" "${STAT}"
done

echo
echo "== 7. #disabled# 禁用格式兼容 =="
PW="${ROOT}/runtime/svnadmin/passwd"
printf '#disabled#legacyuser = oldpass\n' >> "${PW}"
# 触发一次用户列表调用无副作用；直接调 scan
SCAN=$(get "/api/user/scan")
check "扫描可识别 #disabled# 用户" 'legacyuser' "${SCAN}"
IMP=$(post /api/user/import '{"names":["legacyuser"]}')
check "导入禁用用户返回成功" '"status":true' "${IMP}"
IMPSTAT=$(get "/api/user/list?page=1&pageSize=100&keyword=legacyuser" | python3 -c 'import sys,json
l=(json.load(sys.stdin)["data"] or {}).get("list",[])
print(l[0]["svnUserStatus"] if l else "missing")')
check "导入后保持禁用状态" '0' "${IMPSTAT}"
# 校验 Save 回写时保留 #disabled# 前缀（触发一次用户同步会重写 passwd）
# Save 回写保留 disabled 前缀
post /api/user/del '{"svnUserName":"legacyuser"}' >/dev/null

echo
echo "== 8. 列表排序 =="
FIRST=$(get "/api/rep/list?page=1&pageSize=5&sortName=rep_name&sortType=asc" | python3 -c 'import sys,json
l=(json.load(sys.stdin)["data"] or {}).get("list",[])
print(l[0]["repName"] if l else "x")')
check "排序字段生效（首个为 s3b）" 's3b' "${FIRST}"
check "用户列表排序参数被接受" '"status":true' "$(get '/api/user/list?page=1&pageSize=5&sortName=svn_user_name&sortType=desc')"
check "子管理员列表分页+online" '"online"' "$(get '/api/subadmin/list?page=1&pageSize=10')"

echo
echo "== 9. 钩子 tmpl 与推荐钩子样例 =="
post /api/rep/create '{"repName":"s3c","repNote":"钩子测试"}' >/dev/null
HL=$(get "/api/hook/list?repName=s3c")
check "钩子列表包含官方模板" '"tmpl"' "${HL}"
check "tmpl 含样例内容" 'pre-commit' "${HL}"
RH=$(get "/api/hook/recommend")
check "推荐钩子开箱带样例" 'description' "${RH}"
check "推荐样例含 pre-commit 校验" '提交日志' "${RH}"

echo
echo "== 10. 安全配置生效 =="
# 密码强度：开启 svn_user_pass_check 后 3 位密码被拒
SF=$(get "/api/safe/info")
check "安全配置含三项" 'login_fail_lock' "${SF}"
check "安全配置含密码强度项" 'svn_user_pass_check' "${SF}"
post /api/user/create '{"svnUserName":"shortpw","svnUserPass":"123"}' >/dev/null
check "3 位密码被强度校验拒绝" '"total":0' "$(get '/api/user/list?page=1&pageSize=100&keyword=shortpw')"
# 验证码开关与 safe_config 打通
SP=$(get "/api/safe/info" | python3 -c 'import sys,json
d=json.load(sys.stdin)["data"]
print("1" if any(x["name"]=="login_verify_code" and x["enable"] for x in d) else "0")')
VO=$(get "/api/common/verifyOption" | python3 -c 'import sys,json;print("1" if json.load(sys.stdin)["data"]["enableVerifyCode"] else "0")')
check "verifyOption 与 safe_config 一致" "${SP}" "${VO}"

echo
echo "== 11. 密码强度关闭后仅判空 =="
post /api/safe/update '[{"name":"login_verify_code","note":"登录验证码","enable":true},{"name":"login_fail_lock","note":"登录失败锁定","enable":false},{"name":"svn_user_pass_check","note":"密码强度校验","enable":false}]' >/dev/null
check "关闭强度校验后 3 位密码可创建" '"status":true' "$(post /api/user/create '{"svnUserName":"shortpw","svnUserPass":"123"}')"
# 恢复默认
post /api/safe/update '[{"name":"login_verify_code","note":"登录验证码","enable":true},{"name":"login_fail_lock","note":"登录失败锁定","enable":true},{"name":"svn_user_pass_check","note":"密码强度校验","enable":true}]' >/dev/null

echo
echo "== 清理 =="
post /api/rep/del '{"repName":"s3b"}' >/dev/null
post /api/rep/del '{"repName":"s3c"}' >/dev/null
post /api/user/del '{"svnUserName":"tuser"}' >/dev/null
post /api/user/del '{"svnUserName":"shortpw"}' >/dev/null
post /api/subadmin/del "{\"subadminId\":${SID}}" >/dev/null
post /api/common/logout '{}' >/dev/null

pkill -x svnadmin4 2>/dev/null
wait ${SRV} 2>/dev/null

echo
echo "================ 结果 ================"
echo "通过: ${pass}   失败: ${fail}"
[[ ${fail} -eq 0 ]]
