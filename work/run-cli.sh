#!/usr/bin/env bash
# run-cli.sh — 以 enhanced 模式启动 co-shell CLI（增强交互 REPL）
#
# 用法:
#   ./run-cli.sh [co-shell 参数...]
#
# 说明:
#   - 以 --input-mode enhanced 启动增强交互 REPL
#   - 可追加任意 co-shell 参数（如 -w 工作区、-m 模型等）
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

# 以 enhanced 模式启动 CLI，透传所有额外参数
exec "$BIN" --input-mode enhanced "$@"
