# image-generate — 本地 AI 文生图 Skill

> 把「本地 AI 文生图」能力封装为标准 Agent Skill（与 co-shell 的 skills 机制兼容：
> 目录含 `SKILL.md` + `scripts/`）。基于 **Qwen-Image-Edit-2511**（mflux + MLX，
> Apple Silicon 本地推理），**全程离线、无需 API Key**。

## 目录结构

```
image-generate/
├── SKILL.md                  # Skill 定义（frontmatter: name/description + 使用说明）
├── README.md                 # 本文档
└── scripts/
    └── generate_image.py     # 生成脚本（封装 mflux-generate-qwen）
```

## 环境依赖

| 依赖 | 说明 | 检查方式 |
|------|------|----------|
| macOS（Apple Silicon） | M 系列芯片，MLX 加速 | `uname -m` → arm64 |
| mflux | Python 图像生成库（MLX 版） | `~/.venv-vllm-metal/bin/mflux-generate-qwen --help` |
| 模型 | `mlx-community/qwen-image-edit-2511-8bit`（已缓存） | `ls ~/.cache/huggingface/hub/models--mlx-community--qwen-image-edit-2511-8bit` |

> 安装 mflux：`python3 -m venv ~/.venv-vllm-metal && ~/.venv-vllm-metal/bin/pip install mflux`
> 模型缺失时会自动经 hf-mirror 镜像下载（脚本已设 `HF_ENDPOINT`）。

## 用法

```bash
# 基本用法（必填：提示词 + 输出路径）
python scripts/generate_image.py -p "a cute robot, flat vector illustration, white background" -o out.png

# 指定尺寸 / 步数 / 种子（可复现）
python scripts/generate_image.py -p "..." -o out.png -w 1280 -H 720 -s 24 -r 42

# 低内存模式
python scripts/generate_image.py -p "..." -o out.png --low-ram
```

### 参数一览

| 参数 | 说明 | 默认 |
|------|------|------|
| `-p, --prompt` | 英文提示词（必填） | - |
| `-o, --output` | 输出 PNG 路径（必填） | - |
| `-w, --width` | 宽（建议 16 的倍数） | 1024 |
| `-H, --height` | 高（建议 16 的倍数） | 1024 |
| `-s, --steps` | 推理步数（高=精细慢） | 20 |
| `-g, --guidance` | CFG 强度 3.5~6 | 4.5 |
| `-r, --seed` | 随机种子 | 42 |
| `-np, --negative-prompt` | 负向提示词 | 内置默认 |
| `-m, --model` | 模型 repo id / 路径 | `mlx-community/qwen-image-edit-2511-8bit` |
| `--low-ram` | 低内存模式 | 关 |

## 提示词技巧（重要）

1. **用英文**：模型对英文理解最好；中文提示词也可用但效果打折
2. **结构**：主体 → 细节 → 风格 → 背景 → 画质词
   `a cute robot wearing a seashell helmet, big happy eyes, flat vector illustration, soft pastel colors, clean white background, high quality`
3. **画面内文字**：只放少量英文短标签（如 `LLM Core`、`Web UI`）；中文/长句易乱码
4. **需要精确文字排版图**（架构图/海报带大量文字）：改用 HTML/SVG 绘图，不要用本技能

## 性能参考（M5 Max 实测）

- 1280×720 @ 24 步：约 3 分钟，峰值内存 ~37 GB
- 1024×1024 @ 20 步：约 2 分钟，峰值内存 ~37 GB
- 内存 < 32 GB 建议 `--low-ram` 或 768×768 以下

## 常见问题

| 问题 | 处理 |
|------|------|
| `command not found: mflux-generate-qwen` | 安装 mflux 到 `~/.venv-vllm-metal`，或用 `MFLUX_BIN` 指定路径 |
| 模型下载慢 | 已走 hf-mirror 镜像；也可提前 `huggingface-cli download mlx-community/qwen-image-edit-2511-8bit` |
| 文字乱码 | 模型文字渲染局限 → 用英文短标签 / 改 HTML 绘图 |
| 内存不足 | `--low-ram` 或降低宽高/步数 |
| 效果不理想 | 换 `-r` 种子重跑；加强风格词（flat vector / cartoon / realistic...） |

## 验证记录

2026-08-30 已用本链路成功生成 `research/co-shell功能总览/co-shell架构图_卡通.png`
（1280×720，24 步，Qwen-Image-Edit-2511 8bit，MLX 峰值 37.4GB）。
