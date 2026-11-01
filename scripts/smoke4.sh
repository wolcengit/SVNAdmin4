#!/usr/bin/env bash
# SVNAdmin4 第四轮冒烟测试：覆盖 2026-10-30 第二批弱项修复
#   LDAP 写拦截 / svnUserMail / 用户 online / black_token 持久化 / 别名搜索 /
#   crond 服务端分页 / 日志时间范围 / 指定 UUID / hotcopy 目录入列 / treeOld /
#   二次授权自助闭环 / 邮件测试参数化 / ApacheInfo 数据源字段
# 用法：bash scripts/smoke4.sh [端口]

set -u
export no_proxy='*' NO_PROXY='*'

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${1:-18083}"
BASE="http://127.0.0.1:${PORT}"
BIN="${ROOT}/build/svnadmin4"
LOG="${ROOT}/.tmp/smoke4.log"

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
CFG="${ROOT}/.tmp/smoke4-config.yaml"
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
echo "== 1. 创建用户携带邮箱 =="
post /api/user/create '{"svnUserName":"m1","svnUserPass":"pass1234","svnUserNote":"带邮箱","svnUserMail":"m1@example.com"}' >/dev/null
M=$(get '/api/user/list?page=1&pageSize=20&keyword=m1')
check "创建时邮箱被接收" 'm1@example.com' "${M}"
check "用户列表含 online 字段" '"online"' "${M}"

echo
echo "== 2. SVN 用户 online 在线态 =="
PTOKEN=$(login_as m1 pass1234 svnuser)
ON=$(get '/api/user/list?page=1&pageSize=20&keyword=m1' | python3 -c 'import sys,json
l=(json.load(sys.stdin)["data"] or {}).get("list",[])
print(l[0].get("online") if l else "missing")')
check "登录后在线" 'True' "${ON}"
curl -s --noproxy '*' -X POST -H "X-Token: ${PTOKEN}" "${BASE}/api/common/logout" >/dev/null
sleep 1
OFF=$(get '/api/user/list?page=1&pageSize=20&keyword=m1' | python3 -c 'import sys,json
l=(json.load(sys.stdin)["data"] or {}).get("list",[])
print(l[0].get("online") if l else "missing")')
check "注销后离线" 'False' "${OFF}"

echo
echo "== 3. black_token 落库持久化（重启后注销仍生效） =="
PTOKEN2=$(login_as m1 pass1234 svnuser)
curl -s --noproxy '*' -X POST -H "X-Token: ${PTOKEN2}" "${BASE}/api/common/logout" >/dev/null
AFTER=$(curl -s --noproxy '*' -H "X-Token: ${PTOKEN2}" "${BASE}/api/common/userInfo")
check "注销后 token 立即失效" '402' "$(echo "${AFTER}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["code"])' 2>/dev/null || echo "${AFTER}")"
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
AFTER2=$(curl -s --noproxy '*' -H "X-Token: ${PTOKEN2}" "${BASE}/api/common/userInfo")
check "服务重启后注销 token 仍失效" '402' "$(echo "${AFTER2}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["code"])' 2>/dev/null || echo "${AFTER2}")"

echo
echo "== 4. LDAP 数据源写操作拦截 =="
post /api/rep/create '{"repName":"s4a","repNote":"LDAP拦截测试"}' >/dev/null
DB=$(find "${ROOT}/data" -name '*.db' | head -1)
python3 - "${DB}" <<'PYEOF'
import sqlite3, sys, json
c = sqlite3.connect(sys.argv[1])
row = c.execute("SELECT option_value FROM options WHERE option_name='24_svn_datasource'").fetchone()
ds = json.loads(row[0]) if row and row[0] else {}
ds["userSource"] = "ldap"
ds["groupSource"] = "ldap"
val = json.dumps(ds)
if row:
    c.execute("UPDATE options SET option_value=? WHERE option_name='24_svn_datasource'", (val,))
else:
    c.execute("INSERT INTO options (option_name, option_value, option_note) VALUES ('24_svn_datasource', ?, '')", (val,))
c.commit()
print("ldap-on")
PYEOF
check "LDAP 源创建用户被拒" '禁止在本地创建用户' "$(post /api/user/create '{"svnUserName":"ld1","svnUserPass":"pass1234"}')"
check "LDAP 源修改密码被拒" 'LDAP' "$(post /api/user/updPass '{"svnUserName":"m1","svnUserPass":"newpass123"}')"
check "LDAP 源启用/禁用被拒" '禁止在本地启用' "$(post /api/user/updStatus '{"svnUserName":"m1","svnUserStatus":0}')"
check "LDAP 源扫描被拒" 'LDAP' "$(get /api/user/scan)"
check "LDAP 源导入被拒" '禁止导入' "$(post /api/user/import '{"names":["x1"]}')"
check "LDAP 源创建分组被拒" '禁止在本地创建分组' "$(post /api/group/create '{"svnGroupName":"lg1","svnGroupNote":""}')"
check "LDAP 源编辑分组成员被拒" '禁止在本地编辑' "$(post /api/group/updMember '{"svnGroupName":"dev","users":[],"groups":[],"aliases":[]}')"
# 恢复数据源
python3 - "${DB}" <<'PYEOF'
import sqlite3, sys, json
c = sqlite3.connect(sys.argv[1])
row = c.execute("SELECT option_value FROM options WHERE option_name='24_svn_datasource'").fetchone()
ds = json.loads(row[0]) if row and row[0] else {}
ds["userSource"] = "passwd"
ds["groupSource"] = "authz"
c.execute("UPDATE options SET option_value=? WHERE option_name='24_svn_datasource'", (json.dumps(ds),))
c.commit()
print("restored")
PYEOF
check "恢复 passwd 源后可创建用户" '"status":true' "$(post /api/user/create '{"svnUserName":"ld2","svnUserPass":"pass1234"}')"

