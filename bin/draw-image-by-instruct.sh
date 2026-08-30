#!/bin/bash
# Qwen-Image-2512 启动脚本
# 适用于 MacBook Pro M5 Max (128GB RAM, 40 核 GPU)

# 设置虚拟环境
VENV_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$VENV_DIR/bin/activate"

# 设置模型路径
MODEL_DIR="${MODEL_DIR:-$VENV_DIR/../models/Qwen-Image-2512}"

# 设置环境变量
export PYTORCH_ENABLE_MPS_FALLBACK=1
export PYTORCH_MPS_HIGH_WATERMARK_RATIO=0.0

# 解析命令行参数
PROMPT=""
NEGATIVE_PROMPT=""
WIDTH=1280
HEIGHT=1280
NUM_STEPS=20
CFG_SCALE=4.0
SEED=42
OUTPUT_DIR="./output"

while [[ $# -gt 0 ]]; do
    case $1 in
        --prompt|-p)
            PROMPT="$2"
            shift 2
            ;;
        --negative-prompt|-np)
            NEGATIVE_PROMPT="$2"
            shift 2
            ;;
        --width|-w)
            WIDTH="$2"
            shift 2
            ;;
        --height|-H)
            HEIGHT="$2"
            shift 2
            ;;
        --steps|-s)
            NUM_STEPS="$2"
            shift 2
            ;;
        --cfg-scale|-c)
            CFG_SCALE="$2"
            shift 2
            ;;
        --seed|-r)
            SEED="$2"
            shift 2
            ;;
        --output-dir|-o)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --model-dir|-m)
            MODEL_DIR="$2"
            shift 2
            ;;
        --help|-h)
            echo "用法: $0 [选项]"
            echo ""
            echo "选项:"
            echo "  --prompt, -p        生成图像的提示词"
            echo "  --negative-prompt, -np  负向提示词"
            echo "  --width, -w         图像宽度 (默认: 1328)"
            echo "  --height, -H        图像高度 (默认: 1328)"
            echo "  --steps, -s         推理步数 (默认: 50)"
            echo "  --cfg-scale, -c     CFG 缩放比例 (默认: 4.0)"
            echo "  --seed, -r          随机种子 (默认: 42)"
            echo "  --output-dir, -o    输出目录 (默认: ./output)"
            echo "  --model-dir, -m     模型目录 (默认: 当前目录/models/Qwen-Image-2512)"
            echo "  --help, -h          显示帮助信息"
            exit 0
            ;;
        *)
            echo "未知选项: $1"
            exit 1
            ;;
    esac
done

# 创建输出目录
mkdir -p "$OUTPUT_DIR"

# 检查模型是否存在
if [ ! -d "$MODEL_DIR" ]; then
    echo "错误: 模型目录不存在: $MODEL_DIR"
    echo "请先下载模型: https://hf-mirror.com/Qwen/Qwen-Image-2512"
    exit 1
fi

# 生成图像
python3 << EOF
import torch
from diffusers import DiffusionPipeline
import os
from datetime import datetime

# 配置
model_dir = "$MODEL_DIR"
prompt = """$PROMPT"""
negative_prompt = """$NEGATIVE_PROMPT""" if "$NEGATIVE_PROMPT" else "低分辨率，低画质，肢体畸形，手指畸形，画面过饱和，蜡像感，人脸无细节，过度光滑，画面具有AI感。构图混乱。文字模糊，扭曲。"
width = $WIDTH
height = $HEIGHT
num_inference_steps = $NUM_STEPS
true_cfg_scale = $CFG_SCALE
seed = $SEED
output_dir = "$OUTPUT_DIR"

# 加载模型
print("正在加载模型...")
pipe = DiffusionPipeline.from_pretrained(
    model_dir,
    torch_dtype=torch.bfloat16,
    use_safetensors=True
)
pipe = pipe.to("mps")

# 生成图像
print("正在生成图像...")
image = pipe(
    prompt=prompt,
    negative_prompt=negative_prompt,
    width=width,
    height=height,
    num_inference_steps=num_inference_steps,
    true_cfg_scale=true_cfg_scale,
    generator=torch.Generator(device="mps").manual_seed(seed)
).images[0]

# 保存图像
timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
output_path = os.path.join(output_dir, f"generated_{timestamp}.png")
image.save(output_path)
print(f"图像已保存到: {output_path}")
EOF
