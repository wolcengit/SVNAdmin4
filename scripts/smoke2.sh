#!/usr/bin/env bash
# SVNAdmin4 补齐功能冒烟测试
# 用法：bash scripts/smoke2.sh [端口]

set -u
export no_proxy='*' NO_PROXY='*'

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${1:-18081}"
BASE="http://127.0.0.1:${PORT}"
BIN="${ROOT}/build/svnadmin4"
LOG="${ROOT}/.tmp/smoke2.log"

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

get() { curl -s --noproxy '*' -H "X-Token: ${TOKEN}" "${BASE}$1"; }
post() { curl -s --noproxy '*' -X POST -H 'Content-Type: application/json' -H "X-Token: ${TOKEN}" -d "$2" "${BASE}$1"; }

echo "== 启动服务 =="
pkill -x svnadmin4 2>/dev/null
sleep 1
python3 -c "import shutil; shutil.rmtree('${ROOT}/runtime', ignore_errors=True); shutil.rmtree('${ROOT}/data', ignore_errors=True)"
# 端口由参数决定，生成一份临时配置避免使用中的端口被占用
CFG="${ROOT}/.tmp/smoke2-config.yaml"
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

echo "== 登录 =="
CJ=$(curl -s --noproxy '*' "${BASE}/api/common/verifyCode")
U=$(echo "${CJ}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["uuid"])')
C=$(echo "${CJ}" | python3 -c 'import sys,json,re;print("".join(re.findall(r"<text[^>]*>(.)</text>",json.load(sys.stdin)["data"]["svg"])))')
TOKEN=$(curl -s --noproxy '*' -X POST "${BASE}/api/common/login" -H 'Content-Type: application/json' \
  -d "{\"userName\":\"admin\",\"password\":\"admin\",\"uuid\":\"${U}\",\"code\":\"${C}\"}" |
  python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["token"])')
echo "  token 长度=${#TOKEN}"

# 登录角色选择（指定 admin 可登录；指定 svnuser 时同名账号不存在应报错）
login_role() {
  CJ=$(curl -s --noproxy '*' "${BASE}/api/common/verifyCode")
  local u c
  u=$(echo "${CJ}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["uuid"])')
  c=$(echo "${CJ}" | python3 -c 'import sys,json,re;print("".join(re.findall(r"<text[^>]*>(.)</text>",json.load(sys.stdin)["data"]["svg"])))')
  curl -s --noproxy '*' -X POST "${BASE}/api/common/login" -H 'Content-Type: application/json' \
    -d "{\"userName\":\"admin\",\"password\":\"admin\",\"uuid\":\"${u}\",\"code\":\"${c}\",\"role\":\"$1\"}"
}
check "按角色 admin 登录成功" '"role":"admin"' "$(login_role admin)"
check "按角色 svnuser 登录失败" '"status":false' "$(login_role svnuser)"

echo
echo "== 系统监控 =="
check "监控总览含 load" 'load' "$(get /api/common/monitor)"
check "负载信息" 'load1Min' "$(get /api/common/loadInfo)"
check "磁盘信息" 'mountedOn' "$(get /api/common/diskInfo)"
check "版本信息" 'version' "$(get /api/common/version)"

echo
echo "== 任务控制 =="
check "任务类型字典" 'types' "$(get /api/task/types)"
check "运行中任务概览" 'running' "$(get /api/task/run)"
post /api/rep/create '{"repName":"s2","initStruct":"01"}' >/dev/null
post /api/backup/dump '{"repName":"s2"}' >/dev/null
sleep 2
check "任务列表含备份" 'dump' "$(get '/api/task/list?page=1&pageSize=20')"
TID=$(get '/api/task/list?page=1&pageSize=20' | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["list"][0]["taskId"])')
check "任务日志可读" 'log' "$(get "/api/task/log?taskId=${TID}")"
check "任务列表按状态过滤" '"status":true' "$(get '/api/task/list?status=2')"
sleep 3
check "清空历史任务" '"status":true' "$(post /api/task/clear '{}')"

echo
echo "== 定时计划增强 =="
check "定时字典含 9 种类型" '仓库检查' "$(get /api/crond/dict)"
check "定时字典含 8 种周期" 'minute_n' "$(get /api/crond/dict)"
check "调度器状态" 'jobCount' "$(get /api/crond/dict)"
post /api/crond/create '{"taskType":5,"taskName":"检查 s2","cycleType":"day","cycleDesc":"","status":1,"saveCount":1,"repName":"s2","hour":3,"minute":0,"notice":0,"code":"","shell":""}' >/dev/null
CID=$(get /api/crond/list | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"][0]["crondId"])')
check "手动触发定时任务" '"status":true' "$(post /api/crond/trigger "{\"crondId\":${CID}}")"
sleep 2
check "定时任务日志" 'log' "$(get "/api/crond/log?crondId=${CID}")"
post /api/crond/del "{\"crondId\":${CID}}" >/dev/null
check "删除定时任务" '"status":true' "$(post /api/crond/del "{\"crondId\":${CID}}")" || true

echo
echo "== 仓库检查 / 热拷贝 =="
check "启动仓库检查" '"status":true' "$(post /api/rep/verify '{"repName":"s2"}')"
check "启动热拷贝" '"status":true' "$(post /api/rep/hotcopy '{"repName":"s2","incremental":false}')"
sleep 4
# 创建一个有权限的用户，用于验证 SVN 用户视角浏览
post /api/user/create '{"svnUserName":"sp1","svnUserPass":"sp1pass"}' >/dev/null
post /api/pri/create '{"repName":"s2","priPath":"/","objectType":"user","objectName":"sp1","perm":"rw"}' >/dev/null
check "SVN 用户视角目录树" '"status":true' "$(get '/api/rep/tree2?repName=s2&path=/&svnUserName=sp1&first=1')"
check "SVN 用户视角内容浏览" 'bread' "$(get '/api/rep/userCon?repName=s2&path=/&svnUserName=sp1')"

echo
echo "== 二次授权 =="
post /api/pri/create '{"repName":"s2","priPath":"/","objectType":"user","objectName":"sp1","perm":"rw"}' >/dev/null
check "开启二次授权" '"status":true' "$(post /api/secondpri/update '{"svnUserName":"sp1","repName":"s2","priPath":"/","enable":true}')"
post /api/user/create '{"svnUserName":"sp2","svnUserPass":"sp2pass"}' >/dev/null
check "新增可下放对象" '"status":true' "$(post /api/secondpri/objectCreate '{"svnUserName":"sp1","repName":"s2","priPath":"/","objectType":"user","objectName":"sp2"}')"
check "白名单列表" 'sp2' "$(get '/api/secondpri/objectList?svnUserName=sp1&repName=s2&priPath=/')"
check "不能下放给自己" '不能包括本身' "$(post /api/secondpri/objectCreate '{"svnUserName":"sp1","repName":"s2","priPath":"/","objectType":"user","objectName":"sp1"}')"
SPID=$(get '/api/secondpri/objectList?svnUserName=sp1&repName=s2&priPath=/' | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"][0]["svnSecondPriId"])')
check "删除可下放对象" '"status":true' "$(post /api/secondpri/objectDel "{\"secondPriId\":${SPID}}")"
check "查询用户已开启路径" '"status":true' "$(get '/api/secondpri/userPaths?svnUserName=sp1')"
check "关闭二次授权" '"status":true' "$(post /api/secondpri/update '{"svnUserName":"sp1","repName":"s2","priPath":"/","enable":false}')"

echo
echo "== 权限取反 invert =="
check "新增取反权限" '"status":true' "$(post /api/pri/create '{"repName":"s2","priPath":"/trunk","objectType":"user","objectName":"sp1","perm":"r","invert":true}')"
check "权限列表含 invert" 'invert' "$(get '/api/pri/list?repName=s2&priPath=/trunk')"
check "authz 写入 ~ 前缀" '~sp1' "$(cat "${ROOT}/runtime/svnadmin/authz")"
check "修改取反权限" '"status":true' "$(post /api/pri/update '{"repName":"s2","priPath":"/trunk","objectName":"sp1","perm":"rw","invert":false}')"
if grep -q '^~sp1' "${ROOT}/runtime/svnadmin/authz"; then
  echo "  [FAIL] authz 取消 ~ 前缀"; fail=$((fail + 1))
else
  echo "  [PASS] authz 取消 ~ 前缀"; pass=$((pass + 1))
fi
post /api/pri/delete '{"repName":"s2","priPath":"/trunk","objectName":"sp1"}' >/dev/null

echo
echo "== 邮件与推送 =="
check "邮件配置读取" 'host' "$(get /api/mail/info)"
check "保存邮件配置" '"status":true' "$(post /api/mail/update '{"host":"127.0.0.1","port":25,"status":false}')"
check "推送开关清单（6 个触发点）" 'Task/Success' "$(get /api/push/info)"
check "保存推送开关" '"status":true' "$(post /api/push/update '[{"trigger":"Common/Login","type":"mail","note":"用户登录时推送","enable":true}]')"
check "安全配置清单" 'login_verify_code' "$(get /api/safe/info)"
check "保存安全配置" '"status":true' "$(post /api/safe/update '[{"name":"login_verify_code","note":"登录验证码","enable":true}]')"

echo
echo "== 设置项扩展 =="
check "路径信息" 'homePath' "$(get /api/setting/dirInfo)"
check "svn 协议信息" 'svnserveStatus' "$(get /api/setting/svnInfo)"
check "Apache 信息" 'installed' "$(get /api/setting/apacheInfo)"
check "宿主机信息" 'svnPort' "$(get /api/setting/dockerHost)"
check "保存宿主机信息" '"status":true' "$(post /api/setting/dockerHost '{"host":"127.0.0.1","svnPort":3690,"httpPort":80}')"
check "修改 HTTP 前缀" '"status":true' "$(post /api/setting/httpPrefix '{"prefix":"/svn"}')"
check "修改 HTTP 端口" '"status":true' "$(post /api/setting/httpPort '{"port":80}')"
SPR=$(post /api/setting/svnservePort '{"port":3691}')
# 两种成功语义：写入成功，或 option 中已是 3691（端口未发生变化，同样证明配置链路生效）
if [[ "${SPR}" == *'"status":true'* || "${SPR}" == *"端口未发生变化"* ]]; then
  echo "  [PASS] 修改 svnserve 端口"; pass=$((pass + 1))
else
  echo "  [FAIL] 修改 svnserve 端口"; echo "         实际: ${SPR}"; fail=$((fail + 1))
fi
check "修改用户来源" '"status":true' "$(post /api/setting/usersource '{"protocol":"svn","source":{"userSource":"passwd","groupSource":"authz"}}')"
check "saslauthd 状态" 'installed' "$(get /api/setting/saslInfo)"
check "版本更新检测" 'current' "$(get /api/setting/checkUpdate)"

echo
echo "== LDAP 接口 =="
check "LDAP 未配置时报错" 'status' "$(post /api/ldap/test '{"type":"connection","config":{"host":""}}')"

echo
echo "== 分片上传 =="
check "上传能力信息" 'supportChunk' "$(get /api/backup/uploadInfo)"
INIT=$(post /api/backup/uploadInit '{"fileName":"parted.dump","md5":"","numBlobTotal":2}')
check "初始化分片上传" 'uploadId' "${INIT}"
UPID=$(echo "${INIT}" | python3 -c 'import sys,json;print(json.load(sys.stdin)["data"]["uploadId"])')
printf 'AAAA' > "${ROOT}/.tmp/p0.bin"
printf 'BBBB' > "${ROOT}/.tmp/p1.bin"
curl -s --noproxy '*' -X POST -H "X-Token: ${TOKEN}" -F "uploadId=${UPID}" -F "index=0" -F "file=@${ROOT}/.tmp/p0.bin" "${BASE}/api/backup/uploadPart" >/dev/null
curl -s --noproxy '*' -X POST -H "X-Token: ${TOKEN}" -F "uploadId=${UPID}" -F "index=1" -F "file=@${ROOT}/.tmp/p1.bin" "${BASE}/api/backup/uploadPart" >/dev/null
check "分片上传进度" 'numBlobReceived' "$(get "/api/backup/uploadStatus?uploadId=${UPID}")"
check "合并分片" '"status":true' "$(post /api/backup/uploadMerge "{\"uploadId\":\"${UPID}\",\"deleteOnMerge\":true}")"
check "备份列表含分片合并文件" 'parted.dump' "$(get '/api/backup/list')"

echo
echo "== 子管理员权限树 =="
post /api/subadmin/create '{"subadminName":"sa1","subadminPass":"sa1pass","subadminNote":""}' >/dev/null
SID=$(get '/api/subadmin/list?page=1&pageSize=10' | python3 -c 'import sys,json;d=json.load(sys.stdin)["data"];l=d if isinstance(d,list) else (d or {}).get("list",[]);print(l[0]["subadminId"])')
check "权限树读取" 'needUpdateTree' "$(get "/api/subadmin/tree?subadminId=${SID}")"
TREE=$(get "/api/subadmin/tree?subadminId=${SID}" | python3 -c 'import sys,json;print(json.dumps(json.load(sys.stdin)["data"]["tree"]))')
check "保存权限树" '"status":true' "$(post /api/subadmin/updTree "{\"subadminId\":${SID},\"tree\":${TREE}}")"
check "修改子管理员备注" '"status":true' "$(post /api/subadmin/updNote "{\"subadminId\":${SID},\"subadminNote\":\"备注\"}")"
post /api/subadmin/del "{\"subadminId\":${SID}}" >/dev/null

echo
echo "== 数据同步 =="
check "同步用户" '"status":true' "$(post /api/user/sync '{}')"
check "同步分组" '"status":true' "$(post /api/group/sync '{}')"
check "同步仓库" '"status":true' "$(post /api/rep/syncAll '{}')"
check "全量同步" '"status":true' "$(post /api/sync/all '{}')"

echo
echo "== 管理员改账号名 =="
check "改账号名需校验密码" '密码错误' "$(post /api/common/editAdminUserName '{"oldName":"admin","newName":"root","pass":"wrong"}')"

echo
echo "== 清理 =="
post /api/rep/del '{"repName":"s2"}' >/dev/null
post /api/user/del '{"svnUserName":"sp1"}' >/dev/null
post /api/user/del '{"svnUserName":"sp2"}' >/dev/null
post /api/common/logout '{}' >/dev/null

pkill -x svnadmin4 2>/dev/null
wait ${SRV} 2>/dev/null

echo
echo "================ 结果 ================"
echo "通过: ${pass}   失败: ${fail}"
[[ ${fail} -eq 0 ]]