echo
echo "== 5. 别名搜索 =="
AUTHZ="${ROOT}/runtime/svnadmin/authz"
python3 - "${AUTHZ}" <<'PYEOF'
import sys
p = sys.argv[1]
with open(p, 'a', encoding='utf-8') as f:
    f.write("\n[aliases]\n")
    f.write("john = m1\n")
    f.write("jane = ld2\n")
print("aliases-written")
PYEOF
check "别名列表全量" 'john' "$(get '/api/aliase/list')"
check "别名搜索命中" 'john' "$(get '/api/aliase/list?searchKeyword=joh')"
checknot "别名搜索排除未命中" 'jane' "$(get '/api/aliase/list?searchKeyword=joh')"

echo
echo "== 6. 定时任务服务端分页 =="
post /api/crond/create '{"taskName":"c41","taskType":1,"cycleType":"day_n","day":1,"hour":3,"minute":0,"status":1,"saveCount":1,"repName":"s4a","notice":0}' >/dev/null
post /api/crond/create '{"taskName":"c42","taskType":5,"cycleType":"day_n","day":2,"hour":4,"minute":0,"status":1,"saveCount":1,"repName":"s4a","notice":0}' >/dev/null
ALL=$(get '/api/crond/list')
check "无分页参数返回全量数组" 'c41' "${ALL}"
PAGED=$(get '/api/crond/list?page=1&pageSize=1')
check "分页参数返回 total" '"total":' "${PAGED}"
check "分页参数返回 list" '"list":' "${PAGED}"
check "分页搜索 keyword 命中" 'c42' "$(get '/api/crond/list?page=1&pageSize=10&keyword=c42')"

echo
echo "== 7. 日志时间范围搜索 =="
check "beginTime 为未来日期时无记录" '"total":0' "$(get '/api/log/list?page=1&pageSize=10&beginTime=2099-01-01%2000:00:00')"
check "beginTime 为过去日期时有记录" '"total":' "$(get '/api/log/list?page=1&pageSize=10&beginTime=2000-01-01%2000:00:00')"

echo
echo "== 8. 指定 UUID 重设 =="
UUID="11111111-2222-3333-4444-555555555555"
SU=$(post /api/rep/setUUID "{\"repName\":\"s4a\",\"uuid\":\"${UUID}\"}")
check "指定 UUID 设置成功" "${UUID}" "${SU}"
check "仓库属性回读 UUID 一致" "${UUID}" "$(get '/api/rep/detail?repName=s4a')"

echo
echo "== 9. hotcopy 备份目录入列 =="
HC=$(post /api/rep/hotcopy '{"repName":"s4a","incremental":false}')
check "热拷贝任务入队" '"status":true' "${HC}"
for _ in $(seq 1 60); do
  ST=$(get '/api/task/list?page=1&pageSize=5' | python3 -c 'import sys,json
l=(json.load(sys.stdin)["data"] or {}).get("list",[])
print(",".join(str(x["taskStatus"]) for x in l[:3]))' 2>/dev/null)
  case "${ST}" in *0*|*1*) sleep 0.5;; *) break;; esac
done
BL=$(get '/api/backup/list?repName=s4a')
check "备份列表含 hotcopy 产物" 'hotcopy' "${BL}"
check "hotcopy 计入体积" 'sizeHr' "${BL}"

echo
echo "== 10. treeOld 旧权限快照 =="
post /api/subadmin/create '{"subadminName":"told","subadminPass":"pass1234","subadminNote":""}' >/dev/null
SID=$(get '/api/subadmin/list?page=1&pageSize=10' | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];l=d if isinstance(d,list) else (d or {}).get("list",[]);print(next((x["subadminId"] for x in l if x["subadminName"]=="told"),0))')
TREE=$(get "/api/subadmin/tree?subadminId=${SID}" | python3 -c 'import sys,json;print(json.dumps(json.load(sys.stdin)["data"]["tree"]))')
post /api/subadmin/updTree "{\"subadminId\":${SID},\"tree\":${TREE}}" >/dev/null
TL=$(get "/api/subadmin/tree?subadminId=${SID}")
check "保存后返回 treeOld 快照" 'treeOld' "${TL}"
check "treeOld 非空" 'title' "${TL}"

