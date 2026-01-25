#!/usr/bin/env bash
# build/build-images.sh - AIDG 本地镜像构建脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
DOCKERFILES_DIR="${SCRIPT_DIR}/dockerfiles"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印函数
info() { echo -e "${GREEN}[INFO]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; }

# 显示帮助信息
show_help() {
    cat << EOF
AIDG 本地镜像构建脚本

用法:
    ./build-images.sh [OPTIONS] [IMAGES...]

镜像选项:
    main        构建主服务镜像 (aidg:local)
    deps        构建依赖服务镜像 (aidg-deps:local)
    nlp         构建 NLP 服务镜像 (aidg-nlp:local)
    lite        构建轻量镜像 (aidg:local-lite)
    all         构建所有镜像 (默认)

选项:
    -h, --help              显示此帮助信息
    -t, --tag TAG           指定镜像标签 (默认: local)
    --platform PLATFORMS    指定目标平台 (例如: linux/amd64,linux/arm64)
    --no-cache              不使用缓存构建
    --push                  构建后推送到仓库（需要先登录）

示例:
    # 构建所有镜像
    ./build-images.sh all

    # 仅构建主服务镜像
    ./build-images.sh main

    # 构建主服务和依赖服务
    ./build-images.sh main deps

    # 使用自定义标签
    ./build-images.sh --tag dev main

    # 多架构构建
    ./build-images.sh --platform linux/amd64,linux/arm64 main

EOF
}

# 构建主服务镜像
build_main() {
    local tag="${1:-local}"
    info "构建主服务镜像: aidg:${tag}"
    
    docker build \
        -f "${DOCKERFILES_DIR}/Dockerfile" \
        -t "aidg:${tag}" \
        ${DOCKER_BUILD_ARGS} \
        "${PROJECT_ROOT}"
    
    info "✓ 主服务镜像构建完成: aidg:${tag}"
}

# 构建依赖服务镜像
build_deps() {
    local tag="${1:-local}"
    info "构建依赖服务镜像: aidg-deps:${tag}"
    
    docker build \
        -f "${DOCKERFILES_DIR}/Dockerfile.deps" \
        -t "aidg-deps:${tag}" \
        ${DOCKER_BUILD_ARGS} \
        "${PROJECT_ROOT}"
    
    info "✓ 依赖服务镜像构建完成: aidg-deps:${tag}"
}

# 构建 NLP 服务镜像
build_nlp() {
    local tag="${1:-local}"
    info "构建 NLP 服务镜像: aidg-nlp:${tag}"
    
    docker build \
        -f "${DOCKERFILES_DIR}/Dockerfile.nlp" \
        -t "aidg-nlp:${tag}" \
        ${DOCKER_BUILD_ARGS} \
        "${PROJECT_ROOT}/nlp_service"
    
    info "✓ NLP 服务镜像构建完成: aidg-nlp:${tag}"
}

# 构建轻量镜像
build_lite() {
    local tag="${1:-local-lite}"
    info "构建轻量镜像: aidg:${tag}"
    
    docker build \
        -f "${DOCKERFILES_DIR}/Dockerfile.lite" \
        -t "aidg:${tag}" \
        ${DOCKER_BUILD_ARGS} \
        "${PROJECT_ROOT}"
    
    info "✓ 轻量镜像构建完成: aidg:${tag}"
}

# 主函数
main() {
    local tag="local"
    local platform=""
    local images=()
    DOCKER_BUILD_ARGS=""

    # 解析参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -t|--tag)
                tag="$2"
                shift 2
                ;;
            --platform)
                platform="$2"
                DOCKER_BUILD_ARGS="${DOCKER_BUILD_ARGS} --platform ${platform}"
                shift 2
                ;;
            --no-cache)
                DOCKER_BUILD_ARGS="${DOCKER_BUILD_ARGS} --no-cache"
                shift
                ;;
            --push)
                DOCKER_BUILD_ARGS="${DOCKER_BUILD_ARGS} --push"
                shift
                ;;
            main|deps|nlp|lite|all)
                images+=("$1")
                shift
                ;;
            *)
                error "未知选项: $1"
                show_help
                exit 1
                ;;
        esac
    done

    # 默认构建所有镜像
    if [ ${#images[@]} -eq 0 ]; then
        images=("all")
    fi

    # 检查 Docker
    if ! command -v docker &> /dev/null; then
        error "未找到 Docker，请先安装 Docker"
        exit 1
    fi

    info "开始构建镜像，标签: ${tag}"
    [ -n "$platform" ] && info "目标平台: ${platform}"

    # 执行构建
    for image in "${images[@]}"; do
        case $image in
            main)
                build_main "$tag"
                ;;
            deps)
                build_deps "$tag"
                ;;
            nlp)
                build_nlp "$tag"
                ;;
            lite)
                build_lite "$tag"
                ;;
            all)
                build_main "$tag"
                build_deps "$tag"
                build_nlp "$tag"
                ;;
            *)
                warn "未知镜像类型: $image，跳过"
                ;;
        esac
    done

    info "=================="
    info "所有镜像构建完成！"
    info "=================="
    info ""
    info "查看构建的镜像:"
    docker images | grep -E "(aidg|aidg-deps|aidg-nlp)" | grep "$tag" || true
}

main "$@"
