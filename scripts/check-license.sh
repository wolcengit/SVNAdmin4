#!/usr/bin/env bash
# 校验所有源文件是否带有 GPL-3.0 许可头（SPDX-License-Identifier）。
# 用法：bash scripts/check-license.sh
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
missing=0
checked=0

check() {
  local f="$1"
  checked=$((checked + 1))
  if ! head -c 1200 "$f" | grep -q "SPDX-License-Identifier: GPL-3.0-or-later"; then
    echo "缺少许可头: ${f#$ROOT/}"
    missing=$((missing + 1))
  fi
}

while IFS= read -r f; do check "$f"; done < <(find "$ROOT/server" -name '*.go' -not -path '*/node_modules/*')
while IFS= read -r f; do check "$f"; done < <(find "$ROOT/web/src" -type f \( -name '*.ts' -o -name '*.vue' -o -name '*.js' \))

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