echo
echo "== 11. 二次授权自助闭环 =="
post /api/user/create '{"svnUserName":"sp1","svnUserPass":"sp1pass"}' >/dev/null
post /api/user/create '{"svnUserName":"sp2","svnUserPass":"sp2pass"}' >/dev/null
post /api/user/create '{"svnUserName":"sp3","svnUserPass":"sp3pass"}' >/dev/null
post /api/pri/create '{"repName":"s4a","priPath":"/","objectType":"user","objectName":"sp1","perm":"rw"}' >/dev/null
SP1=$(login_as sp1 sp1pass svnuser)
check "SVN 用户可开启自己的二次授权" '"status":true' "$(curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${SP1}" -d '{"repName":"s4a","priPath":"/","enable":true}' "${BASE}/api/secondpri/update")"
check "SVN 用户可添加白名单对象" '"status":true' "$(curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${SP1}" -d '{"repName":"s4a","priPath":"/","objectType":"user","objectName":"sp2"}' "${BASE}/api/secondpri/objectCreate")"
check "白名单列表可查" 'sp2' "$(curl -s --noproxy '*' -H "X-Token: ${SP1}" "${BASE}/api/secondpri/objectList?repName=s4a&priPath=/")"
check "userPaths 返回开启路径" 's4a' "$(curl -s --noproxy '*' -H "X-Token: ${SP1}" "${BASE}/api/secondpri/userPaths")"
check "SVN 用户可下放权限给白名单对象" '"status":true' "$(curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${SP1}" -d '{"repName":"s4a","priPath":"/","objectType":"user","objectName":"sp2","perm":"r"}' "${BASE}/api/pri/create")"
check "下放给白名单外对象被拒" '不在二次授权白名单' "$(curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${SP1}" -d '{"repName":"s4a","priPath":"/","objectType":"user","objectName":"sp3","perm":"r"}' "${BASE}/api/pri/create")"
check "下放给自己被拒" '不能下放权限给自己' "$(curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${SP1}" -d '{"repName":"s4a","priPath":"/","objectType":"user","objectName":"sp1","perm":"r"}' "${BASE}/api/pri/create")"
# 无写权限用户无法授权
SP3=$(login_as sp3 sp3pass svnuser)
check "无写权限用户授权被拒" '没有写权限' "$(curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${SP3}" -d '{"repName":"s4a","priPath":"/","objectType":"user","objectName":"sp2","perm":"r"}' "${BASE}/api/pri/create")"
# 有写权限但对未开启二次授权的路径下放 → 拒绝（写权限校验先于二次授权校验）
check "未开启路径下放被拒" '未开启二次授权' "$(curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${SP1}" -d '{"repName":"s4a","priPath":"/trunk","objectType":"user","objectName":"sp2","perm":"r"}' "${BASE}/api/pri/create")"
# 操作他人二次授权配置被拒
check "操作他人二次授权被拒" '只能操作自己的' "$(curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${SP3}" -d '{"svnUserName":"sp1","repName":"s4a","priPath":"/","enable":false}' "${BASE}/api/secondpri/update")"

echo
echo "== 12. 邮件测试支持未保存参数 =="
MT=$(curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${TOKEN}" -d '{"to":"x@y.z","host":"127.0.0.1","port":1,"timeout":1}' "${BASE}/api/mail/test")
check "未保存参数测试走传入配置（连接失败即证明）" '"status":false' "${MT}"

echo
echo "== 13. ApacheInfo 数据源字段 =="
AI=$(get '/api/setting/apacheInfo')
check "apacheInfo 含 userSource" '"userSource"' "${AI}"
check "apacheInfo 含 groupSource" '"groupSource"' "${AI}"
check "apacheInfo 含 ldap" '"ldap"' "${AI}"
check "apacheInfo 含 passwordDb" '"passwordDb"' "${AI}"

echo
echo "== 清理 =="
curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${SP1}" "${BASE}/api/common/logout" >/dev/null
curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${SP3}" "${BASE}/api/common/logout" >/dev/null
CID1=$(get '/api/crond/list' | python3 -c 'import sys,json
d=json.load(sys.stdin)["data"]; l=d if isinstance(d,list) else d.get("list",[])
print(" ".join(str(x["crondId"]) for x in l if x["taskName"] in ("c41","c42")))' 2>/dev/null)
for id in ${CID1}; do post /api/crond/del "{\"crondId\":${id}}" >/dev/null; done
post /api/rep/del '{"repName":"s4a"}' >/dev/null
for u in m1 ld2 sp1 sp2 sp3; do post /api/user/del "{\"svnUserName\":\"${u}\"}" >/dev/null; done
post /api/subadmin/del "{\"subadminId\":${SID}}" >/dev/null
post /api/common/logout '{}' >/dev/null

pkill -x svnadmin4 2>/dev/null
wait ${SRV} 2>/dev/null

echo
echo "================ 结果 ================"
echo "通过: ${pass}   失败: ${fail}"
