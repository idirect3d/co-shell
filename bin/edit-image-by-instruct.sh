#!/bin/bash
# ============================================================
# Qwen-Image-Edit-2511 (MLX 8bit) 图片编辑脚本
# 基于 mflux 的 Qwen Image Edit，支持搭配 LoRA 适配器
#
# 用法:
#   ./edit-image-by-instruct.sh -i input.png -p "编辑指令" [选项]
#
# 示例:
#   ./edit-image-by-instruct.sh -i input.png -p "remove the object"
#   ./edit-image-by-instruct.sh -i input.png -p "change camera angle" \
#       --lora-paths "fal/Qwen-Image-Edit-2511-Multiple-Angles-LoRA" --lora-scales 1.0
#   ./edit-image-by-instruct.sh -i input.png -p "style transfer" \
#       --lora-paths "lora1" "lora2" --lora-scales 0.5 1.0
# ============================================================

set -u

# ============ 默认配置 ============
MODEL="mlx-community/qwen-image-edit-2511-8bit"   # 默认模型（8bit 量化）
QUANTIZE=""                                        # 量化位数（模型已量化则留空）
OUTPUT_DIR="."                   # 默认输出目录
PYTHON="$HOME/.venv-vllm-metal/bin/python3"        # Python 环境
export HF_ENDPOINT=https://hf-mirror.com           # 镜像站

# ============ 帮助信息 ============
usage() {
    cat <<EOF
用法: $0 -i <输入图片> -p <编辑指令> [选项]

必选参数:
  -i, --image <路径>        输入图片路径（可多个，用空格分隔）
  -p, --prompt <文本>       编辑指令

可选参数:
  -m, --model <名称/路径>   模型（默认: $MODEL）
  -q, --quantize <位数>     量化位数（如 8/6/4，模型已量化则无需指定）
      --lora-paths <路径>   LoRA 适配器路径（可多个）
      --lora-scales <数值>  LoRA 权重比例（与 lora-paths 一一对应）
  -o, --output <路径>       输出文件路径（默认: $OUTPUT_DIR/edit_时间戳.png）
  -w, --width <像素>        输出宽度
  -h, --height <像素>       输出高度
  -s, --steps <步数>        推理步数（默认 10）
  -g, --guidance <数值>     CFG 强度
  -n, --negative <文本>     负面提示词
      --seed <数值>         随机种子
      --metadata            导出元数据 JSON
      --show-intermediate   保存每一步迭代的中间过程图片到输出目录

示例:
  $0 -i a.png -p "remove the object"
  $0 -i a.png -p "change angle" --lora-paths "fal/...-LoRA" --lora-scales 1.0
  $0 -i a.png b.png -p "combine" --lora-paths "l1" "l2" --lora-scales 0.5 1.0
EOF
    exit 0
}

# ============ 参数解析 ============
IMAGE_PATHS=()
PROMPT=""
LORA_PATHS=()
LORA_SCALES=()
OUTPUT=""
WIDTH=""
HEIGHT=""
STEPS=""
GUIDANCE=""
NEGATIVE=""
SEED=""
METADATA=""
SHOW_INTERMEDIATE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -i|--image) shift; while [[ $# -gt 0 && "$1" != -* ]]; do IMAGE_PATHS+=("$1"); shift; done ;;
        -p|--prompt) PROMPT="$2"; shift 2 ;;
        -m|--model) MODEL="$2"; shift 2 ;;
        -q|--quantize) QUANTIZE="$2"; shift 2 ;;
        --lora-paths) shift; while [[ $# -gt 0 && "$1" != -* ]]; do LORA_PATHS+=("$1"); shift; done ;;
        --lora-scales) shift; while [[ $# -gt 0 && "$1" != -* ]]; do LORA_SCALES+=("$1"); shift; done ;;
        -o|--output) OUTPUT="$2"; shift 2 ;;
        -w|--width) WIDTH="$2"; shift 2 ;;
        -h|--height) HEIGHT="$2"; shift 2 ;;
        -s|--steps) STEPS="$2"; shift 2 ;;
        -g|--guidance) GUIDANCE="$2"; shift 2 ;;
        -n|--negative) NEGATIVE="$2"; shift 2 ;;
        --seed) SEED="$2"; shift 2 ;;
        --metadata) METADATA="--metadata"; shift ;;
        --show-intermediate) SHOW_INTERMEDIATE="1"; shift ;;
        --help|-help) usage ;;
        *) echo "未知参数: $1"; usage ;;
    esac
done

# ============ 校验 ============
if [[ ${#IMAGE_PATHS[@]} -eq 0 ]]; then
    echo "错误: 必须指定输入图片 (-i)"; usage
fi
if [[ -z "$PROMPT" ]]; then
    echo "错误: 必须指定编辑指令 (-p)"; usage
fi
if [[ ${#LORA_PATHS[@]} -gt 0 && ${#LORA_SCALES[@]} -gt 0 && ${#LORA_PATHS[@]} -ne ${#LORA_SCALES[@]} ]]; then
    echo "错误: --lora-paths 和 --lora-scales 数量必须一致"; exit 1
fi

# ============ 默认输出路径 ============
if [[ -z "$OUTPUT" ]]; then
    mkdir -p "$OUTPUT_DIR"
    OUTPUT="$OUTPUT_DIR/edit_$(date +%Y%m%d_%H%M%S).png"
fi

# ============ 构建命令 ============
CMD=("$PYTHON" -m mflux.models.qwen.cli.qwen_image_edit_generate
     --model "$MODEL"
     --image-paths "${IMAGE_PATHS[@]}"
     --prompt "$PROMPT"
     --output "$OUTPUT")

[ -n "$QUANTIZE" ] && CMD+=(--quantize "$QUANTIZE")
[ -n "$STEPS" ]    && CMD+=(--steps "$STEPS")
[ -n "$GUIDANCE" ] && CMD+=(--guidance "$GUIDANCE")
[ -n "$WIDTH" ]    && CMD+=(--width "$WIDTH")
[ -n "$HEIGHT" ]   && CMD+=(--height "$HEIGHT")
[ -n "$SEED" ]     && CMD+=(--seed "$SEED")
[ -n "$NEGATIVE" ] && CMD+=(--negative-prompt "$NEGATIVE")
[ -n "$METADATA" ] && CMD+=("$METADATA")

# 中间过程可视化：保存每一步迭代的图片
if [[ -n "$SHOW_INTERMEDIATE" ]]; then
    mkdir -p "$OUTPUT_DIR"
    CMD+=(--stepwise-image-output-dir "$OUTPUT_DIR")
fi

# LoRA 参数
if [[ ${#LORA_PATHS[@]} -gt 0 ]]; then
    CMD+=(--lora-paths "${LORA_PATHS[@]}")
    if [[ ${#LORA_SCALES[@]} -gt 0 ]]; then
        CMD+=(--lora-scales "${LORA_SCALES[@]}")
    fi
fi

# ============ 执行 ============
echo "模型: $MODEL"
echo "输入: ${IMAGE_PATHS[*]}"
echo "LoRA: ${LORA_PATHS[*]:-无}"
echo "输出: $OUTPUT"
echo "执行命令: ${CMD[*]}"
echo "----------------------------------------"

"${CMD[@]}"

if [[ $? -eq 0 ]]; then
    echo "----------------------------------------"
    echo "✅ 编辑完成: $OUTPUT"
else
    echo "❌ 编辑失败，请查看上方错误信息"
    exit 1
fi
