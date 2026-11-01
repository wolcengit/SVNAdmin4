#!/usr/bin/env bash
# 校验所有源文件是否带有 GPL-3.0 许可头（SPDX-License-Identifier）。
# 用法：bash scripts/check-license.sh
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
missing=0
checked=0

# 文件清单写入临时文件后再读取：避免进程替换（< <(...)）在 /dev/fd 不可用的
# 受限 shell（部分沙箱 / CI 镜像）中静默失效导致误报"0 个文件"。
LIST="$(mktemp)"
trap 'rm -f "$LIST"' EXIT
find "$ROOT/server" -name '*.go' -not -path '*/node_modules/*' >>"$LIST"
find "$ROOT/web/src" -type f \( -name '*.ts' -o -name '*.vue' -o -name '*.js' \) >>"$LIST"

check() {
  local f="$1"
  checked=$((checked + 1))
  if ! head -c 1200 "$f" | grep -q "SPDX-License-Identifier: GPL-3.0-or-later"; then
    echo "缺少许可头: ${f#$ROOT/}"
    missing=$((missing + 1))
  fi
}

while IFS= read -r f; do
  [ -n "$f" ] && check "$f"
done <"$LIST"

if [ "$checked" -eq 0 ]; then
  echo "✗ 未扫描到任何源文件，请确认 server/ 与 web/src 存在"
  exit 1
fi

# 开源元文件必须存在
for f in LICENSE NOTICE CONTRIBUTING.md CHANGELOG.md SECURITY.md docs/THIRD-PARTY.md; do
  [ -f "$ROOT/$f" ] || { echo "缺少文件: $f"; missing=$((missing + 1)); }
done

echo "已检查 $checked 个源文件"
if [ "$missing" -gt 0 ]; then
  echo "✗ $missing 处未通过许可头校验"
  exit 1
fi
echo "✓ 许可头校验通过"
