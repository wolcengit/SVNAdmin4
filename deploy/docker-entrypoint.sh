#!/bin/sh
set -e

CONF=/opt/svnadmin4/server/configs/config.yaml

# 允许通过环境变量覆盖数据主目录
if [ -n "$SVN_HOME" ]; then
  sed -i "s|^  home_path:.*|  home_path: ${SVN_HOME}|" "$CONF"
fi

echo "==> 启动 $(cat /opt/svnadmin4/VERSION 2>/dev/null || echo SVNAdmin4)"

exec /opt/svnadmin4/build/svnadmin4 \
  -c "$CONF" \
  -web /opt/svnadmin4/web/dist
