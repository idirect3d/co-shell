#!/bin/bash
#
# build-release.sh — co-shell 全平台 Release 构建脚本
#
# 用法: ./build-release.sh <版本号>
# 示例: ./build-release.sh 0.5.0-Beta2
#
# 功能:
#   1. 编译 co-shell 6 个平台 (darwin/linux/windows × amd64/arm64)
#   2. 编译 co-shell-bridge 6 个平台
#   3. 打包 zip（包内可执行文件统一命名为 co-shell / co-shell.exe）
#   4. 输出到 dist/<版本号>/

set -euo pipefail

if [ $# -lt 1 ]; then
  echo "用法: $0 <版本号>"
  echo "示例: $0 0.5.0-Beta2"
  exit 1
fi

VERSION="$1"
DIST_DIR="dist/${VERSION}"
ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=========================================="
echo " co-shell Release Builder"
echo " Version: ${VERSION}"
echo " Output:  ${DIST_DIR}/"
echo "=========================================="

# 确保 dist 目录存在
mkdir -p "${ROOT_DIR}/${DIST_DIR}"

# 定义平台列表
PLATFORMS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
  "windows/arm64"
)

# ============================================
# Step 1: 编译 co-shell
# ============================================
echo ""
echo "--- Building co-shell ---"

for PLATFORM in "${PLATFORMS[@]}"; do
  GOOS="${PLATFORM%%/*}"
  GOARCH="${PLATFORM##*/}"

  # 生成输出文件名
  if [ "${GOOS}" = "windows" ]; then
    BINARY="co-shell-${GOOS}-${GOARCH}.exe"
  else
    BINARY="co-shell-${GOOS}-${GOARCH}"
  fi

  echo "  ${GOOS}/${GOARCH} -> ${BINARY}"

  cd "${ROOT_DIR}"
  GOOS="${GOOS}" GOARCH="${GOARCH}" go build -o "${DIST_DIR}/${BINARY}" .
done

echo "  co-shell: ALL DONE"

# ============================================
# Step 2: 编译 co-shell-bridge
# ============================================
echo ""
echo "--- Building co-shell-bridge ---"

for PLATFORM in "${PLATFORMS[@]}"; do
  GOOS="${PLATFORM%%/*}"
  GOARCH="${PLATFORM##*/}"

  if [ "${GOOS}" = "windows" ]; then
    BINARY="co-shell-bridge-${GOOS}-${GOARCH}.exe"
  else
    BINARY="co-shell-bridge-${GOOS}-${GOARCH}"
  fi

  echo "  ${GOOS}/${GOARCH} -> ${BINARY}"

  cd "${ROOT_DIR}"
  GOOS="${GOOS}" GOARCH="${GOARCH}" go build -o "${DIST_DIR}/${BINARY}" ./cmd/co-shell-feishu-bridge/
done

echo "  co-shell-bridge: ALL DONE"

# ============================================
# Step 3: 打包 zip
# ============================================
echo ""
echo "--- Packaging zip ---"

cd "${ROOT_DIR}/${DIST_DIR}"

# co-shell zip
for PLATFORM in "${PLATFORMS[@]}"; do
  GOOS="${PLATFORM%%/*}"
  GOARCH="${PLATFORM##*/}"

  if [ "${GOOS}" = "windows" ]; then
    SRC="co-shell-${GOOS}-${GOARCH}.exe"
    ZIP_NAME="co-shell-${VERSION}-${GOOS}-${GOARCH}.zip"
    ZIP_ENTRY="co-shell.exe"
  else
    SRC="co-shell-${GOOS}-${GOARCH}"
    ZIP_NAME="co-shell-${VERSION}-${GOOS}-${GOARCH}.zip"
    ZIP_ENTRY="co-shell"
  fi

  echo "  ${ZIP_NAME}"

  # 用临时目录确保 zip 内只有 co-shell，没有路径前缀
  TMPDIR="$(mktemp -d)"
  cp "${SRC}" "${TMPDIR}/${ZIP_ENTRY}"
  cd "${TMPDIR}"
  zip -q "${ROOT_DIR}/${DIST_DIR}/${ZIP_NAME}" "${ZIP_ENTRY}"
  cd "${ROOT_DIR}/${DIST_DIR}"
  rm -rf "${TMPDIR}"
done

# co-shell-bridge zip
for PLATFORM in "${PLATFORMS[@]}"; do
  GOOS="${PLATFORM%%/*}"
  GOARCH="${PLATFORM##*/}"

  if [ "${GOOS}" = "windows" ]; then
    SRC="co-shell-bridge-${GOOS}-${GOARCH}.exe"
    ZIP_NAME="co-shell-bridge-${VERSION}-${GOOS}-${GOARCH}.zip"
    ZIP_ENTRY="co-shell-bridge.exe"
  else
    SRC="co-shell-bridge-${GOOS}-${GOARCH}"
    ZIP_NAME="co-shell-bridge-${VERSION}-${GOOS}-${GOARCH}.zip"
    ZIP_ENTRY="co-shell-bridge"
  fi

  echo "  ${ZIP_NAME}"

  TMPDIR="$(mktemp -d)"
  cp "${SRC}" "${TMPDIR}/${ZIP_ENTRY}"
  cd "${TMPDIR}"
  zip -q "${ROOT_DIR}/${DIST_DIR}/${ZIP_NAME}" "${ZIP_ENTRY}"
  cd "${ROOT_DIR}/${DIST_DIR}"
  rm -rf "${TMPDIR}"
done

echo "  Packaging: ALL DONE"

# ============================================
# Summary
# ============================================
echo ""
echo "=========================================="
echo " Build Complete!"
echo " Output directory: ${DIST_DIR}/"
echo "=========================================="
echo ""
ls -lh "${ROOT_DIR}/${DIST_DIR}/" | grep -E '\.zip$'
echo ""
