#!/usr/bin/env bash
# 本地编译 new-api Linux 二进制，并通过 SSH 上传到远端容器进行替换部署
# Local-first deploy: build on this machine, upload to remote host, replace /new-api inside container, restart and verify
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
VERSION="$(cat "${ROOT_DIR}/VERSION")"

REMOTE_HOST="${NEWAPI_REMOTE_HOST:-002}"
REMOTE_APP_DIR="${NEWAPI_REMOTE_APP_DIR:-/opt/new-api}"
REMOTE_CONTAINER="${NEWAPI_REMOTE_CONTAINER:-new-api}"
REMOTE_PORT="${NEWAPI_REMOTE_PORT:-3000}"

SSH_OPTS=(
  -o BatchMode=yes
  -o ConnectTimeout=10
)

build_binary() {
  local goarch="$1"
  local output="${DIST_DIR}/new-api-linux-${goarch}"

  echo "==> 本地编译 ${goarch}"
  GOOS=linux GOARCH="${goarch}" CGO_ENABLED=0 GOEXPERIMENT=greenteagc \
    go build \
    -ldflags "-s -w -X github.com/QuantumNous/new-api/common.Version=${VERSION}" \
    -o "${output}" \
    "${ROOT_DIR}"

  echo "✓ 已生成: ${output}"
}

remote_exec() {
  ssh "${SSH_OPTS[@]}" "${REMOTE_HOST}" "$@"
}

choose_arch() {
  case "$1" in
    x86_64|amd64)
      echo "amd64"
      ;;
    aarch64|arm64)
      echo "arm64"
      ;;
    *)
      echo "unsupported:$1"
      ;;
  esac
}

mkdir -p "${DIST_DIR}"

echo "==> 检查远端架构"
REMOTE_UNAME="$(remote_exec 'uname -m')"
REMOTE_ARCH="$(choose_arch "${REMOTE_UNAME}")"
if [[ "${REMOTE_ARCH}" == unsupported:* ]]; then
  echo "✗ 不支持的远端架构: ${REMOTE_UNAME}"
  exit 1
fi
echo "✓ 远端架构: ${REMOTE_UNAME} -> ${REMOTE_ARCH}"

LOCAL_BINARY="${DIST_DIR}/new-api-linux-${REMOTE_ARCH}"
if [[ ! -f "${LOCAL_BINARY}" || "${NEWAPI_FORCE_REBUILD:-0}" == "1" ]]; then
  build_binary "${REMOTE_ARCH}"
else
  echo "==> 复用本地已有产物: ${LOCAL_BINARY}"
fi

TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
REMOTE_UPLOAD="${REMOTE_APP_DIR}/bin-backups/new-api-${TIMESTAMP}-${REMOTE_ARCH}"
REMOTE_BACKUP="${REMOTE_APP_DIR}/bin-backups/new-api-container-${TIMESTAMP}"

echo "==> 远端准备目录"
remote_exec "mkdir -p '${REMOTE_APP_DIR}/bin-backups'"

echo "==> 上传二进制"
scp "${SSH_OPTS[@]}" "${LOCAL_BINARY}" "${REMOTE_HOST}:${REMOTE_UPLOAD}"

echo "==> 备份容器内当前 /new-api"
remote_exec "docker cp '${REMOTE_CONTAINER}:/new-api' '${REMOTE_BACKUP}'"

echo "==> 替换容器内二进制"
remote_exec "docker cp '${REMOTE_UPLOAD}' '${REMOTE_CONTAINER}:/new-api' && docker exec '${REMOTE_CONTAINER}' chmod +x /new-api"

echo "==> 重启容器"
remote_exec "docker restart '${REMOTE_CONTAINER}' >/dev/null"

echo "==> 健康检查"
if remote_exec "for i in 1 2 3 4 5 6; do curl -fsS 'http://127.0.0.1:${REMOTE_PORT}/api/status' && exit 0; sleep 2; done; exit 1"; then
  echo
  echo "✅ 部署成功"
  echo "   主机: ${REMOTE_HOST}"
  echo "   架构: ${REMOTE_ARCH}"
  echo "   上传: ${REMOTE_UPLOAD}"
  echo "   备份: ${REMOTE_BACKUP}"
  exit 0
fi

echo "✗ 健康检查失败，开始回滚"
remote_exec "docker cp '${REMOTE_BACKUP}' '${REMOTE_CONTAINER}:/new-api' && docker exec '${REMOTE_CONTAINER}' chmod +x /new-api && docker restart '${REMOTE_CONTAINER}' >/dev/null"
exit 1
