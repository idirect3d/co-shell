#!/usr/bin/env python3
"""image-generate skill 生成脚本：封装 mflux-generate-qwen（本地 MLX 文生图，Apple Silicon）。

用法示例:
    python generate_image.py -p "a cute robot, flat vector" -o out.png -w 1280 -H 720
依赖: ~/.venv-vllm-metal/bin/mflux-generate-qwen（mflux），模型 mlx-community/qwen-image-edit-2511-8bit
"""
import argparse
import os
import shutil
import subprocess
import sys

DEFAULT_MODEL = "mlx-community/qwen-image-edit-2511-8bit"
DEFAULT_NEGATIVE = "低分辨率，低画质，肢体畸形，手指畸形，画面过饱和，蜡像感，人脸无细节，过度光滑，画面具有AI感。构图混乱。文字模糊，扭曲。"


def find_mflux_bin():
    """定位 mflux-generate-qwen 可执行文件。"""
    env_bin = os.environ.get("MFLUX_BIN")
    if env_bin and os.path.isfile(env_bin):
        return env_bin
    candidates = [
        os.path.expanduser("~/.venv-vllm-metal/bin/mflux-generate-qwen"),
        "/opt/homebrew/bin/mflux-generate-qwen",
    ]
    for c in candidates:
        if os.path.isfile(c):
            return c
    which = shutil.which("mflux-generate-qwen")
    if which:
        return which
    sys.exit("[ERROR] 未找到 mflux-generate-qwen。请确认 ~/.venv-vllm-metal 已安装 mflux，"
             "或用环境变量 MFLUX_BIN 指定其路径。")


def main():
    ap = argparse.ArgumentParser(description="本地 AI 文生图（mflux + Qwen-Image-Edit-2511）")
    ap.add_argument("-p", "--prompt", required=True, help="英文提示词（必填；中文效果弱）")
    ap.add_argument("-o", "--output", required=True, help="输出 PNG 路径（必填）")
    ap.add_argument("-w", "--width", type=int, default=1024, help="宽（建议 16 的倍数，默认 1024）")
    ap.add_argument("-H", "--height", type=int, default=1024, help="高（建议 16 的倍数，默认 1024）")
    ap.add_argument("-s", "--steps", type=int, default=20, help="推理步数（默认 20）")
    ap.add_argument("-g", "--guidance", type=float, default=4.5, help="CFG 强度 3.5~6（默认 4.5）")
    ap.add_argument("-r", "--seed", type=int, default=42, help="随机种子（默认 42）")
    ap.add_argument("-np", "--negative-prompt", default=DEFAULT_NEGATIVE, help="负向提示词")
    ap.add_argument("-m", "--model", default=DEFAULT_MODEL, help="模型 repo id 或本地路径")
    ap.add_argument("--low-ram", action="store_true", help="低内存模式（小尺寸/慢）")
    args = ap.parse_args()

    if args.width % 16 != 0 or args.height % 16 != 0:
        print("[WARN] 宽高不是 16 的倍数，模型可能自动修正。")

    out_dir = os.path.dirname(os.path.abspath(args.output))
    os.makedirs(out_dir, exist_ok=True)
    if os.path.exists(args.output):
        os.remove(args.output)  # 避免旧文件残留导致误判

    cmd = [
        find_mflux_bin(),
        "--model", args.model,
        "--prompt", args.prompt,
        "--negative-prompt", args.negative_prompt,
        "--width", str(args.width),
        "--height", str(args.height),
        "--steps", str(args.steps),
        "--guidance", str(args.guidance),
        "--seed", str(args.seed),
        "--output", args.output,
    ]
    if args.low_ram:
        cmd.append("--low-ram")

    env = dict(os.environ)
    env.setdefault("HF_ENDPOINT", "https://hf-mirror.com")  # 模型缓存缺失时走镜像

    print(f"[image-generate] 生成中: {args.width}x{args.height} @ {args.steps} 步 (seed={args.seed})")
    print(f"[image-generate] 模型: {args.model}")
    print(f"[image-generate] 输出: {args.output}")
    sys.stdout.flush()
    try:
        subprocess.run(cmd, env=env, check=True)
    except subprocess.CalledProcessError as e:
        sys.exit(f"[ERROR] 生成失败（退出码 {e.returncode}）。请检查提示词、内存或模型缓存。")

    if not os.path.isfile(args.output):
        sys.exit("[ERROR] 未找到输出文件，生成可能未完成。")
    size = os.path.getsize(args.output)
    print(f"[image-generate] 完成: {args.output} ({size / 1024:.0f} KB)")


if __name__ == "__main__":
    main()
