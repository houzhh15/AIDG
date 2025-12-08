#!/usr/bin/env bash
# Setup Local Dependencies Script
# 根据 .env 配置安装本地依赖：Whisper 和 PyAnnote
# Usage: ./scripts/setup-local-dependencies.sh

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

log_info "Project root: ${PROJECT_ROOT}"

# 加载 .env 文件
if [ -f "${PROJECT_ROOT}/.env" ]; then
    log_info "Loading environment variables from .env"
    export $(grep -v '^#' "${PROJECT_ROOT}/.env" | xargs)
else
    log_warn ".env file not found, using default values"
fi

# 设置默认值
WHISPER_PROGRAM_PATH=${WHISPER_PROGRAM_PATH:-"./bin/whisper/whisper"}
DIARIZATION_SCRIPT_PATH=${DIARIZATION_SCRIPT_PATH:-"./tmp/pyannote/pyannote_diarize.py"}
EMBEDDING_SCRIPT_PATH=${EMBEDDING_SCRIPT_PATH:-"./tmp/pyannote/generate_speaker_embeddings.py"}
HUGGINGFACE_TOKEN=${HUGGINGFACE_TOKEN:-""}

# 转换为绝对路径
WHISPER_DIR="$(dirname "${PROJECT_ROOT}/${WHISPER_PROGRAM_PATH}")"
PYANNOTE_DIR="$(dirname "${PROJECT_ROOT}/${DIARIZATION_SCRIPT_PATH}")"

log_info "Whisper directory: ${WHISPER_DIR}"
log_info "PyAnnote directory: ${PYANNOTE_DIR}"

# ============================================
# 1. 安装 Whisper (go-whisper)
# ============================================
install_whisper() {
    log_info "=========================================="
    log_info "Installing Whisper (go-whisper)"
    log_info "=========================================="

    # 检查是否已安装
    if [ -f "${PROJECT_ROOT}/${WHISPER_PROGRAM_PATH}" ]; then
        log_info "Whisper already installed at ${WHISPER_PROGRAM_PATH}"
        "${PROJECT_ROOT}/${WHISPER_PROGRAM_PATH}" --version 2>/dev/null || true
        read -p "Reinstall? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            return 0
        fi
    fi

    # 创建目录
    mkdir -p "${WHISPER_DIR}"

    # 检查 Go 是否安装
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed. Please install Go first: https://go.dev/doc/install"
        exit 1
    fi

    log_info "Go version: $(go version)"

    # 克隆 go-whisper 仓库
    WHISPER_REPO_DIR="${PROJECT_ROOT}/tmp/go-whisper"
    if [ -d "${WHISPER_REPO_DIR}" ]; then
        log_info "Updating go-whisper repository"
        cd "${WHISPER_REPO_DIR}"
        git pull
    else
        log_info "Cloning go-whisper repository"
        git clone https://github.com/ggerganov/whisper.cpp.git "${WHISPER_REPO_DIR}"
        cd "${WHISPER_REPO_DIR}"
    fi

    # 编译 whisper.cpp
    log_info "Building whisper.cpp..."
    make clean || true
    make

    # 复制可执行文件
    if [ -f "${WHISPER_REPO_DIR}/main" ]; then
        log_info "Installing whisper binary to ${PROJECT_ROOT}/${WHISPER_PROGRAM_PATH}"
        cp "${WHISPER_REPO_DIR}/main" "${PROJECT_ROOT}/${WHISPER_PROGRAM_PATH}"
        chmod +x "${PROJECT_ROOT}/${WHISPER_PROGRAM_PATH}"
        log_info "Whisper installed successfully"
    else
        log_error "Failed to build whisper"
        exit 1
    fi

    # 下载模型（可选）
    log_info "Downloading Whisper models..."
    MODELS_DIR="${PROJECT_ROOT}/models/whisper"
    mkdir -p "${MODELS_DIR}"
    
    cd "${WHISPER_REPO_DIR}"
    if [ ! -f "${MODELS_DIR}/ggml-large-v3.bin" ]; then
        log_info "Downloading large-v3 model..."
        bash ./models/download-ggml-model.sh large-v3
        mv models/ggml-large-v3.bin "${MODELS_DIR}/"
    else
        log_info "Model already exists: ggml-large-v3.bin"
    fi

    cd "${PROJECT_ROOT}"
    log_info "Whisper installation complete"
}

