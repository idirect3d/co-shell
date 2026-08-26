#!/usr/bin/env bash
# run-web.sh — 以白名单形式启动 co-shell Web UI 服务（供其他主机访问）
#
# 用法:
#   ./run-web.sh [白名单IP列表] [co-shell 参数...]
#   COSHELL_WHITELIST="192.168.1.100,192.168.1.0/24" ./run-web.sh
#
# 说明:
#   - 绑定 0.0.0.0，允许局域网/其他主机访问
#   - 白名单通过第一个参数或环境变量 COSHELL_WHITELIST 指定（逗号分隔 IP/网段）
#   - 未指定白名单时默认仅允许本机回环（127.0.0.1）
#   - 脚本与 co-shell 可执行程序放在同一目录

set -e

# 定位脚本所在目录（与 co-shell 可执行程序同目录）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/co-shell"

if [ ! -x "$BIN" ]; then
  echo "错误: 未找到 co-shell 可执行程序: $BIN" >&2
  echo "请将本脚本与 co-shell 可执行程序放在同一目录。" >&2
  exit 1
fi

# 解析白名单：优先取第一个参数，其次取环境变量 COSHELL_WHITELIST
WHITELIST="${1:-$COSHELL_WHITELIST}"
if [ -n "$1" ]; then
  shift
fi

# 未指定白名单时，默认仅允许本机回环访问
if [ -z "$WHITELIST" ]; then
  WHITELIST="127.0.0.1"
  echo "提示: 未指定白名单，默认仅允许本机访问（127.0.0.1）。" >&2
  echo "      用法: ./run-web.sh \"192.168.1.100,192.168.1.0/24\" 或设置 COSHELL_WHITELIST 环境变量。" >&2
fi

echo "启动 co-shell Web UI 服务（绑定 0.0.0.0，白名单: $WHITELIST）..." >&2

# 以白名单形式启动 Web UI 服务，透传其余参数
exec "$BIN" --serve --bind 0.0.0.0 --whitelist "$WHITELIST" "$@"
