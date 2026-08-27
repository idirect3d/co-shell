#!/usr/bin/env bash
# run-web.sh — 以白名单形式启动 co-shell Web UI 服务（供其他主机访问）
#
# 用法:
#   ./run-web.sh [白名单IP列表] [co-shell 参数...]
#   COSHELL_WHITELIST="192.168.1.100,192.168.1.0/24" ./run-web.sh
#
# 白名单来源（优先级从高到低）:
#   1. 第一个命令行参数（逗号分隔 IP/网段）
#   2. 环境变量 COSHELL_WHITELIST（逗号分隔 IP/网段）
#   3. 脚本同目录下的默认文件 WHITELIST（内容为逗号分隔 IP/网段）
#
# 说明:
#   - 绑定 0.0.0.0，允许局域网/其他主机访问
#   - 若以上三种来源均未提供白名单，则提示用户提供并退出（不继续启动）
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

# 解析白名单：优先级 参数 > 环境变量 > 默认文件 WHITELIST
WHITELIST=""
if [ -n "$1" ]; then
  WHITELIST="$1"
  shift
elif [ -n "$COSHELL_WHITELIST" ]; then
  WHITELIST="$COSHELL_WHITELIST"
elif [ -f "$SCRIPT_DIR/WHITELIST" ]; then
  WHITELIST="$(cat "$SCRIPT_DIR/WHITELIST")"
fi

# 三种来源均未提供白名单时，提示用户提供并退出
if [ -z "$WHITELIST" ]; then
  echo "错误: 未提供白名单，无法启动 Web UI 服务。" >&2
  echo "请通过以下任一方式提供白名单（逗号分隔 IP/网段，如 192.168.1.100,192.168.1.0/24）：" >&2
  echo "  1. 命令行参数: ./run-web.sh \"192.168.1.100,192.168.1.0/24\"" >&2
  echo "  2. 环境变量:   COSHELL_WHITELIST=\"192.168.1.100,192.168.1.0/24\" ./run-web.sh" >&2
  echo "  3. 白名单文件: 在脚本同目录创建 WHITELIST 文件，内容为逗号分隔 IP/网段" >&2
  exit 1
fi

echo "启动 co-shell Web UI 服务（绑定 0.0.0.0，白名单: $WHITELIST）..." >&2

# 以白名单形式启动 Web UI 服务，透传其余参数
exec "$BIN" --serve --bind 0.0.0.0 --whitelist "$WHITELIST" "$@"