# ============================================
# 2. 安装 PyAnnote (使用 conda)
# ============================================
install_pyannote() {
    log_info "=========================================="
    log_info "Installing PyAnnote with Conda"
    log_info "=========================================="

    # 检查 conda 是否安装
    if ! command -v conda &> /dev/null; then
        log_error "Conda is not installed. Please install Miniconda or Anaconda first:"
        log_error "https://docs.conda.io/en/latest/miniconda.html"
        exit 1
    fi

    log_info "Conda version: $(conda --version)"

    # 创建 PyAnnote 目录
    mkdir -p "${PYANNOTE_DIR}"

    # 检查 conda 环境是否存在
    CONDA_ENV_NAME="aidg-pyannote"
    if conda env list | grep -q "^${CONDA_ENV_NAME} "; then
        log_info "Conda environment '${CONDA_ENV_NAME}' already exists"
        read -p "Recreate environment? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Removing existing environment..."
            conda env remove -n "${CONDA_ENV_NAME}" -y
        else
            log_info "Using existing environment"
            return 0
        fi
    fi

    # 创建 conda 环境
    log_info "Creating conda environment: ${CONDA_ENV_NAME}"
    conda create -n "${CONDA_ENV_NAME}" python=3.10 -y

    # 激活环境并安装依赖
    log_info "Installing PyAnnote and dependencies..."
    
    # 使用 conda run 而不是 source activate（更可靠）
    conda run -n "${CONDA_ENV_NAME}" pip install --upgrade pip
    conda run -n "${CONDA_ENV_NAME}" pip install torch torchaudio --index-url https://download.pytorch.org/whl/cpu
    conda run -n "${CONDA_ENV_NAME}" pip install pyannote.audio
    conda run -n "${CONDA_ENV_NAME}" pip install soundfile

    # 验证 HuggingFace Token
    if [ -z "${HUGGINGFACE_TOKEN}" ]; then
        log_warn "HUGGINGFACE_TOKEN not set in .env"
        log_warn "PyAnnote requires authentication with HuggingFace"
        log_warn "Please set HUGGINGFACE_TOKEN in .env or accept the model license manually"
    else
        log_info "HuggingFace token found"
        # 登录到 HuggingFace
        conda run -n "${CONDA_ENV_NAME}" pip install huggingface-hub
        conda run -n "${CONDA_ENV_NAME}" python -c "from huggingface_hub import login; login(token='${HUGGINGFACE_TOKEN}')"
    fi

    # 创建 PyAnnote 脚本
    log_info "Creating PyAnnote diarization script..."
    cat > "${PROJECT_ROOT}/${DIARIZATION_SCRIPT_PATH}" << 'EOF'
#!/usr/bin/env python3
"""
PyAnnote Speaker Diarization Script
Usage: python pyannote_diarize.py <audio_file> <output_file> [min_speakers] [max_speakers]
"""
import sys
import os
from pyannote.audio import Pipeline

def main():
    if len(sys.argv) < 3:
        print("Usage: python pyannote_diarize.py <audio_file> <output_file> [min_speakers] [max_speakers]")
        sys.exit(1)
    
    audio_file = sys.argv[1]
    output_file = sys.argv[2]
    min_speakers = int(sys.argv[3]) if len(sys.argv) > 3 else None
    max_speakers = int(sys.argv[4]) if len(sys.argv) > 4 else None
    
    # 从环境变量获取 HuggingFace token
    hf_token = os.getenv("HUGGINGFACE_TOKEN")
    if not hf_token:
        print("ERROR: HUGGINGFACE_TOKEN not set", file=sys.stderr)
        sys.exit(1)
    
    # 加载模型
    pipeline = Pipeline.from_pretrained(
        "pyannote/speaker-diarization-3.1",
        use_auth_token=hf_token
    )
    
    # 执行说话人识别
    kwargs = {}
    if min_speakers:
        kwargs["min_speakers"] = min_speakers
    if max_speakers:
        kwargs["max_speakers"] = max_speakers
    
    diarization = pipeline(audio_file, **kwargs)
    
    # 保存结果
    with open(output_file, "w") as f:
        for turn, _, speaker in diarization.itertracks(yield_label=True):
            f.write(f"{turn.start:.3f} {turn.end:.3f} {speaker}\n")
    
    print(f"Diarization completed: {output_file}")

if __name__ == "__main__":
    main()
EOF

    chmod +x "${PROJECT_ROOT}/${DIARIZATION_SCRIPT_PATH}"

    # 创建说话人嵌入生成脚本
    log_info "Creating speaker embedding script..."
    cat > "${PROJECT_ROOT}/${EMBEDDING_SCRIPT_PATH}" << 'EOF'
#!/usr/bin/env python3
"""
Speaker Embedding Generation Script
Usage: python generate_speaker_embeddings.py <audio_file> <output_file>
"""
import sys
import os
import torch
from pyannote.audio import Model, Inference

def main():
    if len(sys.argv) < 3:
        print("Usage: python generate_speaker_embeddings.py <audio_file> <output_file>")
        sys.exit(1)
    
    audio_file = sys.argv[1]
    output_file = sys.argv[2]
    
    # 从环境变量获取 HuggingFace token
    hf_token = os.getenv("HUGGINGFACE_TOKEN")
    if not hf_token:
        print("ERROR: HUGGINGFACE_TOKEN not set", file=sys.stderr)
        sys.exit(1)
    
    # 加载模型
    model = Model.from_pretrained(
        "pyannote/embedding",
        use_auth_token=hf_token
    )
    
    inference = Inference(model, window="whole")
    embedding = inference(audio_file)
    
    # 保存嵌入
    torch.save(embedding, output_file)
    print(f"Embedding saved: {output_file}")

if __name__ == "__main__":
    main()
EOF

    chmod +x "${PROJECT_ROOT}/${EMBEDDING_SCRIPT_PATH}"

    log_info "PyAnnote installation complete"
    log_info "To use PyAnnote, activate the environment:"
    log_info "  conda activate ${CONDA_ENV_NAME}"
    log_info "Or use: conda run -n ${CONDA_ENV_NAME} python ${DIARIZATION_SCRIPT_PATH}"
}

# ============================================
# 3. 验证安装
# ============================================
verify_installation() {
    log_info "=========================================="
    log_info "Verifying Installation"
    log_info "=========================================="

    local all_ok=true

    # 检查 Whisper
    if [ -f "${PROJECT_ROOT}/${WHISPER_PROGRAM_PATH}" ]; then
        log_info "✓ Whisper binary found: ${WHISPER_PROGRAM_PATH}"
        "${PROJECT_ROOT}/${WHISPER_PROGRAM_PATH}" --version 2>/dev/null || true
    else
        log_error "✗ Whisper binary not found"
        all_ok=false
    fi

    # 检查 PyAnnote 脚本
    if [ -f "${PROJECT_ROOT}/${DIARIZATION_SCRIPT_PATH}" ]; then
        log_info "✓ PyAnnote diarization script found: ${DIARIZATION_SCRIPT_PATH}"
    else
        log_error "✗ PyAnnote diarization script not found"
        all_ok=false
    fi

    if [ -f "${PROJECT_ROOT}/${EMBEDDING_SCRIPT_PATH}" ]; then
        log_info "✓ Speaker embedding script found: ${EMBEDDING_SCRIPT_PATH}"
    else
        log_error "✗ Speaker embedding script not found"
        all_ok=false
    fi

    # 检查 conda 环境
    if conda env list | grep -q "^aidg-pyannote "; then
        log_info "✓ Conda environment 'aidg-pyannote' exists"
    else
        log_error "✗ Conda environment 'aidg-pyannote' not found"
        all_ok=false
    fi

    if [ "$all_ok" = true ]; then
        log_info "=========================================="
        log_info "Installation verification passed!"
        log_info "=========================================="
    else
        log_error "Some components are missing. Please check the installation."
        exit 1
    fi
}

# ============================================
# 主函数
# ============================================
main() {
    log_info "Starting local dependencies setup..."
    log_info "This script will install:"
    log_info "  1. Whisper (go-whisper / whisper.cpp)"
    log_info "  2. PyAnnote (with conda environment)"
    echo

    # 安装 Whisper
    install_whisper

    echo
    # 安装 PyAnnote
    install_pyannote

    echo
    # 验证安装
    verify_installation

    log_info "=========================================="
    log_info "Setup complete!"
    log_info "=========================================="
    log_info "Next steps:"
    log_info "  1. Ensure .env has correct paths:"
    log_info "     WHISPER_PROGRAM_PATH=${WHISPER_PROGRAM_PATH}"
    log_info "     DIARIZATION_SCRIPT_PATH=${DIARIZATION_SCRIPT_PATH}"
    log_info "  2. Run 'make dev' to start the development server"
    log_info "  3. PyAnnote will use conda environment 'aidg-pyannote'"
}

main "$@"
